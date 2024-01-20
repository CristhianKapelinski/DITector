package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/Musso12138/docker-scan/analyzer"
	"github.com/Musso12138/docker-scan/buildgraph"
	"github.com/Musso12138/docker-scan/myutils"
	"github.com/Musso12138/docker-scan/scripts"
	"github.com/spf13/cobra"
)

var logLevelStr string
var validLogLevel = map[string]int{"debug": 1, "info": 2, "warn": 3, "error": 4, "critical": 5}

const longDesc = `
 _____             _             _____                 
|  __ \           | |           / ____|                
| |  | | ___   ___| | _____ _ _| (___   ___ __ _ _ __  
| |  | |/ _ \ / __| |/ / _ \ '__\___ \ / __/ _ | '_ \ 
| |__| | (_) | (__|   <  __/ |  ____) | (_| (_| | | | |
|_____/ \___/ \___|_|\_\___|_| |_____/ \___\__,_|_| |_|

A Docker security tool built with Go, implementing:
	- crawl Docker container image from Docker Hub
	- build dependency graph
	- pull, save image with Docker CLI and scan weakness of image`

var RootCmd = &cobra.Command{
	Use:   "docker-scan",
	Short: "docker-scan is a security tool for scraping and scanning Docker container images",
	Long:  longDesc,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		configFile, _ := cmd.Flags().GetString("config")
		if logLevel, ok := validLogLevel[logLevelStr]; ok {
			myutils.LoadConfigFromFile(configFile, logLevel)
			analyzer.DefaultAnalyzer, analyzer.DefaultAnalyzerE = analyzer.NewImageAnalyzerGlobalConfig()
		} else {
			log.Fatalln("invalid log_level:", logLevelStr)
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		// 仅用作测试
		for _, imgDigest := range []string{
			// "sha256:5d72b1eb9d3bc467affdf723a3c6adfe893ac291cc8d0bfeb5152ff44769568b",
			// "sha256:8fbeee158a9d9ddfaf118b54136ef696b86c9611847ad99abcb27f78a9bb8f86",
			// "sha256:811e890d0d7b76c0ffcc188905066633f7379c652a460738d2a9d0107e3a4c4f",
			// "sha256:1d48fd47fd497077ced19f0973e45a5da34ce936a99158680d64032028854fc0",
			// "sha256:7cd4e9005ca2273607275817151ccdba0bbd414c63bde8cda58ec14b4f2ee3f1",
			"sha256:3317fc0c3f0bb200d85ef9f3ce455dd045f675c5d0a4792178325cacbbd73169",
			// "sha256:9bfe813d9fdb7bca3f003a8bce3355eb04a2771ce5da6f9b89f018737627d58f",
			// "sha256:e4a2989262c72e4b3eaf7263dae716a548d73c9b4c7cd1718dc3ac96d21ebaf9",
			// "sha256:8089ad2b9b3284a48047f886e61a1b8475dca72227dc94e82f0f704c7aa09939",
			// "sha256:33670cf348538020d957149302af5c92e12b1976ab62bb549729c379905c5593",
		} {
			img, err := myutils.GlobalDBClient.Mongo.FindImageByDigest(imgDigest)
			if err != nil {
				fmt.Println("find image by digest:", imgDigest, "failed with:", err)
				continue
			}
			preID := ""
			for _, layer := range img.Layers {
				fmt.Println(layer.Digest, layer.Instruction)
				dig := ""

				if layer.Digest != "" {
					dig = myutils.Sha256Str(layer.Digest)
				} else {
					dig = myutils.Sha256Str(layer.Instruction)
				}
				if dig == "" {
					fmt.Printf("digest of image %s still none after calculating SHA256\n", imgDigest)
					break
				}

				preID = myutils.Sha256Str(preID + dig)
			}

			fmt.Println(imgDigest, ":", preID)
		}

	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		// 所有命令退出前的清理工作
		myutils.CloseAllConnections()
	},
}

