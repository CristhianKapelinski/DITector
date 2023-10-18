package myutils

import (
	"context"
	"encoding/json"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"log"
	"testing"
	"time"
)

func TestChangeMongoDocumentField(t *testing.T) {
	mymongo, _ := ConfigMongoClient(false)
	filter := bson.M{}
	update := bson.M{
		"$rename": bson.M{"repository": "name"},
	}
	_, err := mymongo.RepositoriesCollection.UpdateMany(context.TODO(), filter, update)
	if err != nil {
		log.Fatalln(err)
	}
}

func TestConfigMongoClient(t *testing.T) {
	mymongo, err := ConfigMongoClient(true)
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Println(mymongo.ImagesCollection.Name())
}

func TestMyMongo_GetRepositoriesCountByText(t *testing.T) {
	mymongo, _ := ConfigMongoClient(false)
	begin := time.Now()
	cnt, err := mymongo.GetRepositoriesCountByText("library")
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Println(cnt)
	fmt.Println(time.Since(begin))
}

func TestMyMongo_FindRepositoriesByText(t *testing.T) {
	mymongo, _ := ConfigMongoClient(false)
	results, err := mymongo.FindRepositoriesByText("library/mongo", 1, 10)
	if err != nil {
		log.Fatalln("[ERROR] find repositories by text failed with err:", err)
	}
	fmt.Println(len(results))
	for _, result := range results {
		res, _ := json.Marshal(result)
		fmt.Println(string(res))
	}
}

func TestMyMongo_GetImagesCountByText(t *testing.T) {
	mymongo, _ := ConfigMongoClient(false)
	cnt, err := mymongo.GetImagesCountByText("3330448b38bedbdfea404a834d2a90f7aeda742e237eac34c97e86d3b31ab36a")
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Println(cnt)
}

func TestMyMongo_FindImagesByText(t *testing.T) {
	mymongo, _ := ConfigMongoClient(false)
	results, err := mymongo.FindImagesByText("", 1, 10)
	if err != nil {
		log.Fatalln("[ERROR] find images by digest text failed with err:", err)
	}
	fmt.Println(len(results))
	for _, result := range results {
		res, _ := json.Marshal(result)
		fmt.Println(string(res))
	}
}

func TestFindImageByDigest(t *testing.T) {
	mymongo, _ := ConfigMongoClient(false)
	//fmt.Println(mymongo.Client.Database("dockerhub").Collection("images").FindOne(context.TODO(), bson.M{}))
	tmpImage, err := mymongo.FindImageByDigest("sha256:7209d3b2285c9ca5a28051a5d8658e64e40888154d753bbd8a22eee214132a81")
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Println(tmpImage.Digest)
}

func TestMyMongo_InsertResult(t *testing.T) {

}

func TestMyMongo_FindResultByDigest(t *testing.T) {
	mymongo, _ := ConfigMongoClient(false)
	results, _ := mymongo.FindResultByDigest("sha256:7cfe7af27bf90963cec63320c6aaf3e25668d552551d58ac0b08ddc497f18ddb")
	fmt.Println(results)
}
