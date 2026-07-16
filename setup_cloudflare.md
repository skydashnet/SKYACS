# Cloudflare deployment notes

Cloudflare Tunnel cocok untuk web console. Karena CWMP adalah HTTP/SOAP, hostname CWMP juga dapat diarahkan ke listener lokal melalui Tunnel selama CPE mendukung TLS modern. Jangan pasang Cloudflare Access pada hostname CPE. Untuk perangkat lama atau fleet tertutup, direct TLS reverse proxy/VPN biasanya lebih mudah diprediksi.

## Recommended topology

```text
Browser -> Cloudflare Access -> acs.example.com -> Nginx/frontend + API
CPE     -> cwmp.example.com/direct VPN -> CWMP listener
CPE     -> public HTTPS URL   -> signed firmware download
```

Install `cloudflared`, buat tunnel, lalu gunakan ingress ke Nginx lokal:

```yaml
tunnel: <TUNNEL_ID>
credentials-file: /home/<user>/.cloudflared/<TUNNEL_ID>.json

ingress:
  - hostname: acs.example.com
    service: http://127.0.0.1:8080
  - hostname: cwmp.example.com
    service: http://127.0.0.1:7547
  - service: http_status:404
```

```bash
cloudflared tunnel route dns miniacs acs.example.com
cloudflared tunnel route dns miniacs cwmp.example.com
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
- `CWMP_TRUSTED_PROXY_CIDRS=127.0.0.1/32` is set when cloudflared runs locally; CWMP has no Access policy.
- Direct CWMP paths are restricted using firewall rules and `CWMP_ALLOWED_CIDRS`.
- Origin ports `5173` and `7548` are not generally reachable from the internet.
- Database and upload directories are backed up and not web-readable.
