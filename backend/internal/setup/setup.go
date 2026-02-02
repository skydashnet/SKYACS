package setup

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	_ "github.com/lib/pq"
	"github.com/skydashnet/miniacs/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	defaultSetupHost     = "localhost"
	defaultSetupPort     = "5432"
	defaultSetupUser     = "postgres"
	defaultSetupPassword = "postgres"

	appDatabase = "miniacs"
	appUser     = "miniacs"

	credentialFile = ".miniacs.env"
)

type Credentials struct {
	Host     string
	Port     string
	Database string
	User     string
	Password string
}

func AutoSetup() (*gorm.DB, error) {
	log.Println("[Setup] Memulai auto-setup database...")

	if creds := muatKredensialDariFile(); creds != nil {
		log.Println("[Setup] Kredensial ditemukan di", credentialFile)
		db, err := connectWithCredentials(creds)
		if err == nil {
			log.Println("[Setup] Koneksi berhasil menggunakan kredensial tersimpan")
			if err := jalankanAutoMigrate(db); err != nil {
				return nil, fmt.Errorf("gagal menjalankan auto-migrate: %w", err)
			}
			return db, nil
		}
		log.Printf("[Setup] Koneksi gagal dengan kredensial tersimpan: %v, mencoba setup ulang...", err)
	}

	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		log.Println("[Setup] Menggunakan DATABASE_URL dari environment")
		db, err := gorm.Open(postgres.Open(dbURL), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Warn),
		})
		if err == nil {
			if err := jalankanAutoMigrate(db); err != nil {
				return nil, fmt.Errorf("gagal menjalankan auto-migrate: %w", err)
			}
			return db, nil
		}
		log.Printf("[Setup] Koneksi DATABASE_URL gagal: %v", err)
	}

	log.Println("[Setup] Memulai setup database baru...")
	return setupDatabaseBaru()
}

func setupDatabaseBaru() (*gorm.DB, error) {
	setupHost := getEnvOrDefault("SETUP_DB_HOST", defaultSetupHost)
	setupPort := getEnvOrDefault("SETUP_DB_PORT", defaultSetupPort)
	setupUser := getEnvOrDefault("SETUP_DB_USER", defaultSetupUser)
	setupPassword := getEnvOrDefault("SETUP_DB_PASSWORD", defaultSetupPassword)

	adminDSN := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=postgres sslmode=disable",
		setupHost, setupPort, setupUser, setupPassword)

	log.Printf("[Setup] Menghubungkan ke PostgreSQL sebagai %s@%s:%s...", setupUser, setupHost, setupPort)

	adminDB, err := sql.Open("postgres", adminDSN)
	if err != nil {
		return nil, fmt.Errorf("gagal koneksi ke PostgreSQL: %w", err)
	}
	defer adminDB.Close()

	if err := adminDB.Ping(); err != nil {
		return nil, fmt.Errorf("gagal ping PostgreSQL: %w", err)
	}

	log.Println("[Setup] Koneksi admin berhasil")

	appPassword, err := buatDatabaseDanUser(adminDB, setupHost, setupPort)
	if err != nil {
		return nil, fmt.Errorf("gagal setup database/user: %w", err)
	}

	creds := &Credentials{
		Host:     setupHost,
		Port:     setupPort,
		Database: appDatabase,
		User:     appUser,
		Password: appPassword,
	}

	if err := simpanKredensialKeFile(creds); err != nil {
		log.Printf("[Setup] Warning: gagal menyimpan kredensial: %v", err)
	}

	db, err := connectWithCredentials(creds)
	if err != nil {
		return nil, fmt.Errorf("gagal koneksi dengan kredensial baru: %w", err)
	}

	if err := jalankanAutoMigrate(db); err != nil {
		return nil, fmt.Errorf("gagal menjalankan auto-migrate: %w", err)
	}

	log.Println("[Setup] Setup database selesai!")
	return db, nil
}

