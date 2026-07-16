package api

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/skydashnet/miniacs/internal/auth"
	"github.com/skydashnet/miniacs/internal/database"
	"github.com/skydashnet/miniacs/internal/models"
)

const (
	maxJSONBody     = 2 << 20
	maxFirmwareBody = 64 << 20
)

type responseRecorder struct {
	http.ResponseWriter
	status int
}

func (r *responseRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *responseRecorder) Write(body []byte) (int, error) {
	if r.status == 0 {
		r.WriteHeader(http.StatusOK)
	}
	return r.ResponseWriter.Write(body)
}

type loginAttempt struct {
	count       int
	windowStart time.Time
}

type loginLimiter struct {
	mu       sync.Mutex
	attempts map[string]loginAttempt
	limit    int
	window   time.Duration
}

func newLoginLimiter(limit int, window time.Duration) *loginLimiter {
	return &loginLimiter{attempts: make(map[string]loginAttempt), limit: limit, window: window}
}

func (l *loginLimiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	attempt := l.attempts[key]
	if attempt.windowStart.IsZero() || now.Sub(attempt.windowStart) >= l.window {
		l.attempts[key] = loginAttempt{count: 1, windowStart: now}
		return true
	}
	if attempt.count >= l.limit {
		return false
	}
	attempt.count++
	l.attempts[key] = attempt
	return true
}

func (l *loginLimiter) Reset(key string) {
	l.mu.Lock()
	delete(l.attempts, key)
	l.mu.Unlock()
}

func clientIP(req *http.Request) string {
	host, _, err := net.SplitHostPort(req.RemoteAddr)
	if err == nil {
		return host
	}
	return req.RemoteAddr
}

func requestBodyLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		limit := int64(maxJSONBody)
		if req.Method == http.MethodPost && req.URL.Path == "/firmwares" {
			limit = maxFirmwareBody
		}
		req.Body = http.MaxBytesReader(w, req.Body, limit)
		next.ServeHTTP(w, req)
	})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'; base-uri 'none'")
		w.Header().Set("Cross-Origin-Resource-Policy", "same-site")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, req)
	})
}

func corsMiddleware(next http.Handler) http.Handler {
	configured := strings.Split(os.Getenv("CORS_ALLOWED_ORIGINS"), ",")
	allowed := map[string]struct{}{
		"http://localhost:5173": {},
		"http://127.0.0.1:5173": {},
	}
	for _, origin := range configured {
		if origin = strings.TrimSpace(origin); origin != "" {
			allowed[origin] = struct{}{}
		}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		origin := req.Header.Get("Origin")
		if origin != "" {
			if _, ok := allowed[origin]; !ok {
				respondError(w, http.StatusForbidden, "Origin not allowed")
				return
			}
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Max-Age", "600")

		if req.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, req)
	})
}

func auditMiddleware(repo *database.AuditRepository, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		recorder := &responseRecorder{ResponseWriter: w}
		next.ServeHTTP(recorder, req)

		if req.Method == http.MethodGet || req.Method == http.MethodHead || req.Method == http.MethodOptions {
			return
		}
		claims := auth.GetUserFromContext(req.Context())
		if claims == nil {
			return
		}
		status := recorder.status
		if status == 0 {
			status = http.StatusOK
		}
		userID := claims.UserID
		entry := &models.AuditLog{
			UserID:    &userID,
			Username:  claims.Username,
			Action:    req.Method,
			Resource:  req.URL.Path,
			Status:    status,
			IPAddress: clientIP(req),
			UserAgent: truncate(req.UserAgent(), 512),
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = repo.Create(ctx, entry)
	})
}

func truncate(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return value[:max]
}

func validUsername(username string) bool {
	if len(username) < 3 || len(username) > 64 {
		return false
	}
	for _, char := range username {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') || char == '_' || char == '-' || char == '.' {
			continue
		}
		return false
	}
	return true
}

func parsePagination(req *http.Request, defaultLimit, maxLimit int) (int, int) {
	limit := defaultLimit
	offset := 0
	if value, err := strconv.Atoi(req.URL.Query().Get("limit")); err == nil && value > 0 && value <= maxLimit {
		limit = value
	}
	if value, err := strconv.Atoi(req.URL.Query().Get("offset")); err == nil && value >= 0 {
		offset = value
	}
	return limit, offset
}

func (r *Router) handleSecurityOverview(w http.ResponseWriter, req *http.Request) {
	totalUsers, err := r.userRepo.Count(req.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to count users")
		return
	}
	fullAdmins, err := r.userRepo.CountByRole(req.Context(), models.RoleFull)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to count administrators")
		return
	}
	failures, err := r.auditRepo.CountFailures(req.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to count audit failures")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"total_users":              totalUsers,
		"full_access_admins":       fullAdmins,
		"recorded_failures":        failures,
		"jwt_configured":           true,
		"login_rate_limit_enabled": true,
		"cors_restricted":          true,
		"audit_logging_enabled":    true,
	})
}

func (r *Router) handleListAuditLogs(w http.ResponseWriter, req *http.Request) {
	limit, offset := parsePagination(req, 50, 200)
	entries, total, err := r.auditRepo.List(req.Context(), limit, offset)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list audit logs")
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"entries": entries,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
	})
}

func (r *Router) handleListBlockedDevices(w http.ResponseWriter, req *http.Request) {
	devices, err := r.blockedRepo.List(req.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list blocked devices")
		return
	}
	respondJSON(w, http.StatusOK, devices)
}

func (r *Router) handleAddBlockedDevice(w http.ResponseWriter, req *http.Request) {
	var body struct {
		SerialNumber string `json:"serial_number"`
		Reason       string `json:"reason"`
	}
	decoder := json.NewDecoder(req.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	body.SerialNumber = strings.TrimSpace(body.SerialNumber)
	if body.SerialNumber == "" || len(body.SerialNumber) > 128 {
		respondError(w, http.StatusBadRequest, "Serial number is required")
		return
	}
	claims := auth.GetUserFromContext(req.Context())
	device := &models.BlockedDevice{
		SerialNumber: body.SerialNumber,
		Reason:       truncate(strings.TrimSpace(body.Reason), 512),
		CreatedBy:    claims.Username,
	}
	if err := r.blockedRepo.Add(req.Context(), device); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to block device")
		return
	}
	respondJSON(w, http.StatusCreated, device)
}

func (r *Router) handleRemoveBlockedDevice(w http.ResponseWriter, req *http.Request) {
	serial := strings.TrimSpace(req.PathValue("serial"))
	if serial == "" {
		respondError(w, http.StatusBadRequest, "Serial number is required")
		return
	}
	if err := r.blockedRepo.Remove(req.Context(), serial); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to unblock device")
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "unblocked"})
}
