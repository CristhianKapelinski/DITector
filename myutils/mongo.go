package myutils

import (
	"context"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"strings"
)

type MyMongo struct {
	Client                 *mongo.Client
	RepositoriesCollection *mongo.Collection
	ImagesCollection       *mongo.Collection
	ResultsCollection      *mongo.Collection
}

// ConfigMongoClient returns a mongo client configured
// for the project
func ConfigMongoClient(initFlag bool) (*MyMongo, error) {
	var mymongo = new(MyMongo)
	var err error

	mongoOptions := options.Client().ApplyURI("mongodb://localhost:27017")
	mymongo.Client, err = mongo.Connect(context.TODO(), mongoOptions)
	if err != nil {
		return mymongo, err
	}

	err = mymongo.Client.Ping(context.TODO(), nil)
	if err != nil {
		return mymongo, err
	}
	// mongoRepositoriesCollection 用于存repository的元数据
	mymongo.RepositoriesCollection = mymongo.Client.Database("dockerhub").Collection("repositories")
	// mongoImagesCollection 用于存image的层信息
	mymongo.ImagesCollection = mymongo.Client.Database("dockerhub").Collection("images")
	// mongoResultsCollection is used to store analysis results of images, indexed by digest
	mymongo.ResultsCollection = mymongo.Client.Database("dockerhub").Collection("results")

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
				{Key: "results.name", Value: "text"},
				{Key: "results.type", Value: "text"},
				{Key: "results.path", Value: "text"},
				{Key: "results.match", Value: "text"},
			},
			Options: options.Index().SetWeights(bson.D{
				{Key: "results.name", Value: 2},
				{Key: "results.type", Value: 2},
				{Key: "results.path", Value: 1},
				{Key: "results.match", Value: 1},
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

// InsertRepository 利用Insert将Repository作为文档存储到Mongo中
func (mymongo *MyMongo) InsertRepository(repo *Repository) error {
	repo.Tags = map[string]Tag{}
	_, err := mymongo.RepositoriesCollection.InsertOne(context.Background(), repo)
	return err
}

// InsertTag 利用Update将TagSource添加到Mongo中存储的对应的repository的tags中
func (mymongo *MyMongo) InsertTag(tag *TagSource) error {
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
		"namespace": tag.Namespace,
		"name":      tag.RepositoryName,
	}
	// Mongo文档的键中不能包含"."，所以将tag.Tag中的"."替换为"$"
	tagKey := strings.Replace(tag.Name, ".", "$", -1)
	update := bson.M{
		"$set": bson.M{"tags." + tagKey: t},
	}
	_, err := mymongo.RepositoriesCollection.UpdateOne(context.TODO(), filter, update)
	return err
}

// InsertImage 将image存储到Mongo中
func (mymongo *MyMongo) InsertImage(image *ImageSource) error {
	// 为tag添加不同架构下的镜像digest
	err := mymongo.AddImageToRepositoriesCollection(image)
	// 将特定镜像的元数据单独存放到images集合
	err = mymongo.InsertImageToImagesCollection(image)
	return err
}

// AddImageToRepositoriesCollection 利用update $set，将image的digest添加到<namespace>/<repository>.tags.<tag>.images.<arch>.<variant>
func (mymongo *MyMongo) AddImageToRepositoriesCollection(image *ImageSource) error {
	// Mongo文档的键中不能包含"."，所以将image.Tag中的"."替换为"$"
	tagKey := strings.Replace(image.TagName, ".", "$", -1)
	filter := bson.M{
		"namespace": image.Namespace,
		"name":      image.RepositoryName,
	}
	arch := image.Image.Architecture
	variant := image.Image.Variant
	// Mongo文档字典类型的键不能为空，将arch, variant为""的修改为"null"
	if arch == "" {
		arch = "null"
	}
	if variant == "" {
		variant = "null"
	}
	update := bson.M{
		"$set": bson.M{"tags." + tagKey + ".images." + arch + "." + variant: image.Image.Digest},
	}
	_, err := mymongo.RepositoriesCollection.UpdateOne(context.TODO(), filter, update)
	return err
}

// InsertImageToImagesCollection 将image元数据作为文档插入到images collection中
func (mymongo *MyMongo) InsertImageToImagesCollection(image *ImageSource) error {
	i := image.Image
	_, err := mymongo.ImagesCollection.InsertOne(context.Background(), i)
	return err
}

// InsertResult insert result to collection results
func (mymongo *MyMongo) InsertResult(image *ImageResult) error {
	_, err := mymongo.ResultsCollection.InsertOne(context.TODO(), image)
	return err
}

// GetAllDocumentsCount 统计已经存入的文档数量（repository数量）
func (mymongo *MyMongo) GetAllDocumentsCount() (map[string]int64, error) {
	res := make(map[string]int64)
	filter := bson.M{}

	repositoryCnt, err := mymongo.RepositoriesCollection.CountDocuments(context.TODO(), filter)
	if err != nil {
		return res, err
	} else {
		res["repositories_cnt"] = repositoryCnt
	}

	imageCnt, err := mymongo.ImagesCollection.CountDocuments(context.TODO(), filter)
	if err != nil {
		return res, err
	} else {
		res["images_cnt"] = imageCnt
	}

	return res, nil
}

// DropAllDocuments 将repository collection从mongo删除
func (mymongo *MyMongo) DropAllDocuments() error {
	mymongo.RepositoriesCollection.Drop(context.TODO())
	mymongo.ImagesCollection.Drop(context.TODO())
	return nil
}

// GetRepositoriesCountByText calculate total count of documents
// in repositories collection searched by keyword, used for el-table page division
func (mymongo *MyMongo) GetRepositoriesCountByText(keyword string) (int64, error) {
	filter := bson.D{}
	if keyword != "" {
		filter = bson.D{
			{"$text", bson.D{{"$search", keyword}}},
		}
	}

	return mymongo.RepositoriesCollection.CountDocuments(context.TODO(), filter)
}

// FindRepositoriesByText search repository in collection mongo.dockerhub.repositories,
// now searched by namespace, name, description, full_description (text index)
func (mymongo *MyMongo) FindRepositoriesByText(search string, page, pageSize int64) ([]*Repository, error) {
	var res = make([]*Repository, 0)

	filter := bson.D{}
	if search != "" {
		if !StrLegalForMongo(search) {
			return nil, fmt.Errorf("invalid search parameters")
		}
		filter = bson.D{
			{"$text", bson.D{{"$search", search}}},
		}
	}

	optLimit := options.Find().SetSkip((page - 1) * pageSize).SetLimit(pageSize)

	cursor, err := mymongo.RepositoriesCollection.Find(context.TODO(), filter, optLimit)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.TODO())

	if err = cursor.All(context.TODO(), &res); err != nil {
		return nil, err
	}

	return res, nil
}

