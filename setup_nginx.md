# Setup Nginx untuk miniACS

Panduan untuk konfigurasi Nginx sebagai reverse proxy untuk miniACS.

---

## Arsitektur

```
                         ┌── /          → :5173 (Frontend)
[Client] → [Nginx:80/443] ─┼── /api      → :7548 (API)
                         └── :7547       → Direct (CWMP - bypass Nginx)
```

> **PENTING**: Port 7547 TIDAK di-proxy karena CWMP memerlukan direct connection untuk session tracking.

---

## Prerequisites

1. Nginx sudah terinstall
2. miniACS sudah running (PM2 atau manual)
3. Domain sudah pointing ke server (opsional, untuk SSL)

---

## 1. Install Nginx

```bash
# Debian/Ubuntu
sudo apt update
sudo apt install nginx

# Start dan enable
sudo systemctl start nginx
sudo systemctl enable nginx
```

---

## 2. Konfigurasi Dasar (HTTP Only)

Buat file `/etc/nginx/sites-available/miniacs`:

```nginx
server {
    listen 80;
    server_name acs.yourdomain.com;  # Ganti dengan domain/IP Anda

    # Logging
    access_log /var/log/nginx/miniacs_access.log;
    error_log /var/log/nginx/miniacs_error.log;

    # Frontend (SolidJS)
    location / {
        proxy_pass http://127.0.0.1:5173;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # SolidJS hot reload (development)
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }

    # API endpoint
    location /api {
        proxy_pass http://127.0.0.1:7548;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # Timeout untuk long-polling
        proxy_read_timeout 300s;
        proxy_connect_timeout 75s;
    }

    # Health check endpoint
    location /health {
        proxy_pass http://127.0.0.1:7548/health;
    }
}
```

Aktifkan konfigurasi:

```bash
sudo ln -s /etc/nginx/sites-available/miniacs /etc/nginx/sites-enabled/
sudo nginx -t
sudo systemctl reload nginx
```

---

## 3. Konfigurasi dengan SSL (HTTPS)

### Menggunakan Certbot (Let's Encrypt)

```bash
# Install certbot
sudo apt install certbot python3-certbot-nginx

# Generate certificate
sudo certbot --nginx -d acs.yourdomain.com

# Auto-renewal (crontab)
sudo crontab -e
# Tambahkan:
0 0 * * * /usr/bin/certbot renew --quiet
```

### Konfigurasi Manual SSL

```nginx
server {
    listen 80;
    server_name acs.yourdomain.com;
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name acs.yourdomain.com;

    ssl_certificate /etc/letsencrypt/live/acs.yourdomain.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/acs.yourdomain.com/privkey.pem;
    
    # SSL Settings
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_prefer_server_ciphers on;
    ssl_ciphers ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256;
    ssl_session_cache shared:SSL:10m;
    ssl_session_timeout 10m;

    # Security Headers
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-XSS-Protection "1; mode=block" always;

    # Logging
    access_log /var/log/nginx/miniacs_access.log;
    error_log /var/log/nginx/miniacs_error.log;

    # Frontend
    location / {
        proxy_pass http://127.0.0.1:5173;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }

    # API
    location /api {
        proxy_pass http://127.0.0.1:7548;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_read_timeout 300s;
    }
}
```

---

## 4. Update Frontend Environment

Setelah Nginx aktif, update `frontend/.env`:

```env
# Jika menggunakan path-based routing via Nginx
VITE_API_URL=/api

# Atau jika menggunakan subdomain terpisah
VITE_API_URL=https://acs.yourdomain.com/api
```

Rebuild:
```bash
cd frontend
npm run build
pm2 restart miniacs-frontend  # atau serve ulang
```

---

## 5. Konfigurasi CWMP (Port 7547)

Port 7547 TIDAK di-proxy untuk menghindari masalah session tracking. Pastikan port ini terbuka:

```bash
# UFW
sudo ufw allow 7547/tcp
```

Modem/CPE akan langsung konek ke:
```
ACS URL: http://<SERVER_IP>:7547/
```

---

## 6. Optimasi Production

Tambahkan di `/etc/nginx/nginx.conf`:

```nginx
http {
    # Gzip compression
    gzip on;
    gzip_vary on;
    gzip_min_length 1024;
    gzip_types text/plain text/css text/javascript application/javascript application/json;

    # Connection settings
    keepalive_timeout 65;
    client_max_body_size 50M;  # Untuk upload firmware
    
    # Buffers
    proxy_buffer_size 128k;
    proxy_buffers 4 256k;
    proxy_busy_buffers_size 256k;
}
```

---

## 7. Firewall Configuration

```bash
# Allow HTTP/HTTPS
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp

# Allow CWMP untuk modem
sudo ufw allow 7547/tcp

# Block direct access ke internal ports (opsional)
sudo ufw deny 5173/tcp
sudo ufw deny 7548/tcp
```

---

## Troubleshooting

### 502 Bad Gateway
```bash
# Cek apakah backend running
pm2 status
curl http://127.0.0.1:7548/health
curl http://127.0.0.1:5173
```

### Permission denied
```bash
# Cek SELinux (jika aktif)
sudo setsebool -P httpd_can_network_connect 1
```

### Test konfigurasi
```bash
sudo nginx -t
sudo nginx -T | grep -A 20 "server_name acs"
```

### Check logs
```bash
sudo tail -f /var/log/nginx/miniacs_error.log
sudo tail -f /var/log/nginx/miniacs_access.log
```

---

## Quick Test

```bash
# Test frontend via Nginx
curl -I http://acs.yourdomain.com/

# Test API via Nginx
curl http://acs.yourdomain.com/api/health

# Test CWMP direct
curl -X POST http://<SERVER_IP>:7547/ -d ""
```

---