func buatDatabaseDanUser(adminDB *sql.DB, host, port string) (string, error) {
	var exists bool
	err := adminDB.QueryRow("SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)", appDatabase).Scan(&exists)
	if err != nil {
		return "", fmt.Errorf("gagal cek database: %w", err)
	}

	if !exists {
		log.Printf("[Setup] Membuat database '%s'...", appDatabase)
		_, err = adminDB.Exec(fmt.Sprintf("CREATE DATABASE %s", appDatabase))
		if err != nil {
			return "", fmt.Errorf("gagal buat database: %w", err)
		}
	} else {
		log.Printf("[Setup] Database '%s' sudah ada", appDatabase)
	}

	err = adminDB.QueryRow("SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname = $1)", appUser).Scan(&exists)
	if err != nil {
		return "", fmt.Errorf("gagal cek user: %w", err)
	}

	var password string
	if !exists {
		password = generateRandomPassword(32)
		log.Printf("[Setup] Membuat user '%s'...", appUser)
		_, err = adminDB.Exec(fmt.Sprintf("CREATE USER %s WITH PASSWORD '%s'", appUser, password))
		if err != nil {
			return "", fmt.Errorf("gagal buat user: %w", err)
		}

		_, err = adminDB.Exec(fmt.Sprintf("GRANT ALL PRIVILEGES ON DATABASE %s TO %s", appDatabase, appUser))
		if err != nil {
			return "", fmt.Errorf("gagal grant privileges: %w", err)
		}

		appDSN := fmt.Sprintf("host=%s port=%s user=postgres password=%s dbname=%s sslmode=disable",
			host, port, getEnvOrDefault("SETUP_DB_PASSWORD", defaultSetupPassword), appDatabase)

		appDB, err := sql.Open("postgres", appDSN)
		if err == nil {
			defer appDB.Close()
			_, _ = appDB.Exec(fmt.Sprintf("GRANT ALL ON SCHEMA public TO %s", appUser))
			_, _ = appDB.Exec(fmt.Sprintf("ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON TABLES TO %s", appUser))
			_, _ = appDB.Exec(fmt.Sprintf("ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON SEQUENCES TO %s", appUser))
		}

		log.Printf("[Setup] User '%s' berhasil dibuat", appUser)
	} else {
		log.Printf("[Setup] User '%s' sudah ada, mencoba load password dari file...", appUser)
		if creds := muatKredensialDariFile(); creds != nil {
			password = creds.Password
		} else {
			password = generateRandomPassword(32)
			_, err = adminDB.Exec(fmt.Sprintf("ALTER USER %s WITH PASSWORD '%s'", appUser, password))
			if err != nil {
				return "", fmt.Errorf("gagal update password: %w", err)
			}
			log.Printf("[Setup] Password user '%s' diperbarui", appUser)
		}
	}

	return password, nil
}

func connectWithCredentials(creds *Credentials) (*gorm.DB, error) {
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		creds.Host, creds.Port, creds.User, creds.Password, creds.Database)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	return db, nil
}

func jalankanAutoMigrate(db *gorm.DB) error {
	log.Println("[Setup] Menjalankan GORM AutoMigrate...")

	allModels := []interface{}{
		&models.Device{},
		&models.DeviceParameter{},
		&models.Task{},
		&models.Fault{},
		&models.Firmware{},
		&models.User{},
		&models.ProvisioningRule{},
		&Setting{},
	}

	for _, model := range allModels {
		if err := db.AutoMigrate(model); err != nil {
			log.Printf("[Setup] Warning: AutoMigrate untuk %T: %v (mungkin tabel sudah ada)", model, err)
		}
	}

	buatDefaultData(db)

	log.Println("[Setup] AutoMigrate selesai")
	return nil
}

type Setting struct {
	Key         string    `gorm:"primaryKey"`
	Value       string    `gorm:"not null"`
	Description string
	UpdatedAt   time.Time `gorm:"autoUpdateTime"`
}

func (Setting) TableName() string {
	return "settings"
}

func buatDefaultData(db *gorm.DB) {
	defaultSettings := []Setting{
		{Key: "acs_url", Value: "http://localhost:7547/", Description: "ACS URL untuk CPE connection"},
		{Key: "acs_username", Value: "", Description: "Username untuk CWMP authentication"},
		{Key: "acs_password", Value: "", Description: "Password untuk CWMP authentication"},
		{Key: "inform_interval", Value: "3600", Description: "Periodic inform interval (seconds)"},
		{Key: "connection_request_username", Value: "admin", Description: "Username untuk connection request"},
		{Key: "connection_request_password", Value: "", Description: "Password untuk connection request"},
	}

	for _, s := range defaultSettings {
		db.Where(Setting{Key: s.Key}).FirstOrCreate(&s)
	}

	var count int64
	db.Model(&models.User{}).Count(&count)
	if count == 0 {
		defaultAdmin := models.User{
			Username:     "admin",
			PasswordHash: "$2a$12$nKdFyIgp4jmSASeOXMjq2eHRjk9J4ypQdXkxEEz2Q2nURZ4Fi9PVW",
			Role:         models.RoleFull,
		}
		db.Create(&defaultAdmin)
		log.Println("[Setup] Default admin user dibuat (username: admin, password: admin)")
	}
}

func muatKredensialDariFile() *Credentials {
	data, err := os.ReadFile(credentialFile)
	if err != nil {
		return nil
	}

	creds := &Credentials{}
	lines := strings.Split(string(data), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.Trim(strings.TrimSpace(parts[1]), "\"'")

		switch key {
		case "DB_HOST":
			creds.Host = value
		case "DB_PORT":
			creds.Port = value
		case "DB_NAME":
			creds.Database = value
		case "DB_USER":
			creds.User = value
		case "DB_PASSWORD":
			creds.Password = value
		}
	}

	if creds.Host == "" || creds.Database == "" || creds.User == "" || creds.Password == "" {
		return nil
	}

	return creds
}

func simpanKredensialKeFile(creds *Credentials) error {
	content := fmt.Sprintf(`# MiniACS Database Credentials
# Generated automatically - DO NOT EDIT

DB_HOST=%s
DB_PORT=%s
DB_NAME=%s
DB_USER=%s
DB_PASSWORD=%s
`, creds.Host, creds.Port, creds.Database, creds.User, creds.Password)

	return os.WriteFile(credentialFile, []byte(content), 0600)
}

func generateRandomPassword(length int) string {
	bytes := make([]byte, length/2)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
