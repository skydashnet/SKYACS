package database

import (
	"context"

	"github.com/skydashnet/miniacs/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type BlockedDeviceRepository struct {
	db *gorm.DB
}

func NewBlockedDeviceRepository(db *gorm.DB) *BlockedDeviceRepository {
	return &BlockedDeviceRepository{db: db}
}

func (r *BlockedDeviceRepository) IsBlocked(ctx context.Context, serialNumber string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&models.BlockedDevice{}).
		Where("serial_number = ?", serialNumber).Count(&count).Error
	return count > 0, err
}

func (r *BlockedDeviceRepository) List(ctx context.Context) ([]models.BlockedDevice, error) {
	var devices []models.BlockedDevice
	err := r.db.WithContext(ctx).Order("created_at DESC").Find(&devices).Error
	return devices, err
}

func (r *BlockedDeviceRepository) Add(ctx context.Context, device *models.BlockedDevice) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "serial_number"}},
		DoUpdates: clause.AssignmentColumns([]string{"reason", "created_by"}),
	}).Create(device).Error
}

func (r *BlockedDeviceRepository) Remove(ctx context.Context, serialNumber string) error {
	return r.db.WithContext(ctx).Where("serial_number = ?", serialNumber).Delete(&models.BlockedDevice{}).Error
}
