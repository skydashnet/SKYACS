package models

import "time"

// AuditLog records security-relevant API activity without storing request bodies
// or credentials. Entries are append-only from the application perspective.
type AuditLog struct {
	ID        int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	UserID    *int64    `json:"user_id,omitempty" gorm:"index"`
	Username  string    `json:"username" gorm:"size:128;not null"`
	Action    string    `json:"action" gorm:"size:32;not null;index"`
	Resource  string    `json:"resource" gorm:"size:512;not null"`
	Status    int       `json:"status" gorm:"not null"`
	IPAddress string    `json:"ip_address" gorm:"size:64;not null"`
	UserAgent string    `json:"user_agent,omitempty" gorm:"size:512"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime;index"`
}

func (AuditLog) TableName() string {
	return "audit_logs"
}
