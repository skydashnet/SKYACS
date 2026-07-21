package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/skydashnet/skyacs/internal/auth"
	"github.com/skydashnet/skyacs/internal/database"
	"github.com/skydashnet/skyacs/internal/models"
	"github.com/skydashnet/skyacs/internal/netutil"
	"gorm.io/gorm"
)

type Router struct {
	db               *gorm.DB
	deviceRepo       *database.DeviceRepository
	settingsRepo     *database.SettingsRepository
	taskRepo         *database.TaskRepository
	parameterRepo    *database.ParameterRepository
	firmwareRepo     *database.FirmwareRepository
	faultRepo        *database.FaultRepository
	userRepo         *database.UserRepository
	provisioningRepo *database.ProvisioningRepository
	auditRepo        *database.AuditRepository
	blockedRepo      *database.BlockedDeviceRepository
	loginLimiter     *loginLimiter
	downloadLimiter  *loginLimiter
	uploadDir        string
	uploadInitErr    error
}

func NewRouter(db *gorm.DB) *Router {
	uploadDir := os.Getenv("FIRMWARE_UPLOAD_DIR")
	if uploadDir == "" {
		uploadDir = "./uploads/firmware"
	}
	uploadInitErr := os.MkdirAll(uploadDir, 0750)
	if uploadInitErr == nil {
		var probe *os.File
		probe, uploadInitErr = os.CreateTemp(uploadDir, ".write-probe-*")
		if uploadInitErr == nil {
			probeName := probe.Name()
			uploadInitErr = probe.Close()
			if removeErr := os.Remove(probeName); uploadInitErr == nil {
				uploadInitErr = removeErr
			}
		}
	}
	if uploadInitErr != nil {
		log.Printf("Firmware storage is not writable: %v", uploadInitErr)
	}

	return &Router{
		db:               db,
		deviceRepo:       database.NewDeviceRepository(db),
		settingsRepo:     database.NewSettingsRepository(db),
		taskRepo:         database.NewTaskRepository(db),
		parameterRepo:    database.NewParameterRepository(db),
		firmwareRepo:     database.NewFirmwareRepository(db),
		faultRepo:        database.NewFaultRepository(db),
		userRepo:         database.NewUserRepository(db),
		provisioningRepo: database.NewProvisioningRepository(db),
		auditRepo:        database.NewAuditRepository(db),
		blockedRepo:      database.NewBlockedDeviceRepository(db),
		loginLimiter:     newLoginLimiter(5, 15*time.Minute),
		downloadLimiter:  newLoginLimiter(60, time.Minute),
		uploadDir:        uploadDir,
		uploadInitErr:    uploadInitErr,
	}
}

func (r *Router) Handler() http.Handler {
	mux := http.NewServeMux()

	// Public endpoints (no auth required)
	mux.HandleFunc("GET /health", r.handleHealth)
	mux.HandleFunc("POST /auth/login", r.handleLogin)
	mux.HandleFunc("GET /files/{id}/{filename}", r.handleFirmwareFile)

	// Protected API mux
	apiMux := http.NewServeMux()

	// Auth endpoints
	apiMux.HandleFunc("GET /auth/me", r.handleAuthMe)
	apiMux.HandleFunc("POST /auth/change-password", r.handleChangePassword)

	// User management (full access only)
	apiMux.HandleFunc("GET /users", r.handleListUsers)
	apiMux.HandleFunc("POST /users", auth.RequireFullAccess(r.handleCreateUser))
	apiMux.HandleFunc("PUT /users/{id}", auth.RequireFullAccess(r.handleUpdateUser))
	apiMux.HandleFunc("DELETE /users/{id}", auth.RequireFullAccess(r.handleDeleteUser))

	// Device endpoints (by ID - legacy)
	apiMux.HandleFunc("GET /devices", r.handleListDevices)
	apiMux.HandleFunc("GET /devices/stats", r.handleDeviceStats)
	apiMux.HandleFunc("GET /devices/analytics", r.handleDeviceAnalytics)
	apiMux.HandleFunc("GET /devices/{id}", r.handleGetDevice)
	apiMux.HandleFunc("DELETE /devices/{id}", auth.RequireFullAccess(r.handleDeleteDevice))
	apiMux.HandleFunc("GET /devices/{id}/parameters", r.handleGetDeviceParameters)
	apiMux.HandleFunc("GET /devices/{id}/tasks", r.handleGetDeviceTasks)

	// Device actions (by ID - legacy)
	apiMux.HandleFunc("POST /devices/{id}/get-parameters", auth.RequireFullAccess(r.handleGetParameterValues))
	apiMux.HandleFunc("POST /devices/{id}/set-parameters", auth.RequireFullAccess(r.handleSetParameterValues))
	apiMux.HandleFunc("POST /devices/{id}/reboot", auth.RequireFullAccess(r.handleReboot))
	apiMux.HandleFunc("POST /devices/{id}/factory-reset", auth.RequireFullAccess(r.handleFactoryReset))
	apiMux.HandleFunc("POST /devices/{id}/connection-request", auth.RequireFullAccess(r.handleConnectionRequest))
	apiMux.HandleFunc("POST /devices/{id}/download-firmware", auth.RequireFullAccess(r.handleDownloadFirmware))

	// Device endpoints (by serial - new)
	apiMux.HandleFunc("GET /device/{serial}", r.handleGetDeviceBySerial)
	apiMux.HandleFunc("DELETE /device/{serial}", auth.RequireFullAccess(r.handleDeleteDeviceBySerial))
	apiMux.HandleFunc("GET /device/{serial}/parameters", r.handleGetDeviceParametersBySerial)
	apiMux.HandleFunc("GET /device/{serial}/tasks", r.handleGetDeviceTasksBySerial)
	apiMux.HandleFunc("POST /device/{serial}/get-parameters", auth.RequireFullAccess(r.handleGetParameterValuesBySerial))
	apiMux.HandleFunc("POST /device/{serial}/set-parameters", auth.RequireFullAccess(r.handleSetParameterValuesBySerial))
	apiMux.HandleFunc("POST /device/{serial}/reboot", auth.RequireFullAccess(r.handleRebootBySerial))
	apiMux.HandleFunc("POST /device/{serial}/factory-reset", auth.RequireFullAccess(r.handleFactoryResetBySerial))
	apiMux.HandleFunc("POST /device/{serial}/connection-request", auth.RequireFullAccess(r.handleConnectionRequestBySerial))
	apiMux.HandleFunc("POST /device/{serial}/download-firmware", auth.RequireFullAccess(r.handleDownloadFirmwareBySerial))

	// Firmware endpoints
	apiMux.HandleFunc("GET /firmwares", r.handleListFirmwares)
	apiMux.HandleFunc("POST /firmwares", auth.RequireFullAccess(r.handleUploadFirmware))
	apiMux.HandleFunc("DELETE /firmwares/{id}", auth.RequireFullAccess(r.handleDeleteFirmware))

	// Settings endpoints
	apiMux.HandleFunc("GET /settings", r.handleGetSettings)
	apiMux.HandleFunc("PUT /settings", auth.RequireFullAccess(r.handleUpdateSettings))

	// Faults endpoints
	apiMux.HandleFunc("GET /faults", r.handleListFaults)
	apiMux.HandleFunc("GET /faults/stats", r.handleFaultStats)
	apiMux.HandleFunc("POST /faults/{id}/resolve", auth.RequireFullAccess(r.handleResolveFault))
	apiMux.HandleFunc("DELETE /faults/{id}", auth.RequireFullAccess(r.handleDeleteFault))

	// Provisioning endpoints
	apiMux.HandleFunc("GET /provisioning", r.handleListProvisioningRules)
	apiMux.HandleFunc("POST /provisioning", auth.RequireFullAccess(r.handleCreateProvisioningRule))
	apiMux.HandleFunc("PUT /provisioning/{id}", auth.RequireFullAccess(r.handleUpdateProvisioningRule))
	apiMux.HandleFunc("DELETE /provisioning/{id}", auth.RequireFullAccess(r.handleDeleteProvisioningRule))
	apiMux.HandleFunc("POST /provisioning/{id}/toggle", auth.RequireFullAccess(r.handleToggleProvisioningRule))

	// Security center (full access only)
	apiMux.HandleFunc("GET /security/overview", auth.RequireFullAccess(r.handleSecurityOverview))
	apiMux.HandleFunc("GET /audit-logs", auth.RequireFullAccess(r.handleListAuditLogs))
	apiMux.HandleFunc("GET /blocked-devices", auth.RequireFullAccess(r.handleListBlockedDevices))
	apiMux.HandleFunc("POST /blocked-devices", auth.RequireFullAccess(r.handleAddBlockedDevice))
	apiMux.HandleFunc("DELETE /blocked-devices/{serial}", auth.RequireFullAccess(r.handleRemoveBlockedDevice))

	// Authentication precedes the audit logger so actor details are available.
	mux.Handle("/", auth.AuthMiddleware(r.userRepo, auditMiddleware(r.auditRepo, apiMux)))

	return securityHeaders(corsMiddleware(requestBodyLimit(mux)))
}

