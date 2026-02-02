package models

import "time"

type Firmware struct {
	ID           int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	Filename     string    `json:"filename" gorm:"not null"`
	Version      string    `json:"version" gorm:"not null"`
	Manufacturer *string   `json:"manufacturer,omitempty"`
	ProductClass *string   `json:"product_class,omitempty"`
	FileSize     int64     `json:"file_size"`
	FilePath     string    `json:"file_path" gorm:"not null"`
	Checksum     *string   `json:"checksum,omitempty"`
	Description  *string   `json:"description,omitempty"`
	CreatedAt    time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt    time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (Firmware) TableName() string {
	return "firmwares"
}
