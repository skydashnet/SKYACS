#!/usr/bin/env bash
set -euo pipefail

if [[ ${EUID} -ne 0 ]]; then
  echo "Run this installer with sudo: sudo ./auto-setup.sh"
  exit 1
fi

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SERVICE_USER="${SKYACS_SERVICE_USER:-skyacs}"
GO_VERSION="${GO_VERSION:-1.25.0}"
NODE_VERSION="${NODE_VERSION:-24.18.0}"
MIN_POSTGRES_MAJOR=14
MAX_TESTED_POSTGRES_MAJOR=18
RUNTIME_TMP="$(mktemp -d -t skyacs-runtime.XXXXXXXX)"
trap 'rm -rf "$RUNTIME_TMP"' EXIT

if [[ ! -r /etc/os-release ]]; then
  echo "Cannot determine the operating system from /etc/os-release." >&2
  exit 1
fi
DISTRO_ID="$(. /etc/os-release; printf '%s' "${ID:-}")"
DISTRO_VERSION="$(. /etc/os-release; printf '%s' "${VERSION_ID:-}")"
case "$DISTRO_ID:$DISTRO_VERSION" in
  ubuntu:22.04|ubuntu:24.04|ubuntu:26.04|debian:12|debian:13) ;;
  *)
    echo "Unsupported platform: $DISTRO_ID $DISTRO_VERSION. Use Ubuntu 22.04/24.04/26.04 LTS or Debian 12/13." >&2
    exit 1
    ;;
esac

if ! id "$SERVICE_USER" >/dev/null 2>&1; then
  if [[ ! "$SERVICE_USER" =~ ^[a-z_][a-z0-9_-]*$ ]]; then
    echo "SKYACS_SERVICE_USER is not a valid Linux username." >&2
    exit 1
  fi
  useradd --system --home-dir "$ROOT_DIR" --shell /usr/sbin/nologin "$SERVICE_USER"
fi
if [[ "$(id -u "$SERVICE_USER")" -eq 0 ]]; then
  echo "SKYACS_SERVICE_USER must not be root." >&2
  exit 1
fi

echo "SKYACS production installer"
echo "Project: ${ROOT_DIR}"

apt-get update -qq
apt-get install -y -qq ca-certificates curl jq nginx openssl postgresql postgresql-contrib xz-utils
systemctl enable --now postgresql

postgres_ready=false
for _ in {1..30}; do
  if runuser -u postgres -- pg_isready -q; then
    postgres_ready=true
    break
  fi
  sleep 1
done
if [[ "$postgres_ready" != true ]]; then
  echo "PostgreSQL did not become ready within 30 seconds." >&2
  exit 1
fi

server_version_num="$(runuser -u postgres -- psql -XtAc 'SHOW server_version_num' | tr -d '[:space:]')"
if [[ ! "$server_version_num" =~ ^[0-9]+$ ]]; then
  echo "Unable to determine the PostgreSQL server version." >&2
  exit 1
fi
postgres_major=$((server_version_num / 10000))
if (( postgres_major < MIN_POSTGRES_MAJOR )); then
  echo "PostgreSQL $postgres_major is unsupported. SKYACS requires PostgreSQL $MIN_POSTGRES_MAJOR or newer." >&2
  exit 1
fi
if (( postgres_major > MAX_TESTED_POSTGRES_MAJOR )); then
  echo "Warning: PostgreSQL $postgres_major is newer than the tested range ($MIN_POSTGRES_MAJOR-$MAX_TESTED_POSTGRES_MAJOR)." >&2
fi
echo "PostgreSQL $postgres_major compatibility check passed."

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
  go_path="$RUNTIME_TMP/$go_archive"
  curl --proto '=https' --tlsv1.2 -fsSL --retry 3 "$go_url" -o "$go_path"
  expected_checksum="$(curl -fsSL 'https://go.dev/dl/?mode=json&include=all' | jq -r --arg file "$go_archive" '.[] | .files[] | select(.filename == $file) | .sha256' | head -n1)"
  if [[ ! "$expected_checksum" =~ ^[0-9a-f]{64}$ ]]; then
    echo "Unable to retrieve the official Go archive checksum." >&2
    exit 1
  fi
  printf '%s  %s\n' "$expected_checksum" "$go_path" | sha256sum --check --status
  rm -rf /usr/local/go
  tar -C /usr/local -xzf "$go_path"
fi
export PATH="/usr/local/go/bin:${PATH}"

node_major=0
if command -v node >/dev/null 2>&1; then
  node_major="$(node --version | sed 's/^v//' | cut -d. -f1)"