func (r *Router) handleHealth(w http.ResponseWriter, req *http.Request) {
	if r.uploadInitErr != nil {
		respondError(w, http.StatusServiceUnavailable, "firmware storage unavailable")
		return
	}
	sqlDB, err := r.db.DB()
	if err != nil {
		respondError(w, http.StatusServiceUnavailable, "database unavailable")
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 2*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		respondError(w, http.StatusServiceUnavailable, "database unavailable")
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (r *Router) handleListDevices(w http.ResponseWriter, req *http.Request) {
	limit := 50
	offset := 0

	if l := req.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	if o := req.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	devices, total, err := r.deviceRepo.List(req.Context(), limit, offset)
	if err != nil {
		log.Printf("Error listing devices: %v", err)
		respondError(w, http.StatusInternalServerError, "Failed to list devices")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"devices": devices,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
	})
}

func (r *Router) handleGetDevice(w http.ResponseWriter, req *http.Request) {
	id, err := r.parseDeviceID(req)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid device ID")
		return
	}

	device, err := r.deviceRepo.GetByID(req.Context(), id)
	if err != nil {
		log.Printf("Error getting device: %v", err)
		respondError(w, http.StatusInternalServerError, "Failed to get device")
		return
	}

	if device == nil {
		respondError(w, http.StatusNotFound, "Device not found")
		return
	}

	respondJSON(w, http.StatusOK, device)
}

func (r *Router) handleDeleteDevice(w http.ResponseWriter, req *http.Request) {
	id, err := r.parseDeviceID(req)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid device ID")
		return
	}

	if err := r.deviceRepo.Delete(req.Context(), id); err != nil {
		log.Printf("Error deleting device: %v", err)
		respondError(w, http.StatusInternalServerError, "Failed to delete device")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (r *Router) handleDeviceStats(w http.ResponseWriter, req *http.Request) {
	stats, err := r.deviceRepo.GetStats(req.Context())
	if err != nil {
		log.Printf("Error getting stats: %v", err)
		respondError(w, http.StatusInternalServerError, "Failed to get stats")
		return
	}

	respondJSON(w, http.StatusOK, stats)
}

func (r *Router) handleDeviceAnalytics(w http.ResponseWriter, req *http.Request) {
	devices, totalDevices, err := r.deviceRepo.ListForAnalytics(req.Context(), 10000)
	if err != nil {
		log.Printf("Error getting devices for analytics: %v", err)
		respondError(w, http.StatusInternalServerError, "Failed to get analytics")
		return
	}

	// Aggregate data
	rxPowerRanges := map[string]int{
		"Sangat Bagus (-15 to -23)": 0,
		"Normal (-23 to -25)":       0,
		"Buruk (-25 to -29)":        0,
		"Kritis (< -29)":            0,
		"N/A":                       0,
	}

	tempRanges := map[string]int{
		"Normal (< 45)":   0,
		"Warm (45-55)":    0,
		"Hot (55-65)":     0,
		"Critical (> 65)": 0,
		"N/A":             0,
	}

	accessTypes := map[string]int{
		"EPON":  0,
		"GPON":  0,
		"Other": 0,
	}

	uptimeRanges := map[string]int{
		"< 1 jam":   0,
		"1-24 jam":  0,
		"1-7 hari":  0,
		"1-30 hari": 0,
		"> 1 bulan": 0,
	}

	lastInformRanges := map[string]int{
		"Online":       0,
		"Offline >30m": 0,
		"Offline >1h":  0,
		"Offline >1d":  0,
		"Offline >7d":  0,
		"Offline >30d": 0,
	}

	wifiStationsRanges := map[string]int{
		"0 Stations":    0,
		"1-5 Stations":  0,
		"6-10 Stations": 0,
		"10+ Stations":  0,
	}
	manufacturers := make(map[string]int)
	productClasses := make(map[string]int)

	now := time.Now()

	for _, device := range devices {
		manufacturer := "Unknown"
		if device.Manufacturer != nil && strings.TrimSpace(*device.Manufacturer) != "" {
			manufacturer = *device.Manufacturer
		}
		manufacturers[manufacturer]++
		productClass := "Unknown"
		if device.ProductClass != nil && strings.TrimSpace(*device.ProductClass) != "" {
			productClass = *device.ProductClass
		} else if device.ModelName != nil && strings.TrimSpace(*device.ModelName) != "" {
			productClass = *device.ModelName
		}
		productClasses[productClass]++
		if device.LastInform != nil {
			diff := now.Sub(*device.LastInform)
			if diff < 5*time.Minute {
				lastInformRanges["Online"]++
			} else if diff < 30*time.Minute {
				lastInformRanges["Online"]++
			} else if diff < time.Hour {
				lastInformRanges["Offline >30m"]++
			} else if diff < 24*time.Hour {
				lastInformRanges["Offline >1h"]++
			} else if diff < 7*24*time.Hour {
				lastInformRanges["Offline >1d"]++
			} else if diff < 30*24*time.Hour {
				lastInformRanges["Offline >7d"]++
			} else {
				lastInformRanges["Offline >30d"]++
			}
		} else {
			lastInformRanges["Offline >30d"]++
		}

		var deviceRxPower float64 = -999
		var deviceTemp float64 = -999
		var deviceUptime float64 = -1
		var deviceAccessType string
		totalStations := 0

		for _, p := range device.Parameters {
			// RX Power - ambil nilai pertama yang valid
			if deviceRxPower == -999 && (strings.Contains(p.Name, "RXPower") || strings.Contains(p.Name, "RxPower")) {
				if val, err := strconv.ParseFloat(p.Value, 64); err == nil {
					if val > 0 && val < 10000 {
						val = 10 * math.Log10(val/10000)
					} else if val > 100 {
						val = val / 100
					}
					deviceRxPower = val
				}
			}

			// Temperature - ambil nilai pertama yang valid
			if deviceTemp == -999 && (strings.Contains(p.Name, "Temperature") || strings.Contains(p.Name, "Temp")) {
				if val, err := strconv.ParseFloat(p.Value, 64); err == nil {
					if val > 1000 {
						val = val / 256
					} else if val > 100 {
						val = val / 10
					}
					deviceTemp = val
				}
			}

			// Uptime - prioritaskan DeviceInfo.UpTime
			if deviceUptime == -1 && (strings.HasSuffix(p.Name, "DeviceInfo.UpTime") || strings.HasSuffix(p.Name, "UpTime")) {
				if val, err := strconv.ParseFloat(p.Value, 64); err == nil {
					deviceUptime = val
				}
			}

			// Access Type - ambil pertama yang valid
			if deviceAccessType == "" && (strings.Contains(p.Name, "PONMode") || strings.Contains(p.Name, "AccessType")) {
				valUpper := strings.ToUpper(p.Value)
				if strings.Contains(valUpper, "EPON") {
					deviceAccessType = "EPON"
				} else if strings.Contains(valUpper, "GPON") {
					deviceAccessType = "GPON"
				} else if strings.Contains(valUpper, "ETH") {
					deviceAccessType = "Ethernet"
				} else if p.Value != "" {
					deviceAccessType = "Other"
				}
			}

			// WiFi Stations - sum semua
			if strings.Contains(p.Name, "TotalAssociations") || strings.Contains(p.Name, "AssociatedDeviceNumberOfEntries") {
				if val, err := strconv.Atoi(p.Value); err == nil {
					totalStations += val
				}
			}
		}

		// Count RX Power
		if deviceRxPower != -999 {
			if deviceRxPower >= -23 && deviceRxPower <= -15 {
				rxPowerRanges["Sangat Bagus (-15 to -23)"]++
			} else if deviceRxPower >= -25 && deviceRxPower < -23 {
				rxPowerRanges["Normal (-23 to -25)"]++
			} else if deviceRxPower >= -29 && deviceRxPower < -25 {
				rxPowerRanges["Buruk (-25 to -29)"]++
			} else if deviceRxPower < -29 {
				rxPowerRanges["Kritis (< -29)"]++
			}
		} else {
			rxPowerRanges["N/A"]++
		}

		// Count Temperature
		if deviceTemp != -999 {
			if deviceTemp < 45 {
				tempRanges["Normal (< 45)"]++
			} else if deviceTemp < 55 {
				tempRanges["Warm (45-55)"]++
			} else if deviceTemp < 65 {
				tempRanges["Hot (55-65)"]++
			} else {
				tempRanges["Critical (> 65)"]++
			}
		} else {
			tempRanges["N/A"]++
		}

		// Count Uptime
		if deviceUptime >= 0 {
			hours := deviceUptime / 3600
			if hours < 1 {
				uptimeRanges["< 1 jam"]++
			} else if hours < 24 {
				uptimeRanges["1-24 jam"]++
			} else if hours < 168 {
				uptimeRanges["1-7 hari"]++
			} else if hours < 720 {
				uptimeRanges["1-30 hari"]++
			} else {
				uptimeRanges["> 1 bulan"]++
			}
		}

		// Count Access Type
		if deviceAccessType != "" {
			accessTypes[deviceAccessType]++
		} else {
			accessTypes["Other"]++
		}

		// Count WiFi Stations
		if totalStations == 0 {
			wifiStationsRanges["0 Stations"]++
		} else if totalStations <= 5 {
			wifiStationsRanges["1-5 Stations"]++
		} else if totalStations <= 10 {
			wifiStationsRanges["6-10 Stations"]++
		} else {
			wifiStationsRanges["10+ Stations"]++
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"rxPower":        rxPowerRanges,
		"temperature":    tempRanges,
		"uptime":         uptimeRanges,
		"accessType":     accessTypes,
		"lastInform":     lastInformRanges,
		"wifiStations":   wifiStationsRanges,
		"manufacturers":  manufacturers,
		"productClasses": productClasses,
		"sampled":        len(devices),
		"total":          totalDevices,
	})
}

func (r *Router) handleGetDeviceParameters(w http.ResponseWriter, req *http.Request) {
	id, err := r.parseDeviceID(req)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid device ID")
		return
	}

	params, err := r.parameterRepo.GetByDeviceID(req.Context(), id)
	if err != nil {
		log.Printf("Error getting parameters: %v", err)
		respondError(w, http.StatusInternalServerError, "Failed to get parameters")
		return
	}
	redactSensitiveParameters(req, params)

	respondJSON(w, http.StatusOK, params)
}

