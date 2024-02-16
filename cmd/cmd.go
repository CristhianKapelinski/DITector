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
		myutils.Logger.Info("start test")
		tags, err := myutils.GlobalDBClient.Mongo.FindAllTagsByRepoName("library", "mongo")
		if err != nil {
			log.Fatalln("获取全部tag失败:", err)
		}
		fmt.Println(len(tags))
		for _, imgDigest := range []string{
			"sha256:fc023e8dd56e83da64030ca599ecd1fbeea8f6c8f3c4d91d00f54d0d6318c575",
		} {
			img, err := myutils.GlobalDBClient.Mongo.FindImageByDigest(imgDigest)
			if err != nil {
				fmt.Println("find image by digest:", imgDigest, "failed with:", err)
				continue
			}
			preID := ""
			for _, layer := range img.Layers {
				// fmt.Println(layer.Digest, layer.Instruction)
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
			node, err := myutils.GlobalDBClient.Neo4j.FindLayerByNodeId(preID)
			if err != nil {
				log.Fatalf("find Layer node with id: %s, failed with: %s\n", preID, err)
			}
			fmt.Println(myutils.GetNodeProps(node))
			fmt.Println(myutils.GlobalDBClient.Neo4j.FindUpstreamImagesByNodeId(preID))
		}
		myutils.Logger.Info("finish test")
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

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "start backend server",
	Run: func(cmd *cobra.Command, args []string) {
		port, _ := cmd.Flags().GetString("port")
		// server.StartServer(port)
		fmt.Println(port)
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
		case "calculate-node-weights":
			file, _ := cmd.Flags().GetString("file")
			page, _ := cmd.Flags().GetInt64("page")
			pageSize, _ := cmd.Flags().GetInt64("page_size")
			threshold, _ := cmd.Flags().GetInt64("threshold")
			err := scripts.CalculateNodeRelyWeights(file, page, int(pageSize), threshold)
			if err != nil {
				log.Fatalln("calculate-node-weights got error:", err)
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

	// startCmd
	startCmd.Flags().StringP("port", "p", "23434", "port listening by backend server")

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
		startCmd,
		executeCmd,
	)
}
