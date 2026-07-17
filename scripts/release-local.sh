#!/bin/sh
set -eu

VERSION="${VERSION:-dev}"
OUT="${OUT:-dist}"
GO="${GO:-go}"
mkdir -p "$OUT/downloads"
CGO_ENABLED=0 "$GO" build -trimpath -ldflags="-s -w" -o "$OUT/netprobe-server" ./cmd/netprobe-server
for os in linux windows; do
  for arch in amd64 arm64; do
    ext=""; [ "$os" = windows ] && ext=".exe"
    target="$OUT/downloads/netcheck-${os}-${arch}${ext}"
    CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" "$GO" build -trimpath -ldflags="-s -w -X main.version=$VERSION" -o "$target" ./cmd/netcheck
    if command -v sha256sum >/dev/null 2>&1; then sha256sum "$target" > "$target.sha256"
    else shasum -a 256 "$target" > "$target.sha256"; fi
  done
done
