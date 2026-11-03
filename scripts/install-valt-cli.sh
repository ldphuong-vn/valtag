#!/usr/bin/env bash
set -euo pipefail

REPO="ldphuong-vn/valtag"
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

# Resolve the newest CLI release asset for this platform. We search the
# release list (not /releases/latest) because MCP-server releases carry no
# CLI archive, and "latest" may resolve to one of those.
ARCHIVE="${BINARY}_${OS}_${ARCH}"
ASSET_URL=$(curl -sSL "https://api.github.com/repos/${REPO}/releases?per_page=30" \
  | grep -o "https://[^\"]*/releases/download/[^\"]*/${ARCHIVE}.tar.gz" \
  | head -1)
if [ -z "$ASSET_URL" ]; then
  echo "Failed to find a valtag CLI release for ${OS}/${ARCH}"
  exit 1
fi
LATEST=$(echo "$ASSET_URL" | sed -E 's|.*/releases/download/([^/]+)/.*|\1|')

echo "Installing valtag ${LATEST} for ${OS}/${ARCH}..."
TMP=$(mktemp -d)
trap "rm -rf $TMP" EXIT

curl -sSL "$ASSET_URL" -o "${TMP}/${ARCHIVE}.tar.gz"
tar -xzf "${TMP}/${ARCHIVE}.tar.gz" -C "$TMP"
chmod +x "${TMP}/${BINARY}"
mv "${TMP}/${BINARY}" "${INSTALL_DIR}/${BINARY}"

echo "✓ valtag installed to ${INSTALL_DIR}/${BINARY}"
echo "  Run: valtag setup"
