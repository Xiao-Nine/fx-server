package main

import (
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Xiao-Nine/fx-server/models"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var db *gorm.DB

const (
	dbPath          = "data/fx.db"
	uploadsDir      = "data/uploads"
	defaultExpire   = 24 * time.Hour
	cleanupInterval = 15 * time.Minute
)

// Configurable max file size (default: 100MB)
var maxFileSize int64 = 100 * 1024 * 1024

func init() {
	// Read max file size from environment variable
	if sizeStr := os.Getenv("MAX_FILE_SIZE"); sizeStr != "" {
		if size, err := strconv.ParseInt(sizeStr, 10, 64); err == nil && size > 0 {
			maxFileSize = size
		} else {
			log.Printf("Warning: invalid MAX_FILE_SIZE value '%s', using default 100MB", sizeStr)
		}
	}
}

// Rate limiter for IP-based request limiting
type IPRateLimiter struct {
	mu       sync.RWMutex
	requests map[string]*IPRecord
	limit    int           // Max requests per IP per window
	window   time.Duration // Time window
}

type IPRecord struct {
	count     int
	expiresAt time.Time
}

func NewIPRateLimiter(limit int, window time.Duration) *IPRateLimiter {
	limiter := &IPRateLimiter{
		requests: make(map[string]*IPRecord),
		limit:    limit,
		window:   window,
	}
	// Start cleanup goroutine
	go limiter.cleanupExpiredRecords()
	return limiter
}

func (rl *IPRateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	record, exists := rl.requests[ip]

	if !exists || now.After(record.expiresAt) {
		// New IP or expired record - start fresh
		rl.requests[ip] = &IPRecord{
			count:     1,
			expiresAt: now.Add(rl.window),
		}
		return true
	}

	// Existing record within window
	if record.count >= rl.limit {
		return false
	}

	record.count++
	return true
}

func (rl *IPRateLimiter) cleanupExpiredRecords() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for ip, record := range rl.requests {
			if now.After(record.expiresAt) {
				delete(rl.requests, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// Global rate limiter (5 requests per minute per IP)
var rateLimiter = NewIPRateLimiter(5, 1*time.Minute)

// Rate limit middleware
func rateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()

		if !rateLimiter.Allow(ip) {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "rate limit exceeded: maximum 5 requests per minute per IP",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

func main() {
	if err := os.MkdirAll("data", 0o755); err != nil {
		panic(err)
	}
	if err := os.MkdirAll(uploadsDir, 0o755); err != nil {
		panic(err)
	}

	var err error
	db, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}

	db.AutoMigrate(&models.FileRecord{})

	if err := cleanupExpiredFiles(db, time.Now()); err != nil {
		log.Printf("cleanup: %v", err)
	}
	go startCleanupTicker(db)

	r := gin.Default()

	// Apply rate limiting middleware to API routes
	api := r.Group("/api")
	api.Use(rateLimitMiddleware())
	{
		files := api.Group("/files")
		{
			files.POST("/upload", uploadFile)
			files.GET("/download/:code", downloadFile)
		}
	}

	r.Run(":8080")
}

func startCleanupTicker(db *gorm.DB) {
	t := time.NewTicker(cleanupInterval)
	defer t.Stop()

	for now := range t.C {
		if err := cleanupExpiredFiles(db, now); err != nil {
			log.Printf("cleanup: %v", err)
		}
	}
}

func cleanupExpiredFiles(db *gorm.DB, now time.Time) error {
	var expired []models.FileRecord
	if err := db.Where("expires_at IS NOT NULL AND expires_at <= ?", now).Find(&expired).Error; err != nil {
		return err
	}

	for _, rec := range expired {
		if rec.FilePath != "" {
			_ = os.Remove(rec.FilePath)
		}
		_ = db.Delete(&models.FileRecord{}, "id = ?", rec.ID).Error
	}

	return nil
}

func uploadFile(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing form file: file"})
		return
	}

	// Check file size
	if file.Size > maxFileSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("file size exceeds maximum allowed size (%d bytes)", maxFileSize)})
		return
	}

	oneTime := parseBool(c.PostForm("one_time")) || parseBool(c.PostForm("oneTime"))
	neverExpire := parseBool(c.PostForm("never_expire")) || parseBool(c.PostForm("neverExpire"))

	expireStr := strings.TrimSpace(c.PostForm("expire"))
	var expiresAt *time.Time
	if !neverExpire {
		d := defaultExpire
		if expireStr != "" {
			parsed, err := time.ParseDuration(expireStr)
			if err != nil || parsed <= 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid expire (use Go duration like 24h, 30m)"})
				return
			}
			d = parsed
		}

		t := time.Now().Add(d)
		expiresAt = &t
	}

	code, err := generateUniqueCode(db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate pickup code"})
		return
	}

	originalName := sanitizeFilename(file.Filename)
	if originalName == "" {
		originalName = "file"
	}

	storedName := code + "_" + originalName
	storedPath := filepath.Join(uploadsDir, storedName)

	if err := c.SaveUploadedFile(file, storedPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save upload"})
		return
	}

	rec := models.FileRecord{
		ID:        code,
		Filename:  originalName,
		FilePath:  storedPath,
		OneTime:   oneTime,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now(),
	}

	if err := db.Create(&rec).Error; err != nil {
		_ = os.Remove(storedPath)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to record upload"})
		return
	}

	var expiresAtStr any
	if rec.ExpiresAt != nil {
		expiresAtStr = rec.ExpiresAt.Format(time.RFC3339)
	} else {
		expiresAtStr = nil
	}

	c.JSON(http.StatusOK, gin.H{
		"code":      rec.ID,
		"filename":  rec.Filename,
		"expiresAt": expiresAtStr,
		"oneTime":   rec.OneTime,
		"msg":       "Upload success",
	})
}

