package models

import (
	"time"
)

// ServerCredential represents a single SSH account/credential for a server.
// A server can have many credentials (e.g. root with password, bizdev with key, etc.)
type ServerCredential struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	ServerID    uint       `gorm:"index;not null" json:"server_id"`
	Label       string     `gorm:"type:varchar(100);not null" json:"label"`       // e.g. "root (password)", "bizops-agent (key)"
	User        string     `gorm:"type:varchar(100);not null" json:"user"`
	SSHPassword string     `gorm:"type:text" json:"-"`                            // AES-encrypted
	SSHKeyPath  string     `gorm:"type:varchar(500)" json:"ssh_key_path"`
	Status      string     `gorm:"type:varchar(50);default:'unchecked'" json:"status"` // valid, invalid, unchecked
	LastChecked *time.Time `json:"last_checked"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}
