package database

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/skydashnet/miniacs/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ProvisioningRepository struct {
	db *gorm.DB
}

func NewProvisioningRepository(db *gorm.DB) *ProvisioningRepository {
	return &ProvisioningRepository{db: db}
}

func (r *ProvisioningRepository) Create(ctx context.Context, rule *models.ProvisioningRule) error {
	stored := *rule
	if IsSensitiveParameterName(stored.ParameterName) {
		value, err := encryptParameterValue(stored.ParameterValue)
		if err != nil {
			return fmt.Errorf("encrypt provisioning value: %w", err)
		}
		stored.ParameterValue = value
	}
	if err := r.db.WithContext(ctx).Create(&stored).Error; err != nil {
		return err
	}
	plaintext := rule.ParameterValue
	*rule = stored
	rule.ParameterValue = plaintext
	return nil
}

func (r *ProvisioningRepository) List(ctx context.Context) ([]*models.ProvisioningRule, error) {
	var rules []*models.ProvisioningRule
	if err := r.db.WithContext(ctx).Order("id").Find(&rules).Error; err != nil {
		return nil, err
	}
	return rules, decryptProvisioningRules(rules)
}

func (r *ProvisioningRepository) ListPendingForDevice(ctx context.Context, deviceID int64, manufacturer, productClass string) ([]*models.ProvisioningRule, error) {
	var rules []*models.ProvisioningRule
	if err := r.db.WithContext(ctx).
		Where("enabled = ?", true).
		Where("(manufacturer = '' OR LOWER(manufacturer) = LOWER(?))", manufacturer).
		Where("(product_class = '' OR LOWER(product_class) = LOWER(?))", productClass).
		Where("NOT EXISTS (SELECT 1 FROM provisioning_applications pa WHERE pa.rule_id = provisioning_rules.id AND pa.rule_version = provisioning_rules.version AND pa.device_id = ?)", deviceID).
		Order("id").Find(&rules).Error; err != nil {
		return nil, err
	}
	return rules, decryptProvisioningRules(rules)
}

func (r *ProvisioningRepository) MarkApplied(ctx context.Context, deviceID int64, applications []models.ProvisioningApplication) error {
	for index := range applications {
		applications[index].DeviceID = deviceID
		applications[index].ID = 0
		applications[index].AppliedAt = time.Time{}
	}
	if len(applications) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&applications).Error
}

func (r *ProvisioningRepository) Update(ctx context.Context, rule *models.ProvisioningRule) error {
	value := rule.ParameterValue
	if IsSensitiveParameterName(rule.ParameterName) {
		var err error
		value, err = encryptParameterValue(value)
		if err != nil {
			return fmt.Errorf("encrypt provisioning value: %w", err)
		}
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return tx.Model(&models.ProvisioningRule{}).Where("id = ?", rule.ID).Updates(map[string]interface{}{
			"parameter_name": rule.ParameterName, "parameter_value": value, "parameter_type": rule.ParameterType,
			"manufacturer": rule.Manufacturer, "product_class": rule.ProductClass, "enabled": rule.Enabled, "description": rule.Description,
			"version": gorm.Expr("version + 1"),
		}).Error
	})
}

func decryptProvisioningRules(rules []*models.ProvisioningRule) error {
	for _, rule := range rules {
		if !IsSensitiveParameterName(rule.ParameterName) {
			continue
		}
		value, err := decryptParameterValue(rule.ParameterValue)
		if err != nil {
			return fmt.Errorf("decrypt provisioning rule %d: %w", rule.ID, err)
		}
		rule.ParameterValue = value
	}
	return nil
}

func (r *ProvisioningRepository) EncryptLegacySensitiveValues(ctx context.Context) (int64, error) {
	var encrypted int64
	var lastID int64
	for {
		var rules []models.ProvisioningRule
		if err := r.db.WithContext(ctx).Select("id", "parameter_name", "parameter_value").
			Where("id > ?", lastID).Order("id ASC").Limit(500).Find(&rules).Error; err != nil {
			return encrypted, err
		}
		if len(rules) == 0 {
			return encrypted, nil
		}
		if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			for _, rule := range rules {
				lastID = rule.ID
				if !IsSensitiveParameterName(rule.ParameterName) || rule.ParameterValue == "" || strings.HasPrefix(rule.ParameterValue, encryptedParameterPrefix) {
					continue
				}
				value, err := encryptParameterValue(rule.ParameterValue)
				if err != nil {
					return err
				}
				if err := tx.Model(&models.ProvisioningRule{}).Where("id = ?", rule.ID).Update("parameter_value", value).Error; err != nil {
					return err
				}
				encrypted++
			}
			return nil
		}); err != nil {
			return encrypted, err
		}
	}
}

func (r *ProvisioningRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("rule_id = ?", id).Delete(&models.ProvisioningApplication{}).Error; err != nil {
			return err
		}
		return tx.Delete(&models.ProvisioningRule{}, id).Error
	})
}

func (r *ProvisioningRepository) ToggleEnabled(ctx context.Context, id int64, enabled bool) error {
	return r.db.WithContext(ctx).Model(&models.ProvisioningRule{}).
		Where("id = ?", id).
		Update("enabled", enabled).Error
}
