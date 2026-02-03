#!/bin/bash

# miniACS Database Setup Script
# Run this once before starting the backend

set -e

# Default PostgreSQL connection
DB_HOST="${SETUP_DB_HOST:-localhost}"
DB_PORT="${SETUP_DB_PORT:-5432}"
DB_ADMIN_USER="${SETUP_DB_USER:-postgres}"
DB_ADMIN_PASSWORD="${SETUP_DB_PASSWORD:-postgres}"

# Application database config
APP_DB="miniacs"
APP_USER="miniacs"
APP_PASSWORD=$(openssl rand -hex 16)

echo "========================================"
echo "  miniACS Database Setup"
echo "========================================"
echo ""
echo "Connecting to PostgreSQL at $DB_HOST:$DB_PORT..."

# Check if database exists
DB_EXISTS=$(PGPASSWORD="$DB_ADMIN_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_ADMIN_USER" -tAc "SELECT 1 FROM pg_database WHERE datname='$APP_DB'" 2>/dev/null || echo "0")

if [ "$DB_EXISTS" = "1" ]; then
    echo "Database '$APP_DB' sudah ada."
    
    # Get existing credentials from .miniacs.env if exists
    if [ -f "backend/.miniacs.env" ]; then
        echo "Menggunakan credentials dari backend/.miniacs.env"
        source backend/.miniacs.env
        APP_PASSWORD="$DB_PASSWORD"
    elif [ -f "backend/.env" ]; then
        echo "Menggunakan credentials dari backend/.env"
        source backend/.env
        APP_PASSWORD="$DB_PASSWORD"
    else
        echo "Warning: Database exists tapi .env tidak ditemukan!"
        echo "Gunakan credentials yang ada atau hapus database untuk fresh install."
        exit 1
    fi
else
    echo "Membuat database '$APP_DB'..."
    PGPASSWORD="$DB_ADMIN_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_ADMIN_USER" -c "CREATE DATABASE $APP_DB;" 2>/dev/null || true
    
    echo "Membuat user '$APP_USER'..."
    PGPASSWORD="$DB_ADMIN_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_ADMIN_USER" -c "CREATE USER $APP_USER WITH PASSWORD '$APP_PASSWORD';" 2>/dev/null || true
    
    echo "Memberikan privileges..."
    PGPASSWORD="$DB_ADMIN_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_ADMIN_USER" -c "GRANT ALL PRIVILEGES ON DATABASE $APP_DB TO $APP_USER;" 2>/dev/null
    PGPASSWORD="$DB_ADMIN_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_ADMIN_USER" -d "$APP_DB" -c "ALTER SCHEMA public OWNER TO $APP_USER;" 2>/dev/null
    PGPASSWORD="$DB_ADMIN_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_ADMIN_USER" -d "$APP_DB" -c "GRANT ALL PRIVILEGES ON SCHEMA public TO $APP_USER;" 2>/dev/null
    PGPASSWORD="$DB_ADMIN_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_ADMIN_USER" -d "$APP_DB" -c "GRANT CREATE ON SCHEMA public TO $APP_USER;" 2>/dev/null
    
    # Save credentials
    cat > backend/.env << EOF
# miniACS Configuration (auto-generated)
HOST=0.0.0.0
PORT=7547
API_PORT=7548
JWT_SECRET=$(openssl rand -hex 32)

# Database
DB_HOST=$DB_HOST
DB_PORT=$DB_PORT
DB_NAME=$APP_DB
DB_USER=$APP_USER
DB_PASSWORD=$APP_PASSWORD
EOF
    chmod 600 backend/.env
    echo "Credentials disimpan di backend/.env"
fi

echo ""
echo "========================================"
echo "  Setup selesai!"
echo "========================================"
echo ""
echo "Database dan user sudah siap."
echo "GORM akan otomatis membuat tabel saat backend dijalankan."
echo ""
echo "Jalankan backend dengan:"
echo "  cd backend && go run cmd/server/main.go"
echo ""
echo "Default login: admin / admin"
echo ""
