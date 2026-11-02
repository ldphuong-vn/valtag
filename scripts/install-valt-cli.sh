#!/usr/bin/env bash
set -euo pipefail

REPO="valtag-dev/valtag"
BINARY="valtag"
INSTALL_DIR="/usr/local/bin"

# Detect OS and architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  x86_64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

# Fetch latest release tag
LATEST=$(curl -sSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | cut -d'"' -f4)
if [ -z "$LATEST" ]; then
  echo "Failed to fetch latest release"
  exit 1
fi

ARCHIVE="${BINARY}_${OS}_${ARCH}"
URL="https://github.com/${REPO}/releases/download/${LATEST}/${ARCHIVE}.tar.gz"

echo "Installing valtag ${LATEST} for ${OS}/${ARCH}..."
TMP=$(mktemp -d)
trap "rm -rf $TMP" EXIT

curl -sSL "$URL" -o "${TMP}/${ARCHIVE}.tar.gz"
tar -xzf "${TMP}/${ARCHIVE}.tar.gz" -C "$TMP"
chmod +x "${TMP}/${BINARY}"
mv "${TMP}/${BINARY}" "${INSTALL_DIR}/${BINARY}"

echo "✓ valtag installed to ${INSTALL_DIR}/${BINARY}"
echo "  Run: valtag setup"
