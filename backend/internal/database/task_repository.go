package database

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/skydashnet/miniacs/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type TaskRepository struct {
	db *gorm.DB
}

func NewTaskRepository(db *gorm.DB) *TaskRepository {
	return &TaskRepository{db: db}
}

func (r *TaskRepository) Create(ctx context.Context, task *models.Task) error {
	stored := *task
	if err := encryptSetTaskPayload(&stored); err != nil {
		return err
	}
	if err := r.db.WithContext(ctx).Create(&stored).Error; err != nil {
		return err
	}
	plaintextPayload := task.Payload
	*task = stored
	task.Payload = plaintextPayload
	return nil
}

// ClaimNextPending atomically reserves the oldest pending task for one CWMP
// session. SKIP LOCKED prevents concurrent sessions from dispatching the same
// command while keeping other devices independent.
func (r *TaskRepository) ClaimNextPending(ctx context.Context, deviceID int64) (*models.Task, error) {
	var claimed *models.Task
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var task models.Task
		err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("device_id = ? AND status = ?", deviceID, models.TaskStatusPending).
			Order("created_at ASC").
			First(&task).Error
		if err == gorm.ErrRecordNotFound {
			return nil
		}
		if err != nil {
			return err
		}

		now := time.Now()
		commandKey := task.CommandKey
		if commandKey == "" {
			commandKey = fmt.Sprintf("miniacs-task-%d", task.ID)
		}
		result := tx.Model(&models.Task{}).
			Where("id = ? AND status = ?", task.ID, models.TaskStatusPending).
			Updates(map[string]interface{}{
				"status":      models.TaskStatusSent,
				"sent_at":     &now,
				"command_key": commandKey,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return nil
		}
		task.Status = models.TaskStatusSent
		task.SentAt = &now
		task.CommandKey = commandKey
		if err := decryptSetTaskPayload(&task); err != nil {
			return err
		}
		claimed = &task
		return nil
	})
	return claimed, err
}

func (r *TaskRepository) UpdateByCommandKey(ctx context.Context, commandKey string, status models.TaskStatus, result interface{}, errorMsg string) error {
	if commandKey == "" {
		return errors.New("command key is required")
	}
	var task models.Task
	if err := r.db.WithContext(ctx).Where("command_key = ?", commandKey).First(&task).Error; err != nil {
		return err
	}
	return r.UpdateStatus(ctx, task.ID, status, result, errorMsg)
}

func (r *TaskRepository) UpdateStatus(ctx context.Context, id int64, status models.TaskStatus, result interface{}, errorMsg string) error {
	updates := map[string]interface{}{
		"status": status,
	}

	if status == models.TaskStatusCompleted || status == models.TaskStatusFailed {
		resultJSON, err := json.Marshal(result)
		if err != nil {
			return fmt.Errorf("encode task result: %w", err)
		}
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
	if err != nil {
		return nil, err
	}
	for _, task := range tasks {
		redactSetTaskPayload(task)
		redactGetTaskResult(task)
	}
	return tasks, nil
}

func (r *TaskRepository) CountActiveFirmwareTasks(ctx context.Context, firmwareID int64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&models.Task{}).
		Where("type = ? AND status IN ? AND (payload ->> 'firmware_id' = ? OR payload ->> 'url' LIKE ?)",
			models.TaskTypeDownload, []models.TaskStatus{models.TaskStatusPending, models.TaskStatusSent}, fmt.Sprint(firmwareID), fmt.Sprintf("%%/files/%d/%%", firmwareID)).
		Count(&count).Error
	return count, err
}

func encryptSetTaskPayload(task *models.Task) error {
	if task.Type != models.TaskTypeSetParameterValues || len(task.Payload) == 0 {
		return nil
	}
	var parameters map[string]string
	if err := json.Unmarshal(task.Payload, &parameters); err != nil {
		return fmt.Errorf("decode set-parameter task payload: %w", err)
	}
	for name, value := range parameters {
		if !IsSensitiveParameterName(name) {
			continue
		}
		encrypted, err := encryptParameterValue(value)
		if err != nil {
			return fmt.Errorf("encrypt task parameter %s: %w", name, err)
		}
		parameters[name] = encrypted
	}
	payload, err := json.Marshal(parameters)
	if err != nil {
		return err
	}
	task.Payload = payload
	return nil
}

