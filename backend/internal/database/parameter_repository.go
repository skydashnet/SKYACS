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

	for i := range params {
		params[i].DeviceID = deviceID
		params[i].UpdatedAt = time.Now()
	}

	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "device_id"}, {Name: "name"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at"}),
	}).Create(&params).Error
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
