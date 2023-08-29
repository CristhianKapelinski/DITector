package server

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
)

// handleRepositoriesSearch return a function used for
// repositories searching API exported by gin framework
//
// URI arguments:
//
// search: keyword for searching repositories from MongoDB,
//
//	now search according to name > namespace > description > full_description
//
// page: current page of the view
//
// page_size: page size of the view
func handleRepositoriesSearch() func(c *gin.Context) {
	return func(c *gin.Context) {
		search := c.DefaultQuery("search", "")
		pageStr := c.DefaultQuery("page", "1")
		pageSizeStr := c.DefaultQuery("page_size", "10")

		page, err := strconv.Atoi(pageStr)
		if err != nil || page < 1 {
			page = 1
		}
		pageSize, err := strconv.Atoi(pageSizeStr)
		if err != nil || pageSize < 1 {
			pageSize = 10
		}

		results, err := myMongo.FindRepositoriesByText(search, int64(page), int64(pageSize))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"msg": err.Error(),
			})
		} else {
			// used to handle CORS requests
			c.Header("Access-Control-Allow-Origin", "*")
			c.JSON(http.StatusOK, gin.H{
				"page":      page,
				"page_size": pageSize,
				"results":   results,
			})
		}
	}
}

// handleImageSearch return a function used for images
// searching API exported by gin framework
//
// URI arguments:
// search: keyword for searching images from MongoDB,
//
//	now only searching according to digest
//
// page: current page of the view
//
// page_size: page size of the view
func handleImageSearch() func(c *gin.Context) {
	return func(c *gin.Context) {
		search := c.DefaultQuery("search", "")
		pageStr := c.DefaultQuery("page", "1")
		pageSizeStr := c.DefaultQuery("page_size", "10")

		page, err := strconv.Atoi(pageStr)
		if err != nil || page < 1 {
			page = 1
		}

		pageSize, err := strconv.Atoi(pageSizeStr)
		if err != nil || pageSize < 1 {
			pageSize = 10
		}

		results, err := myMongo.FindImagesByText(search, int64(page), int64(pageSize))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"msg": err.Error(),
			})
		} else {
			// used to handle CORS requests
			c.Header("Access-Control-Allow-Origin", "*")
			c.JSON(http.StatusOK, gin.H{
				"page":      page,
				"page_size": pageSize,
				"results":   results,
			})
		}
	}
}
