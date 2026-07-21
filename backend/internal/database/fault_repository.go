package database

import (
	"context"
	"time"

	"github.com/skydashnet/skyacs/internal/models"
	"gorm.io/gorm"
)

type FaultRepository struct {
	db *gorm.DB
}

func NewFaultRepository(db *gorm.DB) *FaultRepository {
	return &FaultRepository{db: db}
}

func (r *FaultRepository) Create(ctx context.Context, fault *models.Fault) error {
	return r.db.WithContext(ctx).Create(fault).Error
}

func (r *FaultRepository) List(ctx context.Context, limit, offset int, resolved *bool) ([]*models.Fault, int, error) {
	var total int64
	query := r.db.WithContext(ctx).Model(&models.Fault{})

	if resolved != nil {
		query = query.Where("resolved = ?", *resolved)
	}
	query.Count(&total)

	var faults []*models.Fault
	q := r.db.WithContext(ctx).Table("faults f").
		Select("f.id, f.device_id, d.serial_number, f.fault_code, f.fault_string, f.parameter_name, f.resolved, f.created_at, f.resolved_at").
		Joins("JOIN devices d ON f.device_id = d.id")

	if resolved != nil {
		q = q.Where("f.resolved = ?", *resolved)
	}

	err := q.Order("f.created_at DESC").
		Limit(limit).
		Offset(offset).
		Scan(&faults).Error

	if err != nil {
		return nil, 0, err
	}
	return faults, int(total), nil
}

func (r *FaultRepository) GetByDeviceID(ctx context.Context, deviceID int64) ([]*models.Fault, error) {
	var faults []*models.Fault
	err := r.db.WithContext(ctx).Table("faults f").
		Select("f.id, f.device_id, d.serial_number, f.fault_code, f.fault_string, f.parameter_name, f.resolved, f.created_at, f.resolved_at").
		Joins("JOIN devices d ON f.device_id = d.id").
		Where("f.device_id = ?", deviceID).
		Order("f.created_at DESC").
		Scan(&faults).Error
	return faults, err
}

func (r *FaultRepository) Resolve(ctx context.Context, id int64) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&models.Fault{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"resolved":    true,
			"resolved_at": &now,
		}).Error
}

func (r *FaultRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&models.Fault{}, id).Error
}

func (r *FaultRepository) GetStats(ctx context.Context) (map[string]int, error) {
	var active, resolved, total int64

	r.db.WithContext(ctx).Model(&models.Fault{}).Where("resolved = ?", false).Count(&active)
	r.db.WithContext(ctx).Model(&models.Fault{}).Where("resolved = ?", true).Count(&resolved)
	r.db.WithContext(ctx).Model(&models.Fault{}).Count(&total)

	stats := map[string]int{
		"active":   int(active),
		"resolved": int(resolved),
		"total":    int(total),
	}

	type CodeCount struct {
		FaultCode string
		Count     int
	}
	var codeCounts []CodeCount
	r.db.WithContext(ctx).Model(&models.Fault{}).
		Select("fault_code, COUNT(*) as count").
		Where("resolved = ?", false).
		Group("fault_code").
		Order("count DESC").
		Limit(10).
		Scan(&codeCounts)

	for _, cc := range codeCounts {
		stats["code_"+cc.FaultCode] = cc.Count
	}

	return stats, nil
}
