package buildgraph

import (
	"myutils"
)

// mongo_old.go 用于操作mongodb

var myMongo *myutils.MyMongoOld

// --------------------------------------------------------------------
// Deprecated: 已迁移myutils/mongo.go中实现

//// InsertRepository 利用Insert将Repository作为文档存储到Mongo中
//func InsertRepository(repo *myutils.RepositoryName) {
//	repo.Tags = map[string]myutils.TagName{}
//	_, err := InsertOne(context.Background(), repo)
//	if err != nil {
//		if mongo.IsDuplicateKeyError(err) {
//			//fmt.Println("[WARN] Mongo Duplicate when inserting repository", repo.Namespace, repo.RepositoryName, ", repository already exists")
//			return
//		}
//		LogDockerCrawlerString("[ERROR] Mongo Insert repository " + repo.Namespace + repo.Name + " to collection repository failed with: " + err.Error())
//		fmt.Println("[ERROR] Mongo Insert repository "+repo.Namespace+repo.Name+" to collection repository failed with: ", err)
//		return
//	}
//	//fmt.Println("[INFO] Insert repository", repo.Namespace+"/"+repo.RepositoryName, "succeed with ID", ret.InsertedID)
//}
//
//// InsertTag 利用Update将TagSource添加到Mongo中存储的对应的repository的tags中
//func InsertTag(tag *myutils.TagSource) {
//	var t = myutils.TagName{
//		LastUpdated:         tag.LastUpdated,
//		LastUpdaterUsername: tag.LastUpdaterUsername,
//		TagLastPulled:       tag.TagLastPulled,
//		TagLastPushed:       tag.TagLastPushed,
//		MediaType:           tag.MediaType,
//		ContentType:         tag.ContentType,
//		Images:              map[string]map[string]string{},
//	}
//	filter := bson.M{
//		"namespace":  tag.Namespace,
//		"repository": tag.RepositoryName,
//	}
//	// Mongo文档的键中不能包含"."，所以将tag.Tag中的"."替换为"$"
//	tagKey := strings.Replace(tag.Name, ".", "$", -1)
//	update := bson.M{
//		"$set": bson.M{"tags." + tagKey: t},
//	}
//	_, err := mongoRepositoriesCollection.UpdateOne(context.TODO(), filter, update)
//	if err != nil {
//		LogDockerCrawlerString("[ERROR] Mongo Update tag " + tag.Namespace + "/" + tag.RepositoryName + ":" + tag.Name + " failed with: " + err.Error())
//		fmt.Println("[ERROR] Mongo Update tag", tag.Namespace+"/"+tag.RepositoryName+":"+tag.Name, "failed with:", err)
//		return
//	}
//	//fmt.Println("[INFO] Insert tag", tag.Namespace+"/"+tag.RepositoryName+":"+tag.TagName, "succeed with ID", ret.UpsertedID)
//}
//
//// InsertImage 将image存储到Mongo中
//func InsertImage(image *myutils.ImageSource) error {
//	// 为tag添加不同架构下的镜像digest
//	AddImageToRepositoriesCollection(image)
//	// 将特定镜像的元数据单独存放到images集合
//	InsertImageToImagesCollectionMongo(image)
//}
//
//// AddImageToRepositoriesCollection 利用update $set，将image的digest添加到<namespace>/<repository>.tags.<tag>.images.<arch>.<variant>
//func AddImageToRepositoriesCollection(image *myutils.ImageSource) {
//	// Mongo文档的键中不能包含"."，所以将image.Tag中的"."替换为"$"
//	tagKey := strings.Replace(image.TagName, ".", "$", -1)
//	filter := bson.M{
//		"namespace":  image.Namespace,
//		"repository": image.RepositoryName,
//	}
//	arch := image.ImageOld.Architecture
//	variant := image.ImageOld.Variant
//	// Mongo文档字典类型的键不能为空，将arch, variant为""的修改为"null"
//	if arch == " " {
//		arch = "null"
//	}
//	if variant == "" {
//		variant = "null"
//	}
//	update := bson.M{
//		"$set": bson.M{"tags." + tagKey + ".images." + arch + "." + variant: image.ImageOld.Digest},
//	}
//	_, err := mongoRepositoriesCollection.UpdateOne(context.TODO(), filter, update)
//	if err != nil {
//		LogDockerCrawlerString("[ERROR] Mongo Update image " + image.Namespace + "/" + image.RepositoryName + ":" + image.TagName + " failed with: " + err.Error())
//		fmt.Println("[ERROR] Mongo Update image", image.Namespace+"/"+image.RepositoryName+":"+image.TagName, "failed with:", err)
//		return
//	}
//	//fmt.Println("[INFO] Insert image", image.Namespace+"/"+image.RepositoryName+":"+image.TagName, "succeed with ID", ret.UpsertedID)
//}
//
//// InsertImageToImagesCollectionMongo 将image元数据作为文档插入到images collection中
//func InsertImageToImagesCollectionMongo(image *myutils.ImageSource) {
//	i := image.ImageOld
//	_, err := mongoImagesCollection.InsertOne(context.Background(), i)
//	if err != nil {
//		if mongo.IsDuplicateKeyError(err) {
//			//fmt.Println("[WARN] Mongo Duplicate when inserting image", i.Digest, ", image already exists")
//			return
//		}
//		LogDockerCrawlerString("[ERROR] Mongo Insert image " + i.Digest + " to collection images failed with: " + err.Error())
//		fmt.Println("[ERROR] Mongo Insert image", i.Digest, "to collection images failed with:", err)
//		return
//	}
//	//fmt.Println("[INFO] Insert image", i.Digest, "succeed with ID", ret.UpsertedID)
//}
//
//// GetAllDocumentsCount 统计已经存入的文档数量（repository数量）
//func GetAllDocumentsCount() (map[string]int64, error) {
//	res := make(map[string]int64)
//	filter := bson.M{}
//
//	repositoryCnt, err := mongoRepositoriesCollection.CountDocuments(context.TODO(), filter)
//	if err != nil {
//		LogDockerCrawlerString("[ERROR] mongo count documents of repositories collection failed with err:", err.Error())
//		return res, err
//	} else {
//		res["repositories_cnt"] = repositoryCnt
//	}
//
//	imageCnt, err := mongoImagesCollection.CountDocuments(context.TODO(), filter)
//	if err != nil {
//		LogDockerCrawlerString("[ERROR] mongo count documents of images collection failed with err:", err.Error())
//		return res, err
//	} else {
//		res["images_cnt"] = imageCnt
//	}
//
//	return res, nil
//}
//
//// DropAllDocuments 将repository collection从mongo删除
//func DropAllDocuments() error {
//	mongoRepositoriesCollection.Drop(context.TODO())
//	mongoImagesCollection.Drop(context.TODO())
//	LogDockerCrawlerString("[WARN] drop collections: repository, images from MongoDB")
//	return nil
//}
