package database

import (
	"context"
	"time"

	"github.com/skydashnet/skyacs/internal/models"
	"gorm.io/gorm"
)

type FirmwareRepository struct {
	db *gorm.DB
}

func NewFirmwareRepository(db *gorm.DB) *FirmwareRepository {
	return &FirmwareRepository{db: db}
}

func (r *FirmwareRepository) Create(ctx context.Context, fw *models.Firmware) error {
	return r.db.WithContext(ctx).Create(fw).Error
}

func (r *FirmwareRepository) GetByID(ctx context.Context, id int64) (*models.Firmware, error) {
	var fw models.Firmware
	err := r.db.WithContext(ctx).First(&fw, id).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &fw, nil
}

func (r *FirmwareRepository) List(ctx context.Context) ([]*models.Firmware, error) {
	var firmwares []*models.Firmware
	err := r.db.WithContext(ctx).Order("created_at DESC").Find(&firmwares).Error
	return firmwares, err
}

func (r *FirmwareRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&models.Firmware{}, id).Error
}

func (r *FirmwareRepository) UpdateDownloadToken(ctx context.Context, id int64, token string, expiresAt time.Time) error {
	return r.db.WithContext(ctx).Model(&models.Firmware{}).Where("id = ?", id).Updates(map[string]interface{}{
		"download_token":   token,
		"token_expires_at": &expiresAt,
	}).Error
}
