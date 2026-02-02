package database

import (
	"context"
	"encoding/json"
	"time"

	"github.com/skydashnet/miniacs/internal/models"
	"gorm.io/gorm"
)

type TaskRepository struct {
	db *gorm.DB
}

func NewTaskRepository(db *gorm.DB) *TaskRepository {
	return &TaskRepository{db: db}
}

func (r *TaskRepository) Create(ctx context.Context, task *models.Task) error {
	return r.db.WithContext(ctx).Create(task).Error
}

func (r *TaskRepository) GetPendingByDeviceID(ctx context.Context, deviceID int64) ([]*models.Task, error) {
	var tasks []*models.Task
	err := r.db.WithContext(ctx).Where("device_id = ? AND status = ?", deviceID, models.TaskStatusPending).
		Order("created_at ASC").
		Find(&tasks).Error
	return tasks, err
}

func (r *TaskRepository) UpdateStatus(ctx context.Context, id int64, status models.TaskStatus, result interface{}, errorMsg string) error {
	updates := map[string]interface{}{
		"status": status,
	}

	if status == models.TaskStatusCompleted || status == models.TaskStatusFailed {
		resultJSON, _ := json.Marshal(result)
		updates["result"] = resultJSON
		updates["error_message"] = errorMsg
		now := time.Now()
		updates["completed_at"] = &now
	} else if status == models.TaskStatusSent {
		now := time.Now()
		updates["sent_at"] = &now
	}

	return r.db.WithContext(ctx).Model(&models.Task{}).Where("id = ?", id).Updates(updates).Error
}

func (r *TaskRepository) GetByDeviceID(ctx context.Context, deviceID int64, limit int) ([]*models.Task, error) {
	var tasks []*models.Task
	err := r.db.WithContext(ctx).Where("device_id = ?", deviceID).
		Order("created_at DESC").
		Limit(limit).
		Find(&tasks).Error
	return tasks, err
}

func (r *TaskRepository) MarkOldPendingAsFailed(ctx context.Context, age time.Duration) (int64, error) {
	now := time.Now()
	result := r.db.WithContext(ctx).Model(&models.Task{}).
		Where("status = ? AND created_at < ?", models.TaskStatusPending, time.Now().Add(-age)).
		Updates(map[string]interface{}{
			"status":        models.TaskStatusFailed,
			"error_message": "timeout",
			"completed_at":  &now,
		})
	return result.RowsAffected, result.Error
}
