package database

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/skydashnet/skyacs/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const encryptedParameterPrefix = "enc:v1:"

var parameterAEAD cipher.AEAD

func ConfigureParameterEncryption(secret string) error {
	if len(secret) < 32 {
		return errors.New("PARAMETER_ENCRYPTION_KEY must contain at least 32 characters")
	}
	key := sha256.Sum256([]byte(secret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return fmt.Errorf("initialize parameter encryption: %w", err)
	}
	parameterAEAD, err = cipher.NewGCM(block)
	return err
}

func IsSensitiveParameterName(name string) bool {
	name = strings.ToLower(name)
	for _, marker := range []string{"password", "passphrase", "presharedkey", "pre_shared_key", "privatekey", "secret", "connectionrequestusername"} {
		if strings.Contains(name, marker) {
			return true
		}
	}
	return strings.HasSuffix(name, ".username")
}

func encryptParameterValue(value string) (string, error) {
	if value == "" || strings.HasPrefix(value, encryptedParameterPrefix) {
		return value, nil
	}
	if parameterAEAD == nil {
		return "", errors.New("parameter encryption is not configured")
	}
	nonce := make([]byte, parameterAEAD.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := parameterAEAD.Seal(nonce, nonce, []byte(value), nil)
	return encryptedParameterPrefix + base64.RawStdEncoding.EncodeToString(ciphertext), nil
}

func decryptParameterValue(value string) (string, error) {
	if !strings.HasPrefix(value, encryptedParameterPrefix) {
		return value, nil
	}
	if parameterAEAD == nil {
		return "", errors.New("parameter encryption is not configured")
	}
	payload, err := base64.RawStdEncoding.DecodeString(strings.TrimPrefix(value, encryptedParameterPrefix))
	if err != nil || len(payload) < parameterAEAD.NonceSize() {
		return "", errors.New("invalid encrypted parameter value")
	}
	nonce := payload[:parameterAEAD.NonceSize()]
	plaintext, err := parameterAEAD.Open(nil, nonce, payload[parameterAEAD.NonceSize():], nil)
	if err != nil {
		return "", fmt.Errorf("decrypt parameter value: %w", err)
	}
	return string(plaintext), nil
}

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

	now := time.Now()
	for index := range params {
		params[index].DeviceID = deviceID
		params[index].UpdatedAt = now
		if IsSensitiveParameterName(params[index].Name) {
			value, err := encryptParameterValue(params[index].Value)
			if err != nil {
				return fmt.Errorf("encrypt %s: %w", params[index].Name, err)
			}
			params[index].Value = value
		}
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "device_id"}, {Name: "name"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at"}),
	}).CreateInBatches(&params, 500).Error
}

func (r *ParameterRepository) UpsertWritable(ctx context.Context, deviceID int64, params []models.DeviceParameter) error {
	if len(params) == 0 {
		return nil
	}
	now := time.Now()
	entries := make([]models.DeviceParameter, 0, len(params))
	for _, param := range params {
		if param.Writable != nil {
			entries = append(entries, models.DeviceParameter{DeviceID: deviceID, Name: param.Name, Writable: param.Writable, UpdatedAt: now})
		}
	}
	if len(entries) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "device_id"}, {Name: "name"}},
		DoUpdates: clause.AssignmentColumns([]string{"writable", "updated_at"}),
	}).CreateInBatches(&entries, 500).Error
}

func (r *ParameterRepository) GetByDeviceID(ctx context.Context, deviceID int64) ([]models.DeviceParameter, error) {
	var params []models.DeviceParameter
	err := r.db.WithContext(ctx).Where("device_id = ?", deviceID).
		Order("name").
		Find(&params).Error
	if err != nil {
		return nil, err
	}
	if err := decryptParameters(params); err != nil {
		return nil, err
	}
	return params, nil
}

func decryptParameters(params []models.DeviceParameter) error {
	for index := range params {
		if !IsSensitiveParameterName(params[index].Name) {
			continue
		}
		value, err := decryptParameterValue(params[index].Value)
		if err != nil {
			return fmt.Errorf("decrypt %s: %w", params[index].Name, err)
		}
		params[index].Value = value
	}
	return nil
}

// EncryptLegacySensitiveValues upgrades plaintext values written by older
// releases. Keyset pagination keeps startup memory bounded on large ACS data
// sets, while each batch is committed atomically.
func (r *ParameterRepository) EncryptLegacySensitiveValues(ctx context.Context) (int64, error) {
	var encrypted int64
	var lastID int64
	for {
		var params []models.DeviceParameter
		if err := r.db.WithContext(ctx).
			Select("id", "name", "value").
			Where("id > ?", lastID).
			Order("id ASC").
			Limit(500).
			Find(&params).Error; err != nil {
			return encrypted, err
		}
		if len(params) == 0 {
			return encrypted, nil
		}

		if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			for _, param := range params {
				lastID = param.ID
				if !IsSensitiveParameterName(param.Name) || param.Value == "" || strings.HasPrefix(param.Value, encryptedParameterPrefix) {
					continue
				}
				value, err := encryptParameterValue(param.Value)
				if err != nil {
					return fmt.Errorf("encrypt %s: %w", param.Name, err)
				}
				if err := tx.Model(&models.DeviceParameter{}).Where("id = ?", param.ID).Update("value", value).Error; err != nil {
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

func (r *ParameterRepository) KnownReadOnly(ctx context.Context, deviceID int64, names []string) ([]string, error) {
	if len(names) == 0 {
		return nil, nil
	}
	var readOnly []string
	err := r.db.WithContext(ctx).Model(&models.DeviceParameter{}).
		Where("device_id = ? AND name IN ? AND writable = ?", deviceID, names, false).
		Order("name ASC").
		Pluck("name", &readOnly).Error
	return readOnly, err
}
