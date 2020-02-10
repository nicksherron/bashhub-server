/*
 *
 * Copyright © 2020 nicksherron <nsherron90@gmail.com>
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package internal

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
)

type User struct {
	ID               uint    `json:"id" gorm:"primary_key"`
	Username         string  `json:"Username" gorm:"type:varchar(200);unique_index"`
	Email            string  `json:"email"`
	Password         string  `json:"password"`
	Mac              *string `json:"mac" gorm:"-"`
	RegistrationCode *string `json:"registrationCode"`
	SystemName       string  `json:"systemName" gorm:"-"`
}

type Query struct {
	Uuid       string `json:"uuid"`
	Command    string `json:"command"`
	Created    int64  `json:"created"`
	Path       string `json:"path"`
	ExitStatus int    `json:"exitStatus"`
	Username   string `json:"username"`
	SystemName string `gorm:"-"  json:"systemName"`
	//TODO: implement sessions
	SessionID string `json:"session_id"`
}

type Command struct {
	ProcessId        int    `json:"processId"`
	ProcessStartTime int64  `json:"processStartTime"`
	Uuid             string `json:"uuid"`
	Command          string `json:"command"`
	Created          int64  `json:"created"`
	Path             string `json:"path"`
	SystemName       string `json:"systemName"`
	ExitStatus       int    `json:"exitStatus"`
	User             User   `gorm:"association_foreignkey:ID"`
	UserId           uint
	Limit            int    `gorm:"-"`
	Unique           bool   `gorm:"-"`
	Query            string `gorm:"-"`
}

type System struct {
	ID            uint `json:"id" gorm:"primary_key"`
	Created       int64
	Updated       int64
	Mac           string  `json:"mac" gorm:"default:null"`
	Hostname      *string `json:"hostname"`
	Name          *string `json:"name"`
	ClientVersion *string `json:"clientVersion"`
	User          User    `gorm:"association_foreignkey:ID"`
	UserId        uint    `json:"userId"`
}

var (
	// Addr is the listen and server address for our server (gin)
	Addr string
	// LogFile is the log file location for http logging. Default is stderr.
	LogFile string
)

//TODO: Figure out a better way to do this.
const secret = "bashub-server-secret"

func getLog() *os.File {

	if LogFile != "" {
		f, err := os.Create(LogFile)
		if err != nil {
			log.Fatal(err)
		}
		return f
	}
	return os.Stderr
}

// LoggerWithFormatter instance a Logger middleware with the specified log format function.
func loggerWithFormatterWriter(f gin.LogFormatter) gin.HandlerFunc {
	return gin.LoggerWithConfig(gin.LoggerConfig{
		Formatter: f,
		Output:    getLog(),
	})
}

// Run starts server
func Run() {
	// Initialize backend
	dbInit()

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	r.Use(loggerWithFormatterWriter(func(param gin.LogFormatterParams) string {
		return fmt.Sprintf("[BASHHUB-SERVER] %v | %3d | %13v | %15s | %-7s  %s\n",
			param.TimeStamp.Format("2006/01/02 - 15:04:05"),
			param.StatusCode,
			param.Latency,
			param.ClientIP,
			param.Method,
			param.Path,
		)
	}))

	// the jwt middleware
	authMiddleware, err := jwt.New(&jwt.GinJWTMiddleware{
		Realm:       "bashhub-server zone",
		Key:         []byte(secret),
		Timeout:     10000 * time.Hour,
		MaxRefresh:  10000 * time.Hour,
		IdentityKey: "username",
		LoginResponse: func(c *gin.Context, code int, token string, expire time.Time) {
			c.JSON(http.StatusOK, gin.H{
				"accessToken": token,
			})
		},
		PayloadFunc: func(data interface{}) jwt.MapClaims {
			if v, ok := data.(*User); ok {
				return jwt.MapClaims{
					"username":   v.Username,
					"systemName": v.SystemName,
				}
			}
			return jwt.MapClaims{}
		},
		IdentityHandler: func(c *gin.Context) interface{} {
			claims := jwt.ExtractClaims(c)
			return &User{
				Username:   claims["username"].(string),
				SystemName: claims["systemName"].(string),
			}
		},
		Authenticator: func(c *gin.Context) (interface{}, error) {
			var user User

			if err := c.ShouldBind(&user); err != nil {
				return "", jwt.ErrMissingLoginValues
			}
			if user.userExists() {
				return &User{
					Username:   user.Username,
					SystemName: user.userGetSystemName(),
				}, nil
			}
			fmt.Println("failed")

			return nil, jwt.ErrFailedAuthentication
		},
		Authorizator: func(data interface{}, c *gin.Context) bool {
			if v, ok := data.(*User); ok && v.usernameExists() {
				return true
			}
			return false
		},
		Unauthorized: func(c *gin.Context, code int, message string) {
			c.JSON(code, gin.H{
				"code":    code,
				"message": message,
			})
		},
		TokenLookup:   "header: Authorization, query: token, cookie: jwt",
		TokenHeadName: "Bearer",
		TimeFunc:      time.Now,
	})

	if err != nil {
		log.Fatal("JWT Error:" + err.Error())
	}

	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})

	r.POST("/api/v1/login", authMiddleware.LoginHandler)

	r.POST("/api/v1/user", func(c *gin.Context) {
		var user User
		if err := c.ShouldBindJSON(&user); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if user.Email == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "email required"})
			return
		}
		if user.usernameExists() {
			c.String(409, "Username already taken")
			return
		}
		if user.emailExists() {
			c.String(409, "This email address is already registered.")
			return
		}
		user.userCreate()

	})

	r.Use(authMiddleware.MiddlewareFunc())

	r.GET("/api/v1/command/:path", func(c *gin.Context) {
		var command Command
		var user User
		claims := jwt.ExtractClaims(c)
		user.Username = claims["username"].(string)
		command.User.ID = user.userGetId()

		if c.Param("path") == "search" {
			command.Limit = 100
			if c.Query("limit") != "" {
				if num, err := strconv.Atoi(c.Query("limit")); err != nil {
					command.Limit = 100
				} else {
					command.Limit = num
				}
			}
			if c.Query("unique") == "true" {
				command.Unique = true
			} else {
				command.Unique = false
			}
			command.Path = c.Query("path")
			command.Query = c.Query("query")
			command.SystemName = c.Query("systemName")

			result := command.commandGet()
			if len(result) == 0 {
				c.JSON(http.StatusOK, gin.H{})
				return
			}
			c.IndentedJSON(http.StatusOK, result)
		} else {
			command.Uuid = c.Param("path")
			command.User.ID = user.userGetId()
			result := command.commandGetUUID()
			result.Username = user.Username
			c.IndentedJSON(http.StatusOK, result)
		}

	})

	r.POST("/api/v1/command", func(c *gin.Context) {
		var command Command
		if err := c.ShouldBindJSON(&command); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		var user User
		claims := jwt.ExtractClaims(c)
		user.Username = claims["username"].(string)
		command.User.ID = user.userGetId()
		command.SystemName = claims["systemName"].(string)
		command.commandInsert()
	})

	r.POST("/api/v1/system", func(c *gin.Context) {
		var system System
		var user User
		err := c.Bind(&system)
		if err != nil {
			log.Fatal(err)
		}

		claims := jwt.ExtractClaims(c)
		user.Username = claims["username"].(string)
		system.User.ID = user.userGetId()

		system.systemInsert()
		c.AbortWithStatus(201)
	})

	r.GET("/api/v1/system", func(c *gin.Context) {
		var system System
		var user User
		claims := jwt.ExtractClaims(c)
		mac := c.Query("mac")
		if mac == "" {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		user.Username = claims["username"].(string)
		system.User.ID = user.userGetId()
		result := system.systemGet()
		if len(result.Mac) == 0 {
			c.AbortWithStatus(404)
			return
		}
		c.IndentedJSON(http.StatusOK, result)

	})

	err = r.Run(Addr)
	if err != nil {
		fmt.Println("Error: \t", err)
	}
}
