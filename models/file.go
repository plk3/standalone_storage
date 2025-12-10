package models

import (
	"time"

	"gorm.io/gorm"
)

type File struct {
	ID          string    `gorm:"primaryKey" json:"id"`
	Filename    string    `json:"filename"`
	ContentType string    `json:"content_type"`
	Size        int64     `json:"size"`
	Tags        []string  `json:"tags" gorm:"serializer:json"` // Stored as JSON array in SQLite
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(&File{})
}
