package myutils

import (
	"context"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"time"
)

type MyMongo struct {
	Client          *mongo.Client
	RepoColl        *mongo.Collection
	TagColl         *mongo.Collection
	ImgColl         *mongo.Collection
	ImgResultColl   *mongo.Collection
	LayerResultColl *mongo.Collection
}

func NewMongoGlobalConfig() (*MyMongo, error) {
	return NewMongo(GlobalConfig.MongoConfig.URI, GlobalConfig.MongoConfig.Database,
		GlobalConfig.MongoConfig.Collections.Repositories, GlobalConfig.MongoConfig.Collections.Tags,
		GlobalConfig.MongoConfig.Collections.Images, GlobalConfig.MongoConfig.Collections.ImageResults,
		GlobalConfig.MongoConfig.Collections.LayerResults, false)
}

// NewMongo returns a new mongo client
func NewMongo(uri, database, repositories, tags, images, imgResults, layerResults string, initFlag bool) (*MyMongo, error) {
	var mymongo = new(MyMongo)
	var err error

	// 设置超时时间
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	mongoOptions := options.Client().ApplyURI(uri)
	mymongo.Client, err = mongo.Connect(ctx, mongoOptions)
	if err != nil {
		return mymongo, err
	}

	err = mymongo.Client.Ping(context.TODO(), nil)
	if err != nil {
		return mymongo, err
	}

	dockerhubDB := mymongo.Client.Database(database)
	mymongo.RepoColl = dockerhubDB.Collection(repositories)
	mymongo.TagColl = dockerhubDB.Collection(tags)
	mymongo.ImgColl = dockerhubDB.Collection(images)
	mymongo.ImgResultColl = dockerhubDB.Collection(imgResults)
	mymongo.LayerResultColl = dockerhubDB.Collection(layerResults)

	// TODO: 初次使用建立索引
	if initFlag {
		if err = mymongo.createRepoCollIndexes(); err != nil {
			return mymongo, err
		}

		if err = mymongo.createTagCollIndexes(); err != nil {
			return mymongo, err
		}

		if err = mymongo.createImgCollIndexes(); err != nil {
			return mymongo, err
		}

		if err = mymongo.createImgResultCollIndexes(); err != nil {
			return mymongo, err
		}

		if err = mymongo.createLayerResultCollIndexes(); err != nil {
			return mymongo, err
		}
	}

	return mymongo, nil
}

// createRepoCollIndexes creates indexes on repositories collection.
func (m *MyMongo) createRepoCollIndexes() (err error) {
	indexView := m.RepoColl.Indexes()

	// Unique index: namespace, name
	model := mongo.IndexModel{
		Keys: bson.D{
			{Key: "namespace", Value: 1},
			{Key: "name", Value: 1},
		},
		Options: options.Index().SetUnique(true),
	}
	_, err = indexView.CreateOne(context.Background(), model)
	if err != nil {
		if !mongo.IsDuplicateKeyError(err) {
			return
		}
	}

	// index: namespace
	model2 := mongo.IndexModel{
		Keys: bson.D{
			{Key: "namespace", Value: 1},
		},
		Options: options.Index().SetUnique(false),
	}
	_, err = indexView.CreateOne(context.Background(), model2)
	if err != nil {
		if !mongo.IsDuplicateKeyError(err) {
			return
		}
	}

	// index: name
	model3 := mongo.IndexModel{
		Keys: bson.D{
			{Key: "name", Value: 1},
		},
		Options: options.Index().SetUnique(false),
	}
	_, err = indexView.CreateOne(context.Background(), model3)
	if err != nil {
		if !mongo.IsDuplicateKeyError(err) {
			return
		}
	}

	// Text index: namespace, name, description, full_description
	textModel := mongo.IndexModel{
		Keys: bson.D{
			{Key: "namespace", Value: "text"},
			{Key: "name", Value: "text"},
			{Key: "description", Value: "text"},
			{Key: "full_description", Value: "text"},
		},
		Options: options.Index().SetWeights(bson.D{
			{"namespace", 12},
			{"name", 12},
			{"description", 6},
			{"full_description", 1},
		}),
	}
	_, err = indexView.CreateOne(context.Background(), textModel)
	if err != nil {
		if !mongo.IsDuplicateKeyError(err) {
			return
		}
	}

	// 报错为重复建立索引，将返回值置空
	if mongo.IsDuplicateKeyError(err) {
		err = nil
	}
	return
}

