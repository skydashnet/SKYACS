# Cloudflare deployment notes

Cloudflare Tunnel cocok untuk web console. CWMP pada `7547/tcp` tidak dapat dipublikasikan melalui HTTP Tunnel biasa dan harus menggunakan jalur publik/VPN yang dapat dicapai CPE.

## Recommended topology

```text
Browser -> Cloudflare Access -> acs.example.com -> Nginx/frontend + API
CPE     -> public/VPN address -> :7547 CWMP
CPE     -> public HTTPS URL   -> signed firmware download
```

Install `cloudflared`, buat tunnel, lalu gunakan ingress ke Nginx lokal:

```yaml
tunnel: <TUNNEL_ID>
credentials-file: /home/<user>/.cloudflared/<TUNNEL_ID>.json

ingress:
  - hostname: acs.example.com
    service: http://127.0.0.1:8080
  - service: http_status:404
```

```bash
cloudflared tunnel route dns miniacs acs.example.com
sudo cloudflared service install
sudo systemctl enable --now cloudflared
```

## Important: firmware delivery

Cloudflare Access normally requires an interactive browser session. A CPE cannot complete that login, so do not place its signed firmware path behind an Access challenge.

Choose one controlled route:

1. Create a separate hostname for `/files/...` without Access, proxy only that path to miniACS, and keep the signed token plus HTTPS protection.
2. Publish the signed firmware endpoint through a tightly scoped direct reverse proxy reachable only from CPE networks.

Set `firmware_base_url` to that reachable base URL. Never expose the full authenticated API merely to make firmware downloads work.

## Security checklist

- Cloudflare Access protects the operator console and authenticated API.
- The public firmware hostname only routes `GET /files/...`.
- Query strings are excluded from proxy access logs because they contain bearer tokens.
- `CORS_ALLOWED_ORIGINS` contains only the console origin.
- CWMP is restricted using firewall rules and `CWMP_ALLOWED_CIDRS`.
- Origin ports `5173` and `7548` are not generally reachable from the internet.
- Database and upload directories are backed up and not web-readable.
