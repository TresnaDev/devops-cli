package models

type Setting struct {
	Key   string `gorm:"primaryKey" json:"key"`
	Value string `gorm:"type:text" json:"value"`
}
