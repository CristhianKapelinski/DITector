package buildgraph

import (
	"context"
)

func Build(format string) {
	config(format)
	defer func() {
		mymongo.Client.Disconnect(context.Background())
		myNeo4jDriver.Driver.Close(context.Background())
	}()

	switch format {
	case "json":
		BuildFromJSON()
		//r, _ := FindImageByDigest("sha256:7c8b70990dad7e4325bf26142f59f77c969c51e079918f4631767ac8d49e22fb")
		//b, _ := json.Marshal(r)
		//fmt.Println(string(b))
	case "mysql":
		BuildFromMysql()
	}
}

// BuildFromJSON 根据crawler爬到的json内容建立信息库
func BuildFromJSON() {
	StartFromJSON()
}

// BuildFromMysql 根据crawler爬到的mysql内容建立信息库
func BuildFromMysql() {

}
