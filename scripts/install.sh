#!/usr/bin/env bash
set -euo pipefail

REPO="CyrusSE/agenthop"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
VERSION="${VERSION:-latest}"

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
arch="$(uname -m)"
case "$arch" in
  x86_64) arch="amd64" ;;
  aarch64|arm64) arch="arm64" ;;
  *) echo "unsupported arch: $arch" >&2; exit 1 ;;
esac

if [ "$VERSION" = "latest" ]; then
  VERSION="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | head -1 | cut -d'"' -f4)"
fi

url="https://github.com/${REPO}/releases/download/${VERSION}/agenthop_${VERSION#v}_${os}_${arch}.tar.gz"
tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

echo "Installing agenthop ${VERSION} for ${os}/${arch}..."
curl -fsSL "$url" | tar -xz -C "$tmpdir"
mkdir -p "$INSTALL_DIR"
install -m 755 "$tmpdir/agenthop" "$INSTALL_DIR/agenthop"
echo "Installed to $INSTALL_DIR/agenthop"
echo "Run: agenthop --help"
