package database

import (
	"context"
	"time"

	"github.com/skydashnet/miniacs/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type DeviceRepository struct {
	db *gorm.DB
}

func NewDeviceRepository(db *gorm.DB) *DeviceRepository {
	return &DeviceRepository{db: db}
}

func (r *DeviceRepository) UpsertFromInform(ctx context.Context, device *models.Device) error {
	now := time.Now()
	device.LastInform = &now
	device.Online = true

	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "serial_number"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"manufacturer", "product_class", "hardware_version",
			"software_version", "ip_address", "connection_request_url",
			"last_inform", "online", "updated_at",
		}),
	}).Create(device).Error
}

func (r *DeviceRepository) GetBySerialNumber(ctx context.Context, serialNumber string) (*models.Device, error) {
	var device models.Device
	err := r.db.WithContext(ctx).Where("serial_number = ?", serialNumber).First(&device).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &device, nil
}

func (r *DeviceRepository) GetByID(ctx context.Context, id int64) (*models.Device, error) {
	var device models.Device
	err := r.db.WithContext(ctx).First(&device, id).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &device, nil
}

func (r *DeviceRepository) GetBySerial(ctx context.Context, serial string) (*models.Device, error) {
	return r.GetBySerialNumber(ctx, serial)
}

func (r *DeviceRepository) List(ctx context.Context, limit, offset int) ([]*models.Device, int, error) {
	var total int64
	if err := r.db.WithContext(ctx).Model(&models.Device{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var devices []*models.Device
	err := r.db.WithContext(ctx).
		Preload("Parameters", "name LIKE ? OR name LIKE ?", "%RXPower%", "%Username%").
		Order("last_inform DESC NULLS LAST").
		Limit(limit).
		Offset(offset).
		Find(&devices).Error
	if err != nil {
		return nil, 0, err
	}

	return devices, int(total), nil
}

func (r *DeviceRepository) GetStats(ctx context.Context) (*models.DeviceStats, error) {
	stats := &models.DeviceStats{}
	var total, online, offline int64

	r.db.WithContext(ctx).Model(&models.Device{}).Count(&total)
	r.db.WithContext(ctx).Model(&models.Device{}).Where("online = ?", true).Count(&online)
	r.db.WithContext(ctx).Model(&models.Device{}).Where("online = ?", false).Count(&offline)

	stats.Total = int(total)
	stats.Online = int(online)
	stats.Offline = int(offline)

	return stats, nil
}

func (r *DeviceRepository) SetOffline(ctx context.Context, serialNumber string) error {
	return r.db.WithContext(ctx).Model(&models.Device{}).
		Where("serial_number = ?", serialNumber).
		Update("online", false).Error
}

func (r *DeviceRepository) MarkStaleOffline(ctx context.Context, threshold time.Duration) (int64, error) {
	result := r.db.WithContext(ctx).Model(&models.Device{}).
		Where("online = ? AND last_inform < ?", true, time.Now().Add(-threshold)).
		Update("online", false)
	return result.RowsAffected, result.Error
}

func (r *DeviceRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("device_id = ?", id).Delete(&models.Task{}).Error; err != nil {
			return err
		}
		if err := tx.Where("device_id = ?", id).Delete(&models.DeviceParameter{}).Error; err != nil {
			return err
		}
		if err := tx.Where("device_id = ?", id).Delete(&models.Fault{}).Error; err != nil {
			return err
		}
		return tx.Delete(&models.Device{}, id).Error
	})
}
