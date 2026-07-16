package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

type TaskStatus string
type TaskType string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusSent      TaskStatus = "sent"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
)

const (
	TaskTypeGetParameterValues TaskType = "get_parameter_values"
	TaskTypeSetParameterValues TaskType = "set_parameter_values"
	TaskTypeReboot             TaskType = "reboot"
	TaskTypeFactoryReset       TaskType = "factory_reset"
	TaskTypeDownload           TaskType = "download"
)

type JSON json.RawMessage

func (j JSON) Value() (driver.Value, error) {
	if len(j) == 0 {
		return nil, nil
	}
	return []byte(j), nil
}

func (j *JSON) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	*j = bytes
	return nil
}

func (j JSON) MarshalJSON() ([]byte, error) {
	if len(j) == 0 {
		return []byte("null"), nil
	}
	return j, nil
}

func (j *JSON) UnmarshalJSON(data []byte) error {
	if j == nil {
		return errors.New("JSON: UnmarshalJSON on nil pointer")
	}
	*j = append((*j)[0:0], data...)
	return nil
}

type Task struct {
	ID           int64      `json:"id" gorm:"primaryKey;autoIncrement"`
	DeviceID     int64      `json:"device_id" gorm:"index;index:idx_tasks_queue,priority:1;not null"`
	CommandKey   string     `json:"command_key,omitempty" gorm:"size:64"`
	Type         TaskType   `json:"type" gorm:"type:text;not null"`
	Payload      JSON       `json:"payload" gorm:"type:jsonb"`
	Status       TaskStatus `json:"status" gorm:"type:text;default:'pending';index;index:idx_tasks_queue,priority:2;index:idx_tasks_status_sent,priority:1"`
	Result       JSON       `json:"result" gorm:"type:jsonb"`
	ErrorMessage string     `json:"error_message,omitempty"`
	CreatedAt    time.Time  `json:"created_at" gorm:"autoCreateTime;index;index:idx_tasks_queue,priority:3"`
	SentAt       *time.Time `json:"sent_at,omitempty" gorm:"index:idx_tasks_status_sent,priority:2"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
}

func (Task) TableName() string {
	return "tasks"
}

func NewTaskWithPayload(deviceID int64, taskType TaskType, payload interface{}) (*Task, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return &Task{
		DeviceID: deviceID,
		Type:     taskType,
		Payload:  payloadBytes,
		Status:   TaskStatusPending,
	}, nil
}