// createTagCollIndexes creates indexes on tags collection.
func (m *MyMongo) createTagCollIndexes() (err error) {
	indexView := m.TagColl.Indexes()

	// Unique index: repositories_namespace, repositories_name, name, last_updated
	model := mongo.IndexModel{
		Keys: bson.D{
			{Key: "repositories_namespace", Value: 1},
			{Key: "repositories_name", Value: 1},
			{Key: "name", Value: 1},
			{Key: "last_updated", Value: 1},
		},
		Options: options.Index().SetUnique(true),
	}
	_, err = indexView.CreateOne(context.Background(), model)
	if err != nil {
		if !mongo.IsDuplicateKeyError(err) {
			return
		}
	}

	// index: repositories_namespace
	model2 := mongo.IndexModel{
		Keys: bson.D{
			{Key: "repositories_namespace", Value: 1},
		},
		Options: options.Index().SetUnique(false),
	}
	_, err = indexView.CreateOne(context.Background(), model2)
	if err != nil {
		if !mongo.IsDuplicateKeyError(err) {
			return
		}
	}

	// index: repositories_name
	model3 := mongo.IndexModel{
		Keys: bson.D{
			{Key: "repositories_name", Value: 1},
		},
		Options: options.Index().SetUnique(false),
	}
	_, err = indexView.CreateOne(context.Background(), model3)
	if err != nil {
		if !mongo.IsDuplicateKeyError(err) {
			return
		}
	}

	// 报错为重复建立索引，将返回值置空
	if mongo.IsDuplicateKeyError(err) {
		err = nil
	}
	return
}

// createImgCollIndexes creates indexes on images collection.
func (m *MyMongo) createImgCollIndexes() (err error) {
	indexView := m.ImgColl.Indexes()

	// Unique index: digest
	model := mongo.IndexModel{
		Keys: bson.D{
			{Key: "digest", Value: 1},
		},
		Options: options.Index().SetUnique(true),
	}
	_, err = indexView.CreateOne(context.Background(), model)
	if err != nil {
		if !mongo.IsDuplicateKeyError(err) {
			return
		}
	}

	if mongo.IsDuplicateKeyError(err) {
		err = nil
	}
	return
}

// createImgResultCollIndexes creates indexes on image results collection.
func (m *MyMongo) createImgResultCollIndexes() (err error) {
	indexView := m.ImgResultColl.Indexes()

	// Unique index: namespace, repository_name, tag_name, digest
	model := mongo.IndexModel{
		Keys: bson.D{
			{Key: "namespace", Value: 1},
			{Key: "repository_name", Value: 1},
			{Key: "tag_name", Value: 1},
			{Key: "digest", Value: 1},
		},
		Options: options.Index().SetUnique(true),
	}
	_, err = indexView.CreateOne(context.Background(), model)
	if err != nil {
		if !mongo.IsDuplicateKeyError(err) {
			return
		}
	}

	// index: digest
	model2 := mongo.IndexModel{
		Keys: bson.D{
			{Key: "digest", Value: 1},
		},
		Options: options.Index().SetUnique(false),
	}
	_, err = indexView.CreateOne(context.Background(), model2)
	if err != nil {
		if !mongo.IsDuplicateKeyError(err) {
			return
		}
	}

	// index: namespace
	model3 := mongo.IndexModel{
		Keys: bson.D{
			{Key: "namespace", Value: 1},
		},
		Options: options.Index().SetUnique(false),
	}
	_, err = indexView.CreateOne(context.Background(), model3)
	if err != nil {
		if !mongo.IsDuplicateKeyError(err) {
			return
		}
	}

	// index: digest
	model4 := mongo.IndexModel{
		Keys: bson.D{
			{Key: "repository_name", Value: 1},
		},
		Options: options.Index().SetUnique(false),
	}
	_, err = indexView.CreateOne(context.Background(), model4)
	if err != nil {
		if !mongo.IsDuplicateKeyError(err) {
			return
		}
	}

	if mongo.IsDuplicateKeyError(err) {
		err = nil
	}
	return
}

// createLayerResultCollIndexes creates indexes on layer results collection.
func (m *MyMongo) createLayerResultCollIndexes() (err error) {
	indexView := m.LayerResultColl.Indexes()

	// index: digest
	model := mongo.IndexModel{
		Keys: bson.D{
			{Key: "digest", Value: 1},
		},
		Options: options.Index().SetUnique(true),
	}
	_, err = indexView.CreateOne(context.Background(), model)
	if err != nil {
		if !mongo.IsDuplicateKeyError(err) {
			return
		}
	}

	if mongo.IsDuplicateKeyError(err) {
		err = nil
	}
	return
}

func (m *MyMongo) UpdateRepository(repoMeta *Repository) error {
	filter := bson.M{
		"namespace": repoMeta.Namespace,
		"name":      repoMeta.Name,
	}
	update := bson.M{
		"$set": repoMeta,
	}
	opts := options.Update().SetUpsert(true)

	_, err := m.RepoColl.UpdateOne(context.TODO(), filter, update, opts)
	return err
}

