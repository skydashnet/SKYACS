package cwmp

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/skydashnet/miniacs/internal/database"
	"github.com/skydashnet/miniacs/internal/models"
	"github.com/skydashnet/miniacs/internal/netutil"
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
	retryState     map[int64]watchdogRetry
}

type watchdogRetry struct {
	attempts   int
	lastInform time.Time
	updatedAt  time.Time
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
		retryState:     make(map[int64]watchdogRetry),
	}
}

func (w *DeviceWatchdog) Start(ctx context.Context) {
	log.Printf("[Watchdog] Device watchdog started (interval: %v, threshold: %v)", w.interval, w.staleThreshold)

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			w.checkStaleDevices(ctx)
		case <-ctx.Done():
			log.Println("[Watchdog] Device watchdog stopped")
			return
		}
	}
}

func (w *DeviceWatchdog) checkStaleDevices(ctx context.Context) {
	now := time.Now()
	for deviceID, state := range w.retryState {
		if now.Sub(state.updatedAt) > 24*time.Hour {
			delete(w.retryState, deviceID)
		}
	}
	devices, err := w.getStaleOnlineDevices(ctx)
	if err != nil {
		log.Printf("[Watchdog] Error getting stale devices: %v", err)
		return
	}

	if len(devices) == 0 {
		return
	}

	log.Printf("[Watchdog] Found %d stale online devices requiring a connection request", len(devices))

	settings, err := w.settingsRepo.GetAll(ctx)
	if err != nil {
		log.Printf("[Watchdog] Error loading connection request settings: %v", err)
		return
	}
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
		state := w.retryState[device.ID]
		observedInform := time.Time{}
		if device.LastInform != nil {
			observedInform = *device.LastInform
		}
		if !state.lastInform.Equal(observedInform) {
			state = watchdogRetry{lastInform: observedInform}
		}
		state.attempts++
		state.updatedAt = now
		w.retryState[device.ID] = state
		err := w.summonDevice(ctx, device, connReqUsername, connReqPassword, useAutoCredentials)
		if err != nil {
			log.Printf("[Watchdog] Connection request to %s failed (attempt %d/%d): %v",
				device.SerialNumber, state.attempts, w.maxRetries, err)
			if state.attempts == 1 {
				w.logFault(ctx, device, "WATCHDOG_SUMMON_FAILED", err.Error())
			}
		} else {
			log.Printf("[Watchdog] Connection request diterima %s; menunggu Inform", device.SerialNumber)
		}

		if state.attempts >= w.maxRetries {
			log.Printf("[Watchdog] Device %s did not send Inform after %d connection requests; marking offline",
				device.SerialNumber, w.maxRetries)
			if err := w.deviceRepo.SetOffline(ctx, device.SerialNumber); err != nil {
				log.Printf("[Watchdog] Failed to mark %s offline: %v", device.SerialNumber, err)
			}
			w.logFault(ctx, device, "WATCHDOG_MARKED_OFFLINE",
				fmt.Sprintf("Device did not send Inform after %d connection requests", w.maxRetries))
			delete(w.retryState, device.ID)
		}

		time.Sleep(500 * time.Millisecond)
	}
}

func (w *DeviceWatchdog) getStaleOnlineDevices(ctx context.Context) ([]*models.Device, error) {
	threshold := time.Now().Add(-w.staleThreshold)
	var devices []*models.Device
	err := w.db.WithContext(ctx).
		Where("online = ? AND last_inform < ? AND connection_request_url IS NOT NULL AND connection_request_url != ''", true, threshold).
		Order("last_inform ASC").
		Limit(25).
		Find(&devices).Error
	return devices, err
}

func (w *DeviceWatchdog) summonDevice(ctx context.Context, device *models.Device, username, password string, useAuto bool) error {
	if device.ConnectionRequestURL == nil || *device.ConnectionRequestURL == "" {
		return nil
	}

	connReqURL := *device.ConnectionRequestURL
	if useAuto {
		username = device.SerialNumber
		var err error
		password, err = netutil.DeriveDevicePassword(device.SerialNumber, password)
		if err != nil {
			return err
		}
	}

	client, err := netutil.NewDeviceHTTPClient(connReqURL, username, password, 10*time.Second)
	if err != nil {
		return err
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
		log.Printf("[Watchdog] Failed to record fault: %v", err)
	}
}

type SummonError struct {
	StatusCode int
	Status     string
}

func (e *SummonError) Error() string {
	return "Device merespons dengan status " + e.Status
}
