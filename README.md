# miniACS - TR-069/CWMP Management System

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![Node.js Version](https://img.shields.io/badge/Node.js-20.x-339933?style=flat&logo=node.js)](https://nodejs.org/)
[![PostgreSQL Version](https://img.shields.io/badge/PostgreSQL-14+-4169E1?style=flat&logo=postgresql)](https://www.postgresql.org/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Version](https://img.shields.io/badge/Version-1.0.0--beta-blue.svg)](https://github.com/skydashnet/miniacs)

**TR-069/CWMP Auto Configuration Server**

miniACS adalah ACS (Auto Configuration Server) ringan untuk manajemen perangkat CPE melalui protokol TR-069/CWMP. Dibangun dengan fokus pada kesederhanaan, performa, dan kemudahan deployment.

---

## Fitur Utama

- **Device Management** - Monitoring dan kontrol perangkat CPE secara real-time
- **Parameter Configuration** - Get/Set parameter values melalui TR-069
- **WiFi Management** - Edit SSID, password, dan konfigurasi wireless
- **PPPoE Configuration** - Edit kredensial PPPoE langsung dari dashboard
- **Firmware Management** - Upload dan distribusi firmware ke perangkat
- **Fault Tracking** - Logging dan monitoring fault dari perangkat
- **Auto Provisioning** - Konfigurasi otomatis untuk perangkat baru
- **Auto Summon** - Auto trigger connection request untuk device yang tidak responsif
- **Connection Request** - Trigger inform dari perangkat (Digest Auth support)
- **Dark/Light Theme** - UI modern dengan dukungan tema gelap dan terang

---

## Tech Stack

| Layer | Technology |
|-------|------------|
| Backend | Go 1.21+ |
| Frontend | SolidJS + Vite |
| Database | PostgreSQL 14+ |
| Process Manager | PM2 |

---

## Requirements

### Minimum System
- **OS**: Ubuntu 20.04+ / Debian 11+ (atau distro berbasis Debian lainnya)
- **RAM**: 1 GB
- **Storage**: 500 MB
- **CPU**: 1 core

### Software Dependencies
- Go 1.21+
- Node.js 20.x
- PostgreSQL 14+
- PM2 (opsional, untuk production)

---

## Quick Start

### Automated Setup (Recommended)

```bash
# Clone repository
git clone https://github.com/skydashnet/miniacs.git
cd miniacs

# Jalankan auto setup
chmod +x auto-setup.sh
sudo ./auto-setup.sh
```

Script akan otomatis:
1. Install Go, Node.js, dan PostgreSQL
2. Setup database dan user
3. Build backend dan frontend
4. Konfigurasi PM2 untuk process management
5. Setup systemd service untuk auto-start

### Manual Setup

```bash
# Setup database
chmod +x setup.sh
./setup.sh

# Build backend
cd backend
go build -o miniacs cmd/server/main.go

# Build frontend
cd ../frontend
npm install
npm run build

# Jalankan
cd ../backend
./miniacs
```

---

## Konfigurasi

Buat file `.miniacs.env` di folder `backend/`:

```env
DB_HOST=localhost
DB_PORT=5432
DB_NAME=miniacs
DB_USER=miniacs
DB_PASSWORD=your_secure_password
```

---

## Default Credentials

| Service | Username | Password |
|---------|----------|----------|
| Web UI | admin | admin |

> Ubah password default setelah instalasi melalui menu Settings.

---

## API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/` | TR-069 CWMP endpoint |
| GET | `/api/devices` | List semua device |
| GET | `/api/device/{serial}` | Detail device by serial |
| POST | `/api/device/{serial}/connection-request` | Trigger inform |
| POST | `/api/device/{serial}/set-parameters` | Set parameter values |
| GET | `/api/devices/analytics` | Dashboard analytics |
| GET | `/api/faults` | List semua fault |

---

## Supported Devices

miniACS mendukung perangkat yang mengimplementasikan:
- TR-069 (CWMP)
- TR-098 (InternetGatewayDevice)
- TR-181 (Device:2)

Tested dengan:
- Huawei HG8245H, HG8245H5, HG8546M
- ZTE F660, F670L
- FiberHome AN5506

---

## Troubleshooting

**Device tidak muncul di dashboard**
- Pastikan ACS URL di perangkat sudah benar: `http://server-ip:7547/`
- Cek konektivitas jaringan antara CPE dan server ACS

**Connection Request gagal**
- Pastikan kredensial Connection Request sudah dikonfigurasi di Settings
- Periksa firewall tidak memblokir port CPE

**Database connection error**
- Jalankan `./setup.sh` untuk setup database
- Verifikasi kredensial di `backend/.miniacs.env`

---

## License

MIT License - Lihat file [LICENSE](LICENSE) untuk detail.

---

## Author

**SkydashNET**

---

`v1.0.0-beta`