var crawlCmd = &cobra.Command{
	Use:   "crawl",
	Short: "crawl metadata of repositories and images from specific Docker registry, now supports Docker Hub only",
}

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "read data from crawl results, store metadata to MongoDB, and build dependency graph with Neo4j",
	Run: func(cmd *cobra.Command, args []string) {
		format, _ := cmd.Flags().GetString("format")
		page, _ := cmd.Flags().GetInt64("page")
		pageSize, _ := cmd.Flags().GetInt("page_size")
		pullCountThreshold, _ := cmd.Flags().GetInt64("threshold")
		buildgraph.Build(format, page, pageSize, pullCountThreshold)
	},
}

var analyzeCmd = &cobra.Command{
	Use:   "analyze",
	Short: "analyze Docker image by name/file",
	Run: func(cmd *cobra.Command, args []string) {
		partial, _ := cmd.Flags().GetBool("partial")
		name, _ := cmd.Flags().GetString("name")
		file, _ := cmd.Flags().GetString("file")
		jsonFlag, _ := cmd.Flags().GetBool("json")
		output, _ := cmd.Flags().GetString("output")

		var res *myutils.ImageResult
		var err error

		if name != "" {
			if partial {
				res, err = analyzer.AnalyzeImagePartialByName(name)
			} else {
				res, err = analyzer.AnalyzeImageByName(name, false)
			}
		} else if file != "" {
			// AnalyzeImageByFile暂时未实现
		} else {
			log.Fatalf("invalid use of command: %s analyze, `%s analyze` to see help\n", RootCmd.Use, RootCmd.Use)
		}

		if err != nil {
			log.Fatalln("analyze image got error:", err)
		}

		if res != nil {
			if jsonFlag {
				jsonData, err := json.Marshal(res)
				if err != nil {
					log.Fatalln("json marshal analysis result got error:", err)
				}
				if output != "" {
					err = os.WriteFile(output, jsonData, 0664)
					if err != nil {
						log.Fatalln("json marshal analysis result got error:", err)
					}
				}
			}

			fmt.Println("analysis succeeded, result stored in file:", output)
		} else {
			log.Fatalln("something wrong during analyzing image got result: nil")
		}

		fmt.Println("analyze finish")
	},
}

var executeCmd = &cobra.Command{
	Use:   "execute",
	Short: "execute custom scripts",
	Run: func(cmd *cobra.Command, args []string) {
		script, _ := cmd.Flags().GetString("script")
		switch script {
		case "batch-analyze":
			file, _ := cmd.Flags().GetString("file")
			partial, _ := cmd.Flags().GetBool("partial")
			err := scripts.BatchAnalyzeByName(file, partial)
			if err != nil {
				log.Fatalln("batch-analyze file", file, "got error:", err)
			}
		case "analyze-threshold":
			threshold, _ := cmd.Flags().GetInt64("threshold")
			tagNum, _ := cmd.Flags().GetInt("tags")
			page, _ := cmd.Flags().GetInt64("page")
			err := scripts.AnalyzePullCountOverThreshold(threshold, tagNum, page)
			if err != nil {
				log.Fatalln("analyze-threshold got error:", err)
			}
		case "analyze-all":
			page, _ := cmd.Flags().GetInt64("page")
			pageSize, _ := cmd.Flags().GetInt64("page_size")
			tagCnt, _ := cmd.Flags().GetInt("tags")
			partial, _ := cmd.Flags().GetBool("partial")
			err := scripts.AnalyzeAll(page, pageSize, tagCnt, partial)
			if err != nil {
				log.Fatalln("analyze-all got error:", err)
			}
		}
	},
}

