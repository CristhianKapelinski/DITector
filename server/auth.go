package server

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
)

type Claims struct {
	Username string
	jwt.StandardClaims
}

// 生成JWT令牌
func generateToken(username string, expiredAfter time.Duration) (string, error) {
	// 设置JWT的有效期
	expirationTime := time.Now().Add(expiredAfter)

	// 使用用户名和过期时间创建一个新的JWT声明
	claims := &Claims{
		Username: username,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: expirationTime.Unix(), // 过期时间
			IssuedAt:  time.Now().Unix(),     // 签发时间
		},
	}

	// 使用HS256算法和自定义声明签名生成JWT令牌
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	jwtSecret := os.Getenv(JWTENV)
	signedToken, err := token.SignedString([]byte(jwtSecret)) // 这里的secret应该是一个安全的随机字符串，用来签名JWT令牌
	if err != nil {
		return "", err
	}

	return signedToken, nil
}

// JWT鉴权中间件
func jwtAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// c.Header("Access-Control-Allow-Origin", "*")
		// c.Header("Access-Control-Allow-Methods", "GET, POST")

		tokenString := c.GetHeader("Authorization") // 从请求Header中获取JWT令牌
		fmt.Println("got token:", tokenString)

		if tokenString == "" {
			c.JSON(http.StatusOK, gin.H{"code": 401, "msg": "empty token"})
			c.Abort()
			return
		}

		// 解析JWT令牌
		token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
			return []byte(os.Getenv(JWTENV)), nil // 这里的secret应该与生成JWT令牌时一致
		})

		if err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 401, "msg": err.Error()})
			c.Abort()
			return
		}

		if claims, ok := token.Claims.(*Claims); ok && token.Valid {
			// 如果JWT令牌合法，则将用户信息存储在请求的上下文中，以便后续处理函数使用
			c.Set("username", claims.Username)
			c.Next()
		} else {
			c.JSON(http.StatusOK, gin.H{"code": 401, "msg": "invalid token"})
			c.Abort()
			return
		}
	}
}
