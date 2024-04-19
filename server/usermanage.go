package server

import (
	"fmt"
	"net/http"
	"time"

	"github.com/Musso12138/docker-scan/myutils"
	"github.com/gin-gonic/gin"
)

type User struct {
	Username string `form:"username" binding:"required"`
	Password string `form:"password" binding:"required"`
}

func handleUserLogin() func(c *gin.Context) {
	return func(c *gin.Context) {
		// 验证用户名密码
		var u User
		err := c.ShouldBind(&u)
		if err != nil {
			// 这里返回200然后在body内响应500是合理的，把系统错误和业务错误区分开
			c.JSON(http.StatusOK, gin.H{"code": 500, "msg": err.Error()})
			return
		}
		fmt.Println("received username:", u.Username, ", password:", u.Password)

		backU, err := myutils.GlobalDBClient.Mongo.FindUserByKeyword(map[string]any{"username": u.Username})
		if err != nil {
			fmt.Println("err:", err)
			c.JSON(http.StatusOK, gin.H{"code": 401, "msg": "user not found"})
			return
		}

		hashedPass := myutils.Sha256Str(u.Password)
		if hashedPass != backU.Password {
			c.JSON(http.StatusOK, gin.H{"code": 401, "msg": "wrong password"})
			return
		}

		// 到这用户名密码都对了，生成token以及JWT响应给客户端
		jwtToken, err := generateToken(u.Username, 1*time.Hour)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 500, "msg": err.Error()})
			return
		}

		err = myutils.GlobalDBClient.Mongo.UpdateUserLogin(map[string]any{"username": u.Username}, myutils.GetLocalNowTime())
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 500, "msg": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "login success", "token": jwtToken})
	}
}