func (r *Router) handleGetDeviceTasks(w http.ResponseWriter, req *http.Request) {
	id, err := r.parseDeviceID(req)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid device ID")
		return
	}

	tasks, err := r.taskRepo.GetByDeviceID(req.Context(), id, 50)
	if err != nil {
		log.Printf("Error getting tasks: %v", err)
		respondError(w, http.StatusInternalServerError, "Failed to get tasks")
		return
	}

	respondJSON(w, http.StatusOK, tasks)
}

func (r *Router) handleGetParameterValues(w http.ResponseWriter, req *http.Request) {
	id, err := r.parseDeviceID(req)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid device ID")
		return
	}

	var payload struct {
		Parameters []string `json:"parameters"`
	}
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	if len(payload.Parameters) == 0 {
		respondError(w, http.StatusBadRequest, "No parameters specified")
		return
	}
	if err := validateParameterNames(payload.Parameters); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	task, err := models.NewTaskWithPayload(id, models.TaskTypeGetParameterValues, payload.Parameters)
	if err != nil {
		log.Printf("Error creating task payload: %v", err)
		respondError(w, http.StatusInternalServerError, "Failed to create task")
		return
	}

	if err := r.taskRepo.Create(req.Context(), task); err != nil {
		log.Printf("Error creating task: %v", err)
		respondError(w, http.StatusInternalServerError, "Failed to create task")
		return
	}

	respondJSON(w, http.StatusCreated, task)
}

func (r *Router) handleSetParameterValues(w http.ResponseWriter, req *http.Request) {
	id, err := r.parseDeviceID(req)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid device ID")
		return
	}

	var payload struct {
		Parameters map[string]string `json:"parameters"`
	}
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	if len(payload.Parameters) == 0 {
		respondError(w, http.StatusBadRequest, "No parameters specified")
		return
	}
	if err := validateSetParameters(payload.Parameters); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := r.rejectKnownReadOnly(req.Context(), id, payload.Parameters); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	task, err := models.NewTaskWithPayload(id, models.TaskTypeSetParameterValues, payload.Parameters)
	if err != nil {
		log.Printf("Error creating task payload: %v", err)
		respondError(w, http.StatusInternalServerError, "Failed to create task")
		return
	}

	if err := r.taskRepo.Create(req.Context(), task); err != nil {
		log.Printf("Error creating task: %v", err)
		respondError(w, http.StatusInternalServerError, "Failed to create task")
		return
	}

	respondJSON(w, http.StatusCreated, task)
}

func (r *Router) handleReboot(w http.ResponseWriter, req *http.Request) {
	id, err := r.parseDeviceID(req)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid device ID")
		return
	}

	task, err := models.NewTaskWithPayload(id, models.TaskTypeReboot, nil)
	if err != nil {
		log.Printf("Error creating task payload: %v", err)
		respondError(w, http.StatusInternalServerError, "Failed to create task")
		return
	}

	if err := r.taskRepo.Create(req.Context(), task); err != nil {
		log.Printf("Error creating task: %v", err)
		respondError(w, http.StatusInternalServerError, "Failed to create task")
		return
	}

	respondJSON(w, http.StatusCreated, task)
}

func (r *Router) handleFactoryReset(w http.ResponseWriter, req *http.Request) {
	if !r.requireCurrentPassword(w, req) {
		return
	}
	id, err := r.parseDeviceID(req)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid device ID")
		return
	}

	task, err := models.NewTaskWithPayload(id, models.TaskTypeFactoryReset, nil)
	if err != nil {
		log.Printf("Error creating task payload: %v", err)
		respondError(w, http.StatusInternalServerError, "Failed to create task")
		return
	}

	if err := r.taskRepo.Create(req.Context(), task); err != nil {
		log.Printf("Error creating task: %v", err)
		respondError(w, http.StatusInternalServerError, "Failed to create task")
		return
	}

	respondJSON(w, http.StatusCreated, task)
}

func (r *Router) requireCurrentPassword(w http.ResponseWriter, req *http.Request) bool {
	key := "reauth:" + clientIP(req)
	if !r.loginLimiter.Allow(key) {
		w.Header().Set("Retry-After", "900")
		respondError(w, http.StatusTooManyRequests, "Too many reauthentication attempts; try again later")
		return false
	}
	var body struct {
		CurrentPassword string `json:"current_password"`
	}
	decoder := json.NewDecoder(req.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil || body.CurrentPassword == "" {
		respondError(w, http.StatusBadRequest, "Current password is required")
		return false
	}
	claims := auth.GetUserFromContext(req.Context())
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return false
	}
	user, err := r.userRepo.GetByID(req.Context(), claims.UserID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to verify current password")
		return false
	}
	if user == nil || !auth.CheckPassword(body.CurrentPassword, user.PasswordHash) {
		respondError(w, http.StatusForbidden, "Current password is incorrect")
		return false
	}
	r.loginLimiter.Reset(key)
	return true
}

