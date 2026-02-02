package database

import (
	"context"
	"time"

	"github.com/skydashnet/miniacs/internal/models"
	"gorm.io/gorm"
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

func (r *UserRepository) Update(ctx context.Context, id int64, username string, role models.UserRole) error {
	return r.db.WithContext(ctx).Model(&models.User{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"username": username,
			"role":     role,
		}).Error
}

func (r *UserRepository) UpdatePassword(ctx context.Context, id int64, passwordHash string) error {
	return r.db.WithContext(ctx).Model(&models.User{}).
		Where("id = ?", id).
		Update("password_hash", passwordHash).Error
}

func (r *UserRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&models.User{}, id).Error
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
