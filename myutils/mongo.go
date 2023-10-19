package myutils

import (
	"context"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
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

	mongoOptions := options.Client().ApplyURI(uri)
	mymongo.Client, err = mongo.Connect(context.TODO(), mongoOptions)
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

		// 建立唯一索引，namespace-repository防止插入重复数据
		repoIndexView := mymongo.RepositoriesCollection.Indexes()
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
				return mymongo, err
			}
		}
		// create index on namespace
		repoModel2 := mongo.IndexModel{
			Keys: bson.D{
				{Key: "namespace", Value: 1},
			},
			Options: options.Index().SetUnique(false),
		}
		_, err = repoIndexView.CreateOne(context.Background(), repoModel2)
		if err != nil {
			if !mongo.IsDuplicateKeyError(err) {
				return mymongo, err
			}
		}
		// create index on name
		repoModel3 := mongo.IndexModel{
			Keys: bson.D{
				{Key: "name", Value: 1},
			},
			Options: options.Index().SetUnique(false),
		}
		_, err = repoIndexView.CreateOne(context.Background(), repoModel3)
		if err != nil {
			if !mongo.IsDuplicateKeyError(err) {
				return mymongo, err
			}
		}

		// create text index on namespace, name, description, full_description with weights
		repoModelText := mongo.IndexModel{
			Keys: bson.D{
				{Key: "namespace", Value: "text"},
				{Key: "name", Value: "text"},
				{Key: "description", Value: "text"},
				{Key: "full_description", Value: "text"},
			},
			Options: options.Index().SetWeights(bson.D{
				{"namespace", 12},
				{"name", 18},
				{"description", 6},
				{"full_description", 1},
			}),
		}
		_, err = repoIndexView.CreateOne(context.Background(), repoModelText)
		if err != nil {
			if !mongo.IsDuplicateKeyError(err) {
				return mymongo, err
			}
		}
		// 建立唯一索引digest，防止插入重复数据
		imageIndexView := mymongo.ImagesCollection.Indexes()
		imageModel := mongo.IndexModel{
			Keys: bson.D{
				{Key: "digest", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		}
		_, err = imageIndexView.CreateOne(context.Background(), imageModel)
		if err != nil {
			if !mongo.IsDuplicateKeyError(err) {
				return mymongo, err
			}
		}
		// create text index on digest for search
		imageModelText := mongo.IndexModel{
			Keys: bson.D{
				{Key: "digest", Value: "text"},
			},
		}
		_, err = imageIndexView.CreateOne(context.TODO(), imageModelText)
		if err != nil {
			if !mongo.IsDuplicateKeyError(err) {
				return mymongo, err
			}
		}
		// 建立唯一索引digest，防止插入重复数据
		resultsIndexView := mymongo.ResultsCollection.Indexes()
		resultsModel := mongo.IndexModel{
			Keys: bson.D{
				{Key: "digest", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		}
		_, err = resultsIndexView.CreateOne(context.Background(), resultsModel)
		if err != nil {
			if !mongo.IsDuplicateKeyError(err) {
				return mymongo, err
			}
		}
		// create text index on digest for search
		resultsModelText := mongo.IndexModel{
			Keys: bson.D{
				{Key: "results.rulename", Value: "text"},
				{Key: "results.type", Value: "text"},
				{Key: "results.path", Value: "text"},
				{Key: "results.match", Value: "text"},
				{Key: "results.severity", Value: "text"},
				{Key: "results.layerdigest", Value: "text"},
			},
			Options: options.Index().SetWeights(bson.D{
				{Key: "results.name", Value: 2},
				{Key: "results.type", Value: 2},
				{Key: "results.path", Value: 1},
				{Key: "results.match", Value: 1},
				{Key: "results.severity", Value: 1},
				{Key: "results.layerdigest", Value: 1},
			}),
		}
		_, err = resultsIndexView.CreateOne(context.TODO(), resultsModelText)
		if err != nil {
			if !mongo.IsDuplicateKeyError(err) {
				return mymongo, err
			}
		}
	}

	return mymongo, nil
}
