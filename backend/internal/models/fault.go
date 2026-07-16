package models

import "time"

type Fault struct {
	ID            int64      `json:"id" gorm:"primaryKey;autoIncrement"`
	DeviceID      int64      `json:"device_id" gorm:"index:idx_faults_device_id;not null"`
	SerialNumber  string     `json:"serial_number,omitempty" gorm:"-"`
	FaultCode     string     `json:"fault_code" gorm:"not null"`
	FaultString   string     `json:"fault_string"`
	ParameterName string     `json:"parameter_name,omitempty"`
	Resolved      bool       `json:"resolved" gorm:"default:false;index"`
	CreatedAt     time.Time  `json:"created_at" gorm:"autoCreateTime;index"`
	ResolvedAt    *time.Time `json:"resolved_at,omitempty"`
}

func (Fault) TableName() string {
	return "faults"
}
