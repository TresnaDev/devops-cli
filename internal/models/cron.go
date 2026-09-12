package models

import (
	"time"
)

type Cronjob struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"type:varchar(100);not null" json:"name"`
	Schedule    string    `gorm:"type:varchar(50);not null" json:"schedule"` // e.g., "*/5 * * * *"
	Command     string    `gorm:"type:text;not null" json:"command"`
	IsActive    bool      `gorm:"default:true" json:"is_active"`
	LastRun     time.Time `json:"last_run"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
