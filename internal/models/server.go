package models

import (
	"time"
)

type Server struct {
	ID                    uint       `gorm:"primaryKey" json:"id"`
	Name                  string     `gorm:"type:varchar(100);not null" json:"name"`
	Host                  string     `gorm:"type:varchar(255);not null" json:"host"`
	Port                  int        `gorm:"not null;default:22" json:"port"`
	User                  string     `gorm:"type:varchar(100);not null;default:'root'" json:"user"`
	Description           string     `gorm:"type:text" json:"description"`
	IsActive              bool       `gorm:"default:true" json:"is_active"`
	LastStatus            string     `gorm:"type:varchar(50);default:'unknown'" json:"last_status"`
	LastBanner            string     `gorm:"type:text" json:"last_banner"`
	// Credential fields
	SSHPassword           string     `gorm:"type:text" json:"-"`                          // AES-encrypted password
	SSHKeyPath            string     `gorm:"type:varchar(500)" json:"ssh_key_path"`        // Path to private key file
	CredentialStatus      string     `gorm:"type:varchar(50);default:'unchecked'" json:"credential_status"`
	CredentialLastChecked *time.Time `json:"credential_last_checked"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
}
