package database

import (
	"log"
	"os"
	"path/filepath"

	"devops-cli/internal/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func InitDB() {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	
	dbPath := filepath.Join(home, ".devops")
	if err := os.MkdirAll(dbPath, 0755); err != nil {
		log.Fatalf("Failed to create config directory: %v", err)
	}

	dbFile := filepath.Join(dbPath, "devops.db")

	db, err := gorm.Open(sqlite.Open(dbFile), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Auto Migrate all models including Setting, ServerEvent and ServerCredential
	err = db.AutoMigrate(&models.Server{}, &models.Cronjob{}, &models.Setting{}, &models.ServerEvent{}, &models.ServerCredential{})
	if err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}

	DB = db
}

// GetSetting retrieves a setting value by key safely
func GetSetting(key string) string {
	var s models.Setting
	if err := DB.Where("key = ?", key).First(&s).Error; err != nil {
		return ""
	}
	return s.Value
}
