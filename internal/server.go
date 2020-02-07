package internal

import (
	"crypto/sha256"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	jwt_lib "github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
)

type User struct {
	ID               uint    `gorm:"primary_key"`
	Username         string  `form:"Username" json:"Username" xml:"Username"  gorm:"type:varchar(200);unique_index"`
	Email            string  `form:"email" json:"email" xml:"email"`
	Password         string  `form:"password" json:"password" xml:"password"`
	Mac              *string `form:"mac" json:"mac" xml:"mac"`
	RegistrationCode *string `form:"registrationCode" json:"registrationCode" xml:"registrationCode"`
	Token            string
}

type Query struct {
	Uuid    string `form:"uuid" json:"uuid" xml:"uuid"`
	Command string `form:"command" json:"command" xml:"command"`
	Created int64  `form:"created" json:"created" xml:"created"`
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
	ExitStatus       int    `form:"exitStatus" json:"exitStatus" xml:"exitStatus"`
	User             User   `gorm:"association_foreignkey:ID"`
	UserId           uint
	Token            string `gorm:"-"`
	Limit            int    `gorm:"-"`
	Unique           bool   `gorm:"-"`
}

// {"mac": "83779604164095", "hostname": "yay.local", "name": "yay.local", "clientVersion": "1.2.0"}
//{"name":"Home","mac":"83779604164095","userId":"5b5d53b6e4b02a6c4914bec8","hostname":"yay.local","clientVersion":"1.2.0","id":"5b5d53c8e4b02a6c4914bec9","created":1532842952382,"updated":1581032237766}
type System struct {
	ID            uint `form:"id" json:"id" xml:"id" gorm:"primary_key"`
	Created       int64
	Updated       int64
	Mac           string  `form:"mac" json:"mac" xml:"mac"`
	Hostname      *string `form:"hostname" json:"hostname" xml:"hostname"`
	Name          *string `form:"name" json:"name" xml:"name"`
	ClientVersion *string `form:"clientVersion" json:"clientVersion" xml:"clientVersion"`
	User          User    `gorm:"association_foreignkey:ID"`
	UserId        uint    `form:"userId" json:"userId" xml:"userId"`
	Token         string  `gorm:"-"`
}

func auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		var user User
		err := func() error {
			user.Token = strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
			if user.tokenExists() {
				return nil
			} else {
				return fmt.Errorf("token doesn't exist")
			}
		}()
		if err != nil {
			c.AbortWithError(401, err)
		}
	}
}

func Run() {

	DbInit()

	r := gin.Default()

	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})

	r.POST("/api/v1/login", func(c *gin.Context) {
		var user User
		if err := c.ShouldBindJSON(&user); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		user.Password = fmt.Sprintf("%v", sha256.Sum256([]byte(user.Password)))

		if !user.userExists() {
			c.String(401, "Bad credentials")
			return
		}

		token := jwt_lib.New(jwt_lib.GetSigningMethod("HS256"))

		token.Claims = jwt_lib.MapClaims{
			"Id":  user.Username,
			"exp": time.Now().Add(time.Hour * 20000).Unix(),
		}
		// Sign and get the complete encoded token as a string
		tokenString, err := token.SignedString([]byte(user.Password))
		if err != nil {
			c.JSON(500, gin.H{"message": "Could not generate token"})
		}
		user.Token = tokenString
		user.updateToken()

		c.JSON(200, gin.H{"accessToken": tokenString})

	})

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

		user.Password = fmt.Sprintf("%v", sha256.Sum256([]byte(user.Password)))

		user.userCreate()

	})
	r.Use(auth())
	r.GET("/api/v1/command/search", func(c *gin.Context) {
		var command Command
		command.Token = strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
		command.Limit = 100
		if c.Query("limit") != "" {
			if num, err := strconv.Atoi(c.Query("limit")); err != nil {
				command.Limit = num
			}
		}
		if c.Query("unique") == "true" {
			command.Unique = true
		}else {
			command.Unique = false
		}
		result := command.commandGet()
		c.IndentedJSON(http.StatusOK, result)

	})

	r.POST("/api/v1/command", func(c *gin.Context) {
		var command Command
		if err := c.ShouldBindJSON(&command); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		command.Token = strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
		command.commandInsert()
	})

	r.POST("/api/v1/system", func(c *gin.Context) {
		var system System
		err := c.Bind(&system)
		if err != nil {
			log.Fatal(err)
		}
		system.Token = strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
		system.systemInsert()
		c.AbortWithStatus(201)
	})

	r.GET("/api/v1/system", func(c *gin.Context) {
		var system System
		system.Token = strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
		system.Mac = c.Query("mac")
		if system.Mac == "" {
			c.AbortWithStatus(http.StatusBadRequest)
			return

		}
		result, err := system.systemGet()
		if err != nil {
			c.AbortWithStatus(404)
			return
		}
		c.IndentedJSON(http.StatusOK, result)

	})
	r.Run()

}
