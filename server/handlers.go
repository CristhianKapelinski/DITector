package server

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/Musso12138/docker-scan/myutils"
	"github.com/gin-gonic/gin"
)

func handleRepositoriesSearch() func(c *gin.Context) {
	return func(c *gin.Context) {
		// c.Header("Access-Control-Allow-Origin", "*")
		// c.Header("Access-Control-Allow-Methods", "GET, POST")

		repoNamespace := c.DefaultQuery("repo_namespace", "")
		repoName := c.DefaultQuery("repo_name", "")
		pageStr := c.DefaultQuery("page", "1")
		pageSizeStr := c.DefaultQuery("page_size", "10")

		page, err := strconv.ParseInt(pageStr, 10, 64)
		if err != nil || page < 1 {
			page = 1
		}

		pageSize, err := strconv.ParseInt(pageSizeStr, 10, 64)
		if err != nil || pageSize < 1 {
			pageSize = 10
		}

		var totalCnt int64
		var results []*myutils.Repository

		// search允许是带/的名称
		if repoNamespace == "" && repoName == "" {
			// 使用stats获取集合元素数量
			totalCnt = totalRepoCnt
			results, err = myutils.GlobalDBClient.Mongo.FindRepositoriesByKeywordPaged(map[string]any{}, page, pageSize)
		} else {
			// 没有/的时候通过$text匹配
			keyMap := map[string]any{}
			if repoNamespace != "" {
				keyMap["namespace"] = repoNamespace
			}
			if repoName != "" {
				keyMap["name"] = repoName
			}
			totalCnt, _ = myutils.GlobalDBClient.Mongo.CountRepoByKeyword(keyMap)
			results, err = myutils.GlobalDBClient.Mongo.FindRepositoriesByKeywordPaged(keyMap, page, pageSize)
		}

		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code": 404,
				"msg":  err.Error(),
			})
		} else {
			c.JSON(http.StatusOK, gin.H{
				"code":      200,
				"count":     totalCnt,
				"page":      page,
				"page_size": pageSize,
				"results":   results,
			})
		}
	}
}

func handleTagsSearch() func(c *gin.Context) {
	return func(c *gin.Context) {
		// c.Header("Access-Control-Allow-Origin", "*")
		// c.Header("Access-Control-Allow-Methods", "GET, POST")

		repoNamespace := c.DefaultQuery("repo_namespace", "")
		repoName := c.DefaultQuery("repo_name", "")
		tagName := c.DefaultQuery("tag_name", "")
		imgDigest := c.DefaultQuery("digest", "")
		pageStr := c.DefaultQuery("page", "1")
		pageSizeStr := c.DefaultQuery("page_size", "10")

		page, err := strconv.ParseInt(pageStr, 10, 64)
		if err != nil || page < 1 {
			page = 1
		}

		pageSize, err := strconv.ParseInt(pageSizeStr, 10, 64)
		if err != nil || pageSize < 1 {
			pageSize = 10
		}

		var totalCnt int64
		var results []*myutils.Tag

		// search允许是带/的名称
		if repoNamespace == "" && repoName == "" && tagName == "" && imgDigest == "" {
			// 使用stats获取集合元素数量
			totalCnt = totalTagCnt
			results, err = myutils.GlobalDBClient.Mongo.FindTagByKeywordPaged(map[string]any{}, page, pageSize)
		} else {
			if imgDigest == "" {
				keyMap := make(map[string]any)
				if repoNamespace != "" {
					keyMap["repositories_namespace"] = repoNamespace
				}
				if repoName != "" {
					keyMap["repositories_name"] = repoName
				}
				if tagName != "" {
					keyMap["name"] = tagName
				}
				totalCnt, _ = myutils.GlobalDBClient.Mongo.CountTagByKeyword(keyMap)
				results, _ = myutils.GlobalDBClient.Mongo.FindTagByKeywordPaged(keyMap, page, pageSize)
			} else {
				totalCnt, _ = myutils.GlobalDBClient.Mongo.CountTagByImgDigest(imgDigest)
				results, err = myutils.GlobalDBClient.Mongo.FindTagByImgDigestPaged(imgDigest, page, pageSize)
			}
		}

		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code": 404,
				"msg":  err.Error(),
			})
		} else {
			// used to handle CORS requests
			c.JSON(http.StatusOK, gin.H{
				"code":      200,
				"count":     totalCnt,
				"page":      page,
				"page_size": pageSize,
				"results":   results,
			})
		}
	}
}

