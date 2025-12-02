#!/bin/bash
set -euo pipefail

# GenieOS Cosmos Deployment Script
# Usage: ./deploy.sh <tag>
# Example: ./deploy.sh v0.19.0

TAG="${1:-}"
if [[ -z "$TAG" ]]; then
    echo "Usage: $0 <tag>"
    echo "Example: $0 v0.19.0"
    exit 1
fi

VERSION="${TAG#v}"  # Strip 'v' prefix
INSTALL_DIR="/opt/genieos1-cosmos"
GITHUB_REPO="namastexlabs/genieos1-cosmos"
DOWNLOAD_URL="https://github.com/${GITHUB_REPO}/releases/download/${TAG}/cosmos-genieos-${VERSION}-amd64.tar.gz"

echo "=== Deploying genieos1-cosmos ${TAG} ==="
echo "Download URL: ${DOWNLOAD_URL}"

# Create backup of current version
if [[ -f "${INSTALL_DIR}/bin/cosmos" ]]; then
    echo "Backing up current version..."
    cp "${INSTALL_DIR}/bin/cosmos" "${INSTALL_DIR}/bin/cosmos.bak" || true
fi

# Download new version
echo "Downloading ${TAG}..."
curl -fSL "${DOWNLOAD_URL}" -o /tmp/cosmos.tar.gz

# Stop service before update
echo "Stopping service..."
systemctl stop genieos1-cosmos || true

# Extract to install directory
echo "Extracting..."
mkdir -p "${INSTALL_DIR}/bin"
tar -xzf /tmp/cosmos.tar.gz -C "${INSTALL_DIR}/bin/"

# Ensure correct permissions
chmod +x "${INSTALL_DIR}/bin/cosmos" "${INSTALL_DIR}/bin/cosmos-launcher" || true

# Update meta.json with deployment info
cat > "${INSTALL_DIR}/meta.json" << EOF
{
  "version": "${VERSION}",
  "tag": "${TAG}",
  "deployedAt": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "deployedBy": "$(whoami)@$(hostname)"
}
EOF

# Start service
echo "Starting service..."
systemctl start genieos1-cosmos

# Wait and verify
sleep 3
if systemctl is-active --quiet genieos1-cosmos; then
    echo "=== Deployment successful ==="
    systemctl status genieos1-cosmos --no-pager
else
    echo "=== Deployment failed - rolling back ==="
    if [[ -f "${INSTALL_DIR}/bin/cosmos.bak" ]]; then
        mv "${INSTALL_DIR}/bin/cosmos.bak" "${INSTALL_DIR}/bin/cosmos"
        systemctl start genieos1-cosmos
    fi
    exit 1
fi

# Cleanup
rm -f /tmp/cosmos.tar.gz
echo "Done!"
