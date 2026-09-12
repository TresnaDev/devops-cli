package models

import (
	"time"
)

type ServerEvent struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	ServerID  uint      `gorm:"index" json:"server_id"`
	EventType string    `gorm:"type:varchar(50);not null" json:"event_type"` // e.g. "TCP", "CREDENTIAL"
	Status    string    `gorm:"type:varchar(50);not null" json:"status"`     // e.g. "UP", "DOWN", "VALID", "INVALID"
	Message   string    `gorm:"type:text" json:"message"`                    // e.g. error reason
	CreatedAt time.Time `json:"created_at"`
}
