package models

import (
	"time"
)

type EndpointEvent struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	EndpointID uint      `gorm:"index" json:"endpoint_id"`
	Status     string    `gorm:"type:varchar(50);not null" json:"status"`     // e.g. "UP", "DOWN"
	Message    string    `gorm:"type:text" json:"message"`                    // e.g. error reason
	CreatedAt  time.Time `json:"created_at"`
}