func (r *Router) handleConnectionRequest(w http.ResponseWriter, req *http.Request) {
	id, err := r.parseDeviceID(req)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid device ID")
		return
	}

	device, err := r.deviceRepo.GetByID(req.Context(), id)
	if err != nil || device == nil {
		respondError(w, http.StatusNotFound, "Device not found")
		return
	}
	r.executeConnectionRequest(w, req, device)
}

func (r *Router) executeConnectionRequest(w http.ResponseWriter, req *http.Request, device *models.Device) {
	if device.ConnectionRequestURL == nil || *device.ConnectionRequestURL == "" {
		respondError(w, http.StatusBadRequest, "Device has no connection request URL")
		return
	}
	connReqURL := *device.ConnectionRequestURL
	settings, err := r.settingsRepo.GetAll(req.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to load connection request settings")
		return
	}
	username, password, useAuto := "", "", false
	for _, setting := range settings {
		switch setting.Key {
		case "connection_request_username":
			username = setting.Value
		case "connection_request_password":
			password = setting.Value
		case "use_auto_conn_credentials":
			useAuto = setting.Value == "true"
		}
	}
	if useAuto {
		username = device.SerialNumber
		password, err = netutil.DeriveDevicePassword(device.SerialNumber, password)
		if err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	client, err := netutil.NewDeviceHTTPClient(connReqURL, username, password, 10*time.Second)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Unsafe connection request URL: "+err.Error())
		return
	}
	httpReq, err := http.NewRequestWithContext(req.Context(), http.MethodGet, connReqURL, nil)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to create connection request")
		return
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		log.Printf("[ConnReq] %s failed: %v", device.SerialNumber, err)
		respondJSON(w, http.StatusOK, map[string]string{"status": "failed", "url": connReqURL, "message": "Device tidak dapat dijangkau: " + err.Error()})
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		respondJSON(w, http.StatusOK, map[string]string{"status": "success", "url": connReqURL, "message": "Device akan mengirim Inform dalam beberapa detik"})
		return
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		respondJSON(w, http.StatusOK, map[string]string{"status": "auth_failed", "url": connReqURL, "message": "Connection request credentials were rejected by the device"})
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "unknown", "url": connReqURL, "message": "Device merespons dengan status " + resp.Status})
}

func (r *Router) handleGetSettings(w http.ResponseWriter, req *http.Request) {
	settings, err := r.settingsRepo.GetAll(req.Context())
	if err != nil {
		log.Printf("Error getting settings: %v", err)
		respondError(w, http.StatusInternalServerError, "Failed to get settings")
		return
	}

	result := make(map[string]string)
	for _, s := range settings {
		if isSecretSetting(s.Key) && s.Value != "" {
			result[s.Key] = "••••••••"
		} else {
			result[s.Key] = s.Value
		}
	}

	respondJSON(w, http.StatusOK, result)
}

func (r *Router) handleUpdateSettings(w http.ResponseWriter, req *http.Request) {
	var settings map[string]string
	decoder := json.NewDecoder(req.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&settings); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	allowed := map[string]bool{
		"firmware_base_url": true, "connection_request_username": true,
		"connection_request_password": true, "use_auto_conn_credentials": true,
	}
	for key, value := range settings {
		if !allowed[key] {
			respondError(w, http.StatusBadRequest, "Unsupported setting: "+key)
			return
		}
		if isSecretSetting(key) && value == "••••••••" {
			delete(settings, key)
		}
	}
	for _, key := range []string{"firmware_base_url"} {
		if raw, ok := settings[key]; ok {
			raw = strings.TrimSpace(raw)
			if raw == "" {
				settings[key] = ""
				continue
			}
			parsed, err := url.Parse(raw)
			if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
				respondError(w, http.StatusBadRequest, key+" must be an absolute HTTP or HTTPS URL")
				return
			}
			settings[key] = raw
		}
	}
	if value, ok := settings["use_auto_conn_credentials"]; ok && value != "true" && value != "false" {
		respondError(w, http.StatusBadRequest, "use_auto_conn_credentials must be true or false")
		return
	}
	autoCredentials := settings["use_auto_conn_credentials"] == "true"
	if _, supplied := settings["use_auto_conn_credentials"]; !supplied {
		current, err := r.settingsRepo.Get(req.Context(), "use_auto_conn_credentials")
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to validate connection request mode")
			return
		}
		autoCredentials = current != nil && current.Value == "true"
	}
	if autoCredentials {
		masterSecret := settings["connection_request_password"]
		if masterSecret == "" {
			current, err := r.settingsRepo.Get(req.Context(), "connection_request_password")
			if err != nil {
				respondError(w, http.StatusInternalServerError, "Failed to validate connection request secret")
				return
			}
			if current != nil {
				masterSecret = current.Value
			}
		}
		if len(masterSecret) < 16 {
			respondError(w, http.StatusBadRequest, "Auto credentials require a connection request master secret of at least 16 characters")
			return
		}
	}

	if err := r.settingsRepo.SetMultiple(req.Context(), settings); err != nil {
		log.Printf("Error updating settings: %v", err)
		respondError(w, http.StatusInternalServerError, "Failed to update settings")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func isSecretSetting(key string) bool {
	return key == "connection_request_password"
}

func (r *Router) handleListFirmwares(w http.ResponseWriter, req *http.Request) {
	firmwares, err := r.firmwareRepo.List(req.Context())
	if err != nil {
		log.Printf("Error listing firmwares: %v", err)
		respondError(w, http.StatusInternalServerError, "Failed to list firmwares")
		return
	}
	respondJSON(w, http.StatusOK, firmwares)
}

func (r *Router) handleUploadFirmware(w http.ResponseWriter, req *http.Request) {
	if err := req.ParseMultipartForm(maxFirmwareBody); err != nil {
		respondError(w, http.StatusBadRequest, "File too large or invalid form")
		return
	}

	file, header, err := req.FormFile("file")
	if err != nil {
		respondError(w, http.StatusBadRequest, "No file uploaded")
		return
	}
	defer file.Close()

	originalFilename := filepath.Base(header.Filename)
	if originalFilename == "." || len(originalFilename) > 255 || strings.ContainsAny(originalFilename, "\r\n\x00") {
		respondError(w, http.StatusBadRequest, "Invalid firmware filename")
		return
	}
	extension := strings.ToLower(filepath.Ext(originalFilename))
	allowedExtensions := map[string]bool{".bin": true, ".img": true, ".tar": true, ".gz": true, ".zip": true}
	if !allowedExtensions[extension] {
		respondError(w, http.StatusBadRequest, "Unsupported firmware file type")
		return
	}
	version := strings.TrimSpace(req.FormValue("version"))
	if version == "" || len(version) > 128 || strings.ContainsAny(version, "\r\n\x00") {
		respondError(w, http.StatusBadRequest, "Firmware version is required and must be under 128 characters")
		return
	}
	manufacturer := strings.TrimSpace(req.FormValue("manufacturer"))
	productClass := strings.TrimSpace(req.FormValue("product_class"))
	if manufacturer == "" || productClass == "" || len(manufacturer) > 128 || len(productClass) > 128 || strings.ContainsAny(manufacturer+productClass, "\r\n\x00") {
		respondError(w, http.StatusBadRequest, "Firmware manufacturer and product class are required and must be under 128 characters")
		return
	}
	var description *string
	if value := strings.TrimSpace(req.FormValue("description")); value != "" {
		if len(value) > 2048 || strings.ContainsRune(value, '\x00') {
			respondError(w, http.StatusBadRequest, "Firmware description is invalid or exceeds 2048 characters")
			return
		}
		description = &value
	}

	tempFile, err := os.CreateTemp(r.uploadDir, ".firmware-*")
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to save file")
		return
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)

	hash := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(tempFile, hash), file)
	closeErr := tempFile.Close()
	if copyErr != nil || closeErr != nil || written == 0 {
		respondError(w, http.StatusBadRequest, "Failed to store firmware file")
		return
	}
	checksum := hex.EncodeToString(hash.Sum(nil))
	filename := strconv.FormatInt(time.Now().UnixNano(), 10) + extension
	filePath := filepath.Join(r.uploadDir, filename)
	if err := os.Chmod(tempPath, 0600); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to secure firmware file")
		return
	}
	if err := os.Rename(tempPath, filePath); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to finalize firmware file")
		return
	}

	fw := &models.Firmware{
		Filename:     originalFilename,
		Version:      version,
		FileSize:     written,
		FilePath:     filePath,
		Checksum:     &checksum,
		Manufacturer: &manufacturer,
		ProductClass: &productClass,
		Description:  description,
	}

	if err := r.firmwareRepo.Create(req.Context(), fw); err != nil {
		log.Printf("Error saving firmware: %v", err)
		if removeErr := os.Remove(filePath); removeErr != nil {
			log.Printf("Failed to remove orphaned firmware file %s: %v", filePath, removeErr)
		}
		respondError(w, http.StatusInternalServerError, "Failed to save firmware")
		return
	}

	respondJSON(w, http.StatusCreated, fw)
}

