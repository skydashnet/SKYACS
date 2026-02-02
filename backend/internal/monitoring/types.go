package monitoring

import "time"

type CheckType string

const (
	CheckTypeICMP CheckType = "icmp"
	CheckTypeHTTP CheckType = "http"
	CheckTypeTCP  CheckType = "tcp"
)

type Target struct {
	ID          int64     `json:"id"`
	DeviceID    int64     `json:"device_id"`
	Name        string    `json:"name"`
	Type        CheckType `json:"type"`
	Host        string    `json:"host"`
	Port        int       `json:"port,omitempty"`
	Path        string    `json:"path,omitempty"`
	Interval    int       `json:"interval"` // seconds
	Timeout     int       `json:"timeout"`  // seconds
	Enabled     bool      `json:"enabled"`
	LastCheck   time.Time `json:"last_check"`
	LastStatus  Status    `json:"last_status"`
}

type Status string

const (
	StatusUp      Status = "up"
	StatusDown    Status = "down"
	StatusUnknown Status = "unknown"
)

type CheckResult struct {
	TargetID     int64         `json:"target_id"`
	Status       Status        `json:"status"`
	ResponseTime time.Duration `json:"response_time"`
	Error        string        `json:"error,omitempty"`
	CheckedAt    time.Time     `json:"checked_at"`
}

type Stats struct {
	TotalTargets int `json:"total_targets"`
	Up           int `json:"up"`
	Down         int `json:"down"`
	Unknown      int `json:"unknown"`
}