func handleImagesSearch() func(c *gin.Context) {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST")

		search := c.DefaultQuery("search", "")
		pageStr := c.DefaultQuery("page", "1")
		pageSizeStr := c.DefaultQuery("page_size", "10")

		page, err := strconv.ParseInt(pageStr, 10, 64)
		if err != nil || page < 1 {
			page = 1
		}

		pageSize, err := strconv.ParseInt(pageSizeStr, 10, 64)
		if err != nil || pageSize < 1 {
			pageSize = 10
		}

		var totalCnt int64
		var results []*myutils.Image

		if search == "" {
			// 使用stats获取集合元素数量
			totalCnt = totalImgCnt
			results, err = myutils.GlobalDBClient.Mongo.FindImageByKeywordPaged(map[string]any{}, page, pageSize)
		} else {
			if len(search) != 71 || !strings.HasPrefix(search, "sha256:") {
				c.JSON(http.StatusOK, gin.H{
					"code": 400,
					"msg":  "invalid input string for search, need to be a valid digest start with sha256:",
				})
				return
			} else {
				totalCnt, _ = myutils.GlobalDBClient.Mongo.CountImageByKeyword(map[string]any{
					"digest": search,
				})
				results, err = myutils.GlobalDBClient.Mongo.FindImageByKeywordPaged(map[string]any{
					"digest": search,
				}, page, pageSize)
			}
		}

		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code": 404,
				"msg":  err.Error(),
			})
		} else {
			c.JSON(http.StatusOK, gin.H{
				"code":      200,
				"count":     totalCnt,
				"page":      page,
				"page_size": pageSize,
				"results":   results,
			})
		}
	}
}

func handleResultsSearch() func(c *gin.Context) {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST")

		repoNamespace := c.DefaultQuery("repo_namespace", "")
		repoName := c.DefaultQuery("repo_name", "")
		tagName := c.DefaultQuery("tag_name", "")
		imgDigest := c.DefaultQuery("digest", "")
		search := c.DefaultQuery("search", "")
		pageStr := c.DefaultQuery("page", "1")
		pageSizeStr := c.DefaultQuery("page_size", "10")

		page, err := strconv.ParseInt(pageStr, 10, 64)
		if err != nil || page < 1 {
			page = 1
		}

		pageSize, err := strconv.ParseInt(pageSizeStr, 10, 64)
		if err != nil || pageSize < 1 {
			pageSize = 10
		}

		var totalCnt int64
		var results []*myutils.ImageResult

		// 有search就用text查攻击向量，没有就根据repo查找
		if search == "" {
			totalCnt, _ = myutils.GlobalDBClient.Mongo.CountImgResultsByName(repoNamespace, repoName, tagName, imgDigest)
			results, err = myutils.GlobalDBClient.Mongo.FindImgResultsByNamePaged(repoNamespace, repoName, tagName, imgDigest, page, pageSize)
		} else {
			totalCnt, _ = myutils.GlobalDBClient.Mongo.CountImgResByText(search)
			results, err = myutils.GlobalDBClient.Mongo.FindImgResultByTextPaged(search, page, pageSize)
		}

		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code": 404,
				"msg":  err.Error(),
			})
		} else {
			c.JSON(http.StatusOK, gin.H{
				"code":      200,
				"count":     totalCnt,
				"page":      page,
				"page_size": pageSize,
				"results":   results,
			})
		}
	}
}

func handleResultSearch() func(c *gin.Context) {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST")

		repoNamespace := c.DefaultQuery("repo_namespace", "")
		repoName := c.DefaultQuery("repo_name", "")
		tagName := c.DefaultQuery("tag_name", "")
		imgDigest := c.DefaultQuery("digest", "")

		result, err := myutils.GlobalDBClient.Mongo.FindImgResultByName(repoNamespace, repoName, tagName, imgDigest)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code": 400,
				"msg":  err.Error(),
			})
		} else {
			c.JSON(http.StatusOK, gin.H{
				"code":    200,
				"results": result,
			})
		}
	}
}
