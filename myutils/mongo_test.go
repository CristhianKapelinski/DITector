package myutils

import (
	"context"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"log"
	"testing"
)

func TestChangeMongoDocumentField(t *testing.T) {
	mymongo, _ := ConfigMongoClient()
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
	mymongo, err := ConfigMongoClient()
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Println(mymongo.ImagesCollection.Name())
}

func TestFindImageByDigest(t *testing.T) {
	mymongo, _ := ConfigMongoClient()
	//fmt.Println(mymongo.Client.Database("dockerhub").Collection("images").FindOne(context.TODO(), bson.M{}))
	tmpImage, err := mymongo.FindImageByDigest("sha256:7209d3b2285c9ca5a28051a5d8658e64e40888154d753bbd8a22eee214132a81")
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Println(tmpImage.Digest)
}
