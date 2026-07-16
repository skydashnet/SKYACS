package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/skydashnet/miniacs/internal/api"
	"github.com/skydashnet/miniacs/internal/auth"
	"github.com/skydashnet/miniacs/internal/cwmp"
	"github.com/skydashnet/miniacs/internal/database"
	"github.com/skydashnet/miniacs/internal/models"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found; using process environment")
	}
	if err := godotenv.Load(".miniacs.env"); err != nil {
		log.Println("No .miniacs.env file found")
	}
	if err := auth.ConfigureJWT(os.Getenv("JWT_SECRET")); err != nil {
		log.Fatalf("Security configuration error: %v", err)
	}
	if err := database.ConfigureParameterEncryption(os.Getenv("PARAMETER_ENCRYPTION_KEY")); err != nil {
		log.Fatalf("Security configuration error: %v", err)
	}
	if err := validateCWMPAuthenticationConfig(); err != nil {
		log.Fatalf("Security configuration error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, err := connectDatabase()
	if err != nil {
		log.Fatalf("Database connection failed: %v", err)
	}
	log.Println("Database connected successfully")
	if err := runAutoMigrate(db); err != nil {
		log.Fatalf("Database migration failed: %v", err)
	}
	if count, err := database.NewParameterRepository(db).EncryptLegacySensitiveValues(ctx); err != nil {
		log.Fatalf("Sensitive parameter migration failed: %v", err)
	} else if count > 0 {
		log.Printf("Encrypted %d legacy sensitive parameter values", count)
	}
	if count, err := database.NewProvisioningRepository(db).EncryptLegacySensitiveValues(ctx); err != nil {
		log.Fatalf("Sensitive provisioning migration failed: %v", err)
	} else if count > 0 {
		log.Printf("Encrypted %d legacy sensitive provisioning values", count)
	}
	if count, err := database.NewSettingsRepository(db).EncryptLegacySensitiveValues(ctx); err != nil {
		log.Fatalf("Sensitive settings migration failed: %v", err)
	} else if count > 0 {
		log.Printf("Encrypted %d legacy sensitive settings", count)
	}
	if count, err := database.NewTaskRepository(db).ProtectLegacyTaskSecrets(ctx); err != nil {
		log.Fatalf("Sensitive task migration failed: %v", err)
	} else if count > 0 {
		log.Printf("Protected %d legacy task records containing sensitive values", count)
	}
	go runTaskRecovery(ctx, database.NewTaskRepository(db))
	go runRetentionMaintenance(ctx, db)

	cwmpPort := os.Getenv("PORT")
	if cwmpPort == "" {
		cwmpPort = "7547"
	}
	apiPort := os.Getenv("API_PORT")
	if apiPort == "" {
		apiPort = "7548"
	}
	cwmpBindAddress := envOrDefault("CWMP_BIND_ADDR", "127.0.0.1")
	apiBindAddress := envOrDefault("API_BIND_ADDR", "127.0.0.1")

	cwmpMux := http.NewServeMux()
	cwmpHandler := cwmp.NewHandler(db)
	cwmpMux.Handle("/", cwmpHandler)

	cwmpServer := &http.Server{
		Addr:              net.JoinHostPort(cwmpBindAddress, cwmpPort),
		Handler:           cwmpMux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	apiMux := http.NewServeMux()
	apiRouter := api.NewRouter(db)
	apiMux.Handle("/", apiRouter.Handler())

	apiServer := &http.Server{
		Addr:              net.JoinHostPort(apiBindAddress, apiPort),
		Handler:           apiMux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	watchdog := cwmp.NewDeviceWatchdog(db)
	go watchdog.Start(ctx)

	go func() {
		log.Printf("miniACS CWMP server listening on %s", cwmpServer.Addr)
		if err := cwmpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("CWMP server failed: %v", err)
		}
	}()

	go func() {
		log.Printf("miniACS API server listening on %s", apiServer.Addr)
		if err := apiServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("API server failed: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down servers...")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := cwmpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("CWMP Server shutdown failed: %v", err)
	}
	if err := apiServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("API Server shutdown failed: %v", err)
	}

	log.Println("Servers stopped")
}

func validateCWMPAuthenticationConfig() error {
	usernameSet := strings.TrimSpace(os.Getenv("CWMP_USERNAME")) != ""
	passwordSet := os.Getenv("CWMP_PASSWORD") != ""
	if usernameSet != passwordSet {
		return errors.New("CWMP_USERNAME and CWMP_PASSWORD must be configured together")
	}
	if usernameSet && strings.TrimSpace(os.Getenv("CWMP_TRUSTED_PROXY_CIDRS")) == "" {
		return errors.New("CWMP Basic authentication requires TLS termination and CWMP_TRUSTED_PROXY_CIDRS")
	}
	for _, key := range []string{"CWMP_ALLOWED_CIDRS", "CWMP_TRUSTED_PROXY_CIDRS", "TRUSTED_PROXY_CIDRS", "CONNECTION_REQUEST_ALLOWED_CIDRS"} {
		if err := validateCIDRList(key, os.Getenv(key)); err != nil {
			return err
		}
	}
	return nil
}

func validateCIDRList(key, raw string) error {
	for _, item := range strings.Split(raw, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if net.ParseIP(item) != nil {
			continue
		}
		if _, _, err := net.ParseCIDR(item); err != nil {
			return fmt.Errorf("%s contains invalid address or CIDR %q", key, item)
		}
	}
	return nil
}

func envOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func connectDatabase() (*gorm.DB, error) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		host := os.Getenv("DB_HOST")
		port := os.Getenv("DB_PORT")
		dbname := os.Getenv("DB_NAME")
		user := os.Getenv("DB_USER")
		password := os.Getenv("DB_PASSWORD")

		if host == "" || dbname == "" || user == "" {
			return nil, fmt.Errorf("database credentials not found - run ./setup.sh first")
		}
		if port == "" {
			port = "5432"
		}

		sslMode := os.Getenv("DB_SSLMODE")
		if sslMode == "" {
			if host == "localhost" || host == "127.0.0.1" || host == "::1" {
				sslMode = "disable"
			} else {
				sslMode = "verify-full"
			}
		}
		switch sslMode {
		case "disable", "require", "verify-ca", "verify-full":
		default:
			return nil, fmt.Errorf("unsupported DB_SSLMODE %q", sslMode)
		}
		query := url.Values{"sslmode": []string{sslMode}}
		dsn = (&url.URL{
			Scheme:   "postgres",
			User:     url.UserPassword(user, password),
			Host:     net.JoinHostPort(host, port),
			Path:     "/" + dbname,
			RawQuery: query.Encode(),
		}).String()
	}

	gormLogger := logger.New(
		log.New(os.Stdout, "\n", log.LstdFlags),
		logger.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  logger.Warn,
			IgnoreRecordNotFoundError: true,
			Colorful:                  !strings.Contains(os.Getenv("TERM"), "dumb"),
		},
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: gormLogger,
	})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	maxOpen, err := envInt("DB_MAX_OPEN_CONNS", 50, 1, 500)
	if err != nil {
		return nil, err
	}
	maxIdle, err := envInt("DB_MAX_IDLE_CONNS", 10, 0, maxOpen)
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxIdleConns(maxIdle)
	sqlDB.SetMaxOpenConns(maxOpen)
	sqlDB.SetConnMaxLifetime(time.Hour)

	return db, nil
}

