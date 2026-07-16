package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
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
		log.Println("Tidak menemukan .env file")
	}
	if err := godotenv.Load(".miniacs.env"); err != nil {
		log.Println("Tidak menemukan .miniacs.env file")
	}
	if err := auth.ConfigureJWT(os.Getenv("JWT_SECRET")); err != nil {
		log.Fatalf("Security configuration error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, err := connectDatabase()
	if err != nil {
		log.Printf("Warning: Database connection failed: %v", err)
		log.Println("Jalankan ./setup.sh dulu untuk setup database")
		log.Println("Server will run without database features")
	} else {
		log.Println("Database connected successfully")
		if err := runAutoMigrate(db); err != nil {
			log.Printf("Warning: AutoMigrate failed: %v", err)
		}
	}

	cwmpPort := os.Getenv("PORT")
	if cwmpPort == "" {
		cwmpPort = "7547"
	}
	apiPort := os.Getenv("API_PORT")
	if apiPort == "" {
		apiPort = "7548"
	}

	cwmpMux := http.NewServeMux()
	cwmpHandler := cwmp.NewHandler(db)
	cwmpMux.Handle("/", cwmpHandler)

	cwmpServer := &http.Server{
		Addr:              ":" + cwmpPort,
		Handler:           cwmpMux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	var apiServer *http.Server
	if db != nil {
		apiMux := http.NewServeMux()
		apiRouter := api.NewRouter(db)
		apiMux.Handle("/", apiRouter.Handler())

		apiServer = &http.Server{
			Addr:              ":" + apiPort,
			Handler:           apiMux,
			ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      30 * time.Second,
			IdleTimeout:       60 * time.Second,
			MaxHeaderBytes:    1 << 20,
		}

		watchdog := cwmp.NewDeviceWatchdog(db)
		go watchdog.Start(ctx)
	}

	go func() {
		log.Printf("miniACS CWMP server berjalan di :%s", cwmpPort)
		if err := cwmpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("CWMP Server gagal start: %v", err)
		}
	}()

	if apiServer != nil {
		go func() {
			log.Printf("miniACS API server berjalan di :%s", apiPort)
			if err := apiServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatalf("API Server gagal start: %v", err)
			}
		}()
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down servers...")

	shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 10*time.Second)
	defer shutdownCancel()

	if err := cwmpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("CWMP Server shutdown failed: %v", err)
	}
	if apiServer != nil {
		if err := apiServer.Shutdown(shutdownCtx); err != nil {
			log.Printf("API Server shutdown failed: %v", err)
		}
	}

	log.Println("Servers stopped")
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

		dsn = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
			host, port, user, password, dbname)
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
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetMaxOpenConns(10)
	sqlDB.SetConnMaxLifetime(time.Hour)

	return db, nil
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
		&models.AuditLog{},
		&models.BlockedDevice{},
		&database.Setting{},
	}

	for _, model := range allModels {
		if err := db.AutoMigrate(model); err != nil {
			log.Printf("Warning: AutoMigrate for %T: %v", model, err)
		}
	}

	defaultSettings := []database.Setting{
		{Key: "acs_url", Value: "http://localhost:7547/", Description: "Public CWMP endpoint advertised to devices"},
		{Key: "firmware_base_url", Value: "http://localhost:7548", Description: "Public API base URL used by CPE firmware downloads"},
		{Key: "acs_username", Value: "", Description: "Optional CPE-to-ACS username"},
		{Key: "acs_password", Value: "", Description: "Optional CPE-to-ACS password"},
		{Key: "inform_interval", Value: "3600", Description: "Periodic Inform interval in seconds"},
		{Key: "connection_request_username", Value: "", Description: "Connection request username"},
		{Key: "connection_request_password", Value: "", Description: "Connection request password"},
		{Key: "use_auto_conn_credentials", Value: "true", Description: "Derive connection request credentials per device"},
	}
	for index := range defaultSettings {
		if err := db.Where(database.Setting{Key: defaultSettings[index].Key}).FirstOrCreate(&defaultSettings[index]).Error; err != nil {
			log.Printf("Warning: Failed to seed setting %s: %v", defaultSettings[index].Key, err)
		}
	}

	var count int64
	db.Model(&models.User{}).Count(&count)
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
			log.Printf("Warning: Failed to create admin user: %v", err)
		} else {
			log.Printf("Bootstrap administrator created. Username: admin Password: %s", initialPassword)
			log.Println("Store this password securely; it is only printed once and should be rotated after first login.")
		}
	}

	log.Println("Database migrations completed")
	return nil
}

func generateBootstrapPassword() string {
	buffer := make([]byte, 12)
	if _, err := rand.Read(buffer); err != nil {
		panic(fmt.Sprintf("cannot generate bootstrap password: %v", err))
	}
	return "MiniACS-" + hex.EncodeToString(buffer)
}
