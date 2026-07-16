#!/usr/bin/env bash
set -euo pipefail

if [[ ${EUID} -ne 0 ]]; then
  echo "Run this installer with sudo: sudo ./auto-setup.sh"
  exit 1
fi

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SERVICE_USER="${SUDO_USER:-root}"
GO_VERSION="${GO_VERSION:-1.25.0}"

echo "miniACS production installer"
echo "Project: ${ROOT_DIR}"

apt-get update -qq
apt-get install -y -qq ca-certificates curl openssl postgresql postgresql-contrib
systemctl enable --now postgresql

install_go=true
if command -v go >/dev/null 2>&1; then
  current_go="$(go env GOVERSION | sed 's/^go//')"
  if [[ "$(printf '%s\n' '1.25.0' "$current_go" | sort -V | head -n1)" == "1.25.0" ]]; then
    install_go=false
  fi
fi
if [[ "$install_go" == true ]]; then
  architecture="$(dpkg --print-architecture)"
  case "$architecture" in
    amd64) go_arch="amd64" ;;
    arm64) go_arch="arm64" ;;
    *) echo "Unsupported architecture: $architecture"; exit 1 ;;
  esac
  curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-${go_arch}.tar.gz" -o /tmp/miniacs-go.tar.gz
  rm -rf /usr/local/go
  tar -C /usr/local -xzf /tmp/miniacs-go.tar.gz
  rm -f /tmp/miniacs-go.tar.gz
fi
export PATH="/usr/local/go/bin:${PATH}"

node_major=0
if command -v node >/dev/null 2>&1; then
  node_major="$(node --version | sed 's/^v//' | cut -d. -f1)"
fi
if (( node_major < 20 )); then
  curl -fsSL https://deb.nodesource.com/setup_20.x | bash -
  apt-get install -y -qq nodejs
fi

cd "$ROOT_DIR"
chmod +x setup.sh
lan_ip="$(hostname -I 2>/dev/null | awk '{print $1}')"
if [[ -n "$lan_ip" ]]; then
  export MINIACS_CORS_ALLOWED_ORIGINS="http://localhost:5173,http://127.0.0.1:5173,http://${lan_ip}:5173"
fi
./setup.sh

cd "$ROOT_DIR/backend"
go build -trimpath -ldflags="-s -w" -o miniacs ./cmd/server

cd "$ROOT_DIR/frontend"
npm ci --no-audit --no-fund
npm run build
npm install --global serve@14 --no-audit --no-fund

mkdir -p "$ROOT_DIR/backend/uploads/firmware"
chown -R "$SERVICE_USER":"$SERVICE_USER" "$ROOT_DIR/backend/uploads"

cat >/etc/systemd/system/miniacs.service <<EOF
[Unit]
Description=miniACS control plane and CWMP server
After=network-online.target postgresql.service
Wants=network-online.target

[Service]
Type=simple
User=$SERVICE_USER
WorkingDirectory=$ROOT_DIR/backend
ExecStart=$ROOT_DIR/backend/miniacs
Restart=on-failure
RestartSec=5
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=full
ReadWritePaths=$ROOT_DIR/backend/uploads

[Install]
WantedBy=multi-user.target
EOF

cat >/etc/systemd/system/miniacs-web.service <<EOF
[Unit]
Description=miniACS web console
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=$SERVICE_USER
WorkingDirectory=$ROOT_DIR/frontend
ExecStart=/usr/bin/serve -s dist -l 5173
Restart=on-failure
RestartSec=5
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=full

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable --now miniacs miniacs-web

echo
echo "Installation complete"
echo "Web UI: http://localhost:5173"
echo "CWMP:   http://localhost:7547/"
echo "API:    http://localhost:7548/health"
echo "Run: journalctl -u miniacs -n 50 to retrieve the one-time bootstrap password."
