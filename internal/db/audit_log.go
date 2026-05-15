package db

import (
	"time"
)

type AuditLog struct {
	ID         int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	CreatedAt  time.Time `gorm:"index" json:"created_at"`
	User       string    `gorm:"size:100;not null" json:"user"`
	Action     string    `gorm:"size:50;not null" json:"action"`
	Resource   string    `gorm:"size:50;not null" json:"resource"`
	ResourceID string   `gorm:"size:100" json:"resource_id"`
	OldValue   string    `gorm:"type:text" json:"old_value,omitempty"`
	NewValue   string    `gorm:"type:text" json:"new_value,omitempty"`
	IPAddress  string    `gorm:"size:50" json:"ip_address"`
}

func (AuditLog) TableName() string { return "audit_log" }
