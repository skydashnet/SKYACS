package cwmp

import (
	"context"
	"crypto/md5"
	"encoding/binary"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/icholy/digest"
	"github.com/skydashnet/miniacs/internal/database"
	"github.com/skydashnet/miniacs/internal/models"
	"gorm.io/gorm"
)

type DeviceWatchdog struct {
	db             *gorm.DB
	deviceRepo     *database.DeviceRepository
	faultRepo      *database.FaultRepository
	settingsRepo   *database.SettingsRepository
	interval       time.Duration
	staleThreshold time.Duration
	maxRetries     int
	retryCount     map[int64]int
}

func NewDeviceWatchdog(db *gorm.DB) *DeviceWatchdog {
	return &DeviceWatchdog{
		db:             db,
		deviceRepo:     database.NewDeviceRepository(db),
		faultRepo:      database.NewFaultRepository(db),
		settingsRepo:   database.NewSettingsRepository(db),
		interval:       10 * time.Minute,
		staleThreshold: 15 * time.Minute,
		maxRetries:     3,
		retryCount:     make(map[int64]int),
	}
}

func (w *DeviceWatchdog) Start(ctx context.Context) {
	log.Printf("[Watchdog] Device watchdog started (interval: %v, threshold: %v)", w.interval, w.staleThreshold)

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			w.cekDeviceBandel(ctx)
		case <-ctx.Done():
			log.Println("[Watchdog] Device watchdog stopped")
			return
		}
	}
}

func (w *DeviceWatchdog) cekDeviceBandel(ctx context.Context) {
	devices, err := w.getStaleOnlineDevices(ctx)
	if err != nil {
		log.Printf("[Watchdog] Error getting stale devices: %v", err)
		return
	}

	if len(devices) == 0 {
		return
	}

	log.Printf("[Watchdog] Ditemukan %d device bandel yang perlu di-summon", len(devices))

	settings, _ := w.settingsRepo.GetAll(ctx)
	connReqUsername := ""
	connReqPassword := ""
	useAutoCredentials := false
	for _, s := range settings {
		if s.Key == "connection_request_username" {
			connReqUsername = s.Value
		}
		if s.Key == "connection_request_password" {
			connReqPassword = s.Value
		}
		if s.Key == "use_auto_conn_credentials" && s.Value == "true" {
			useAutoCredentials = true
		}
	}

	for _, device := range devices {
		err := w.summonDevice(ctx, device, connReqUsername, connReqPassword, useAutoCredentials)
		if err != nil {
			w.retryCount[device.ID]++
			log.Printf("[Watchdog] Gagal summon %s (attempt %d/%d): %v",
				device.SerialNumber, w.retryCount[device.ID], w.maxRetries, err)

			w.logFault(ctx, device, "WATCHDOG_SUMMON_FAILED", err.Error())

			if w.retryCount[device.ID] >= w.maxRetries {
				log.Printf("[Watchdog] Device %s sudah %dx gagal, mark sebagai offline",
					device.SerialNumber, w.maxRetries)
				w.deviceRepo.SetOffline(ctx, device.SerialNumber)
				w.logFault(ctx, device, "WATCHDOG_MARKED_OFFLINE",
					"Device tidak responsif setelah "+string(rune(w.maxRetries))+" kali percobaan summon")
				delete(w.retryCount, device.ID)
			}
		} else {
			log.Printf("[Watchdog] Berhasil summon %s", device.SerialNumber)
			delete(w.retryCount, device.ID)
		}

		time.Sleep(500 * time.Millisecond)
	}
}

func (w *DeviceWatchdog) getStaleOnlineDevices(ctx context.Context) ([]*models.Device, error) {
	threshold := time.Now().Add(-w.staleThreshold)
	var devices []*models.Device
	err := w.db.WithContext(ctx).
		Where("online = ? AND last_inform < ? AND connection_request_url IS NOT NULL AND connection_request_url != ''", true, threshold).
		Limit(10).
		Find(&devices).Error
	return devices, err
}

func (w *DeviceWatchdog) summonDevice(ctx context.Context, device *models.Device, username, password string, useAuto bool) error {
	if device.ConnectionRequestURL == nil || *device.ConnectionRequestURL == "" {
		return nil
	}

	connReqURL := *device.ConnectionRequestURL

	// Jika mode auto atau credentials kosong, pakai serial number
	if useAuto || username == "" {
		username = device.SerialNumber
	}
	if useAuto || password == "" {
		password = generateGenieACSPassword(device.SerialNumber)
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &digest.Transport{
			Username: username,
			Password: password,
		},
	}

	httpReq, err := http.NewRequestWithContext(ctx, "GET", connReqURL, nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	return &SummonError{StatusCode: resp.StatusCode, Status: resp.Status}
}

func (w *DeviceWatchdog) logFault(ctx context.Context, device *models.Device, code, message string) {
	fault := &models.Fault{
		DeviceID:    device.ID,
		FaultCode:   code,
		FaultString: message,
	}
	if err := w.faultRepo.Create(ctx, fault); err != nil {
		log.Printf("[Watchdog] Gagal log fault: %v", err)
	}
}

type SummonError struct {
	StatusCode int
	Status     string
}

func (e *SummonError) Error() string {
	return "Device merespons dengan status " + e.Status
}

// generateGenieACSPassword generates password compatible with GenieACS
func generateGenieACSPassword(serialNumber string) string {
	hash := md5.Sum([]byte(serialNumber))
	seed := binary.BigEndian.Uint64(hash[:8])
	maxSafeInt := uint64(9007199254740991)
	value := seed % maxSafeInt
	return strconv.FormatUint(value, 36)
}
