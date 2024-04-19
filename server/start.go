package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func StartServer(port string) {
	configServer()

	router := gin.Default()

	router.Use(CORSMiddleWare())

	router.GET("/login", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"msg": "this is login page",
		})
	})
	router.POST("/login", handleUserLogin())

	router.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})
	})

	// 以下方法需要JWT鉴权中间件
	router.GET("/repositories", jwtAuthMiddleware(), handleRepositoriesSearch())
	router.GET("/tags", jwtAuthMiddleware(), handleTagsSearch())
	router.GET("/images", jwtAuthMiddleware(), handleImagesSearch())
	router.GET("/results", jwtAuthMiddleware(), handleResultsSearch())
	router.GET("/result", jwtAuthMiddleware(), handleResultSearch())

	router.Run(":" + port)
}

func CORSMiddleWare() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 设置允许的来源
		c.Header("Access-Control-Allow-Origin", "*")

		// 预检间隔时间为24小时
		c.Header("Access-Control-Max-Age", "86400")

		// 允许的请求方法
		// OPTIONS方法用于axios预检语句的处理
		c.Header("Access-Control-Allow-Methods", "POST, GET, OPTIONS")

		// 允许的头信息
		c.Header("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

		// 允许携带凭证（cookies）
		c.Header("Access-Control-Allow-Credentials", "true")

		// 判断请求是否为OPTIONS请求
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		// 继续处理请求
		c.Next()
	}
}
