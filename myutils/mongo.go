package myutils

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MyMongo struct {
	Client          *mongo.Client
	DockerHubDB     *mongo.Database
	RepoColl        *mongo.Collection
	TagColl         *mongo.Collection
	ImgColl         *mongo.Collection
	ImgResultColl   *mongo.Collection
	LayerResultColl *mongo.Collection
	UserColl        *mongo.Collection
	// KeywordsColl stores the set of keywords fully crawled by Stage I.
	// _id = keyword string; crawled_at = RFC3339 timestamp.
	// Enables O(1) resume: already-crawled keywords are skipped on restart.
	KeywordsColl *mongo.Collection
}

func NewMongoGlobalConfig() (*MyMongo, error) {
	return NewMongo(GlobalConfig.MongoConfig.URI, GlobalConfig.MongoConfig.Database,
		GlobalConfig.MongoConfig.Collections.Repositories, GlobalConfig.MongoConfig.Collections.Tags,
		GlobalConfig.MongoConfig.Collections.Images, GlobalConfig.MongoConfig.Collections.ImageResults,
		GlobalConfig.MongoConfig.Collections.LayerResults, GlobalConfig.MongoConfig.Collections.User, false)
}

// NewMongo returns a new mongo client
func NewMongo(uri, database, repositories, tags, images, imgResults, layerResults, user string, initFlag bool) (*MyMongo, error) {
	var mymongo = new(MyMongo)
	var err error

	// 设置超时时间
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	mongoOptions := options.Client().ApplyURI(uri).
		SetMaxPoolSize(100).
		SetMinPoolSize(5).
		SetMaxConnIdleTime(5 * time.Minute)
	mymongo.Client, err = mongo.Connect(ctx, mongoOptions)
	if err != nil {
		return mymongo, err
	}

	err = mymongo.Client.Ping(context.TODO(), nil)
	if err != nil {
		return mymongo, err
	}

	mymongo.DockerHubDB = mymongo.Client.Database(database)
	mymongo.RepoColl = mymongo.DockerHubDB.Collection(repositories)
	mymongo.TagColl = mymongo.DockerHubDB.Collection(tags)
	mymongo.ImgColl = mymongo.DockerHubDB.Collection(images)
	mymongo.ImgResultColl = mymongo.DockerHubDB.Collection(imgResults)
	// mymongo.LayerResultColl = mymongo.DockerHubDB.Collection(layerResults)
	mymongo.UserColl = mymongo.DockerHubDB.Collection(user)
	mymongo.KeywordsColl = mymongo.DockerHubDB.Collection("crawler_keywords")

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

	if err = mymongo.createUserCollIndexes(); err != nil {
		return mymongo, err
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

func (m *MyMongo) createUserCollIndexes() (err error) {
	indexView := m.UserColl.Indexes()

	var model mongo.IndexModel

	// index: username
	model = mongo.IndexModel{
		Keys: bson.D{
			{Key: "username", Value: 1},
		},
		Options: options.Index().SetUnique(true),
	}
	_, err = indexView.CreateOne(context.Background(), model)
	if err != nil {
		if !mongo.IsDuplicateKeyError(err) {
			return
		}
	}

	// index: uid
	model = mongo.IndexModel{
		Keys: bson.D{
			{Key: "uid", Value: 1},
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

// BulkUpsertRepositories inserts or updates a batch of repositories in a single
// MongoDB round-trip using an unordered bulk write. This is ~10-50× faster than
// calling UpdateRepository in a loop when processing a full search results page.
func (m *MyMongo) BulkUpsertRepositories(repos []*Repository) error {
	if len(repos) == 0 {
		return nil
	}
	models := make([]mongo.WriteModel, 0, len(repos))
	for _, r := range repos {
		filter := bson.M{"namespace": r.Namespace, "name": r.Name}
		update := bson.M{"$set": r}
		models = append(models, mongo.NewUpdateOneModel().
			SetFilter(filter).
			SetUpdate(update).
			SetUpsert(true))
	}
	opts := options.BulkWrite().SetOrdered(false) // unordered = parallel execution on server
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err := m.RepoColl.BulkWrite(ctx, models, opts)
	return err
}

// --- Stage I checkpoint: crawler keyword tracking ---

// IsKeywordCrawled reports whether keyword was fully crawled in a previous run.
func (m *MyMongo) IsKeywordCrawled(keyword string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	count, err := m.KeywordsColl.CountDocuments(ctx, bson.M{"_id": keyword})
	return err == nil && count > 0
}

// MarkKeywordCrawled records keyword as fully crawled. Idempotent (upsert).
func (m *MyMongo) MarkKeywordCrawled(keyword string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := m.KeywordsColl.UpdateOne(
		ctx,
		bson.M{"_id": keyword},
		bson.M{"$setOnInsert": bson.M{"_id": keyword, "crawled_at": time.Now().UTC().Format(time.RFC3339)}},
		options.Update().SetUpsert(true),
	)
	return err
}

// DropKeywordCheckpoint removes all crawled-keyword records. Called at the end
// of a complete DFS run so the next restart performs a full re-crawl cycle.
func (m *MyMongo) DropKeywordCheckpoint() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return m.KeywordsColl.Drop(ctx)
}

// --- Stage II checkpoint: per-repo graph build tracking ---

// MarkRepoGraphBuilt sets graph_built_at on the repo document. Called by Stage II
// after all tags/images for the repo have been inserted into Neo4j.
func (m *MyMongo) MarkRepoGraphBuilt(namespace, name string) error {
	_, err := m.RepoColl.UpdateOne(
		context.TODO(),
		bson.M{"namespace": namespace, "name": name},
		bson.M{"$set": bson.M{"graph_built_at": time.Now().UTC().Format(time.RFC3339)}},
	)
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

func (m *MyMongo) FindRepositoriesByPullCountPaged(threshold, page, pageSize int64) ([]*Repository, error) {
	res := make([]*Repository, 0)

	filter := bson.M{
		"pull_count": bson.M{
			"$gt": threshold,
		},
	}

	optLimit := options.Find().SetSkip((page - 1) * pageSize).SetLimit(pageSize)
	cursor, err := m.RepoColl.Find(context.TODO(), filter, optLimit)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.TODO())

	if err = cursor.All(context.TODO(), &res); err != nil {
		return nil, err
	}

	return res, nil
}

func (m *MyMongo) FindRepositoriesByKeywordPaged(KeyMap map[string]any, page, pageSize int64) ([]*Repository, error) {
	res := make([]*Repository, 0)

	filter := bson.M{}
	for k, v := range KeyMap {
		if k == "" {
			continue
		}
		filter[k] = v
	}

	optLimit := options.Find().SetSkip((page - 1) * pageSize).SetLimit(pageSize)
	cursor, err := m.RepoColl.Find(context.TODO(), filter, optLimit)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.TODO())

	if err = cursor.All(context.TODO(), &res); err != nil {
		return nil, err
	}

	return res, nil
}

func (m *MyMongo) CountRepoByKeyword(KeyMap map[string]any) (int64, error) {
	filter := bson.M{}
	for k, v := range KeyMap {
		if k == "" {
			continue
		}
		filter[k] = v
	}
	return m.RepoColl.CountDocuments(context.TODO(), filter)
}

func (m *MyMongo) FindRepositoriesByText(search string, page, pageSize int64) ([]*Repository, error) {
	res := make([]*Repository, 0)

	filter := bson.D{{"$text", bson.D{{"$search", sanitizeSearchString(search)}}}}

	optLimit := options.Find().SetSkip((page - 1) * pageSize).SetLimit(pageSize)
	cursor, err := m.RepoColl.Find(context.TODO(), filter, optLimit)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.TODO())

	if err = cursor.All(context.TODO(), &res); err != nil {
		return nil, err
	}

	return res, nil
}

func (m *MyMongo) CountRepoByText(search string) (int64, error) {
	filter := bson.D{{"$text", bson.D{{"$search", sanitizeSearchString(search)}}}}
	return m.RepoColl.CountDocuments(context.TODO(), filter)
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

// FindTagsByRepoNamePaged 根据namespace/repo_name查找tag元数据，按照page, pageSize进行分页查找
func (m *MyMongo) FindTagsByRepoNamePaged(repoNamespace, repoName string, page, pageSize int64) ([]*Tag, error) {
	res := make([]*Tag, 0)

	// 创建管道流程，根据名称匹配返回最新的tag信息
	pipeline := []bson.M{
		bson.M{
			"$match": bson.M{
				"repositories_namespace": repoNamespace,
				"repositories_name":      repoName,
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
			"$skip": (page - 1) * pageSize,
		},
		bson.M{
			"$limit": pageSize,
		},
	}

	cursor, err := m.TagColl.Aggregate(context.Background(), pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.TODO())

	if err = cursor.All(context.TODO(), &res); err != nil {
		return nil, err
	}

	return res, nil
}

// FindAllTagsByRepoName 查询返回指定repo名称下的全部tag
func (m *MyMongo) FindAllTagsByRepoName(repoNamespace, repoName string) ([]*Tag, error) {
	res := make([]*Tag, 0)

	var page int64 = 1
	var pageSize int64 = 100

	for {
		tmp, err := m.FindTagsByRepoNamePaged(repoNamespace, repoName, page, pageSize)
		if err != nil {
			Logger.Error("FindTagsByRepoNamePaged for repo", repoNamespace, repoName, "failed with:", err.Error())
			if page == 1 {
				return nil, err
			} else {
				break
			}
		}
		if len(tmp) == 0 {
			break
		} else if len(tmp) < int(pageSize) {
			res = append(res, tmp...)
			break
		} else {
			res = append(res, tmp...)
			page++
			continue
		}
	}

	return res, nil
}

func (m *MyMongo) FindTagByImgDigestPaged(digest string, page, pageSize int64) ([]*Tag, error) {
	if len(digest) != 71 || !strings.HasPrefix(digest, "sha256:") {
		return nil, fmt.Errorf("inputed FindTagByImgDigestPaged digest is not legal")
	}
	res := make([]*Tag, 0)

	filter := bson.M{"images.digest": digest}
	optLimit := options.Find().SetSkip((page - 1) * pageSize).SetLimit(pageSize)

	cursor, err := m.TagColl.Find(context.TODO(), filter, optLimit)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.TODO())

	if err = cursor.All(context.TODO(), &res); err != nil {
		return nil, err
	}

	return res, nil
}

func (m *MyMongo) CountTagByImgDigest(digest string) (int64, error) {
	if len(digest) != 71 || !strings.HasPrefix(digest, "sha256:") {
		return 0, fmt.Errorf("inputed FindTagByImgDigestPaged digest is not legal")
	}
	filter := bson.M{"images.digest": digest}
	return m.TagColl.CountDocuments(context.TODO(), filter)
}

func (m *MyMongo) FindTagByKeywordPaged(KeyMap map[string]any, page, pageSize int64) ([]*Tag, error) {
	res := make([]*Tag, 0)

	filter := bson.M{}
	for k, v := range KeyMap {
		if k == "" {
			continue
		}
		filter[k] = v
	}

	optLimit := options.Find().SetSkip((page - 1) * pageSize).SetLimit(pageSize)
	cursor, err := m.TagColl.Find(context.TODO(), filter, optLimit)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.TODO())

	if err = cursor.All(context.TODO(), &res); err != nil {
		return nil, err
	}

	return res, nil
}

func (m *MyMongo) CountTagByKeyword(KeyMap map[string]any) (int64, error) {
	filter := bson.M{}
	for k, v := range KeyMap {
		if k == "" {
			continue
		}
		filter[k] = v
	}
	return m.TagColl.CountDocuments(context.TODO(), filter)
}

func (m *MyMongo) FindTagByTextPaged(search string, page, pageSize int64) ([]*Tag, error) {
	res := make([]*Tag, 0)

	filter := bson.D{{"$text", bson.D{{"$search", sanitizeSearchString(search)}}}}

	optLimit := options.Find().SetSkip((page - 1) * pageSize).SetLimit(pageSize)
	cursor, err := m.TagColl.Find(context.TODO(), filter, optLimit)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.TODO())

	if err = cursor.All(context.TODO(), &res); err != nil {
		return nil, err
	}

	return res, nil
}

func (m *MyMongo) CountTagByText(search string) (int64, error) {
	filter := bson.D{{"$text", bson.D{{"$search", sanitizeSearchString(search)}}}}
	return m.TagColl.CountDocuments(context.TODO(), filter)
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

// 传入的KeyMap应该仅为空，或仅包含digest字段
func (m *MyMongo) FindImageByKeywordPaged(KeyMap map[string]any, page, pageSize int64) ([]*Image, error) {
	res := make([]*Image, 0)

	filter := bson.M{}
	for k, v := range KeyMap {
		if k == "" {
			continue
		}
		filter[k] = v
	}

	optLimit := options.Find().SetSkip((page - 1) * pageSize).SetLimit(pageSize)
	cursor, err := m.ImgColl.Find(context.TODO(), filter, optLimit)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.TODO())

	if err = cursor.All(context.TODO(), &res); err != nil {
		return nil, err
	}

	return res, nil
}

func (m *MyMongo) CountImageByKeyword(KeyMap map[string]any) (int64, error) {
	filter := bson.M{}
	for k, v := range KeyMap {
		if k == "" {
			continue
		}
		filter[k] = v
	}
	return m.ImgColl.CountDocuments(context.TODO(), filter)
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

// 仅用于更新image结果中的content_result.vulnerabilities字段
func (m *MyMongo) UpdateImgResultVul(imgRes *ImageResult) error {
	filter := bson.M{
		"namespace":       imgRes.Namespace,
		"repository_name": imgRes.RepoName,
		"tag_name":        imgRes.TagName,
		"digest":          imgRes.Digest,
	}
	update := bson.M{
		"$set": bson.M{
			"content_result.vulnerabilities": imgRes.ContentResult.Vulnerabilities,
		},
	}
	opts := options.Update()

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

func (m *MyMongo) FindImgResultByExactName(namespace, repoName, tagName, digest string) (*ImageResult, error) {
	res := new(ImageResult)

	filter := bson.M{
		"namespace":       namespace,
		"repository_name": repoName,
		"tag_name":        tagName,
		"digest":          digest,
	}

	err := m.ImgResultColl.FindOne(context.TODO(), filter).Decode(res)

	return res, err
}

func (m *MyMongo) FindImgResultByName(namespace, repoName, tagName, digest string) (*ImageResult, error) {
	res := new(ImageResult)

	filter := bson.M{}
	if namespace != "" {
		filter["namespace"] = namespace
	}
	if repoName != "" {
		filter["repository_name"] = repoName
	}
	if tagName != "" {
		filter["tag_name"] = tagName
	}
	if digest != "" {
		filter["digest"] = digest
	}

	err := m.ImgResultColl.FindOne(context.TODO(), filter).Decode(res)

	return res, err
}

func (m *MyMongo) FindImgResultsByNamePaged(namespace, repoName, tagName, digest string, page, pageSize int64) ([]*ImageResult, error) {
	res := make([]*ImageResult, 0)

	filter := bson.M{}
	if namespace != "" {
		filter["namespace"] = namespace
	}
	if repoName != "" {
		filter["repository_name"] = repoName
	}
	if tagName != "" {
		filter["tag_name"] = tagName
	}
	if digest != "" {
		filter["digest"] = digest
	}

	optLimit := options.Find().SetSkip((page - 1) * pageSize).SetLimit(pageSize)
	cursor, err := m.ImgResultColl.Find(context.TODO(), filter, optLimit)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.TODO())

	if err = cursor.All(context.TODO(), &res); err != nil {
		return nil, err
	}

	return res, err
}

func (m *MyMongo) CountImgResultsByName(namespace, repoName, tagName, digest string) (int64, error) {
	filter := bson.M{}
	allEmpty := true
	if namespace != "" {
		filter["namespace"] = namespace
		allEmpty = false
	}
	if repoName != "" {
		filter["repository_name"] = repoName
		allEmpty = false
	}
	if tagName != "" {
		filter["tag_name"] = tagName
		allEmpty = false
	}
	if digest != "" {
		filter["digest"] = digest
		allEmpty = false
	}

	if allEmpty {
		return m.ImgResultColl.EstimatedDocumentCount(context.TODO())
	}
	return m.ImgResultColl.CountDocuments(context.TODO(), filter)
}

func (m *MyMongo) FindImgResultByTextPaged(search string, page, pageSize int64) ([]*ImageResult, error) {
	res := make([]*ImageResult, 0)

	filter := bson.D{{"$text", bson.D{{"$search", search}}}}
	optLimit := options.Find().SetSkip((page - 1) * pageSize).SetLimit(pageSize)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	cursor, err := m.ImgResultColl.Find(ctx, filter, optLimit)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.TODO())

	if err = cursor.All(context.TODO(), &res); err != nil {
		return nil, err
	}

	return res, err
}

func (m *MyMongo) CountImgResByText(search string) (int64, error) {
	filter := bson.D{{"$text", bson.D{{"$search", search}}}}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	return m.ImgResultColl.CountDocuments(ctx, filter)
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

func (m *MyMongo) UpdateLayerResultVul(layerRes *LayerResult) error {
	filter := bson.M{
		"digest": layerRes.Digest,
	}
	update := bson.M{
		"$set": bson.M{
			"vulnerabilities": layerRes.Vulnerabilities,
		},
	}
	opts := options.Update()

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

func (m *MyMongo) FindUserByKeyword(keyMap map[string]any) (*User, error) {
	if len(keyMap) == 0 {
		return nil, fmt.Errorf("find user with empty keyword")
	}

	res := new(User)
	filter := bson.M{}
	for k, v := range keyMap {
		if k != "" {
			filter[k] = v
		}
	}

	err := m.UserColl.FindOne(context.TODO(), filter).Decode(res)

	return res, err
}

// TODO: 以后再加修改密码、新增用户、修改状态什么的
func (m *MyMongo) InsertUser(username, password, lastname, firstname, email, phone string) error {
	var err error

	insert := bson.M{
		"username":          username,
		"password":          password,
		"fullname":          firstname + lastname,
		"firstname":         firstname,
		"lastname":          lastname,
		"email":             email,
		"phone":             phone,
		"registration_time": GetLocalNowTime(),
		"status":            1,
		"type":              1,
	}

	// 尝试三次，基本是uid冲突，三次还不过就太倒霉了。。。
	for i := 0; i < 3; i++ {
		insert["uid"] = GetRandStr(32)

		_, err = m.UserColl.InsertOne(context.Background(), insert)
		if err == nil {
			return nil
		} else {
			continue
		}
	}

	return err
}

func (m *MyMongo) UpdateUserLogin(keyMap map[string]any, loginTime time.Time) error {
	if len(keyMap) == 0 {
		return fmt.Errorf("update user with empty keyword")
	}

	filter := bson.M{}
	for k, v := range keyMap {
		if k != "" {
			filter[k] = v
		}
	}

	update := bson.M{
		"$set": bson.M{
			"last_login_time": loginTime,
		},
	}

	_, err := m.UserColl.UpdateOne(context.TODO(), filter, update)
	return err
}

// sanitizeSearchString 净化用于text索引检索的用户输入字符串
func sanitizeSearchString(search string) string {
	clean := strings.ToLower(search)

	return clean
}
