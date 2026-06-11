#!/usr/bin/env sh
set -e

REPO="trackfw/trackfw"
BIN="trackfw"
INSTALL_DIR="/usr/local/bin"

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  x86_64) ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH" && exit 1 ;;
esac

LATEST=$(curl -sSfL "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')
URL="https://github.com/$REPO/releases/download/$LATEST/${BIN}_${OS}_${ARCH}"

echo "Installing trackfw $LATEST ($OS/$ARCH)..."
curl -sSfL "$URL" -o "/tmp/$BIN"
chmod +x "/tmp/$BIN"
mv "/tmp/$BIN" "$INSTALL_DIR/$BIN"

echo "✓ trackfw installed at $INSTALL_DIR/$BIN"
trackfw --version
