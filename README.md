# miniACS

[![CI](https://github.com/skydashnet/miniACS/actions/workflows/ci.yml/badge.svg)](https://github.com/skydashnet/miniACS/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go)](https://go.dev/)
[![Node.js](https://img.shields.io/badge/Node.js-20+-339933?logo=node.js)](https://nodejs.org/)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-14+-4169E1?logo=postgresql)](https://www.postgresql.org/)
[![License](https://img.shields.io/badge/License-AGPL--3.0-blue.svg)](LICENSE)

miniACS adalah control plane TR-069/CWMP mandiri untuk inventarisasi, monitoring, provisioning, dan konfigurasi CPE. Backend CWMP, API, scheduler, database, dan web console berjalan langsung di miniACS; instalasi tidak membutuhkan GenieACS.

> Status: beta. Uji di lab dan siapkan jalur recovery perangkat sebelum menjalankan perubahan massal atau firmware upgrade.

## Highlights

- TR-098 (`InternetGatewayDevice`) dan TR-181 (`Device`) data-model discovery
- Inventaris, statistik, parameter tree, fault, provisioning rule, dan task history
- Get/Set parameter, reboot, factory reset, connection request, serta firmware delivery
- Full/read-only role, session-only web token, password policy, login throttling, dan audit trail
- Device blocklist, optional CWMP Basic Auth, dan allowlist CIDR
- SSRF guard untuk connection request URL
- Signed firmware URL dengan token acak per file
- Responsive enterprise console menggunakan IBM Plex Sans dan IBM Plex Mono
- Dark/light theme tanpa dependency font atau UI dari GenieACS

Implementasi full-root parameter discovery dan metadata writable mengadaptasi pola yang sudah dipakai pada miniACS di `skydash-netsh`. Struktur provisioning terinspirasi oleh katalog pada `skydashnet/genieacs-installer`, tetapi diterapkan langsung ke task engine miniACS.

## Arsitektur

| Service | Default | Fungsi |
| --- | ---: | --- |
| Web console | `5173/tcp` | SolidJS production bundle |
| API | `7548/tcp` | Authenticated management API dan signed firmware files |
| CWMP | `7547/tcp` | Session TR-069 dari CPE |
| PostgreSQL | `5432/tcp` | Persistent state |

Backend menggunakan Go 1.25, GORM, dan PostgreSQL. Frontend menggunakan SolidJS, TypeScript, Tailwind CSS, dan Vite.

## Quick start

### Automated install (Debian/Ubuntu)

```bash
git clone https://github.com/skydashnet/miniACS.git
cd miniACS
chmod +x auto-setup.sh
sudo ./auto-setup.sh
```

Installer membuat database, secret, build produksi, dan dua unit systemd: `miniacs` serta `miniacs-web`. Password bootstrap dicetak sekali dan juga dapat dilihat pada startup pertama dengan:

```bash
sudo journalctl -u miniacs -n 50 --no-pager
```

### Manual development

```bash
cp backend/.env.example backend/.env
# edit backend/.env; JWT_SECRET wajib diisi
./setup.sh

cd backend
go run ./cmd/server
```

Pada terminal lain:

```bash
cd frontend
npm ci
npm run dev
```

Login awal memakai username `admin`. Password berasal dari `INITIAL_ADMIN_PASSWORD`; jika variabel itu kosong, backend mencetak password acak sekali saat membuat user pertama. Tidak ada default `admin/admin`.

## Konfigurasi keamanan

Variabel penting di `backend/.env`:

```env
PORT=7547
API_PORT=7548
JWT_SECRET=<minimum-32-random-characters>

DB_HOST=localhost
DB_PORT=5432
DB_NAME=miniacs
DB_USER=miniacs
DB_PASSWORD=<database-password>

CORS_ALLOWED_ORIGINS=https://acs.example.com
CWMP_USERNAME=
CWMP_PASSWORD=
CWMP_ALLOWED_CIDRS=10.0.0.0/8,192.0.2.0/24
FIRMWARE_UPLOAD_DIR=/var/lib/miniacs/firmware
```

Atur dari **Settings** setelah login:

- `acs_url`: endpoint CWMP yang diterima CPE, contoh `https://cwmp.example.com/`.
- `firmware_base_url`: base URL API yang dapat dijangkau CPE, contoh `https://acs.example.com/api` bila Nginx memakai prefix `/api`.
- connection request credentials dan Inform interval.

Jika `CWMP_USERNAME` digunakan, `CWMP_PASSWORD` juga wajib diset dan CPE harus dikonfigurasi dengan pasangan yang sama. Batasi `7547/tcp` ke jaringan CPE memakai firewall meskipun allowlist aplikasi sudah aktif.

## Reverse proxy

Gunakan [setup_nginx.md](setup_nginx.md) untuk TLS, static frontend, API prefix, dan firmware upload limit. Untuk deployment melalui Cloudflare, baca [setup_cloudflare.md](setup_cloudflare.md); endpoint firmware harus tetap dapat dijangkau CPE tanpa interactive Access login.

## Validasi

```bash
cd backend
go test ./...
go vet ./...

cd ../frontend
npm ci
npm run build
```

CI menjalankan rangkaian yang sama pada setiap push dan pull request.

## Dukungan perangkat

miniACS menangani perangkat yang mematuhi CWMP/TR-069 dengan root TR-098 atau TR-181. Vendor extension tetap berbeda antar firmware; selalu verifikasi parameter writable di lab. Profil UI saat ini mengenali pola umum Huawei, ZTE, dan FiberHome, tetapi kompatibilitas tidak dijamin untuk setiap versi firmware.

## Reporting security issues

Jangan membuka detail kerentanan yang belum ditangani sebagai public issue. Ikuti proses pada [SECURITY.md](SECURITY.md).

## License

GNU Affero General Public License v3.0. Lihat [LICENSE](LICENSE).
