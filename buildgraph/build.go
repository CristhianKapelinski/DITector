package buildgraph

func Build(format string, page int64, pageSize int, pullCountThreshold int64) {
	config(format)

	switch format {
	case "json":
		BuildFromJSON()
	case "mongo":
		BuildFromMongo(page, pageSize, pullCountThreshold)
	}
}

// BuildFromJSON 根据crawler爬到的json内容建立信息库
func BuildFromJSON() {
	StartFromJSON()
}

// BuildFromMongo 根据crawler爬到的mysql内容建立信息库
func BuildFromMongo(page int64, pageSize int, pullCountThreshold int64) {
	StartFromMongo(page, pageSize, pullCountThreshold)
}