func (r *Router) handleDeleteFirmware(w http.ResponseWriter, req *http.Request) {
	idStr := req.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid firmware ID")
		return
	}

	fw, err := r.firmwareRepo.GetByID(req.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get firmware")
		return
	}
	if fw == nil {
		respondError(w, http.StatusNotFound, "Firmware not found")
		return
	}
	activeTasks, err := r.taskRepo.CountActiveFirmwareTasks(req.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to check active firmware tasks")
		return
	}
	if activeTasks > 0 {
		respondError(w, http.StatusConflict, "Firmware has active download tasks and cannot be deleted")
		return
	}

	quarantinePath := ""
	if _, statErr := os.Stat(fw.FilePath); statErr == nil {
		suffix, tokenErr := newFirmwareToken()
		if tokenErr != nil {
			respondError(w, http.StatusInternalServerError, "Failed to prepare firmware deletion")
			return
		}
		quarantinePath = fw.FilePath + ".deleting-" + suffix[:16]
		if err := os.Rename(fw.FilePath, quarantinePath); err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to quarantine firmware file")
			return
		}
	} else if !os.IsNotExist(statErr) {
		respondError(w, http.StatusInternalServerError, "Failed to inspect firmware file")
		return
	}
	if err := r.firmwareRepo.Delete(req.Context(), id); err != nil {
		if quarantinePath != "" {
			if restoreErr := os.Rename(quarantinePath, fw.FilePath); restoreErr != nil {
				log.Printf("Failed to restore firmware after database deletion error: %v", restoreErr)
			}
		}
		respondError(w, http.StatusInternalServerError, "Failed to delete firmware record")
		return
	}
	if quarantinePath != "" {
		if err := os.Remove(quarantinePath); err != nil {
			log.Printf("Failed to remove quarantined firmware file %s: %v", quarantinePath, err)
		}
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func newFirmwareToken() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}

func (r *Router) firmwareDownloadURL(req *http.Request, fw *models.Firmware) (string, error) {
	minimumExpiry := time.Now().Add(24 * time.Hour)
	if fw.DownloadToken == "" {
		token, err := newFirmwareToken()
		if err != nil {
			return "", fmt.Errorf("failed to secure firmware download")
		}
		if err := r.firmwareRepo.UpdateDownloadToken(req.Context(), fw.ID, token, minimumExpiry); err != nil {
			return "", fmt.Errorf("failed to persist firmware download token")
		}
		fw.DownloadToken = token
		fw.TokenExpiresAt = &minimumExpiry
	} else if fw.TokenExpiresAt == nil || fw.TokenExpiresAt.Before(minimumExpiry) {
		// Extend the existing grant instead of rotating it: rotation would break
		// download tasks already queued for offline devices.
		if err := r.firmwareRepo.UpdateDownloadToken(req.Context(), fw.ID, fw.DownloadToken, minimumExpiry); err != nil {
			return "", fmt.Errorf("failed to extend firmware download token")
		}
		fw.TokenExpiresAt = &minimumExpiry
	}

	setting, err := r.settingsRepo.Get(req.Context(), "firmware_base_url")
	if err != nil || setting == nil || strings.TrimSpace(setting.Value) == "" {
		return "", fmt.Errorf("configure firmware_base_url before scheduling a download")
	}
	baseURL := strings.TrimRight(strings.TrimSpace(setting.Value), "/")
	parsed, err := url.Parse(baseURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return "", fmt.Errorf("firmware_base_url must be an absolute HTTP or HTTPS URL")
	}
	if parsed.Scheme != "https" && os.Getenv("ALLOW_INSECURE_FIRMWARE_URL") != "true" {
		return "", fmt.Errorf("firmware_base_url must use HTTPS")
	}

	return fmt.Sprintf("%s/files/%d/%s?token=%s", baseURL, fw.ID, url.PathEscape(fw.Filename), url.QueryEscape(fw.DownloadToken)), nil
}

func (r *Router) handleFirmwareFile(w http.ResponseWriter, req *http.Request) {
	if !r.downloadLimiter.Allow(clientIP(req)) {
		w.Header().Set("Retry-After", "60")
		http.Error(w, "Too many download requests", http.StatusTooManyRequests)
		return
	}
	id, err := strconv.ParseInt(req.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, req)
		return
	}
	fw, err := r.firmwareRepo.GetByID(req.Context(), id)
	if err != nil || fw == nil || fw.DownloadToken == "" || fw.TokenExpiresAt == nil || time.Now().After(*fw.TokenExpiresAt) || req.PathValue("filename") != fw.Filename {
		http.NotFound(w, req)
		return
	}
	providedToken := req.URL.Query().Get("token")
	if subtle.ConstantTimeCompare([]byte(providedToken), []byte(fw.DownloadToken)) != 1 {
		http.NotFound(w, req)
		return
	}

	uploadRoot, err := filepath.Abs(r.uploadDir)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Firmware storage is unavailable")
		return
	}
	firmwarePath, err := filepath.Abs(fw.FilePath)
	if err != nil {
		http.NotFound(w, req)
		return
	}
	relativePath, err := filepath.Rel(uploadRoot, firmwarePath)
	if err != nil || relativePath == ".." || strings.HasPrefix(relativePath, ".."+string(filepath.Separator)) {
		http.NotFound(w, req)
		return
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filepath.Base(fw.Filename)))
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeFile(w, req, firmwarePath)
}

func (r *Router) handleDownloadFirmware(w http.ResponseWriter, req *http.Request) {
	deviceID, err := r.parseDeviceID(req)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid device ID")
		return
	}

	var payload struct {
		FirmwareID int64  `json:"firmware_id"`
		FileType   string `json:"file_type"` // "1 Firmware Upgrade Image", "3 Vendor Configuration File"
	}
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	fw, err := r.firmwareRepo.GetByID(req.Context(), payload.FirmwareID)
	if err != nil || fw == nil {
		respondError(w, http.StatusNotFound, "Firmware not found")
		return
	}
	device, err := r.deviceRepo.GetByID(req.Context(), deviceID)
	if err != nil || device == nil {
		respondError(w, http.StatusNotFound, "Device not found")
		return
	}
	if err := validateFirmwareCompatibility(device, fw); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	downloadURL, err := r.firmwareDownloadURL(req, fw)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	fileType := payload.FileType
	if fileType == "" {
		fileType = "1 Firmware Upgrade Image"
	}
	if err := validateFirmwareFileType(fileType); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	task, err := models.NewTaskWithPayload(deviceID, models.TaskTypeDownload, map[string]interface{}{
		"firmware_id":     fw.ID,
		"file_type":       fileType,
		"url":             downloadURL,
		"file_size":       fw.FileSize,
		"filename":        fw.Filename,
		"target_filename": fw.Filename,
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to create task")
		return
	}

	if err := r.taskRepo.Create(req.Context(), task); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to create task")
		return
	}

	respondJSON(w, http.StatusCreated, task)
}

