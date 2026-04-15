package models

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	gorm.Model
	Email    string `gorm:"uniqueIndex;not null"`
	Password string // Hash
	Verified bool
}

type OTPCode struct {
	Email     string `gorm:"primaryKey"`
	Code      string
	ExpiresAt time.Time
}

type FileRecord struct {
	ID        string `gorm:"primaryKey"` // Pickup code
	UserID    uint
	Filename  string
	FilePath  string
	IsDir     bool
	IsArchive bool
	OneTime   bool
	ExpiresAt *time.Time
	CreatedAt time.Time
}
