package buildgraph

import (
	"context"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"strings"
)

// mongo.go 用于操作mongodb

var mongoClient *mongo.Client
var mongoRepositoryCollection *mongo.Collection

// InsertRepositoryToMongo 利用Insert将Repository作为文档存储到Mongo中
func InsertRepositoryToMongo(repo *Repository) {
	repo.Tags = map[string]Tag{}
	_, err := mongoRepositoryCollection.InsertOne(context.Background(), repo)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			fmt.Println("[WARN] Duplicate when inserting repository", repo.Namespace, repo.Repository, ", repository already exists")
			return
		}
		logBuilderString("[ERROR] Insert repository" + repo.Namespace + repo.Repository + "failed with: " + err.Error())
		fmt.Println("[ERROR] Insert repository"+repo.Namespace+repo.Repository+"failed with: ", err)
		return
	}
	//fmt.Println("[INFO] Insert repository", repo.Namespace+"/"+repo.Repository, "succeed with ID", ret.InsertedID)
}

// InsertTagToMongo 利用Update将TagSource添加到Mongo中存储的对应的repository的tags中
func InsertTagToMongo(tag *TagSource) {
	var t = Tag{
		LastUpdated:         tag.LastUpdated,
		LastUpdaterUsername: tag.LastUpdaterUsername,
		TagLastPulled:       tag.TagLastPulled,
		TagLastPushed:       tag.TagLastPushed,
		MediaType:           tag.MediaType,
		ContentType:         tag.ContentType,
		Images:              []Image{},
	}
	filter := bson.M{
		"namespace":  tag.Namespace,
		"repository": tag.Repository,
	}
	// Mongo文档的键中不能包含"."，所以将tag.Tag中的"."替换为"$"
	tagKey := strings.Replace(tag.Tag, ".", "$", -1)
	update := bson.M{
		"$set": bson.M{"tags." + tagKey: t},
	}
	_, err := mongoRepositoryCollection.UpdateOne(context.TODO(), filter, update)
	if err != nil {
		fmt.Println("[ERROR] Update tag", tag.Namespace+"/"+tag.Repository+":"+tag.Tag, "failed with:", err)
		return
	}
	//fmt.Println("[INFO] Insert tag", tag.Namespace+"/"+tag.Repository+":"+tag.Tag, "succeed with ID", ret.UpsertedID)
}

// InsertImageToMongo 利用Update将Image添加到Mongo中存储的对应的repository对应的tag中
func InsertImageToMongo(image *ImageSource) {
	var i = image.Image
	// Mongo文档的键中不能包含"."，所以将image.Tag中的"."替换为"$"
	tagKey := strings.Replace(image.Tag, ".", "$", -1)
	filter := bson.M{
		"namespace":  image.Namespace,
		"repository": image.Repository,
	}
	update := bson.M{
		"$push": bson.M{"tags." + tagKey + ".images": i},
	}
	_, err := mongoRepositoryCollection.UpdateOne(context.TODO(), filter, update)
	if err != nil {
		fmt.Println("[ERROR] Update image", image.Namespace+"/"+image.Repository+":"+image.Tag, "failed with:", err)
		return
	}
	//fmt.Println("[INFO] Insert image", image.Namespace+"/"+image.Repository+":"+image.Tag, "succeed with ID", ret.UpsertedID)
}

// FindRepositoryFromMongoByName 根据Namespace、Repository寻找MongoDB中存储的Repository
func FindRepositoryFromMongoByName(namespace, repository string) (*Repository, error) {
	var repo = new(Repository)

	// 传入条件
	filter := bson.M{}
	if namespace != "" {
		filter["namespace"] = namespace
	}
	if repository != "" {
		filter["repository"] = repository
	}

	// 查询并返回结果
	err := mongoRepositoryCollection.FindOne(context.Background(), filter).Decode(repo)
	if err != nil {
		return &Repository{}, err
	}
	return repo, err
}

// CountDocumentsFromMongo 统计已经存入的文档数量（repository数量）
func CountDocumentsFromMongo() int {
	filter := bson.M{}
	cursor, _ := mongoRepositoryCollection.Find(context.TODO(), filter)
	defer cursor.Close(context.TODO())

	var docs []RepositorySource
	cursor.All(context.TODO(), &docs)
	return len(docs)
}

// DropRepositoryCollectionFromMongo 将repository collection从mongo删除
func DropRepositoryCollectionFromMongo() error {
	return mongoRepositoryCollection.Drop(context.TODO())
}