func downloadFile(c *gin.Context) {
	code := c.Param("code")

	var rec models.FileRecord
	if err := db.First(&rec, "id = ?", code).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "code not found"})
		return
	}

	if rec.ExpiresAt != nil && time.Now().After(*rec.ExpiresAt) {
		if rec.FilePath != "" {
			_ = os.Remove(rec.FilePath)
		}
		_ = db.Delete(&models.FileRecord{}, "id = ?", rec.ID).Error
		c.JSON(http.StatusGone, gin.H{"error": "file expired"})
		return
	}

	if rec.FilePath == "" {
		_ = db.Delete(&models.FileRecord{}, "id = ?", rec.ID).Error
		c.JSON(http.StatusNotFound, gin.H{"error": "file missing"})
		return
	}
	if _, err := os.Stat(rec.FilePath); err != nil {
		_ = db.Delete(&models.FileRecord{}, "id = ?", rec.ID).Error
		c.JSON(http.StatusNotFound, gin.H{"error": "file missing"})
		return
	}

	c.FileAttachment(rec.FilePath, rec.Filename)

	if rec.OneTime {
		_ = os.Remove(rec.FilePath)
		_ = db.Delete(&models.FileRecord{}, "id = ?", rec.ID).Error
	}
}

func generateUniqueCode(db *gorm.DB) (string, error) {
	// Try up to 100 times to generate a unique 6-digit code
	for i := 0; i < 100; i++ {
		// Generate 6-digit number (100000-999999)
		code := fmt.Sprintf("%06d", rand.Intn(900000)+100000)

		// Check if code already exists
		var count int64
		if err := db.Model(&models.FileRecord{}).Where("id = ?", code).Count(&count).Error; err != nil {
			return "", err
		}
		if count == 0 {
			return code, nil
		}
	}

	return "", errors.New("unable to generate unique pickup code after 100 attempts")
}

func parseBool(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "1", "true", "t", "yes", "y", "on":
		return true
	default:
		return false
	}
}

func sanitizeFilename(name string) string {
	name = strings.TrimSpace(name)
	name = filepath.Base(name)
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	return name
}