fi
if (( node_major < 24 )); then
  architecture="$(dpkg --print-architecture)"
  case "$architecture" in
    amd64) node_arch="x64" ;;
    arm64) node_arch="arm64" ;;
    *) echo "Unsupported architecture: $architecture"; exit 1 ;;
  esac
  node_archive="node-v${NODE_VERSION}-linux-${node_arch}.tar.xz"
  node_base_url="https://nodejs.org/dist/v${NODE_VERSION}"
  curl --proto '=https' --tlsv1.2 -fsSL --retry 3 "$node_base_url/$node_archive" -o "$RUNTIME_TMP/$node_archive"
  curl --proto '=https' --tlsv1.2 -fsSL --retry 3 "$node_base_url/SHASUMS256.txt" -o "$RUNTIME_TMP/SHASUMS256.txt"
  expected_node_checksum="$(awk -v file="$node_archive" '$2 == file { print $1 }' "$RUNTIME_TMP/SHASUMS256.txt")"
  if [[ ! "$expected_node_checksum" =~ ^[0-9a-f]{64}$ ]]; then
    echo "Unable to retrieve the official Node.js archive checksum." >&2
    exit 1
  fi
  printf '%s  %s\n' "$expected_node_checksum" "$RUNTIME_TMP/$node_archive" | sha256sum --check --status
  node_home="/usr/local/lib/nodejs/node-v${NODE_VERSION}-linux-${node_arch}"
  rm -rf "$node_home"
  mkdir -p "$node_home"
  tar -C "$node_home" -xJf "$RUNTIME_TMP/$node_archive" --strip-components=1
  for executable in node npm npx corepack; do
    ln -sfn "$node_home/bin/$executable" "/usr/local/bin/$executable"
  done
fi

cd "$ROOT_DIR"
chmod +x setup.sh
export SKYACS_SERVICE_USER="$SERVICE_USER"
./setup.sh

cd "$ROOT_DIR/backend"
go build -trimpath -ldflags="-s -w" -o skyacs ./cmd/server

cd "$ROOT_DIR/frontend"
npm ci --include=dev --no-audit --no-fund
printf 'VITE_API_URL=/api\n' >.env.production
npm run build

mkdir -p "$ROOT_DIR/backend/uploads/firmware"
chown -R "$SERVICE_USER":"$SERVICE_USER" "$ROOT_DIR/backend/uploads"

cat >/etc/systemd/system/skyacs.service <<EOF
[Unit]
Description=SKYACS control plane and CWMP server
After=network-online.target postgresql.service
Wants=network-online.target

[Service]
Type=simple
User=$SERVICE_USER
WorkingDirectory="$ROOT_DIR/backend"
EnvironmentFile="$ROOT_DIR/backend/.env"
ExecStart="$ROOT_DIR/backend/skyacs"
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

cat >/etc/nginx/conf.d/skyacs.conf <<EOF
server {
    listen 8080 default_server;
    listen [::]:8080 default_server;
    server_name _;
    server_tokens off;

    root "$ROOT_DIR/frontend/dist";
    index index.html;
    client_max_body_size 64m;

    add_header X-Content-Type-Options nosniff always;
    add_header X-Frame-Options DENY always;
    add_header Referrer-Policy no-referrer always;
    add_header Permissions-Policy "camera=(), microphone=(), geolocation=()" always;
    add_header Content-Security-Policy "default-src 'self'; object-src 'none'; frame-ancestors 'none'; base-uri 'self'; img-src 'self' data:; font-src 'self'; connect-src 'self'; script-src 'self'; style-src 'self'" always;

    location = /api {
        return 308 /api/;
    }

    location /api/ {
        proxy_pass http://127.0.0.1:7548/;
        proxy_http_version 1.1;
        proxy_set_header Host \$http_host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
        proxy_read_timeout 60s;
        access_log off;
    }

    location / {
        try_files \$uri \$uri/ /index.html;
    }
}
EOF

# SKYACS releases before v2.0.0 used a standalone Node.js web service.
if systemctl list-unit-files skyacs-web.service --no-legend 2>/dev/null | grep -q '^skyacs-web.service'; then
  systemctl disable --now skyacs-web.service || true
fi
rm -f /etc/systemd/system/skyacs-web.service

systemctl daemon-reload
nginx -t
systemctl enable skyacs nginx
systemctl restart skyacs nginx

ready=false
for _ in {1..30}; do
  if curl -fsS http://127.0.0.1:8080/api/health >/dev/null; then
    sed -i '/^INITIAL_ADMIN_PASSWORD=/d' "$ROOT_DIR/backend/.env"
    ready=true
    break
  fi
  sleep 1
done
if [[ "$ready" != true ]]; then
  echo "SKYACS failed its startup health check. Recent logs:" >&2
  journalctl -u skyacs -n 50 --no-pager >&2 || true
  exit 1
fi

echo
echo "Installation complete"
echo "Web UI: http://<server-address>:8080"
echo "CWMP:   http://localhost:7547/"
echo "API:    http://localhost:8080/api/health"
echo "Run: journalctl -u skyacs -n 50 to retrieve the one-time bootstrap password."
