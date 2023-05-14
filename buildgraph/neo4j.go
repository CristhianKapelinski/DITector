package buildgraph

import "fmt"

// neo4j.go 用于操作neo4j

// TODO: 声明neo4j connector

// InsertImageToNeo4j 将
func InsertImageToNeo4j(image *ImageSource) {
	// TODO: 计算hash(1-2-5)，逐个插入neo4j并建立relation，i=len(Layers)-1时为节点添加属性：镜像
	for i, _ := range image.Image.Layers {
		fmt.Sprintf(string(i))
	}
}
