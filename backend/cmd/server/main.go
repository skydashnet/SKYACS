package main

import (
	"context"
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
	"github.com/skydashnet/miniacs/internal/cwmp"
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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, err := connectDatabase()
	if err != nil {
		log.Printf("Warning: Database connection failed: %v", err)
		log.Println("Jalankan ./setup.sh dulu untuk setup database")
		log.Println("Server will run without database features")
	} else {
		log.Println("Database connected successfully")
	}

	mux := http.NewServeMux()

	cwmpHandler := cwmp.NewHandler(db)
	mux.Handle("/", cwmpHandler)

	if db != nil {
		apiRouter := api.NewRouter(db)
		mux.Handle("/api/", http.StripPrefix("/api", apiRouter.Handler()))

		watchdog := cwmp.NewDeviceWatchdog(db)
		go watchdog.Start(ctx)
	} else {
		mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"status":"ok","database":"disconnected"}`))
		})
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "7547"
	}

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	go func() {
		log.Printf("miniACS server berjalan di :%s", port)
		log.Printf("  CWMP endpoint: http://localhost:%s/", port)
		log.Printf("  API endpoint: http://localhost:%s/api/", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server gagal start: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server shutdown failed: %v", err)
	}

	log.Println("Server stopped")
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
