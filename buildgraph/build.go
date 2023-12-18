package buildgraph

func Build(format string) {
	config(format)

	switch format {
	case "json":
		BuildFromJSON()
	case "mongo":
		BuildFromMongo()
	}
}

// BuildFromJSON 根据crawler爬到的json内容建立信息库
func BuildFromJSON() {
	StartFromJSON()
}

// BuildFromMongo 根据crawler爬到的mysql内容建立信息库
func BuildFromMongo() {
	StartFromMongo()
}
