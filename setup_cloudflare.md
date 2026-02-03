# Setup Cloudflare Zero Trust untuk miniACS

Panduan untuk mengamankan akses miniACS menggunakan Cloudflare Zero Trust (Cloudflare Access + Cloudflare Tunnel).

---

## Arsitektur

```
[User] → [Cloudflare Access] → [Cloudflare Tunnel] → [Server miniACS]
                                                      ├── :5173 (Frontend)
                                                      ├── :7548 (API)
                                                      └── :7547 (CWMP - tidak di-tunnel)
```

> **PENTING**: Port 7547 (CWMP) TIDAK boleh di-tunnel karena modem/CPE harus bisa langsung akses ke server ACS. Cloudflare Tunnel hanya untuk Web UI dan API.

---

## Prerequisites

1. Akun Cloudflare dengan domain yang sudah terdaftar
2. Cloudflare Zero Trust aktif (free tier tersedia)
3. Server sudah running miniACS

---

## 1. Install cloudflared

```bash
# Debian/Ubuntu
curl -L --output cloudflared.deb https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-amd64.deb
sudo dpkg -i cloudflared.deb

# Verify
cloudflared --version
```

---

## 2. Authenticate dan Buat Tunnel

```bash
# Login ke Cloudflare
cloudflared tunnel login

# Buat tunnel baru
cloudflared tunnel create miniacs

# Catat Tunnel ID yang muncul (contoh: a1b2c3d4-xxxx-xxxx-xxxx-xxxxxxxxxxxx)
```

---

## 3. Konfigurasi Tunnel

Buat file konfigurasi di `~/.cloudflared/config.yml`:

```yaml
tunnel: <TUNNEL_ID>
credentials-file: /home/<username>/.cloudflared/<TUNNEL_ID>.json

ingress:
  # Frontend (Web UI)
  - hostname: acs.yourdomain.com
    service: http://localhost:5173
  
  # API endpoint
  - hostname: acs-api.yourdomain.com
    service: http://localhost:7548
  
  # Fallback (required)
  - service: http_status:404
```

> Ganti `<TUNNEL_ID>` dan `<username>` sesuai environment Anda.

---

## 4. Setup DNS Records

```bash
# Route traffic ke tunnel
cloudflared tunnel route dns miniacs acs.yourdomain.com
cloudflared tunnel route dns miniacs acs-api.yourdomain.com
```

Atau di dashboard Cloudflare, tambahkan CNAME record:
- `acs` → `<TUNNEL_ID>.cfargotunnel.com`
- `acs-api` → `<TUNNEL_ID>.cfargotunnel.com`

---

## 5. Konfigurasi Cloudflare Access (Zero Trust)

### Di Dashboard Zero Trust (https://one.dash.cloudflare.com)

1. **Access → Applications → Add an Application**
   - Type: Self-hosted
   - Application name: `miniACS`
   - Application domain: `acs.yourdomain.com`
   - (Tambahkan juga `acs-api.yourdomain.com` jika perlu)

2. **Add Policy**
   - Policy name: `Allow Admin`
   - Action: Allow
   - Include: Emails → `admin@yourdomain.com`
   - Atau: One-time PIN (untuk login via email)

3. **Settings**
   - Session Duration: 24 hours
   - Enable CORS headers jika perlu untuk API

---

## 6. Update Frontend Environment

Edit `frontend/.env` untuk production:

```env
VITE_API_URL=https://acs-api.yourdomain.com
```

Rebuild frontend:
```bash
cd frontend
npm run build
pm2 restart miniacs-frontend
```

---

## 7. Jalankan Tunnel sebagai Service

```bash
# Install service
sudo cloudflared service install

# Start service
sudo systemctl start cloudflared
sudo systemctl enable cloudflared

# Check status
sudo systemctl status cloudflared
```

---

## 8. Konfigurasi Modem/CPE

Modem tetap menggunakan IP publik langsung (bypass Cloudflare):

```
ACS URL: http://<PUBLIC_IP>:7547/
```

Pastikan port 7547 terbuka di firewall untuk akses dari jaringan modem.

---

## Firewall Rules

```bash
# UFW contoh
sudo ufw allow 7547/tcp  # CWMP untuk modem
sudo ufw deny 5173/tcp   # Block direct frontend (via tunnel saja)
sudo ufw deny 7548/tcp   # Block direct API (via tunnel saja)
```

---

## Troubleshooting

### Tunnel tidak konek
```bash
cloudflared tunnel run miniacs --loglevel debug
```

### Check tunnel status
```bash
cloudflared tunnel info miniacs
```

### Test koneksi lokal
```bash
curl http://localhost:5173  # Frontend
curl http://localhost:7548/health  # API
```

---

## Security Checklist

- [ ] Port 5173 dan 7548 hanya bisa diakses via Cloudflare Tunnel
- [ ] Port 7547 hanya terbuka untuk IP range modem/CPE
- [ ] Cloudflare Access policy sudah dikonfigurasi
- [ ] HTTPS enforced di Cloudflare
- [ ] Session timeout sesuai kebutuhan
