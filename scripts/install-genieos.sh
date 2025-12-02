#!/bin/bash
# Cosmos Server Installation Script for GenieOS
# Installs Cosmos as a systemd service at /opt/cosmos
set -e

INSTALL_DIR="/opt/cosmos"
COSMOS_USER="cosmos"
VERSION="${1:-latest}"
ARCH="${2:-amd64}"

echo "=== Cosmos Server Installation for GenieOS ==="
echo "Install directory: $INSTALL_DIR"
echo "Version: $VERSION"
echo "Architecture: $ARCH"
echo ""

# Check if running as root
if [ "$EUID" -ne 0 ]; then
    echo "Error: Please run as root (sudo ./install-genieos.sh)"
    exit 1
fi

# Create cosmos user if not exists
if ! id -u "$COSMOS_USER" &>/dev/null; then
    echo "Creating cosmos user..."
    useradd -r -s /bin/false -d "$INSTALL_DIR" "$COSMOS_USER"
fi

# Create installation directories
echo "Creating directories..."
mkdir -p "$INSTALL_DIR"/{bin,static,config,data,data/containers,logs}

# Determine source of binaries
if [ -f "./build/cosmos" ]; then
    echo "Installing from local build..."
    cp ./build/cosmos "$INSTALL_DIR/bin/"
    cp ./build/cosmos-launcher "$INSTALL_DIR/bin/" 2>/dev/null || true
    cp -r ./build/static "$INSTALL_DIR/"
    cp ./build/meta.json "$INSTALL_DIR/" 2>/dev/null || true
    cp ./build/start.sh "$INSTALL_DIR/bin/" 2>/dev/null || true

    # Copy optional assets
    cp ./build/GeoLite2-Country.mmdb "$INSTALL_DIR/" 2>/dev/null || true
    cp ./build/restic "$INSTALL_DIR/bin/" 2>/dev/null || true
    cp ./build/nebula "$INSTALL_DIR/bin/" 2>/dev/null || true
    cp ./build/nebula-cert "$INSTALL_DIR/bin/" 2>/dev/null || true

elif [ "$VERSION" != "latest" ] && [ -n "$VERSION" ]; then
    echo "Downloading from GitHub releases (version: $VERSION)..."
    RELEASE_URL="https://github.com/namastexlabs/Cosmos-Server-1/releases/download/${VERSION}/cosmos-genieos-${VERSION#v}-${ARCH}.zip"

    echo "Downloading from: $RELEASE_URL"
    curl -L "$RELEASE_URL" -o /tmp/cosmos.zip

    echo "Extracting..."
    unzip -o /tmp/cosmos.zip -d "$INSTALL_DIR/"
    rm /tmp/cosmos.zip

    # Move binaries to bin directory
    mv "$INSTALL_DIR/cosmos"* "$INSTALL_DIR/bin/" 2>/dev/null || true

else
    echo "Downloading latest release..."
    RELEASE_URL="https://api.github.com/repos/namastexlabs/Cosmos-Server-1/releases/latest"
    DOWNLOAD_URL=$(curl -s "$RELEASE_URL" | grep "browser_download_url.*${ARCH}.zip" | cut -d '"' -f 4 | head -1)

    if [ -z "$DOWNLOAD_URL" ]; then
        echo "Error: Could not find release for architecture: $ARCH"
        exit 1
    fi

    echo "Downloading from: $DOWNLOAD_URL"
    curl -L "$DOWNLOAD_URL" -o /tmp/cosmos.zip

    echo "Extracting..."
    unzip -o /tmp/cosmos.zip -d "$INSTALL_DIR/"
    rm /tmp/cosmos.zip

    # Move binaries to bin directory
    mv "$INSTALL_DIR/cosmos"* "$INSTALL_DIR/bin/" 2>/dev/null || true
fi

# Set permissions
echo "Setting permissions..."
chown -R "$COSMOS_USER:$COSMOS_USER" "$INSTALL_DIR"
chmod +x "$INSTALL_DIR/bin/"* 2>/dev/null || true

# Install systemd service
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [ -f "$SCRIPT_DIR/cosmos.service" ]; then
    echo "Installing systemd service..."
    cp "$SCRIPT_DIR/cosmos.service" /etc/systemd/system/
else
    echo "Creating systemd service..."
    cat > /etc/systemd/system/cosmos.service << 'EOF'
[Unit]
Description=Cosmos Server - Container Management Platform
Documentation=https://github.com/namastexlabs/Cosmos-Server-1
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=cosmos
Group=cosmos
WorkingDirectory=/opt/cosmos
ExecStart=/opt/cosmos/bin/cosmos
Restart=on-failure
RestartSec=10
StandardOutput=append:/opt/cosmos/logs/cosmos.log
StandardError=append:/opt/cosmos/logs/cosmos-error.log

# Environment
Environment=COSMOS_CONFIG_FOLDER=/opt/cosmos/config
Environment=COSMOS_HTTP_PORT=8080
Environment=COSMOS_HTTPS_MODE=DISABLED

# Security hardening
NoNewPrivileges=true
ProtectSystem=strict
ReadWritePaths=/opt/cosmos/config /opt/cosmos/data /opt/cosmos/logs

[Install]
WantedBy=multi-user.target
EOF
fi

# Reload systemd
echo "Reloading systemd..."
systemctl daemon-reload

# Enable and start service
echo "Enabling and starting Cosmos service..."
systemctl enable cosmos
systemctl start cosmos

# Check status
sleep 2
if systemctl is-active --quiet cosmos; then
    echo ""
    echo "=== Installation Complete ==="
    echo "Cosmos is running at: http://localhost:8080"
    echo ""
    echo "Service commands:"
    echo "  sudo systemctl status cosmos   # Check status"
    echo "  sudo systemctl restart cosmos  # Restart"
    echo "  sudo journalctl -u cosmos -f   # View logs"
    echo ""
    echo "Configuration: $INSTALL_DIR/config/"
    echo "Logs: $INSTALL_DIR/logs/"
else
    echo ""
    echo "Warning: Service may not have started correctly"
    echo "Check logs with: sudo journalctl -u cosmos -e"
fi