func (m *MyMongo) FindRepositoryByName(namespace, name string) (*Repository, error) {
	rMeta := new(Repository)

	filter := bson.M{}
	if namespace != "" {
		filter["namespace"] = namespace
	}
	if name != "" {
		filter["name"] = name
	}

	err := m.RepoColl.FindOne(context.Background(), filter).Decode(rMeta)

	return rMeta, err
}

func (m *MyMongo) UpdateTag(tMeta *Tag) error {
	filter := bson.M{
		"repositories_namespace": tMeta.RepositoryNamespace,
		"repositories_name":      tMeta.RepositoryName,
		"name":                   tMeta.Name,
	}
	update := bson.M{
		"$set": tMeta,
	}
	opts := options.Update().SetUpsert(true)

	_, err := m.TagColl.UpdateOne(context.TODO(), filter, update, opts)
	return err
}

func (m *MyMongo) FindTagByName(repoNamespace, repoName, name string) (*Tag, error) {
	tMeta := new(Tag)

	// 创建管道流程，根据名称匹配返回最新的tag信息
	pipeline := []bson.M{
		bson.M{
			"$match": bson.M{
				"repositories_namespace": repoNamespace,
				"repositories_name":      repoName,
				"name":                   name,
			},
		},
		bson.M{
			"$addFields": bson.M{
				"last_updated_time": bson.M{
					"$dateFromString": bson.M{
						"dateString": "$last_updated",
					},
				},
			},
		},
		bson.M{
			"$sort": bson.M{
				"last_updated_time": -1,
			},
		},
		bson.M{
			"$limit": 1,
		},
	}

	cursor, err := m.TagColl.Aggregate(context.Background(), pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.TODO())

	if cursor.Next(context.TODO()) {
		if err = cursor.Decode(tMeta); err != nil {
			Logger.Error("mongo cursor.Decode metadata of tag", repoNamespace, repoName, name, "failed with:", err.Error())
			return nil, err
		}
		return tMeta, nil
	}

	return nil, fmt.Errorf("no metadata of tag %s/%s:%s found in mongo", repoNamespace, repoName, name)
}

func (m *MyMongo) UpdateImage(iMeta *Image) error {
	filter := bson.M{
		"digest": iMeta.Digest,
	}
	update := bson.M{
		"$set": iMeta,
	}
	opts := options.Update().SetUpsert(true)

	_, err := m.ImgColl.UpdateOne(context.TODO(), filter, update, opts)
	return err
}

func (m *MyMongo) FindImageByDigest(digest string) (*Image, error) {
	iMeta := new(Image)

	filter := bson.M{
		"digest": digest,
	}

	err := m.ImgColl.FindOne(context.Background(), filter).Decode(iMeta)

	return iMeta, err
}

func (m *MyMongo) UpdateImgResult(imgRes *ImageResult) error {
	filter := bson.M{
		"namespace":       imgRes.Namespace,
		"repository_name": imgRes.RepoName,
		"tag_name":        imgRes.TagName,
		"digest":          imgRes.Digest,
	}
	update := bson.M{
		"$set": imgRes,
	}
	opts := options.Update().SetUpsert(true)

	_, err := m.ImgResultColl.UpdateOne(context.TODO(), filter, update, opts)
	return err
}

func (m *MyMongo) FindImgResultByDigest(digest string) (*ImageResult, error) {
	res := new(ImageResult)

	filter := bson.M{
		"digest": digest,
	}

	err := m.ImgResultColl.FindOne(context.Background(), filter).Decode(res)

	return res, err
}

func (m *MyMongo) FindImgResultByName(namespace, repoName, tagName string) (*ImageResult, error) {
	res := new(ImageResult)

	filter := bson.M{
		"namespace":       namespace,
		"repository_name": repoName,
		"tag_name":        tagName,
	}

	err := m.ImgResultColl.FindOne(context.TODO(), filter).Decode(res)

	return res, err
}

func (m *MyMongo) UpdateLayerResult(layerRes *LayerResult) error {
	filter := bson.M{
		"digest": layerRes.Digest,
	}
	update := bson.M{
		"$set": layerRes,
	}
	opts := options.Update().SetUpsert(true)

	_, err := m.LayerResultColl.UpdateOne(context.TODO(), filter, update, opts)
	return err
}

func (m *MyMongo) FindLayerResultByDigest(digest string) (*LayerResult, error) {
	res := new(LayerResult)

	filter := bson.M{
		"digest": digest,
	}

	err := m.LayerResultColl.FindOne(context.Background(), filter).Decode(res)

	return res, err
}