func (r *Router) parseDeviceID(req *http.Request) (int64, error) {
	idStr := req.PathValue("id")
	return strconv.ParseInt(idStr, 10, 64)
}

func validateParameterNames(names []string) error {
	if len(names) > 100 {
		return errors.New("a maximum of 100 parameters can be requested per task")
	}
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" || len(name) > 512 || (!strings.HasPrefix(name, "Device.") && !strings.HasPrefix(name, "InternetGatewayDevice.")) {
			return fmt.Errorf("invalid parameter name %q", name)
		}
	}
	return nil
}

func validateSetParameters(parameters map[string]string) error {
	if len(parameters) > 100 {
		return errors.New("a maximum of 100 parameters can be changed per task")
	}
	names := make([]string, 0, len(parameters))
	for name, value := range parameters {
		names = append(names, name)
		if len(value) > 4096 {
			return fmt.Errorf("value for %q exceeds 4096 bytes", name)
		}
	}
	return validateParameterNames(names)
}

func (r *Router) rejectKnownReadOnly(ctx context.Context, deviceID int64, parameters map[string]string) error {
	names := make([]string, 0, len(parameters))
	for name := range parameters {
		names = append(names, name)
	}
	readOnly, err := r.parameterRepo.KnownReadOnly(ctx, deviceID, names)
	if err != nil {
		return errors.New("could not verify parameter write access")
	}
	if len(readOnly) > 0 {
		return fmt.Errorf("parameter is reported read-only by the CPE: %s", strings.Join(readOnly, ", "))
	}
	return nil
}

func validateFirmwareFileType(fileType string) error {
	switch fileType {
	case "1 Firmware Upgrade Image", "3 Vendor Configuration File":
		return nil
	default:
		return errors.New("unsupported firmware file type")
	}
}

func (r *Router) parseDeviceBySerial(req *http.Request) (*models.Device, error) {
	serial := req.PathValue("serial")
	device, err := r.deviceRepo.GetBySerial(req.Context(), serial)
	if err != nil {
		return nil, err
	}
	return device, nil
}

func (r *Router) handleGetDeviceBySerial(w http.ResponseWriter, req *http.Request) {
	device, err := r.parseDeviceBySerial(req)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get device")
		return
	}
	if device == nil {
		respondError(w, http.StatusNotFound, "Device not found")
		return
	}
	respondJSON(w, http.StatusOK, device)
}

func (r *Router) handleDeleteDeviceBySerial(w http.ResponseWriter, req *http.Request) {
	device, err := r.parseDeviceBySerial(req)
	if err != nil || device == nil {
		respondError(w, http.StatusNotFound, "Device not found")
		return
	}
	if err := r.deviceRepo.Delete(req.Context(), device.ID); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to delete device")
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (r *Router) handleGetDeviceParametersBySerial(w http.ResponseWriter, req *http.Request) {
	device, err := r.parseDeviceBySerial(req)
	if err != nil || device == nil {
		respondError(w, http.StatusNotFound, "Device not found")
		return
	}
	params, err := r.parameterRepo.GetByDeviceID(req.Context(), device.ID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get parameters")
		return
	}
	redactSensitiveParameters(req, params)
	respondJSON(w, http.StatusOK, params)
}

func redactSensitiveParameters(req *http.Request, params []models.DeviceParameter) {
	claims := auth.GetUserFromContext(req.Context())
	if claims != nil && claims.Role == models.RoleFull {
		return
	}
	for index := range params {
		if database.IsSensitiveParameterName(params[index].Name) {
			params[index].Value = "[REDACTED]"
		}
	}
}

func (r *Router) handleGetDeviceTasksBySerial(w http.ResponseWriter, req *http.Request) {
	device, err := r.parseDeviceBySerial(req)
	if err != nil || device == nil {
		respondError(w, http.StatusNotFound, "Device not found")
		return
	}
	tasks, err := r.taskRepo.GetByDeviceID(req.Context(), device.ID, 50)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get tasks")
		return
	}
	respondJSON(w, http.StatusOK, tasks)
}

func (r *Router) handleGetParameterValuesBySerial(w http.ResponseWriter, req *http.Request) {
	device, err := r.parseDeviceBySerial(req)
	if err != nil || device == nil {
		respondError(w, http.StatusNotFound, "Device not found")
		return
	}
	var payload struct {
		Parameters []string `json:"parameters"`
	}
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}
	if len(payload.Parameters) == 0 {
		respondError(w, http.StatusBadRequest, "No parameters specified")
		return
	}
	if err := validateParameterNames(payload.Parameters); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	task, err := models.NewTaskWithPayload(device.ID, models.TaskTypeGetParameterValues, payload.Parameters)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to create task")
		return
	}
	if err := r.taskRepo.Create(req.Context(), task); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to create task")
		return
	}
	respondJSON(w, http.StatusCreated, task)
}

func (r *Router) handleSetParameterValuesBySerial(w http.ResponseWriter, req *http.Request) {
	device, err := r.parseDeviceBySerial(req)
	if err != nil || device == nil {
		respondError(w, http.StatusNotFound, "Device not found")
		return
	}
	var payload struct {
		Parameters map[string]string `json:"parameters"`
	}
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}
	if len(payload.Parameters) == 0 {
		respondError(w, http.StatusBadRequest, "No parameters specified")
		return
	}
	if err := validateSetParameters(payload.Parameters); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := r.rejectKnownReadOnly(req.Context(), device.ID, payload.Parameters); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	task, err := models.NewTaskWithPayload(device.ID, models.TaskTypeSetParameterValues, payload.Parameters)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to create task")
		return
	}
	if err := r.taskRepo.Create(req.Context(), task); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to create task")
		return
	}
	respondJSON(w, http.StatusCreated, task)
}

func (r *Router) handleRebootBySerial(w http.ResponseWriter, req *http.Request) {
	device, err := r.parseDeviceBySerial(req)
	if err != nil || device == nil {
		respondError(w, http.StatusNotFound, "Device not found")
		return
	}
	task, err := models.NewTaskWithPayload(device.ID, models.TaskTypeReboot, nil)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to create task")
		return
	}
	if err := r.taskRepo.Create(req.Context(), task); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to create task")
		return
	}
	respondJSON(w, http.StatusCreated, task)
}

func (r *Router) handleFactoryResetBySerial(w http.ResponseWriter, req *http.Request) {
	if !r.requireCurrentPassword(w, req) {
		return
	}
	device, err := r.parseDeviceBySerial(req)
	if err != nil || device == nil {
		respondError(w, http.StatusNotFound, "Device not found")
		return
	}
	task, err := models.NewTaskWithPayload(device.ID, models.TaskTypeFactoryReset, nil)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to create task")
		return
	}
	if err := r.taskRepo.Create(req.Context(), task); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to create task")
		return
	}
	respondJSON(w, http.StatusCreated, task)
}

func (r *Router) handleConnectionRequestBySerial(w http.ResponseWriter, req *http.Request) {
	device, err := r.parseDeviceBySerial(req)
	if err != nil || device == nil {
		respondError(w, http.StatusNotFound, "Device not found")
		return
	}
	r.executeConnectionRequest(w, req, device)
}

