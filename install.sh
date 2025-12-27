#!/bin/sh
# DockBridge installer script
# Usage: curl -fsSL https://raw.githubusercontent.com/dockbridge/dockbridge/main/install.sh | sh

set -e

REPO="dockbridge/dockbridge"
BINARY="dockbridge"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

# Detect OS
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$OS" in
    linux) OS="linux" ;;
    darwin) OS="darwin" ;;
    *) echo "Unsupported OS: $OS"; exit 1 ;;
esac

# Detect architecture
ARCH=$(uname -m)
case "$ARCH" in
    x86_64|amd64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

echo "Detected: $OS/$ARCH"

# Get latest version
VERSION=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')
if [ -z "$VERSION" ]; then
    echo "Failed to get latest version"
    exit 1
fi
echo "Latest version: $VERSION"

# Download binary
DOWNLOAD_URL="https://github.com/$REPO/releases/download/$VERSION/$BINARY-$OS-$ARCH"
TMP_FILE=$(mktemp)

echo "Downloading $DOWNLOAD_URL..."
curl -fsSL "$DOWNLOAD_URL" -o "$TMP_FILE"

# Make executable
chmod +x "$TMP_FILE"

# Install
echo "Installing to $INSTALL_DIR/$BINARY..."
if [ -w "$INSTALL_DIR" ]; then
    mv "$TMP_FILE" "$INSTALL_DIR/$BINARY"
else
    echo "Need sudo to install to $INSTALL_DIR"
    sudo mv "$TMP_FILE" "$INSTALL_DIR/$BINARY"
fi

echo ""
echo "✅ DockBridge $VERSION installed successfully!"
echo ""
echo "Get started:"
echo "  1. Create dockbridge.yaml config file"
echo "  2. Run: dockbridge start"
echo "  3. Export: export DOCKER_HOST=unix:///tmp/dockbridge.sock"
echo ""
