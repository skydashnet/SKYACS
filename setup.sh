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
    else
        echo "Warning: Database exists tapi .miniacs.env tidak ditemukan!"
        echo "Gunakan credentials yang ada atau hapus database untuk fresh install."
        exit 1
    fi
else
    echo "Membuat database '$APP_DB'..."
    PGPASSWORD="$DB_ADMIN_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_ADMIN_USER" -c "CREATE DATABASE $APP_DB;" 2>/dev/null || true
    
    echo "Membuat user '$APP_USER'..."
    PGPASSWORD="$DB_ADMIN_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_ADMIN_USER" -c "CREATE USER $APP_USER WITH PASSWORD '$APP_PASSWORD';" 2>/dev/null || true
    PGPASSWORD="$DB_ADMIN_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_ADMIN_USER" -c "GRANT ALL PRIVILEGES ON DATABASE $APP_DB TO $APP_USER;" 2>/dev/null
    PGPASSWORD="$DB_ADMIN_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_ADMIN_USER" -d "$APP_DB" -c "GRANT ALL ON SCHEMA public TO $APP_USER;" 2>/dev/null
    
    # Save credentials
    cat > backend/.miniacs.env << EOF
# miniACS Database Credentials (auto-generated)
DB_HOST=$DB_HOST
DB_PORT=$DB_PORT
DB_NAME=$APP_DB
DB_USER=$APP_USER
DB_PASSWORD=$APP_PASSWORD
EOF
    chmod 600 backend/.miniacs.env
    echo "Credentials disimpan di backend/.miniacs.env"
fi

echo ""
echo "Menjalankan migrasi..."

# Create tables
PGPASSWORD="$APP_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$APP_USER" -d "$APP_DB" << 'EOSQL'

-- Devices table
CREATE TABLE IF NOT EXISTS devices (
    id BIGSERIAL PRIMARY KEY,
    serial_number VARCHAR(255) UNIQUE NOT NULL,
    oui VARCHAR(50),
    manufacturer VARCHAR(255),
    product_class VARCHAR(255),
    model_name VARCHAR(255),
    hardware_version VARCHAR(255),
    software_version VARCHAR(255),
    ip_address VARCHAR(50),
    connection_request_url TEXT,
    last_inform TIMESTAMP,
    online BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Device parameters table
CREATE TABLE IF NOT EXISTS device_parameters (
    id BIGSERIAL PRIMARY KEY,
    device_id BIGINT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    value TEXT,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_device_parameters_device_id ON device_parameters(device_id);
CREATE INDEX IF NOT EXISTS idx_device_parameters_name ON device_parameters(name);

-- Tasks table
DO $$ BEGIN
    CREATE TYPE task_status AS ENUM ('pending', 'sent', 'completed', 'failed', 'cancelled');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

CREATE TABLE IF NOT EXISTS tasks (
    id BIGSERIAL PRIMARY KEY,
    device_id BIGINT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    type VARCHAR(50) NOT NULL,
    status task_status DEFAULT 'pending',
    payload JSONB,
    result JSONB,
    error_message TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_tasks_device_id ON tasks(device_id);
CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);

-- Faults table
CREATE TABLE IF NOT EXISTS faults (
    id BIGSERIAL PRIMARY KEY,
    device_id BIGINT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    fault_code VARCHAR(50),
    fault_message TEXT,
    detail TEXT,
    timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_faults_device_id ON faults(device_id);

-- Firmwares table
CREATE TABLE IF NOT EXISTS firmwares (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    version VARCHAR(100),
    file_path TEXT NOT NULL,
    file_size BIGINT,
    checksum VARCHAR(64),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Users table
CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    username VARCHAR(255) UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    role VARCHAR(50) DEFAULT 'viewer',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Settings table
CREATE TABLE IF NOT EXISTS settings (
    id BIGSERIAL PRIMARY KEY,
    key VARCHAR(255) UNIQUE NOT NULL,
    value TEXT,
    description TEXT
);

-- Provisioning rules table
CREATE TABLE IF NOT EXISTS provisioning_rules (
    id BIGSERIAL PRIMARY KEY,
    parameter_name TEXT NOT NULL,
    value TEXT NOT NULL,
    priority INT DEFAULT 0,
    active BOOLEAN DEFAULT TRUE,
    model_pattern TEXT,
    manufacturer_pattern TEXT,
    description TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Insert default settings
INSERT INTO settings (key, value, description) VALUES
    ('acs_url', 'http://localhost:7547/', 'ACS URL untuk CPE connection'),
    ('acs_username', '', 'Username untuk CWMP authentication'),
    ('acs_password', '', 'Password untuk CWMP authentication'),
    ('inform_interval', '3600', 'Periodic inform interval (seconds)'),
    ('connection_request_username', 'admin', 'Username untuk connection request'),
    ('connection_request_password', '', 'Password untuk connection request')
ON CONFLICT (key) DO NOTHING;

-- Insert default admin user (password: admin)
INSERT INTO users (username, password_hash, role) VALUES
    ('admin', '$2a$12$nKdFyIgp4jmSASeOXMjq2eHRjk9J4ypQdXkxEEz2Q2nURZ4Fi9PVW', 'full')
ON CONFLICT (username) DO NOTHING;

EOSQL

echo ""
echo "========================================"
echo "  Setup selesai!"
echo "========================================"
echo ""
echo "Jalankan backend dengan:"
echo "  cd backend && go run cmd/server/main.go"
echo ""
echo "Default login: admin / admin"
echo ""