func (r *Router) handleDownloadFirmwareBySerial(w http.ResponseWriter, req *http.Request) {
	device, err := r.parseDeviceBySerial(req)
	if err != nil || device == nil {
		respondError(w, http.StatusNotFound, "Device not found")
		return
	}
	var payload struct {
		FirmwareID int64  `json:"firmware_id"`
		FileType   string `json:"file_type"`
	}
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}
	fw, err := r.firmwareRepo.GetByID(req.Context(), payload.FirmwareID)
	if err != nil || fw == nil {
		respondError(w, http.StatusNotFound, "Firmware not found")
		return
	}
	if err := validateFirmwareCompatibility(device, fw); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	downloadURL, err := r.firmwareDownloadURL(req, fw)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	fileType := payload.FileType
	if fileType == "" {
		fileType = "1 Firmware Upgrade Image"
	}
	if err := validateFirmwareFileType(fileType); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	task, err := models.NewTaskWithPayload(device.ID, models.TaskTypeDownload, map[string]interface{}{
		"firmware_id":     fw.ID,
		"file_type":       fileType,
		"url":             downloadURL,
		"file_size":       fw.FileSize,
		"filename":        fw.Filename,
		"target_filename": fw.Filename,
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to create task")
		return
	}
	if err := r.taskRepo.Create(req.Context(), task); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to create task")
		return
	}
	respondJSON(w, http.StatusCreated, task)
}

func validateFirmwareCompatibility(device *models.Device, firmware *models.Firmware) error {
	if firmware.Manufacturer == nil || strings.TrimSpace(*firmware.Manufacturer) == "" ||
		firmware.ProductClass == nil || strings.TrimSpace(*firmware.ProductClass) == "" {
		return errors.New("firmware manufacturer and product class are required before scheduling an upgrade")
	}
	if device.Manufacturer == nil || !strings.EqualFold(strings.TrimSpace(*device.Manufacturer), strings.TrimSpace(*firmware.Manufacturer)) {
		return errors.New("firmware manufacturer does not match device")
	}
	if device.ProductClass == nil || !strings.EqualFold(strings.TrimSpace(*device.ProductClass), strings.TrimSpace(*firmware.ProductClass)) {
		return errors.New("firmware product class does not match device")
	}
	return nil
}

// Fault handlers
func (r *Router) handleListFaults(w http.ResponseWriter, req *http.Request) {
	limit := 50
	offset := 0
	var resolved *bool

	if l := req.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if o := req.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}
	if rs := req.URL.Query().Get("resolved"); rs != "" {
		val := rs == "true"
		resolved = &val
	}

	faults, total, err := r.faultRepo.List(req.Context(), limit, offset, resolved)
	if err != nil {
		log.Printf("Error listing faults: %v", err)
		respondError(w, http.StatusInternalServerError, "Failed to list faults")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"faults": faults,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

func (r *Router) handleFaultStats(w http.ResponseWriter, req *http.Request) {
	stats, err := r.faultRepo.GetStats(req.Context())
	if err != nil {
		log.Printf("Error getting fault stats: %v", err)
		respondError(w, http.StatusInternalServerError, "Failed to get stats")
		return
	}

	respondJSON(w, http.StatusOK, stats)
}

func (r *Router) handleResolveFault(w http.ResponseWriter, req *http.Request) {
	idStr := req.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid fault ID")
		return
	}

	if err := r.faultRepo.Resolve(req.Context(), id); err != nil {
		log.Printf("Error resolving fault: %v", err)
		respondError(w, http.StatusInternalServerError, "Failed to resolve fault")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "resolved"})
}

func (r *Router) handleDeleteFault(w http.ResponseWriter, req *http.Request) {
	idStr := req.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid fault ID")
		return
	}

	if err := r.faultRepo.Delete(req.Context(), id); err != nil {
		log.Printf("Error deleting fault: %v", err)
		respondError(w, http.StatusInternalServerError, "Failed to delete fault")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Failed to encode HTTP response: %v", err)
	}
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}

func (r *Router) handleLogin(w http.ResponseWriter, req *http.Request) {
	ip := clientIP(req)
	if !r.loginLimiter.Allow(ip) {
		r.recordLoginAudit(req, "unknown", http.StatusTooManyRequests)
		w.Header().Set("Retry-After", "900")
		respondError(w, http.StatusTooManyRequests, "Too many login attempts; try again later")
		return
	}

	var loginReq models.LoginRequest
	decoder := json.NewDecoder(req.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&loginReq); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	loginReq.Username = strings.TrimSpace(loginReq.Username)

	if loginReq.Username == "" || loginReq.Password == "" {
		respondError(w, http.StatusBadRequest, "Username and password required")
		return
	}

	user, err := r.userRepo.GetByUsername(req.Context(), loginReq.Username)
	if err != nil || user == nil {
		// Keep invalid-user and invalid-password paths close in cost.
		auth.CheckPassword(loginReq.Password, "$2a$12$nKdFyIgp4jmSASeOXMjq2eHRjk9J4ypQdXkxEEz2Q2nURZ4Fi9PVW")
		r.recordLoginAudit(req, loginReq.Username, http.StatusUnauthorized)
		respondError(w, http.StatusUnauthorized, "Invalid username or password")
		return
	}

	if !auth.CheckPassword(loginReq.Password, user.PasswordHash) {
		r.recordLoginAudit(req, loginReq.Username, http.StatusUnauthorized)
		respondError(w, http.StatusUnauthorized, "Invalid username or password")
		return
	}

	token, err := auth.GenerateToken(user)
	if err != nil {
		log.Printf("Error generating token: %v", err)
		respondError(w, http.StatusInternalServerError, "Failed to generate token")
		return
	}

	if err := r.userRepo.UpdateLastLogin(req.Context(), user.ID); err != nil {
		log.Printf("Failed to record last login for user %d: %v", user.ID, err)
	}
	r.loginLimiter.Reset(ip)
	r.recordLoginAudit(req, user.Username, http.StatusOK)

	respondJSON(w, http.StatusOK, models.LoginResponse{
		Token: token,
		User:  user,
	})
}

func (r *Router) recordLoginAudit(req *http.Request, username string, status int) {
	entry := &models.AuditLog{
		Username:  truncate(username, 128),
		Action:    "LOGIN",
		Resource:  "/auth/login",
		Status:    status,
		IPAddress: clientIP(req),
		UserAgent: truncate(req.UserAgent(), 512),
	}
	if user, err := r.userRepo.GetByUsername(req.Context(), username); err == nil && user != nil {
		entry.UserID = &user.ID
	}
	if err := r.auditRepo.Create(req.Context(), entry); err != nil {
		log.Printf("Failed to record login audit event: %v", err)
	}
}

func (r *Router) handleAuthMe(w http.ResponseWriter, req *http.Request) {
	claims := auth.GetUserFromContext(req.Context())
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	user, err := r.userRepo.GetByID(req.Context(), claims.UserID)
	if err != nil || user == nil {
		respondError(w, http.StatusNotFound, "User not found")
		return
	}

	respondJSON(w, http.StatusOK, user)
}

func (r *Router) handleChangePassword(w http.ResponseWriter, req *http.Request) {
	claims := auth.GetUserFromContext(req.Context())
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var body struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	decoder := json.NewDecoder(req.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := auth.ValidatePassword(body.NewPassword); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	user, err := r.userRepo.GetByID(req.Context(), claims.UserID)
	if err != nil || user == nil {
		respondError(w, http.StatusNotFound, "User not found")
		return
	}

	if !auth.CheckPassword(body.CurrentPassword, user.PasswordHash) {
		respondError(w, http.StatusForbidden, "Current password is incorrect")
		return
	}
	if auth.CheckPassword(body.NewPassword, user.PasswordHash) {
		respondError(w, http.StatusBadRequest, "New password must be different from the current password")
		return
	}

	hash, err := auth.HashPassword(body.NewPassword)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to hash password")
		return
	}

	if err := r.userRepo.UpdatePassword(req.Context(), user.ID, hash); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to update password")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "password changed"})
}

func (r *Router) handleListUsers(w http.ResponseWriter, req *http.Request) {
	claims := auth.GetUserFromContext(req.Context())
	if claims == nil || claims.Role != models.RoleFull {
		respondError(w, http.StatusForbidden, "Full access required")
		return
	}

	users, err := r.userRepo.List(req.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list users")
		return
	}

	respondJSON(w, http.StatusOK, users)
}

