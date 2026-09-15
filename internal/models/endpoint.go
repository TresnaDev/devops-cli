package models

import "time"

// Endpoint represents an HTTP/HTTPS URL to be monitored.
type Endpoint struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	Name           string     `gorm:"type:varchar(100);not null" json:"name"`
	URL            string     `gorm:"type:varchar(500);not null" json:"url"`
	ExpectedStatus int        `gorm:"not null;default:200" json:"expected_status"` // Expected HTTP status code
	Keyword        string     `gorm:"type:varchar(255)" json:"keyword"`             // Optional: keyword to match in response body
	TimeoutSeconds int        `gorm:"not null;default:10" json:"timeout_seconds"`
	IsActive       bool       `gorm:"default:true" json:"is_active"`
	LastStatus     string     `gorm:"type:varchar(50);default:'unknown'" json:"last_status"` // UP, DOWN, unknown
	LastStatusCode int        `json:"last_status_code"`
	LastChecked    *time.Time `json:"last_checked"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}
