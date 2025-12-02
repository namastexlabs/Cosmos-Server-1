#!/bin/bash
set -euo pipefail

# GenieOS Cosmos Initial Setup Script
# Run this once to set up the production environment

INSTALL_DIR="/opt/genieos1-cosmos"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "=== Setting up genieos1-cosmos production environment ==="

# Create directory structure
echo "Creating directories..."
mkdir -p "${INSTALL_DIR}"/{bin,static,data,logs,scripts}

# Copy deployment script
echo "Installing deployment script..."
cp "${SCRIPT_DIR}/deploy.sh" "${INSTALL_DIR}/scripts/"
chmod +x "${INSTALL_DIR}/scripts/deploy.sh"

# Install systemd service
echo "Installing systemd service..."
cp "${SCRIPT_DIR}/genieos1-cosmos.service" /etc/systemd/system/
systemctl daemon-reload

# Create initial config
if [[ ! -f "${INSTALL_DIR}/cosmos.env" ]]; then
    echo "Creating default config..."
    cat > "${INSTALL_DIR}/cosmos.env" << 'EOF'
# GenieOS Cosmos Configuration
COSMOS_HTTP_PORT=80
COSMOS_HTTPS_PORT=443
EOF
fi

# Set permissions
echo "Setting permissions..."
chmod -R 755 "${INSTALL_DIR}"

echo "=== Setup complete ==="
echo ""
echo "Next steps:"
echo "1. Deploy initial version:"
echo "   ${INSTALL_DIR}/scripts/deploy.sh v0.19.0-rc.1"
echo ""
echo "2. Enable service:"
echo "   systemctl enable genieos1-cosmos"
echo ""
echo "3. Start service:"
echo "   systemctl start genieos1-cosmos"
echo ""
echo "4. Check status:"
echo "   systemctl status genieos1-cosmos"
