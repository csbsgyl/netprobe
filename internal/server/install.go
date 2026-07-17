package server

import (
	"fmt"
	"net/http"
	"strings"
)

func (s *Server) installShell(w http.ResponseWriter, request *http.Request) {
	base := publicBaseURL(request)
	w.Header().Set("Content-Type", "text/x-shellscript; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = fmt.Fprintf(w, `#!/bin/sh
set -eu

BASE_URL=%s
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$OS" in linux) ;; *) echo "Unsupported system: $OS" >&2; exit 1 ;; esac
case "$ARCH" in
  x86_64|amd64) ARCH=amd64 ;;
  aarch64|arm64) ARCH=arm64 ;;
  *) echo "Unsupported architecture: $ARCH" >&2; exit 1 ;;
esac

TMP_DIR=$(mktemp -d 2>/dev/null || mktemp -d -t netprobe)
trap 'rm -rf "$TMP_DIR"' EXIT INT TERM
BIN="$TMP_DIR/netcheck"
SUM="$TMP_DIR/netcheck.sha256"
URL="$BASE_URL/downloads/netcheck-linux-$ARCH"

echo "NetProbe: preparing deep network test..."
curl -fsSL "$URL" -o "$BIN"
curl -fsSL "$URL.sha256" -o "$SUM"
EXPECTED=$(awk '{print $1}' "$SUM")
if command -v sha256sum >/dev/null 2>&1; then ACTUAL=$(sha256sum "$BIN" | awk '{print $1}')
elif command -v shasum >/dev/null 2>&1; then ACTUAL=$(shasum -a 256 "$BIN" | awk '{print $1}')
else echo "sha256sum or shasum is required" >&2; exit 1; fi
[ "$EXPECTED" = "$ACTUAL" ] || { echo "Checksum verification failed" >&2; exit 1; }
chmod 700 "$BIN"
exec "$BIN" --server "$BASE_URL"
`, shellQuote(base))
}

func (s *Server) installPowerShell(w http.ResponseWriter, request *http.Request) {
	base := strings.ReplaceAll(publicBaseURL(request), "'", "''")
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = fmt.Fprintf(w, `$ErrorActionPreference = 'Stop'
$BaseUrl = '%s'
$Arch = if ([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture -eq 'Arm64') { 'arm64' } else { 'amd64' }
$TempDir = Join-Path ([IO.Path]::GetTempPath()) ("netprobe-" + [guid]::NewGuid())
New-Item -ItemType Directory -Path $TempDir | Out-Null
try {
  $Exe = Join-Path $TempDir 'netcheck.exe'
  $Url = "$BaseUrl/downloads/netcheck-windows-$Arch.exe"
  Write-Host 'NetProbe: preparing deep network test...'
  Invoke-WebRequest -UseBasicParsing -Uri $Url -OutFile $Exe
  $Expected = ((Invoke-WebRequest -UseBasicParsing -Uri "$Url.sha256").Content -split '\s+')[0].Trim().ToLowerInvariant()
  $Actual = (Get-FileHash -Algorithm SHA256 -Path $Exe).Hash.ToLowerInvariant()
  if ($Actual -ne $Expected) { throw 'Checksum verification failed' }
  & $Exe --server $BaseUrl
  exit $LASTEXITCODE
} finally {
  Remove-Item -Recurse -Force $TempDir -ErrorAction SilentlyContinue
}
`, base)
}

func publicBaseURL(request *http.Request) string {
	scheme := "http"
	if request.TLS != nil || forwardedProto(request) == "https" {
		scheme = "https"
	}
	host := request.Host
	if forwarded := strings.TrimSpace(request.Header.Get("X-Forwarded-Host")); forwarded != "" {
		host = strings.TrimSpace(strings.Split(forwarded, ",")[0])
	}
	return scheme + "://" + host
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
