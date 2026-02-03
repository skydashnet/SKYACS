package models

import (
	"time"
)

type Device struct {
	ID                   int64              `json:"id" gorm:"primaryKey;autoIncrement"`
	SerialNumber         string             `json:"serial_number" gorm:"uniqueIndex:idx_devices_serial_number;not null"`
	OUI                  string             `json:"oui" gorm:"column:oui;not null"`
	Manufacturer         *string            `json:"manufacturer"`
	ProductClass         *string            `json:"product_class"`
	ModelName            *string            `json:"model_name"`
	HardwareVersion      *string            `json:"hardware_version"`
	SoftwareVersion      *string            `json:"software_version"`
	IPAddress            *string            `json:"ip_address"`
	ConnectionRequestURL *string            `json:"connection_request_url"`
	LastInform           *time.Time         `json:"last_inform"`
	Online               bool               `json:"online" gorm:"default:false"`
	CreatedAt            time.Time          `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt            time.Time          `json:"updated_at" gorm:"autoUpdateTime"`
	Parameters           []DeviceParameter  `json:"parameters,omitempty" gorm:"foreignKey:DeviceID"`
}

func (Device) TableName() string {
	return "devices"
}

type DeviceStats struct {
	Total   int `json:"total"`
	Online  int `json:"online"`
	Offline int `json:"offline"`
}

type DeviceParameter struct {
	ID        int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	DeviceID  int64     `json:"device_id" gorm:"uniqueIndex:idx_device_parameters_unique,priority:1;not null"`
	Name      string    `json:"name" gorm:"uniqueIndex:idx_device_parameters_unique,priority:2;not null"`
	Value     string    `json:"value"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (DeviceParameter) TableName() string {
	return "device_parameters"
}