func (r *Router) handleCreateUser(w http.ResponseWriter, req *http.Request) {
	claims := auth.GetUserFromContext(req.Context())
	if claims == nil || claims.Role != models.RoleFull {
		respondError(w, http.StatusForbidden, "Full access required")
		return
	}

	var body models.CreateUserRequest
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	body.Username = strings.TrimSpace(body.Username)
	if !validUsername(body.Username) {
		respondError(w, http.StatusBadRequest, "Username must be 3-64 characters and use only letters, numbers, dot, dash, or underscore")
		return
	}
	if err := auth.ValidatePassword(body.Password); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if body.Role != models.RoleFull && body.Role != models.RoleRead {
		body.Role = models.RoleRead
	}

	hash, err := auth.HashPassword(body.Password)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to hash password")
		return
	}

	user := &models.User{
		Username:     body.Username,
		PasswordHash: hash,
		Role:         body.Role,
	}

	if err := r.userRepo.Create(req.Context(), user); err != nil {
		if strings.Contains(err.Error(), "duplicate") {
			respondError(w, http.StatusConflict, "Username already exists")
			return
		}
		respondError(w, http.StatusInternalServerError, "Failed to create user")
		return
	}

	respondJSON(w, http.StatusCreated, user)
}

func (r *Router) handleUpdateUser(w http.ResponseWriter, req *http.Request) {
	claims := auth.GetUserFromContext(req.Context())
	if claims == nil || claims.Role != models.RoleFull {
		respondError(w, http.StatusForbidden, "Full access required")
		return
	}

	idStr := req.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	var body models.UpdateUserRequest
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	user, err := r.userRepo.GetByID(req.Context(), id)
	if err != nil || user == nil {
		respondError(w, http.StatusNotFound, "User not found")
		return
	}
	var passwordHash string
	if body.Password != "" {
		if err := auth.ValidatePassword(body.Password); err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		passwordHash, err = auth.HashPassword(body.Password)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to hash password")
			return
		}
	}

	if body.Username != "" {
		body.Username = strings.TrimSpace(body.Username)
		if !validUsername(body.Username) {
			respondError(w, http.StatusBadRequest, "Invalid username format")
			return
		}
		user.Username = body.Username
	}
	if body.Role != "" {
		if body.Role != models.RoleFull && body.Role != models.RoleRead {
			respondError(w, http.StatusBadRequest, "Invalid role")
			return
		}
		if user.Role == models.RoleFull && body.Role != models.RoleFull {
			fullAdmins, countErr := r.userRepo.CountByRole(req.Context(), models.RoleFull)
			if countErr != nil || fullAdmins <= 1 {
				respondError(w, http.StatusBadRequest, "Cannot remove the last full-access administrator")
				return
			}
		}
		user.Role = body.Role
	}

	if err := r.userRepo.UpdateAccount(req.Context(), id, user.Username, user.Role, passwordHash); err != nil {
		if errors.Is(err, database.ErrLastFullAdmin) {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			respondError(w, http.StatusConflict, "Username already exists")
			return
		}
		respondError(w, http.StatusInternalServerError, "Failed to update user")
		return
	}

	respondJSON(w, http.StatusOK, user)
}

func (r *Router) handleDeleteUser(w http.ResponseWriter, req *http.Request) {
	claims := auth.GetUserFromContext(req.Context())
	if claims == nil || claims.Role != models.RoleFull {
		respondError(w, http.StatusForbidden, "Full access required")
		return
	}

	idStr := req.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	if id == claims.UserID {
		respondError(w, http.StatusBadRequest, "Cannot delete yourself")
		return
	}

	user, err := r.userRepo.GetByID(req.Context(), id)
	if err != nil || user == nil {
		respondError(w, http.StatusNotFound, "User not found")
		return
	}
	if err := r.userRepo.DeleteSafely(req.Context(), id); err != nil {
		if errors.Is(err, database.ErrLastFullAdmin) || errors.Is(err, database.ErrLastUser) {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		respondError(w, http.StatusInternalServerError, "Failed to delete user")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (r *Router) handleListProvisioningRules(w http.ResponseWriter, req *http.Request) {
	rules, err := r.provisioningRepo.List(req.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list provisioning rules")
		return
	}
	claims := auth.GetUserFromContext(req.Context())
	if claims == nil || claims.Role != models.RoleFull {
		for _, rule := range rules {
			if database.IsSensitiveParameterName(rule.ParameterName) {
				rule.ParameterValue = "[REDACTED]"
			}
		}
	}
	respondJSON(w, http.StatusOK, rules)
}

func (r *Router) handleCreateProvisioningRule(w http.ResponseWriter, req *http.Request) {
	claims := auth.GetUserFromContext(req.Context())
	if claims == nil || claims.Role != models.RoleFull {
		respondError(w, http.StatusForbidden, "Full access required")
		return
	}

	var body models.CreateProvisioningRuleRequest
	decoder := json.NewDecoder(req.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := validateProvisioningRule(body.ParameterName, body.ParameterValue, body.ParameterType, body.Manufacturer, body.ProductClass, body.Description); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if body.ParameterType == "" {
		body.ParameterType = "string"
	}

	rule := &models.ProvisioningRule{
		ParameterName:  body.ParameterName,
		ParameterValue: body.ParameterValue,
		ParameterType:  body.ParameterType,
		Manufacturer:   strings.TrimSpace(body.Manufacturer),
		ProductClass:   strings.TrimSpace(body.ProductClass),
		Enabled:        body.Enabled,
		Description:    body.Description,
	}

	if err := r.provisioningRepo.Create(req.Context(), rule); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to create rule")
		return
	}

	respondJSON(w, http.StatusCreated, rule)
}

func (r *Router) handleUpdateProvisioningRule(w http.ResponseWriter, req *http.Request) {
	claims := auth.GetUserFromContext(req.Context())
	if claims == nil || claims.Role != models.RoleFull {
		respondError(w, http.StatusForbidden, "Full access required")
		return
	}

	idStr := req.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid rule ID")
		return
	}

	var body models.ProvisioningRule
	decoder := json.NewDecoder(req.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if err := validateProvisioningRule(body.ParameterName, body.ParameterValue, body.ParameterType, body.Manufacturer, body.ProductClass, body.Description); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	body.ID = id
	if err := r.provisioningRepo.Update(req.Context(), &body); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to update rule")
		return
	}

	respondJSON(w, http.StatusOK, body)
}

func validateProvisioningRule(name, value, valueType, manufacturer, productClass, description string) error {
	if err := validateParameterNames([]string{name}); err != nil {
		return err
	}
	if len(value) > 4096 {
		return errors.New("provisioning value exceeds 4096 bytes")
	}
	switch valueType {
	case "", "string", "boolean", "int", "unsignedInt", "long", "unsignedLong", "dateTime", "base64", "hexBinary":
	default:
		return errors.New("unsupported CWMP parameter type")
	}
	if len(manufacturer) > 128 || len(productClass) > 128 {
		return errors.New("provisioning scope exceeds 128 characters")
	}
	if len(description) > 2048 || strings.ContainsRune(description, '\x00') {
		return errors.New("provisioning description is invalid or exceeds 2048 characters")
	}
	return nil
}

func (r *Router) handleDeleteProvisioningRule(w http.ResponseWriter, req *http.Request) {
	claims := auth.GetUserFromContext(req.Context())
	if claims == nil || claims.Role != models.RoleFull {
		respondError(w, http.StatusForbidden, "Full access required")
		return
	}

	idStr := req.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid rule ID")
		return
	}

	if err := r.provisioningRepo.Delete(req.Context(), id); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to delete rule")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (r *Router) handleToggleProvisioningRule(w http.ResponseWriter, req *http.Request) {
	claims := auth.GetUserFromContext(req.Context())
	if claims == nil || claims.Role != models.RoleFull {
		respondError(w, http.StatusForbidden, "Full access required")
		return
	}

	idStr := req.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid rule ID")
		return
	}

	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := r.provisioningRepo.ToggleEnabled(req.Context(), id, body.Enabled); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to toggle rule")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "toggled"})
}
