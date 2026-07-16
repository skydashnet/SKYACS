package database

import (
	"context"
	"errors"
	"time"

	"github.com/skydashnet/miniacs/internal/models"
	"gorm.io/gorm"
)

var (
	ErrLastFullAdmin = errors.New("cannot remove the last full-access administrator")
	ErrLastUser      = errors.New("cannot remove the last user")
)

type UserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(ctx context.Context, user *models.User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

func (r *UserRepository) GetByUsername(ctx context.Context, username string) (*models.User, error) {
	var user models.User
	err := r.db.WithContext(ctx).Where("username = ?", username).First(&user).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) GetByID(ctx context.Context, id int64) (*models.User, error) {
	var user models.User
	err := r.db.WithContext(ctx).First(&user, id).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) List(ctx context.Context) ([]*models.User, error) {
	var users []*models.User
	err := r.db.WithContext(ctx).Order("id").Find(&users).Error
	return users, err
}

func (r *UserRepository) UpdatePassword(ctx context.Context, id int64, passwordHash string) error {
	return r.db.WithContext(ctx).Model(&models.User{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"password_hash": passwordHash,
			"token_version": gorm.Expr("token_version + 1"),
		}).Error
}

func (r *UserRepository) UpdateAccount(ctx context.Context, id int64, username string, role models.UserRole, passwordHash string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("LOCK TABLE users IN SHARE ROW EXCLUSIVE MODE").Error; err != nil {
			return err
		}
		var current models.User
		if err := tx.First(&current, id).Error; err != nil {
			return err
		}
		if current.Role == models.RoleFull && role != models.RoleFull {
			var count int64
			if err := tx.Model(&models.User{}).Where("role = ?", models.RoleFull).Count(&count).Error; err != nil {
				return err
			}
			if count <= 1 {
				return ErrLastFullAdmin
			}
		}
		updates := map[string]interface{}{
			"username":      username,
			"role":          role,
			"token_version": gorm.Expr("token_version + 1"),
		}
		if passwordHash != "" {
			updates["password_hash"] = passwordHash
		}
		return tx.Model(&models.User{}).Where("id = ?", id).Updates(updates).Error
	})
}

func (r *UserRepository) DeleteSafely(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("LOCK TABLE users IN SHARE ROW EXCLUSIVE MODE").Error; err != nil {
			return err
		}
		var user models.User
		if err := tx.First(&user, id).Error; err != nil {
			return err
		}
		var userCount int64
		if err := tx.Model(&models.User{}).Count(&userCount).Error; err != nil {
			return err
		}
		if userCount <= 1 {
			return ErrLastUser
		}
		if user.Role == models.RoleFull {
			var count int64
			if err := tx.Model(&models.User{}).Where("role = ?", models.RoleFull).Count(&count).Error; err != nil {
				return err
			}
			if count <= 1 {
				return ErrLastFullAdmin
			}
		}
		return tx.Delete(&models.User{}, id).Error
	})
}

func (r *UserRepository) UpdateLastLogin(ctx context.Context, id int64) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&models.User{}).
		Where("id = ?", id).
		Update("last_login", &now).Error
}

func (r *UserRepository) Count(ctx context.Context) (int, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&models.User{}).Count(&count).Error
	return int(count), err
}

func (r *UserRepository) CountByRole(ctx context.Context, role models.UserRole) (int, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&models.User{}).Where("role = ?", role).Count(&count).Error
	return int(count), err
}
