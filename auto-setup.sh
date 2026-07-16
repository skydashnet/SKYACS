#!/usr/bin/env bash
set -euo pipefail

if [[ ${EUID} -ne 0 ]]; then
  echo "Run this installer with sudo: sudo ./auto-setup.sh"
  exit 1
fi

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SERVICE_USER="${MINIACS_SERVICE_USER:-${SUDO_USER:-}}"
GO_VERSION="${GO_VERSION:-1.25.0}"

if [[ -z "$SERVICE_USER" || "$SERVICE_USER" == "root" ]] || ! id "$SERVICE_USER" >/dev/null 2>&1; then
  echo "A valid non-root service user is required. Run with sudo from that user or set MINIACS_SERVICE_USER." >&2
  exit 1
fi

echo "miniACS production installer"
echo "Project: ${ROOT_DIR}"

apt-get update -qq
apt-get install -y -qq ca-certificates curl jq openssl postgresql postgresql-contrib
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
  go_archive="go${GO_VERSION}.linux-${go_arch}.tar.gz"
  go_url="https://go.dev/dl/${go_archive}"
  curl -fsSL "$go_url" -o /tmp/miniacs-go.tar.gz
  expected_checksum="$(curl -fsSL 'https://go.dev/dl/?mode=json&include=all' | jq -r --arg file "$go_archive" '.[] | .files[] | select(.filename == $file) | .sha256' | head -n1)"
  if [[ ! "$expected_checksum" =~ ^[0-9a-f]{64}$ ]]; then
    echo "Unable to retrieve the official Go archive checksum." >&2
    exit 1
  fi
  printf '%s  %s\n' "$expected_checksum" /tmp/miniacs-go.tar.gz | sha256sum --check --status
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
export MINIACS_SERVICE_USER="$SERVICE_USER"
./setup.sh

cd "$ROOT_DIR/backend"
go build -trimpath -ldflags="-s -w" -o miniacs ./cmd/server

cd "$ROOT_DIR/frontend"
npm ci --no-audit --no-fund
npm run build
npm install --global serve@14.2.5 --no-audit --no-fund
SERVE_BIN="$(command -v serve)"

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
WorkingDirectory="$ROOT_DIR/backend"
EnvironmentFile="$ROOT_DIR/backend/.env"
ExecStart="$ROOT_DIR/backend/miniacs"
Restart=on-failure
RestartSec=5
NoNewPrivileges=true
CapabilityBoundingSet=
AmbientCapabilities=
PrivateTmp=true
ProtectSystem=full
ProtectHome=read-only
PrivateDevices=true
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true
RestrictSUIDSGID=true
LockPersonality=true
RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6
UMask=0027
ReadOnlyPaths="$ROOT_DIR"
ReadWritePaths="$ROOT_DIR/backend/uploads"

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
WorkingDirectory="$ROOT_DIR/frontend"
ExecStart="$SERVE_BIN" -s dist -l tcp://127.0.0.1:5173
Restart=on-failure
RestartSec=5
NoNewPrivileges=true
CapabilityBoundingSet=
AmbientCapabilities=
PrivateTmp=true
ProtectSystem=full
ProtectHome=read-only
PrivateDevices=true
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true
RestrictSUIDSGID=true
LockPersonality=true
RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6
UMask=0027
ReadOnlyPaths="$ROOT_DIR"

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable --now miniacs miniacs-web

ready=false
for _ in {1..30}; do
  if curl -fsS http://127.0.0.1:7548/health >/dev/null; then
    sed -i '/^INITIAL_ADMIN_PASSWORD=/d' "$ROOT_DIR/backend/.env"
    ready=true
    break
  fi
  sleep 1
done
if [[ "$ready" != true ]]; then
  echo "miniACS failed its startup health check. Recent logs:" >&2
  journalctl -u miniacs -n 50 --no-pager >&2 || true
  exit 1
fi

echo
echo "Installation complete"
echo "Web UI: http://localhost:5173"
echo "CWMP:   http://localhost:7547/"
echo "API:    http://localhost:7548/health"
echo "Run: journalctl -u miniacs -n 50 to retrieve the one-time bootstrap password."
