package main

import (
	"flag"
	"fmt"
	"github.com/Musso12138/dockercrawler/buildgraph"
	"github.com/Musso12138/dockercrawler/crawler"
	"github.com/Musso12138/dockercrawler/myutils"
	"github.com/Musso12138/dockercrawler/scripts"
	"github.com/Musso12138/dockercrawler/server"
	"github.com/spf13/cobra"
	"os"
)

var art = `
 _____             _             _____                 
|  __ \           | |           / ____|                
| |  | | ___   ___| | _____ _ _| (___   ___ __ _ _ __  
| |  | |/ _ \ / __| |/ / _ \ '__\___ \ / __/ _ | '_ \ 
| |__| | (_) | (__|   <  __/ |  ____) | (_| (_| | | | |
|_____/ \___/ \___|_|\_\___|_| |_____/ \___\__,_|_| |_|
`

func main() {
	rootCmd := &cobra.Command{
		Run: "docker-scan",
		Long: art +
			"\n\t- crawl Docker container image from Docker Hub" +
			"\n\t- build dependency graph" +
			"\n\t- pull, save image with Docker CLI and scan weakness of image",
		Args:
	}

	// 命令行参数定义与绑定
	var (
		crawl       bool   // 是否要爬镜像仓库数据
		registry    string // 指定要爬的镜像仓库，比如dockerhub
		libraryFlag bool   // 爬虫是否爬官方镜像
		buildGraph  bool   // 是否要建信息库
		format      string // 爬虫存储格式/信息库从什么格式中取内容，json、mysql
		startServer bool   // 启动前端服务器
		execScript  bool   // 执行特制脚本
		rulePath    string // filepath of rules
		scan        bool   // 是否要扫描镜像
		image       string // 待扫描镜像名称
		file        string // 待扫描镜像文件
	)

	flag.BoolVar(&crawl, "crawl", false, "crawl images metadata if not nil")
	flag.StringVar(&registry, "registry", "dockerhub", "registry the register if not nil, e.g. dockerhub")
	flag.BoolVar(&libraryFlag, "official", false, "true for crawling official images; false for crawling community images")
	flag.BoolVar(&buildGraph, "build-graph", false, "true for building graph based on crawler results")
	flag.StringVar(&format, "format", "json", "format for crawling or building graph, e.g. json, mysql, clear")
	flag.BoolVar(&startServer, "start-server", false, "true for building graph based on crawler results")
	flag.BoolVar(&execScript, "exec-script", false, "true for specific script execution")
	flag.StringVar(&rulePath, "rule-path", "rules/secret_rules.yaml", "yaml file path storing rules")
	flag.BoolVar(&scan, "scan", false, "true for scanning image")
	flag.StringVar(&image, "image", "", "image name to be scanned, e.g. ")
	flag.StringVar(&file, "file", "", "image file to be scanned, formatted like file from `docker save`")
	flag.Parse()

	// 主函数退出前清理工作（最后一个执行的defer函数）
	defer myutils.CloseAllConnections()

	if crawl {
		if registry == "dockerhub" {
			crawler.StartRecursive(format, libraryFlag)
		}
	} else if buildGraph {
		buildgraph.Build(format)
	} else if startServer {
		// 10.10.21.122:23434
		server.StartServer()
	} else if scan {

	} else if execScript {
		scripts.ScanTop100DownstreamImagesVul()
		//scripts.StatisticRepositoriesDependentWeights()
		//scripts.ScanAllSecretsInImageMetadata()
		//scripts.CalculateRepositoriesDependentWeights()
	} else {
		flag.Usage()
		os.Exit(-1)
	}

	if err := rootCmd.Execute(); err != nil {
		myutils.Logger.Critical("execute root cmd failed with:", err.Error())
		os.Exit(1)
	}
}
