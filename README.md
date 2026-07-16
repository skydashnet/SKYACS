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
- Full/read-only role, revocable session-only web token, password policy, login throttling, dan audit trail
- Device blocklist, CWMP Basic Auth over TLS, allowlist CIDR, dan trusted-proxy validation
- DNS-rebinding/redirect-resistant SSRF guard untuk connection request URL
- Signed firmware URL dengan token acak dan expiry; task firmware kedaluwarsa setelah 24 jam
- Enkripsi AES-GCM untuk parameter sensitif di database dan masking untuk operator read-only
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
# edit backend/.env; JWT_SECRET dan PARAMETER_ENCRYPTION_KEY wajib diisi
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
CWMP_BIND_ADDR=127.0.0.1
API_BIND_ADDR=127.0.0.1
JWT_SECRET=<minimum-32-random-characters>
PARAMETER_ENCRYPTION_KEY=<minimum-32-random-characters>

DB_HOST=localhost
DB_PORT=5432
DB_NAME=miniacs
DB_USER=miniacs
DB_PASSWORD=<database-password>
DB_MAX_OPEN_CONNS=50
DB_MAX_IDLE_CONNS=10

CORS_ALLOWED_ORIGINS=https://acs.example.com
TRUSTED_PROXY_CIDRS=127.0.0.1/32
CWMP_USERNAME=
CWMP_PASSWORD=
CWMP_ALLOWED_CIDRS=10.0.0.0/8,192.0.2.0/24
CWMP_TRUSTED_PROXY_CIDRS=127.0.0.1/32
CONNECTION_REQUEST_ALLOWED_CIDRS=10.0.0.0/8,192.0.2.0/24
FIRMWARE_UPLOAD_DIR=/var/lib/miniacs/firmware
ALLOW_INSECURE_FIRMWARE_URL=false
```

Atur dari **Settings** setelah login:

- `firmware_base_url`: base URL API yang dapat dijangkau CPE, contoh `https://acs.example.com/api` bila Nginx memakai prefix `/api`.
- connection request credentials; mode otomatis membutuhkan master secret minimum 16 karakter dan menghasilkan password HMAC-SHA256 unik per serial number.

Jika `CWMP_USERNAME` digunakan, `CWMP_PASSWORD` juga wajib diset. Basic Auth ditolak pada HTTP biasa: terminasi TLS harus berasal dari alamat pada `CWMP_TRUSTED_PROXY_CIDRS`. `TRUSTED_PROXY_CIDRS` mengontrol header client-IP API dan harus berisi alamat proxy saja, bukan jaringan pengguna. `CONNECTION_REQUEST_ALLOWED_CIDRS` membatasi alamat tujuan yang boleh dipanggil ACS dan sebaiknya diisi jaringan CPE. Batasi endpoint CWMP ke jaringan CPE memakai firewall meskipun allowlist aplikasi sudah aktif.

`PARAMETER_ENCRYPTION_KEY` mengenkripsi password dan parameter sensitif yang tersimpan. Backup key bersama backup database dan jangan menggantinya langsung; rotasi key memerlukan migrasi/re-enkripsi data.

## Reverse proxy

Gunakan [setup_nginx.md](setup_nginx.md) untuk TLS, static frontend, API prefix, dan firmware upload limit. Untuk deployment melalui Cloudflare, baca [setup_cloudflare.md](setup_cloudflare.md); endpoint firmware harus tetap dapat dijangkau CPE tanpa interactive Access login.

## Production baseline

Sebelum membawa miniACS ke jaringan operasional:

1. Pasang TLS reverse proxy untuk web/API dan CWMP; jangan expose `5173` atau `7548` langsung.
2. Isi CORS dan trusted proxy dengan nilai eksplisit, lalu batasi CWMP menggunakan firewall/CIDR.
3. Gunakan user service non-root, aktifkan backup PostgreSQL dan direktori firmware, serta uji restore.
4. Uji Get/Set, reboot, factory reset, dan firmware pada setiap kombinasi vendor/model/version. Factory reset membutuhkan re-authentication akun.
5. Monitor `/api/health`, systemd, kapasitas database, disk firmware, serta task/fault yang gagal.

Contoh backup single-node:

```bash
sudo -u postgres pg_dump -Fc miniacs > miniacs-$(date +%F).dump
sudo tar -C /var/lib/miniacs -czf miniacs-firmware-$(date +%F).tar.gz firmware
```

miniACS saat ini adalah control plane **single-node**. Storage firmware lokal dan scheduler in-process belum dirancang untuk active-active/HA; gunakan satu instance backend per database. Status proyek tetap beta sampai matriks interoperabilitas vendor dan uji beban fleet dipublikasikan.

## Validasi

```bash
cd backend
go test -race ./...
go vet ./...
go run honnef.co/go/tools/cmd/staticcheck@2025.1.1 ./...
go run golang.org/x/vuln/cmd/govulncheck@v1.6.0 ./...
go run github.com/securego/gosec/v2/cmd/gosec@v2.22.9 -quiet ./...

cd ../frontend
npm ci
npm audit
npm run build

cd ..
bash -n setup.sh auto-setup.sh
```

CI menjalankan rangkaian yang sama pada setiap push dan pull request.

## Dukungan perangkat

miniACS menangani perangkat yang mematuhi CWMP/TR-069 dengan root TR-098 atau TR-181. Vendor extension tetap berbeda antar firmware; selalu verifikasi parameter writable di lab. Profil UI saat ini mengenali pola umum Huawei, ZTE, dan FiberHome, tetapi kompatibilitas tidak dijamin untuk setiap versi firmware.

RPC yang saat ini ditangani meliputi `Inform`, `TransferComplete`, SOAP Fault, serta response untuk `GetParameterValues`, `GetParameterNames`, `SetParameterValues`, `Reboot`, `FactoryReset`, dan `Download`. Method di luar matriks tersebut menerima CWMP fault `8000` dan harus diuji sebelum perangkat yang bergantung padanya dimasukkan ke fleet produksi.

## Dukung miniACS

Jika miniACS membantu operasional jaringanmu, dukung pengembangan dan pemeliharaan proyek ini melalui Saweria.

[![Dukung miniACS di Saweria](https://img.shields.io/badge/Saweria-Dukung%20miniACS-faae2b?style=for-the-badge)](https://saweria.co/skydashnet)

## Reporting security issues

Jangan membuka detail kerentanan yang belum ditangani sebagai public issue. Ikuti proses pada [SECURITY.md](SECURITY.md).

## License

GNU Affero General Public License v3.0. Lihat [LICENSE](LICENSE).
