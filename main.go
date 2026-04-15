package main

import (
	"crypto/rand"
	"encoding/hex"

	"fx-server/models"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var db *gorm.DB

func main() {
	var err error
	db, err = gorm.Open(sqlite.Open("data/fx.db"), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}

	db.AutoMigrate(&models.User{}, &models.OTPCode{}, &models.FileRecord{})

	r := gin.Default()

	api := r.Group("/api")
	{
		auth := api.Group("/auth")
		{
			auth.POST("/register", register)
			auth.POST("/verify", verify)
			auth.POST("/login", login)
		}

		files := api.Group("/files") // Add auth middleware for real usage
		{
			files.POST("/upload", uploadFile)
			files.GET("/download/:code", downloadFile)
		}
	}

	r.Run(":8080")
}

func register(c *gin.Context) {
	// 简单实现
	c.JSON(200, gin.H{"msg": "Please check your email for OTP."})
}

func verify(c *gin.Context) {
	c.JSON(200, gin.H{"msg": "Verify success."})
}

func login(c *gin.Context) {
	c.JSON(200, gin.H{"token": "fake-jwt-token"})
}

func uploadFile(c *gin.Context) {
	b := make([]byte, 4)
	rand.Read(b)
	code := hex.EncodeToString(b)
	c.JSON(200, gin.H{"code": code, "msg": "Upload success"})
}

func downloadFile(c *gin.Context) {
	c.String(200, "file content here based on pick-up code")
}