func decryptSetTaskPayload(task *models.Task) error {
	if task.Type != models.TaskTypeSetParameterValues || len(task.Payload) == 0 {
		return nil
	}
	var parameters map[string]string
	if err := json.Unmarshal(task.Payload, &parameters); err != nil {
		return fmt.Errorf("decode set-parameter task payload: %w", err)
	}
	for name, value := range parameters {
		if !IsSensitiveParameterName(name) {
			continue
		}
		decrypted, err := decryptParameterValue(value)
		if err != nil {
			return fmt.Errorf("decrypt task parameter %s: %w", name, err)
		}
		parameters[name] = decrypted
	}
	payload, err := json.Marshal(parameters)
	if err != nil {
		return err
	}
	task.Payload = payload
	return nil
}

func redactSetTaskPayload(task *models.Task) {
	if task.Type != models.TaskTypeSetParameterValues || len(task.Payload) == 0 {
		return
	}
	var parameters map[string]string
	if json.Unmarshal(task.Payload, &parameters) != nil {
		return
	}
	for name := range parameters {
		if IsSensitiveParameterName(name) {
			parameters[name] = "[REDACTED]"
		}
	}
	if payload, err := json.Marshal(parameters); err == nil {
		task.Payload = payload
	}
}

func redactGetTaskResult(task *models.Task) {
	if task.Type != models.TaskTypeGetParameterValues || len(task.Result) == 0 {
		return
	}
	var result map[string]interface{}
	if json.Unmarshal(task.Result, &result) != nil {
		return
	}
	for name := range result {
		if IsSensitiveParameterName(name) {
			result[name] = "[REDACTED]"
		}
	}
	if encoded, err := json.Marshal(result); err == nil {
		task.Result = encoded
	}
}

func (r *TaskRepository) ProtectLegacyTaskSecrets(ctx context.Context) (int64, error) {
	var protected int64
	var lastID int64
	for {
		var tasks []models.Task
		if err := r.db.WithContext(ctx).Select("id", "type", "payload", "result").
			Where("id > ? AND type IN ?", lastID, []models.TaskType{models.TaskTypeSetParameterValues, models.TaskTypeGetParameterValues}).
			Order("id ASC").Limit(500).Find(&tasks).Error; err != nil {
			return protected, err
		}
		if len(tasks) == 0 {
			return protected, nil
		}
		if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			for index := range tasks {
				task := &tasks[index]
				lastID = task.ID
				originalPayload, originalResult := string(task.Payload), string(task.Result)
				if err := encryptSetTaskPayload(task); err != nil {
					return fmt.Errorf("protect task %d: %w", task.ID, err)
				}
				redactGetTaskResult(task)
				updates := map[string]interface{}{}
				if string(task.Payload) != originalPayload {
					updates["payload"] = task.Payload
				}
				if string(task.Result) != originalResult {
					updates["result"] = task.Result
				}
				if len(updates) == 0 {
					continue
				}
				if err := tx.Model(&models.Task{}).Where("id = ?", task.ID).Updates(updates).Error; err != nil {
					return err
				}
				protected++
			}
			return nil
		}); err != nil {
			return protected, err
		}
	}
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

func (r *TaskRepository) MarkOldPendingDownloadsAsFailed(ctx context.Context, age time.Duration) (int64, error) {
	now := time.Now()
	result := r.db.WithContext(ctx).Model(&models.Task{}).
		Where("status = ? AND type = ? AND created_at < ?", models.TaskStatusPending, models.TaskTypeDownload, now.Add(-age)).
		Updates(map[string]interface{}{
			"status":        models.TaskStatusFailed,
			"error_message": "firmware download grant expired; reschedule the task",
			"completed_at":  &now,
		})
	return result.RowsAffected, result.Error
}

func (r *TaskRepository) MarkOldSentAsFailed(ctx context.Context, age time.Duration) (int64, error) {
	now := time.Now()
	result := r.db.WithContext(ctx).Model(&models.Task{}).
		Where("status = ? AND sent_at IS NOT NULL AND sent_at < ?", models.TaskStatusSent, now.Add(-age)).
		Updates(map[string]interface{}{
			"status":        models.TaskStatusFailed,
			"error_message": "device response timeout",
			"completed_at":  &now,
		})
	return result.RowsAffected, result.Error
}
