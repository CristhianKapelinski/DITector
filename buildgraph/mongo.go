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
var mongoImagesCollection *mongo.Collection

// InsertRepositoryToMongo 利用Insert将Repository作为文档存储到Mongo中
func InsertRepositoryToMongo(repo *Repository) {
	repo.Tags = map[string]Tag{}
	_, err := mongoRepositoryCollection.InsertOne(context.Background(), repo)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			//fmt.Println("[WARN] Mongo Duplicate when inserting repository", repo.Namespace, repo.Repository, ", repository already exists")
			return
		}
		logBuilderString("[ERROR] Mongo Insert repository " + repo.Namespace + repo.Repository + " to collection repository failed with: " + err.Error())
		fmt.Println("[ERROR] Mongo Insert repository "+repo.Namespace+repo.Repository+" to collection repository failed with: ", err)
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
		Images:              map[string]map[string]string{},
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
		logBuilderString("[ERROR] Mongo Update tag " + tag.Namespace + "/" + tag.Repository + ":" + tag.Tag + " failed with: " + err.Error())
		fmt.Println("[ERROR] Mongo Update tag", tag.Namespace+"/"+tag.Repository+":"+tag.Tag, "failed with:", err)
		return
	}
	//fmt.Println("[INFO] Insert tag", tag.Namespace+"/"+tag.Repository+":"+tag.Tag, "succeed with ID", ret.UpsertedID)
}

// InsertImageToMongo 将image存储到Mongo中
func InsertImageToMongo(image *ImageSource) {
	// 为tag添加不同架构下的镜像digest
	AddImageToRepositoryMongo(image)
	// 将特定镜像的元数据单独存放到images集合
	InsertImageToImagesCollectionMongo(image)
}

// AddImageToRepositoryMongo 利用update $set，将image的digest添加到<namespace>/<repository>.tags.<tag>.images.<arch>.<variant>
func AddImageToRepositoryMongo(image *ImageSource) {
	// Mongo文档的键中不能包含"."，所以将image.Tag中的"."替换为"$"
	tagKey := strings.Replace(image.Tag, ".", "$", -1)
	filter := bson.M{
		"namespace":  image.Namespace,
		"repository": image.Repository,
	}
	arch := image.Image.Architecture
	variant := image.Image.Variant
	// Mongo文档字典类型的键不能为空，将variant为""的修改为"null"
	if variant == "" {
		variant = "null"
	}
	update := bson.M{
		"$set": bson.M{"tags." + tagKey + ".images." + arch + "." + variant: image.Image.Digest},
	}
	_, err := mongoRepositoryCollection.UpdateOne(context.TODO(), filter, update)
	if err != nil {
		logBuilderString("[ERROR] Mongo Update image " + image.Namespace + "/" + image.Repository + ":" + image.Tag + " failed with: " + err.Error())
		fmt.Println("[ERROR] Mongo Update image", image.Namespace+"/"+image.Repository+":"+image.Tag, "failed with:", err)
		return
	}
	//fmt.Println("[INFO] Insert image", image.Namespace+"/"+image.Repository+":"+image.Tag, "succeed with ID", ret.UpsertedID)
}

// InsertImageToImagesCollectionMongo 将image元数据作为文档插入到images collection中
func InsertImageToImagesCollectionMongo(image *ImageSource) {
	i := image.Image
	_, err := mongoImagesCollection.InsertOne(context.Background(), i)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			//fmt.Println("[WARN] Mongo Duplicate when inserting image", i.Digest, ", image already exists")
			return
		}
		logBuilderString("[ERROR] Mongo Insert image " + i.Digest + " to collection images failed with: " + err.Error())
		fmt.Println("[ERROR] Mongo Insert image", i.Digest, "to collection images failed with:", err)
		return
	}
	//fmt.Println("[INFO] Insert image", i.Digest, "succeed with ID", ret.UpsertedID)
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
	mongoRepositoryCollection.Drop(context.TODO())
	mongoImagesCollection.Drop(context.TODO())
	return nil
}