func envInt(key string, fallback, minimum, maximum int) (int, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < minimum || value > maximum {
		return 0, fmt.Errorf("%s must be between %d and %d", key, minimum, maximum)
	}
	return value, nil
}

func runAutoMigrate(db *gorm.DB) error {
	log.Println("Running database migrations...")

	allModels := []interface{}{
		&models.Device{},
		&models.DeviceParameter{},
		&models.Task{},
		&models.User{},
		&models.Fault{},
		&models.Firmware{},
		&models.ProvisioningRule{},
		&models.ProvisioningApplication{},
		&models.AuditLog{},
		&models.BlockedDevice{},
		&database.Setting{},
	}

	for _, model := range allModels {
		if err := db.AutoMigrate(model); err != nil {
			return fmt.Errorf("migrate %T: %w", model, err)
		}
	}
	if db.Migrator().HasIndex(&models.ProvisioningApplication{}, "idx_provisioning_application") {
		if err := db.Migrator().DropIndex(&models.ProvisioningApplication{}, "idx_provisioning_application"); err != nil {
			return fmt.Errorf("drop obsolete provisioning application index: %w", err)
		}
	}
	if err := db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_tasks_command_key_unique ON tasks (command_key) WHERE command_key <> ''").Error; err != nil {
		return fmt.Errorf("create unique task command key index: %w", err)
	}

	defaultSettings := []database.Setting{
		{Key: "firmware_base_url", Value: "", Description: "Public HTTPS API base URL used by CPE firmware downloads"},
		{Key: "connection_request_username", Value: "", Description: "Connection request username"},
		{Key: "connection_request_password", Value: "", Description: "Connection request password"},
		{Key: "use_auto_conn_credentials", Value: "false", Description: "Derive unique per-device credentials with HMAC-SHA256"},
	}
	for index := range defaultSettings {
		if err := db.Where(database.Setting{Key: defaultSettings[index].Key}).FirstOrCreate(&defaultSettings[index]).Error; err != nil {
			return fmt.Errorf("seed setting %s: %w", defaultSettings[index].Key, err)
		}
	}
	if err := db.Where("key IN ?", []string{"acs_url", "acs_username", "acs_password", "inform_interval"}).Delete(&database.Setting{}).Error; err != nil {
		return fmt.Errorf("remove obsolete settings: %w", err)
	}

	var count int64
	if err := db.Model(&models.User{}).Count(&count).Error; err != nil {
		return fmt.Errorf("count bootstrap users: %w", err)
	}
	if count == 0 {
		log.Println("Creating bootstrap administrator...")
		initialPassword := os.Getenv("INITIAL_ADMIN_PASSWORD")
		if initialPassword != "" {
			if err := auth.ValidatePassword(initialPassword); err != nil {
				return fmt.Errorf("invalid INITIAL_ADMIN_PASSWORD: %w", err)
			}
		} else {
			initialPassword = generateBootstrapPassword()
		}
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(initialPassword), 12)
		if err != nil {
			return fmt.Errorf("hash bootstrap password: %w", err)
		}
		admin := &models.User{
			Username:     "admin",
			PasswordHash: string(hashedPassword),
			Role:         models.RoleFull,
		}
		if err := db.Create(admin).Error; err != nil {
			return fmt.Errorf("create bootstrap administrator: %w", err)
		} else {
			log.Printf("Bootstrap administrator created. Username: admin Password: %s", initialPassword)
			log.Println("Store this password securely; it is only printed once and should be rotated after first login.")
		}
	}

	log.Println("Database migrations completed")
	return nil
}

