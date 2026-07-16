package database

import (
	"context"
	"time"

	"github.com/skydashnet/miniacs/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ParameterRepository struct {
	db *gorm.DB
}

func NewParameterRepository(db *gorm.DB) *ParameterRepository {
	return &ParameterRepository{db: db}
}

func (r *ParameterRepository) UpsertMany(ctx context.Context, deviceID int64, params []models.DeviceParameter) error {
	if len(params) == 0 {
		return nil
	}

	now := time.Now()
	for index := range params {
		params[index].DeviceID = deviceID
		params[index].UpdatedAt = now
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "device_id"}, {Name: "name"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at"}),
	}).CreateInBatches(&params, 500).Error
}

func (r *ParameterRepository) UpsertWritable(ctx context.Context, deviceID int64, params []models.DeviceParameter) error {
	if len(params) == 0 {
		return nil
	}
	now := time.Now()
	entries := make([]models.DeviceParameter, 0, len(params))
	for _, param := range params {
		if param.Writable != nil {
			entries = append(entries, models.DeviceParameter{DeviceID: deviceID, Name: param.Name, Writable: param.Writable, UpdatedAt: now})
		}
	}
	if len(entries) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "device_id"}, {Name: "name"}},
		DoUpdates: clause.AssignmentColumns([]string{"writable", "updated_at"}),
	}).CreateInBatches(&entries, 500).Error
}

func (r *ParameterRepository) GetByDeviceID(ctx context.Context, deviceID int64) ([]models.DeviceParameter, error) {
	var params []models.DeviceParameter
	err := r.db.WithContext(ctx).Where("device_id = ?", deviceID).
		Order("name").
		Find(&params).Error
	return params, err
}

func (r *ParameterRepository) GetByPrefix(ctx context.Context, deviceID int64, prefix string) ([]models.DeviceParameter, error) {
	var params []models.DeviceParameter
	err := r.db.WithContext(ctx).Where("device_id = ? AND name LIKE ?", deviceID, prefix+"%").
		Order("name").
		Find(&params).Error
	return params, err
}

func (r *ParameterRepository) DeleteOlderThan(ctx context.Context, deviceID int64, age time.Duration) (int64, error) {
	result := r.db.WithContext(ctx).Where("device_id = ? AND updated_at < ?", deviceID, time.Now().Add(-age)).
		Delete(&models.DeviceParameter{})
	return result.RowsAffected, result.Error
}
