#!/usr/bin/env bash
#
# Installer for "laravel" (laravel-installer). Downloads the binary from the
# latest GitHub Release matching your system and places it in /usr/local/bin.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/cmayorgahilario/laravel-installer/main/install.sh | bash
#
# Optional variables:
#   VERSION=vX.Y.Z   install a specific version (default: latest)
#   BIN_DIR=~/.local/bin   destination directory (default: /usr/local/bin)
set -euo pipefail

REPO="cmayorgahilario/laravel-installer"
BIN_NAME="laravel"
BIN_DIR="${BIN_DIR:-/usr/local/bin}"

err() {
    echo "Error: $*" >&2
    exit 1
}

command -v curl >/dev/null 2>&1 || err "'curl' is required"
command -v tar >/dev/null 2>&1 || err "'tar' is required"

# This tool targets WSL/Linux only.
os="$(uname -s)"
case "$os" in
    Linux) OS="Linux" ;;
    *) err "unsupported operating system: $os (this tool targets WSL/Linux)" ;;
esac

# Detect architecture in the same format GoReleaser produces
# (name_template in .goreleaser.yaml): ARCH as x86_64/arm64.

arch="$(uname -m)"
case "$arch" in
    x86_64 | amd64) ARCH="x86_64" ;;
    aarch64 | arm64) ARCH="arm64" ;;
    *) err "unsupported architecture: $arch" ;;
esac

# Resolve the version to install.
VERSION="${VERSION:-}"
if [ -z "$VERSION" ]; then
    echo "→ Looking up the latest version..."
    VERSION="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" |
        grep '"tag_name":' | head -n1 | sed -E 's/.*"([^"]+)".*/\1/')"
    [ -n "$VERSION" ] || err "could not determine the latest version (are there published releases?)"
fi

ASSET="laravel-installer_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${VERSION}/${ASSET}"

echo "→ Downloading ${ASSET} (${VERSION})..."
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

curl -fsSL "$URL" -o "$tmp/$ASSET" ||
    err "could not download $URL"
tar -xzf "$tmp/$ASSET" -C "$tmp" ||
    err "could not extract $ASSET"

[ -f "$tmp/$BIN_NAME" ] || err "the archive does not contain the '$BIN_NAME' binary"

# Install into BIN_DIR; use sudo/doas if there are no write permissions.
echo "→ Installing to ${BIN_DIR}/${BIN_NAME}..."
if [ -w "$BIN_DIR" ]; then
    install -m 755 "$tmp/$BIN_NAME" "$BIN_DIR/$BIN_NAME"
elif command -v sudo >/dev/null 2>&1; then
    sudo install -m 755 "$tmp/$BIN_NAME" "$BIN_DIR/$BIN_NAME"
elif command -v doas >/dev/null 2>&1; then
    doas install -m 755 "$tmp/$BIN_NAME" "$BIN_DIR/$BIN_NAME"
else
    err "no write permission in $BIN_DIR and no sudo/doas; set BIN_DIR=~/.local/bin"
fi

echo "✓ Installed: $($BIN_DIR/$BIN_NAME --version 2>/dev/null || echo "$BIN_NAME $VERSION")"
echo "  Run 'laravel' to get started."