func init() {
	// RootCmd
	RootCmd.PersistentFlags().StringP("config", "c", "config.yaml", "path to config file")
	RootCmd.PersistentFlags().StringVarP(&logLevelStr, "log_level", "l", "debug", "log level: debug, info, warn, error, critical")

	// buildCmd
	buildCmd.Flags().String("format", "mongo", "format of the source data, including: json, mongo")
	buildCmd.Flags().Int64("page", 1, "start page for building from mongo")
	buildCmd.Flags().Int("page_size", 20, "page size of each tag metadata API for custom repo")
	buildCmd.Flags().Int64("threshold", 1000000, "threshold of pull_count for getting all tags from API")

	// analyzeCmd
	analyzeCmd.Flags().Bool("partial", false, "only analyze metadata of the Docker image")
	analyzeCmd.Flags().StringP("name", "n", "", "analyze Docker image by name")
	analyzeCmd.Flags().StringP("file", "f", "", "analyze Docker image by file")
	analyzeCmd.Flags().Bool("json", true, "output in JSON")
	analyzeCmd.Flags().StringP("output", "o", fmt.Sprintf("%s_result.json", myutils.GetLocalNowTimeNoSpace()), "analysis result output filepath")

	// executeCmd
	executeCmd.Flags().String("script", "", "execute custom script, including: batch-analyze, analyze-threshold, analyze-all")
	executeCmd.Flags().Bool("partial", false, "only analyze metadata of the Docker images")
	executeCmd.Flags().StringP("file", "f", "", "input file for scripts, like batch-analyze")
	executeCmd.Flags().Int64("threshold", 1000000, "pull_count threshold to analyze an image")
	executeCmd.Flags().Int("tags", 3, "the top tag-num recently updated tags to analyze")
	executeCmd.Flags().Int64("page", 1, "start page for analyzing multiple repos from MongoDB")
	executeCmd.Flags().Int64("page_size", 5, "page size of finding multiple repos from MongoDB")

	// 向root命令中注册命令
	RootCmd.AddCommand(
		crawlCmd,
		buildCmd,
		analyzeCmd,
		executeCmd,
	)
}

// Deprecated: 旧版main函数实现，转为使用cobra实现

//// 命令行参数定义与绑定
//var (
//	crawl       bool   // 是否要爬镜像仓库数据
//	registry    string // 指定要爬的镜像仓库，比如dockerhub
//	libraryFlag bool   // 爬虫是否爬官方镜像
//	buildGraph  bool   // 是否要建信息库
//	format      string // 爬虫存储格式/信息库从什么格式中取内容，json、mysql
//	startServer bool   // 启动前端服务器
//	execScript  bool   // 执行特制脚本
//	rulePath    string // filepath of rules
//	scan        bool   // 是否要扫描镜像
//	image       string // 待扫描镜像名称
//	file        string // 待扫描镜像文件
//)
//
//flag.BoolVar(&crawl, "crawl", false, "crawl images metadata if not nil")
//flag.StringVar(&registry, "registry", "dockerhub", "registry the register if not nil, e.g. dockerhub")
//flag.BoolVar(&libraryFlag, "official", false, "true for crawling official images; false for crawling community images")
//flag.BoolVar(&buildGraph, "build-graph", false, "true for building graph based on crawler results")
//flag.StringVar(&format, "format", "json", "format for crawling or building graph, e.g. json, mysql, clear")
//flag.BoolVar(&startServer, "start-server", false, "true for building graph based on crawler results")
//flag.BoolVar(&execScript, "exec-script", false, "true for specific script execution")
//flag.StringVar(&rulePath, "rule-path", "rules/secret_rules.yaml", "yaml file path storing rules")
//flag.BoolVar(&scan, "scan", false, "true for scanning image")
//flag.StringVar(&image, "image", "", "image name to be scanned, e.g. ")
//flag.StringVar(&file, "file", "", "image file to be scanned, formatted like file from `docker save`")
//flag.Parse()
//
//// 主函数退出前清理工作（最后一个执行的defer函数）
//defer myutils.CloseAllConnections()
//
//if crawl {
//if registry == "dockerhub" {
//crawler.StartRecursive(format, libraryFlag)
//}
//} else if buildGraph {
//buildgraph.Build(format)
//} else if startServer {
//// 10.10.21.122:23434
//server.StartServer()
//} else if scan {
//
//} else if execScript {
//scripts.ScanTop100DownstreamImagesVul()
////scripts.StatisticRepositoriesDependentWeights()
////scripts.ScanAllSecretsInImageMetadata()
////scripts.CalculateRepositoriesDependentWeights()
//} else {
//flag.Usage()
//os.Exit(-1)
//}
