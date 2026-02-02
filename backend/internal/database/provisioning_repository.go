package database

import (
	"context"

	"github.com/skydashnet/miniacs/internal/models"
	"gorm.io/gorm"
)

type ProvisioningRepository struct {
	db *gorm.DB
}

func NewProvisioningRepository(db *gorm.DB) *ProvisioningRepository {
	return &ProvisioningRepository{db: db}
}

func (r *ProvisioningRepository) Create(ctx context.Context, rule *models.ProvisioningRule) error {
	return r.db.WithContext(ctx).Create(rule).Error
}

func (r *ProvisioningRepository) List(ctx context.Context) ([]*models.ProvisioningRule, error) {
	var rules []*models.ProvisioningRule
	err := r.db.WithContext(ctx).Order("id").Find(&rules).Error
	return rules, err
}

func (r *ProvisioningRepository) ListEnabled(ctx context.Context) ([]*models.ProvisioningRule, error) {
	var rules []*models.ProvisioningRule
	err := r.db.WithContext(ctx).Where("enabled = ?", true).Order("id").Find(&rules).Error
	return rules, err
}

func (r *ProvisioningRepository) GetByID(ctx context.Context, id int64) (*models.ProvisioningRule, error) {
	var rule models.ProvisioningRule
	err := r.db.WithContext(ctx).First(&rule, id).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &rule, nil
}

func (r *ProvisioningRepository) Update(ctx context.Context, rule *models.ProvisioningRule) error {
	return r.db.WithContext(ctx).Save(rule).Error
}

func (r *ProvisioningRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&models.ProvisioningRule{}, id).Error
}

func (r *ProvisioningRepository) ToggleEnabled(ctx context.Context, id int64, enabled bool) error {
	return r.db.WithContext(ctx).Model(&models.ProvisioningRule{}).
		Where("id = ?", id).
		Update("enabled", enabled).Error
}
