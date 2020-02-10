package internal

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
)

type User struct {
	ID               uint    `form:"id" json:"id" xml:"id" gorm:"primary_key"`
	Username         string  `form:"Username" json:"Username" xml:"Username"  gorm:"type:varchar(200);unique_index"`
	Email            string  `form:"email" json:"email" xml:"email"`
	Password         string  `form:"password" json:"password" xml:"password"`
	Mac              *string `gorm:"-" form:"mac" json:"mac" xml:"mac"`
	RegistrationCode *string `form:"registrationCode" json:"registrationCode" xml:"registrationCode"`
	SystemName       string  `gorm:"-"  json:"systemName" `
}

type Query struct {
	Uuid       string `form:"uuid" json:"uuid" xml:"uuid"`
	Command    string `form:"command" json:"command" xml:"command"`
	Created    int64  `form:"created" json:"created" xml:"created"`
	Path       string `form:"path" json:"path" xml:"path"`
	ExitStatus int    `form:"exitStatus" json:"exitStatus" xml:"exitStatus"`
	Username   string `form:"username" json:"username" xml:"username"`
	SystemName string `gorm:"-"  json:"systemName"`
	SessionID string `form:"session_id" json:"session_id" xml:"session_id"`
}
type SystemQuery struct {
	ID            uint `form:"id" json:"id" xml:"id" gorm:"primary_key"`
	Created       int64
	Updated       int64
	Mac           string  `form:"mac" json:"mac" xml:"mac"`
	Hostname      *string `form:"hostname" json:"hostname" xml:"hostname"`
	Name          *string `form:"name" json:"name" xml:"name"`
	ClientVersion *string `form:"clientVersion" json:"clientVersion" xml:"clientVersion"`
}

type Command struct {
	ProcessId        int    `form:"processId" json:"processId" xml:"processId"`
	ProcessStartTime int64  `form:"processStartTime" json:"processStartTime" xml:"processStartTime"`
	Uuid             string `form:"uuid" json:"uuid" xml:"uuid"`
	Command          string `form:"command" json:"command" xml:"command"`
	Created          int64  `form:"created" json:"created" xml:"created"`
	Path             string `form:"path" json:"path" xml:"path"`
	SystemName       string `form:"systemName" json:"systemName" xml:"systemName"`
	ExitStatus       int    `form:"exitStatus" json:"exitStatus" xml:"exitStatus"`
	User             User   `gorm:"association_foreignkey:ID"`
	UserId           uint
	Limit            int    `gorm:"-"`
	Unique           bool   `gorm:"-"`
	Query            string `gorm:"-"`
}

// {"mac": "83779604164095", "hostname": "yay.local", "name": "yay.local", "clientVersion": "1.2.0"}
//{"name":"Home","mac":"83779604164095","userId":"5b5d53b6e4b02a6c4914bec8","hostname":"yay.local","clientVersion":"1.2.0","id":"5b5d53c8e4b02a6c4914bec9","created":1532842952382,"updated":1581032237766}
type System struct {
	ID            uint `form:"id" json:"id" xml:"id" gorm:"primary_key"`
	Created       int64
	Updated       int64
	Mac           *string `form:"mac" json:"mac" xml:"mac"`
	Hostname      *string `form:"hostname" json:"hostname" xml:"hostname"`
	Name          *string `form:"name" json:"name" xml:"name"`
	ClientVersion *string `form:"clientVersion" json:"clientVersion" xml:"clientVersion"`
	User          User    `gorm:"association_foreignkey:ID"`
	UserId        uint    `form:"userId" json:"userId" xml:"userId"`
}

var (
	Addr string
)

//TODO: Figure out a better way to do this.
const secret = "bashub-server-secret"

func Run() {

	DbInit()
	r := gin.Default()
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

		user.Password = hashAndSalt(user.Password)
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

	r.Run(Addr)
}
