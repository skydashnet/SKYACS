# Nginx deployment

Contoh ini menyajikan frontend statis dan meneruskan `/api/` ke API miniACS. CWMP tetap memakai listener `7547/tcp` langsung atau hostname terpisah.

```nginx
server {
    listen 443 ssl http2;
    server_name acs.example.com;

    ssl_certificate /etc/letsencrypt/live/acs.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/acs.example.com/privkey.pem;
    ssl_protocols TLSv1.2 TLSv1.3;

    root /opt/miniacs/frontend/dist;
    index index.html;
    client_max_body_size 64m;

    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
    add_header X-Content-Type-Options nosniff always;
    add_header X-Frame-Options DENY always;
    add_header Referrer-Policy no-referrer always;

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

Konfigurasi firewall minimum:

```bash
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw allow from <CPE_NETWORK> to any port 7547 proto tcp
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
