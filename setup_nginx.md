# Nginx deployment

Contoh ini menyajikan frontend statis, meneruskan `/api/` ke API miniACS, dan menempatkan CWMP pada hostname TLS terpisah.

```nginx
server {
    listen 443 ssl http2;
    server_name acs.example.com;

    ssl_certificate /etc/letsencrypt/live/acs.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/acs.example.com/privkey.pem;
    ssl_protocols TLSv1.2 TLSv1.3;
    server_tokens off;

    root /opt/miniacs/frontend/dist;
    index index.html;
    client_max_body_size 64m;

    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
    add_header X-Content-Type-Options nosniff always;
    add_header X-Frame-Options DENY always;
    add_header Referrer-Policy no-referrer always;
    add_header Content-Security-Policy "default-src 'self'; object-src 'none'; frame-ancestors 'none'; base-uri 'self'; img-src 'self' data:; font-src 'self'; connect-src 'self'; script-src 'self'; style-src 'self'" always;

    location /api/ {
        proxy_pass http://127.0.0.1:7548/;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_read_timeout 60s;

        # Signed firmware URLs are credentials; omit query strings from access logs.
        access_log off;
    }

    location / {
        try_files $uri $uri/ /index.html;
    }
}

server {
    listen 443 ssl http2;
    server_name cwmp.example.com;

    ssl_certificate /etc/letsencrypt/live/cwmp.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/cwmp.example.com/privkey.pem;
    ssl_protocols TLSv1.2 TLSv1.3;
    server_tokens off;
    client_max_body_size 16m;

    location / {
        proxy_pass http://127.0.0.1:7547;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-Proto https;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_read_timeout 180s;
        proxy_send_timeout 180s;
    }
}
```

Build frontend untuk prefix API yang sama:

```bash
cd frontend
printf 'VITE_API_URL=/api\n' > .env.production
npm ci
npm run build
```

Di menu **Settings**, gunakan:

```text
Firmware base URL: https://acs.example.com/api
```

Di `backend/.env`, setidaknya sesuaikan:

```env
CORS_ALLOWED_ORIGINS=https://acs.example.com
TRUSTED_PROXY_CIDRS=127.0.0.1/32
CWMP_TRUSTED_PROXY_CIDRS=127.0.0.1/32
CONNECTION_REQUEST_ALLOWED_CIDRS=<CPE_NETWORK>
CWMP_USERNAME=<cpe-to-acs-user>
CWMP_PASSWORD=<random-password>
```

Konfigurasi firewall minimum:

```bash
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw deny 7547/tcp
sudo ufw deny 5173/tcp
sudo ufw deny 7548/tcp
```

Validasi dan reload:

```bash
sudo nginx -t
sudo systemctl reload nginx
curl https://acs.example.com/api/health
sudo systemctl status miniacs miniacs-web
```
