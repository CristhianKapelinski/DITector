package myutils

import (
	"context"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"time"
)

type MyMongo struct {
	Client                 *mongo.Client
	RepositoriesCollection *mongo.Collection
	TagsCollection         *mongo.Collection
	ImagesCollection       *mongo.Collection
	ResultsCollection      *mongo.Collection
}

// NewMongo returns a new mongo client
func NewMongo(uri, database, repositories, tags, images, results string, initFlag bool) (*MyMongo, error) {
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
	mymongo.RepositoriesCollection = dockerhubDB.Collection(repositories)
	mymongo.TagsCollection = dockerhubDB.Collection(tags)
	mymongo.ImagesCollection = dockerhubDB.Collection(images)
	mymongo.ResultsCollection = dockerhubDB.Collection(results)

	// TODO: 初次使用建立索引
	if initFlag {
		if err = mymongo.createReposCollectionIndexes(); err != nil {
			return mymongo, err
		}

		if err = mymongo.createTagsCollectionIndexes(); err != nil {
			return mymongo, err
		}

		if err = mymongo.createImagesCollectionIndexes(); err != nil {
			return mymongo, err
		}

		if err = mymongo.createResultsCollectionIndexes(); err != nil {
			return mymongo, err
		}
	}

	return mymongo, nil
}

// createReposCollectionIndexes creates indexes on repositories collection.
func (m *MyMongo) createReposCollectionIndexes() (err error) {
	repoIndexView := m.RepositoriesCollection.Indexes()

	// Unique index: namespace, name
	repoModel := mongo.IndexModel{
		Keys: bson.D{
			{Key: "namespace", Value: 1},
			{Key: "name", Value: 1},
		},
		Options: options.Index().SetUnique(true),
	}
	_, err = repoIndexView.CreateOne(context.Background(), repoModel)
	if err != nil {
		if !mongo.IsDuplicateKeyError(err) {
			return
		}
	}

	// index: namespace
	repoModel2 := mongo.IndexModel{
		Keys: bson.D{
			{Key: "namespace", Value: 1},
		},
		Options: options.Index().SetUnique(false),
	}
	_, err = repoIndexView.CreateOne(context.Background(), repoModel2)
	if err != nil {
		if !mongo.IsDuplicateKeyError(err) {
			return
		}
	}

	// index: name
	repoModel3 := mongo.IndexModel{
		Keys: bson.D{
			{Key: "name", Value: 1},
		},
		Options: options.Index().SetUnique(false),
	}
	_, err = repoIndexView.CreateOne(context.Background(), repoModel3)
	if err != nil {
		if !mongo.IsDuplicateKeyError(err) {
			return
		}
	}

	// Text index: namespace, name, description, full_description
	repoModelText := mongo.IndexModel{
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
	_, err = repoIndexView.CreateOne(context.Background(), repoModelText)
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

// createTagsCollectionIndexes creates indexes on tags collection.
func (m *MyMongo) createTagsCollectionIndexes() (err error) {
	tagIndexView := m.TagsCollection.Indexes()

	// Unique index: repositories_namespace, repositories_name, name, last_updated
	tagModel := mongo.IndexModel{
		Keys: bson.D{
			{Key: "repositories_namespace", Value: 1},
			{Key: "repositories_name", Value: 1},
			{Key: "name", Value: 1},
			{Key: "last_updated", Value: 1},
		},
		Options: options.Index().SetUnique(true),
	}
	_, err = tagIndexView.CreateOne(context.Background(), tagModel)
	if err != nil {
		if !mongo.IsDuplicateKeyError(err) {
			return
		}
	}

	// index: repositories_namespace
	tagModel2 := mongo.IndexModel{
		Keys: bson.D{
			{Key: "repositories_namespace", Value: 1},
		},
		Options: options.Index().SetUnique(false),
	}
	_, err = tagIndexView.CreateOne(context.Background(), tagModel2)
	if err != nil {
		if !mongo.IsDuplicateKeyError(err) {
			return
		}
	}

	// index: repositories_name
	tagModel3 := mongo.IndexModel{
		Keys: bson.D{
			{Key: "repositories_name", Value: 1},
		},
		Options: options.Index().SetUnique(false),
	}
	_, err = tagIndexView.CreateOne(context.Background(), tagModel3)
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

// createImagesCollectionIndexes creates indexes on images collection.
func (m *MyMongo) createImagesCollectionIndexes() (err error) {
	imageIndexView := m.ImagesCollection.Indexes()

	// Unique index: digest
	imageModel := mongo.IndexModel{
		Keys: bson.D{
			{Key: "digest", Value: 1},
		},
		Options: options.Index().SetUnique(true),
	}
	_, err = imageIndexView.CreateOne(context.Background(), imageModel)
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

// createResultsCollectionIndexes creates indexes on results collection.
// TODO: 具体使用哪些索引有待商榷
func (m *MyMongo) createResultsCollectionIndexes() (err error) {
	resultsIndexView := m.ResultsCollection.Indexes()

	// index: digest
	resultsModel := mongo.IndexModel{
		Keys: bson.D{
			{Key: "digest", Value: 1},
		},
		Options: options.Index().SetUnique(false),
	}
	_, err = resultsIndexView.CreateOne(context.Background(), resultsModel)
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

	_, err := m.RepositoriesCollection.UpdateOne(context.TODO(), filter, update, opts)
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

	err := m.RepositoriesCollection.FindOne(context.Background(), filter).Decode(rMeta)

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

	_, err := m.TagsCollection.UpdateOne(context.TODO(), filter, update, opts)
	return err
}

// FindTagByName TODO: 待单元测试
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

	cursor, err := m.TagsCollection.Aggregate(context.Background(), pipeline)
	if err != nil {
		return tMeta, err
	}
	defer cursor.Close(context.TODO())

	for cursor.Next(context.TODO()) {
		err := cursor.Decode(tMeta)
		return tMeta, err
	}

	return tMeta, err
}

func (m *MyMongo) FindImageByDigest(digest string) (*Image, error) {
	iMeta := new(Image)

	filter := bson.M{
		"digest": digest,
	}

	err := m.ImagesCollection.FindOne(context.Background(), filter).Decode(iMeta)

	return iMeta, err
}