// FindRepositoryByName 根据Namespace、Repository寻找mongo.dockerhub.repository中存储的Repository
func (mymongo *MyMongo) FindRepositoryByName(namespace, repository string) (*Repository, error) {
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
	err := mymongo.RepositoriesCollection.FindOne(context.Background(), filter).Decode(repo)
	if err != nil {
		return &Repository{}, err
	}
	return repo, err
}

// GetImagesCountByText calculate total count of documents
// in images collection searched by keyword (digest), used for el-table page division
func (mymongo *MyMongo) GetImagesCountByText(keyword string) (int64, error) {
	filter := bson.D{}
	if keyword != "" {
		filter = bson.D{
			{"$text", bson.D{{"$search", keyword}}},
		}
	}

	return mymongo.ImagesCollection.CountDocuments(context.TODO(), filter)
}

// FindImagesByText search image in collection mongo.dockerhub.images,
// now searched by digest (text index)
func (mymongo *MyMongo) FindImagesByText(search string, page, pageSize int64) ([]*Image, error) {
	var res = make([]*Image, 0)

	filter := bson.D{}
	if search != "" {
		if !StrLegalForMongo(search) {
			return nil, fmt.Errorf("invalid search keywords")
		}
		filter = bson.D{
			{"$text", bson.D{{"$search", search}}},
		}
	}

	optLimit := options.Find().SetSkip((page - 1) * pageSize).SetLimit(pageSize)

	cursor, err := mymongo.ImagesCollection.Find(context.TODO(), filter, optLimit)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.TODO())

	if err = cursor.All(context.TODO(), &res); err != nil {
		return nil, err
	}

	return res, nil
}

// FindImageByDigest 根据Digest寻找mongo.dockerhub.images中存储的Image
func (mymongo *MyMongo) FindImageByDigest(digest string) (*Image, error) {
	var img = new(Image)

	// 传入条件
	filter := bson.M{}
	if digest != "" {
		filter["digest"] = digest
	}

	// 查询并返回结果
	err := mymongo.ImagesCollection.FindOne(context.Background(), filter).Decode(img)
	if err != nil {
		return &Image{}, err
	}

	return img, nil
}

func (mymongo *MyMongo) FindResultByDigest(digest string) (*ImageResult, error) {
	var imgres = new(ImageResult)

	// 传入条件
	filter := bson.M{}
	if digest != "" {
		filter["digest"] = digest
	}

	// 查询并返回结果
	err := mymongo.ImagesCollection.FindOne(context.Background(), filter).Decode(imgres)
	if err != nil {
		return nil, err
	}

	return imgres, nil
}
