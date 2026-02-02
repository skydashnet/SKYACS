#!/bin/bash
set -e


echo "============================================"
echo "       miniACS Auto Setup v1.0.0-beta"
echo "============================================"
echo ""

if [ "$EUID" -ne 0 ]; then
  echo "Error: Jalankan script ini dengan sudo"
  echo "Usage: sudo ./auto-setup.sh"
  exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MINIACS_USER="${SUDO_USER:-$USER}"
MINIACS_HOME="$SCRIPT_DIR"

echo "[1/7] Updating system packages..."
apt-get update -qq

echo "[2/7] Installing Go..."
if ! command -v go &> /dev/null; then
  GO_VERSION="1.21.6"
  wget -q "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" -O /tmp/go.tar.gz
  rm -rf /usr/local/go
  tar -C /usr/local -xzf /tmp/go.tar.gz
  rm /tmp/go.tar.gz
  
  echo 'export PATH=$PATH:/usr/local/go/bin' > /etc/profile.d/go.sh
  export PATH=$PATH:/usr/local/go/bin
  echo "  Go ${GO_VERSION} installed"
else
  echo "  Go already installed: $(go version)"
fi

echo "[3/7] Installing PostgreSQL..."
if ! command -v psql &> /dev/null; then
  apt-get install -y -qq postgresql postgresql-contrib
  systemctl start postgresql
  systemctl enable postgresql
  echo "  PostgreSQL installed and started"
else
  echo "  PostgreSQL already installed"
  systemctl start postgresql || true
fi

echo "[4/7] Installing Node.js..."
if ! command -v node &> /dev/null; then
  curl -fsSL https://deb.nodesource.com/setup_20.x | bash - > /dev/null 2>&1
  apt-get install -y -qq nodejs
  echo "  Node.js $(node -v) installed"
else
  echo "  Node.js already installed: $(node -v)"
fi

echo "[5/7] Installing PM2..."
if ! command -v pm2 &> /dev/null; then
  npm install -g pm2 --silent
  echo "  PM2 installed"
else
  echo "  PM2 already installed"
fi

echo "[6/7] Setting up database..."
if [ -f "$MINIACS_HOME/setup.sh" ]; then
  chmod +x "$MINIACS_HOME/setup.sh"
  cd "$MINIACS_HOME"
  sudo -u postgres bash -c "cd $MINIACS_HOME && ./setup.sh" || {
    ./setup.sh
  }
  echo "  Database setup complete"
else
  echo "  Warning: setup.sh not found, skipping database setup"
fi

echo "[7/7] Building application..."

cd "$MINIACS_HOME/backend"
export PATH=$PATH:/usr/local/go/bin
go build -o miniacs cmd/server/main.go
echo "  Backend built"

cd "$MINIACS_HOME/frontend"
npm install --silent
npm run build --silent
echo "  Frontend built"

echo ""
echo "Configuring PM2..."
cat > "$MINIACS_HOME/ecosystem.config.js" << 'EOF'
module.exports = {
  apps: [
    {
      name: 'miniacs',
      cwd: './backend',
      script: './miniacs',
      instances: 1,
      autorestart: true,
      watch: false,
      max_memory_restart: '500M',
      env: {
        NODE_ENV: 'production'
      }
    },
    {
      name: 'miniacs-frontend',
      cwd: './frontend',
      script: 'npx',
      args: 'serve -s dist -l 5173',
      instances: 1,
      autorestart: true,
      watch: false,
      env: {
        NODE_ENV: 'production'
      }
    }
  ]
};
EOF

npm install -g serve --silent

cd "$MINIACS_HOME"
pm2 delete all 2>/dev/null || true
pm2 start ecosystem.config.js
pm2 save

pm2 startup systemd -u "$MINIACS_USER" --hp "/home/$MINIACS_USER" > /dev/null 2>&1 || pm2 startup

cat > /etc/systemd/system/miniacs.service << EOF
[Unit]
Description=miniACS TR-069 Server
After=network.target postgresql.service

[Service]
Type=simple
User=$MINIACS_USER
WorkingDirectory=$MINIACS_HOME/backend
ExecStart=$MINIACS_HOME/backend/miniacs
Restart=always
RestartSec=5
Environment=PATH=/usr/local/go/bin:/usr/bin:/bin

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable miniacs

echo ""
echo "============================================"
echo "       Setup Complete!"
echo "============================================"
echo ""
echo "Services running:"
pm2 status
echo ""
echo "Access points:"
echo "  - Web UI:  http://localhost:5173"
echo "  - ACS:     http://localhost:7547/"
echo "  - API:     http://localhost:7547/api"
echo ""
echo "Default login: admin / admin"
echo ""
echo "Useful commands:"
echo "  pm2 status          - Check service status"
echo "  pm2 logs miniacs    - View backend logs"
echo "  pm2 restart all     - Restart all services"
echo ""
echo "Selesai! Server akan auto-start setelah reboot."
