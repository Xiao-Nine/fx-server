package models

import "time"

type FileRecord struct {
	ID        string `gorm:"primaryKey"` // Pickup code (6-digit number: 100000-999999)
	Filename  string
	FilePath  string
	OneTime   bool
	ExpiresAt *time.Time `gorm:"index"`
	CreatedAt time.Time
}
