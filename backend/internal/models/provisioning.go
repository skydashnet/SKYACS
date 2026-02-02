package models

import "time"

type ProvisioningRule struct {
	ID             int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	ParameterName  string    `json:"parameter_name" gorm:"not null"`
	ParameterValue string    `json:"parameter_value"`
	ParameterType  string    `json:"parameter_type" gorm:"default:'string'"`
	Enabled        bool      `json:"enabled" gorm:"default:true"`
	Description    string    `json:"description,omitempty"`
	CreatedAt      time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt      time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (ProvisioningRule) TableName() string {
	return "provisioning_rules"
}

type CreateProvisioningRuleRequest struct {
	ParameterName  string `json:"parameter_name"`
	ParameterValue string `json:"parameter_value"`
	ParameterType  string `json:"parameter_type"`
	Enabled        bool   `json:"enabled"`
	Description    string `json:"description"`
}
