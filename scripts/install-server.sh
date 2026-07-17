#!/bin/sh
set -eu

REPO="${NETPROBE_REPO:-csbsgyl/netprobe}"
VERSION="${NETPROBE_VERSION:-latest}"
case "$VERSION" in *[!A-Za-z0-9._-]*) echo "Invalid NETPROBE_VERSION" >&2; exit 1 ;; esac
case "$(uname -m)" in
  x86_64|amd64) ARCH=amd64 ;;
  aarch64|arm64) ARCH=arm64 ;;
  *) echo "Unsupported architecture: $(uname -m)" >&2; exit 1 ;;
esac
command -v curl >/dev/null 2>&1 || { echo "curl is required" >&2; exit 1; }
command -v sha256sum >/dev/null 2>&1 || { echo "sha256sum is required" >&2; exit 1; }

ASSET="netprobe-deploy-linux-$ARCH"
if [ "$VERSION" = latest ]; then
  BASE_URL="https://github.com/$REPO/releases/latest/download"
else
  BASE_URL="https://github.com/$REPO/releases/download/$VERSION"
fi
TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT INT TERM

curl -fsSL "$BASE_URL/$ASSET" -o "$TMP_DIR/$ASSET"
curl -fsSL "$BASE_URL/$ASSET.sha256" -o "$TMP_DIR/$ASSET.sha256"
(cd "$TMP_DIR" && sha256sum -c "$ASSET.sha256")
chmod 700 "$TMP_DIR/$ASSET"
"$TMP_DIR/$ASSET" "$@"
