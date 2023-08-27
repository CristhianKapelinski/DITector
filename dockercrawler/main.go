package main

import (
	"buildgraph"
	"crawler"
	"flag"
	"os"
	"scripts"
	"server"
)

func main() {
	var (
		crawl       bool   // 是否要爬镜像仓库数据
		registry    string // 指定要爬的镜像仓库，比如dockerhub
		libraryFlag bool   // 爬虫是否爬官方镜像
		buildGraph  bool   // 是否要建信息库
		format      string // 爬虫存储格式/信息库从什么格式中取内容，json、mysql
		startServer bool   // 启动前端服务器
		execScript  bool   // 执行特制脚本
	)

	flag.BoolVar(&crawl, "crawl", false, "crawl images metadata if not nil")
	flag.StringVar(&registry, "registry", "dockerhub", "registry the register if not nil, e.g. dockerhub")
	flag.BoolVar(&libraryFlag, "official", false, "true for crawling official images; false for crawling community images")
	flag.BoolVar(&buildGraph, "build-graph", false, "true for building graph based on crawler results")
	flag.StringVar(&format, "format", "json", "format for crawling or building graph, e.g. json, mysql, clear")
	flag.BoolVar(&startServer, "start-server", false, "true for building graph based on crawler results")
	flag.BoolVar(&execScript, "exec-script", false, "true for specific script execution")
	flag.Parse()

	if crawl {
		if registry == "dockerhub" {
			crawler.StartRecursive(format, libraryFlag)
		}
	} else if buildGraph {
		buildgraph.Build(format)
	} else if startServer {
		// 10.10.21.122:23434
		server.StartServer()
	} else if execScript {
		scripts.CalculateRepositoriesDependentWeights()
	} else {
		flag.Usage()
		os.Exit(-1)
	}
}
