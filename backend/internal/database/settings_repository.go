package database

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Setting struct {
	Key         string    `json:"key" gorm:"primaryKey"`
	Value       string    `json:"value"`
	Description string    `json:"description,omitempty"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (Setting) TableName() string {
	return "settings"
}

type SettingsRepository struct {
	db *gorm.DB
}

func NewSettingsRepository(db *gorm.DB) *SettingsRepository {
	return &SettingsRepository{db: db}
}

func (r *SettingsRepository) GetAll(ctx context.Context) ([]Setting, error) {
	var settings []Setting
	err := r.db.WithContext(ctx).Order("key").Find(&settings).Error
	return settings, err
}

func (r *SettingsRepository) Get(ctx context.Context, key string) (*Setting, error) {
	var s Setting
	err := r.db.WithContext(ctx).Where("key = ?", key).First(&s).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *SettingsRepository) Set(ctx context.Context, key, value string) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at"}),
	}).Create(&Setting{Key: key, Value: value, UpdatedAt: time.Now()}).Error
}

func (r *SettingsRepository) SetMultiple(ctx context.Context, settings map[string]string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for key, value := range settings {
			err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "key"}},
				DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at"}),
			}).Create(&Setting{Key: key, Value: value, UpdatedAt: time.Now()}).Error
			if err != nil {
				return err
			}
		}
		return nil
	})
}
