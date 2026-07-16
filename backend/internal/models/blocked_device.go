package models

import "time"

type BlockedDevice struct {
	ID           int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	SerialNumber string    `json:"serial_number" gorm:"uniqueIndex;size:128;not null"`
	Reason       string    `json:"reason" gorm:"size:512"`
	CreatedBy    string    `json:"created_by" gorm:"size:128"`
	CreatedAt    time.Time `json:"created_at" gorm:"autoCreateTime"`
}

func (BlockedDevice) TableName() string {
	return "blocked_devices"
}