func runTaskRecovery(ctx context.Context, repo *database.TaskRepository) {
	cleanup := func() {
		if count, err := repo.MarkOldPendingDownloadsAsFailed(ctx, 24*time.Hour); err != nil {
			log.Printf("Firmware task cleanup failed: %v", err)
		} else if count > 0 {
			log.Printf("Task cleanup marked %d expired firmware tasks as failed", count)
		}
		if count, err := repo.MarkOldSentAsFailed(ctx, 24*time.Hour); err != nil {
			log.Printf("Task recovery failed: %v", err)
		} else if count > 0 {
			log.Printf("Task recovery marked %d stale sent tasks as failed", count)
		}
		if count, err := repo.MarkOldPendingAsFailed(ctx, 30*24*time.Hour); err != nil {
			log.Printf("Pending task cleanup failed: %v", err)
		} else if count > 0 {
			log.Printf("Task cleanup marked %d expired pending tasks as failed", count)
		}
	}
	cleanup()
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cleanup()
		}
	}
}

func runRetentionMaintenance(ctx context.Context, db *gorm.DB) {
	cleanup := func() {
		cutoffs := []struct {
			model interface{}
			query string
			args  []interface{}
		}{
			{&models.AuditLog{}, "created_at < ?", []interface{}{time.Now().Add(-180 * 24 * time.Hour)}},
			{&models.Fault{}, "resolved = ? AND resolved_at < ?", []interface{}{true, time.Now().Add(-90 * 24 * time.Hour)}},
			{&models.Task{}, "status IN ? AND completed_at < ?", []interface{}{[]models.TaskStatus{models.TaskStatusCompleted, models.TaskStatusFailed}, time.Now().Add(-90 * 24 * time.Hour)}},
		}
		for _, item := range cutoffs {
			if err := db.WithContext(ctx).Where(item.query, item.args...).Delete(item.model).Error; err != nil {
				log.Printf("Retention cleanup failed for %T: %v", item.model, err)
			}
		}
	}
	cleanup()
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cleanup()
		}
	}
}

func generateBootstrapPassword() string {
	buffer := make([]byte, 12)
	if _, err := rand.Read(buffer); err != nil {
		panic(fmt.Sprintf("cannot generate bootstrap password: %v", err))
	}
	return "MiniACS-" + hex.EncodeToString(buffer)
}
