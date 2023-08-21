package myutils

import (
	"context"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"strings"
)

type MyMongo struct {
	Client                 *mongo.Client
	RepositoriesCollection *mongo.Collection
	ImagesCollection       *mongo.Collection
}

func ConfigMongoClient() (*MyMongo, error) {
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
	// mongoImagesCollection 用于存image的层信息
	mymongo.ImagesCollection = mymongo.Client.Database("dockerhub").Collection("images")
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

	return mymongo, nil
}

// InsertRepositoryToMongo 利用Insert将Repository作为文档存储到Mongo中
func (mymongo *MyMongo) InsertRepositoryToMongo(repo *Repository) error {
	repo.Tags = map[string]Tag{}
	_, err := mymongo.RepositoriesCollection.InsertOne(context.Background(), repo)
	return err
}

// InsertTagToMongo 利用Update将TagSource添加到Mongo中存储的对应的repository的tags中
func (mymongo *MyMongo) InsertTagToMongo(tag *TagSource) error {
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

// InsertImageToMongo 将image存储到Mongo中
func (mymongo *MyMongo) InsertImageToMongo(image *ImageSource) error {
	// 为tag添加不同架构下的镜像digest
	err := mymongo.AddImageToRepositoryMongo(image)
	// 将特定镜像的元数据单独存放到images集合
	err = mymongo.InsertImageToImagesCollection(image)
	return err
}

// AddImageToRepositoryMongo 利用update $set，将image的digest添加到<namespace>/<repository>.tags.<tag>.images.<arch>.<variant>
func (mymongo *MyMongo) AddImageToRepositoryMongo(image *ImageSource) error {
	// Mongo文档的键中不能包含"."，所以将image.Tag中的"."替换为"$"
	tagKey := strings.Replace(image.TagName, ".", "$", -1)
	filter := bson.M{
		"namespace": image.Namespace,
		"name":      image.RepositoryName,
	}
	arch := image.Image.Architecture
	variant := image.Image.Variant
	// Mongo文档字典类型的键不能为空，将arch, variant为""的修改为"null"
	if arch == " " {
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

// CountDocumentsFromMongo 统计已经存入的文档数量（repository数量）
func (mymongo *MyMongo) CountDocumentsFromMongo() (map[string]int64, error) {
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

// DropCollectionsFromMongo 将repository collection从mongo删除
func (mymongo *MyMongo) DropCollectionsFromMongo() error {
	mymongo.RepositoriesCollection.Drop(context.TODO())
	mymongo.ImagesCollection.Drop(context.TODO())
	return nil
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
