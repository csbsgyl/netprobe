#!/bin/sh
set -eu

REPO_SLUG="${NETPROBE_REPO:-csbsgyl/netprobe}"
REPO_BRANCH="${NETPROBE_BRANCH:-main}"
INSTALL_DIR="${NETPROBE_INSTALL_DIR:-/opt/netprobe}"

info() { printf '\033[1;32m[NetProbe]\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m[NetProbe]\033[0m %s\n' "$*" >&2; }
fail() { printf '\033[1;31m[NetProbe]\033[0m %s\n' "$*" >&2; exit 1; }

[ "$(id -u)" -eq 0 ] || fail "Please run this installer as root."
command -v curl >/dev/null 2>&1 || fail "curl is required."

DOMAIN="${DOMAIN:-}"
if [ -z "$DOMAIN" ] && [ -r /dev/tty ]; then
  printf "Domain for NetProbe (example: check.example.com): " > /dev/tty
  read -r DOMAIN < /dev/tty
fi
DOMAIN=$(printf '%s' "$DOMAIN" | tr '[:upper:]' '[:lower:]' | sed 's#^https\?://##; s#/$##')
case "$DOMAIN" in
  ""|*/*|*:*|.*|*.) fail "Please enter a plain domain without protocol, path, or port." ;;
esac
printf '%s' "$DOMAIN" | grep -Eq '^([a-z0-9]([a-z0-9-]*[a-z0-9])?\.)+[a-z]{2,63}$' || fail "Domain format is invalid."

public_ips() {
  { curl -4fsS --max-time 8 https://api.ipify.org 2>/dev/null || true; printf '\n';
    curl -6fsS --max-time 8 https://api64.ipify.org 2>/dev/null || true; printf '\n'; } |
    awk 'NF && !seen[$0]++'
}

resolved_ips() {
  if command -v getent >/dev/null 2>&1; then
    getent ahosts "$DOMAIN" 2>/dev/null | awk '{print $1}' | awk '!seen[$0]++'
  elif command -v dig >/dev/null 2>&1; then
    { dig +short A "$DOMAIN"; dig +short AAAA "$DOMAIN"; } | awk 'NF && !seen[$0]++'
  else
    return 1
  fi
}

SERVER_IPS=$(public_ips)
[ -n "$SERVER_IPS" ] || fail "Could not determine this server's public IP."
DNS_IPS=$(resolved_ips || true)
[ -n "$DNS_IPS" ] || fail "$DOMAIN does not currently resolve. Create an A or AAAA record pointing to this server, wait for DNS propagation, then rerun."

MATCH=false
for server_ip in $SERVER_IPS; do
  for dns_ip in $DNS_IPS; do
    [ "$server_ip" = "$dns_ip" ] && MATCH=true
  done
done
[ "$MATCH" = true ] || fail "DNS mismatch. $DOMAIN resolves to: $(printf '%s' "$DNS_IPS" | tr '\n' ' '). This server reports: $(printf '%s' "$SERVER_IPS" | tr '\n' ' ')."
info "DNS points to this server."

install_docker() {
  if command -v docker >/dev/null 2>&1 && docker compose version >/dev/null 2>&1; then return; fi
  info "Installing Docker Engine from Docker's official convenience installer..."
  curl -fsSL https://get.docker.com -o /tmp/netprobe-get-docker.sh
  sh /tmp/netprobe-get-docker.sh
  rm -f /tmp/netprobe-get-docker.sh
  command -v docker >/dev/null 2>&1 || fail "Docker installation failed."
}
install_docker

if command -v ufw >/dev/null 2>&1 && ufw status | grep -q '^Status: active'; then
  info "Opening required UFW ports."
  ufw allow 80/tcp >/dev/null
  ufw allow 443/tcp >/dev/null
  ufw allow 3478/udp >/dev/null
  ufw allow 3479/udp >/dev/null
fi

mkdir -p "$INSTALL_DIR"
ARCHIVE="/tmp/netprobe-source.tar.gz"
info "Downloading NetProbe source."
curl -fsSL "https://codeload.github.com/$REPO_SLUG/tar.gz/refs/heads/$REPO_BRANCH" -o "$ARCHIVE"
tar -xzf "$ARCHIVE" -C "$INSTALL_DIR" --strip-components=1
rm -f "$ARCHIVE"

SECRET=$(dd if=/dev/urandom bs=32 count=1 2>/dev/null | od -An -tx1 | tr -d ' \n')
umask 077
cat > "$INSTALL_DIR/.env" <<EOF
DOMAIN=$DOMAIN
NETPROBE_SECRET=$SECRET
NETPROBE_IMAGE=netprobe:local
UDP_PORT_PRIMARY=3478
UDP_PORT_ALTERNATE=3479
EOF

cd "$INSTALL_DIR"
docker compose -f deploy/compose.yaml pull caddy || warn "Could not pre-pull Caddy; Docker Compose will retry during startup."
docker compose -f deploy/compose.yaml up -d --build

info "Waiting for HTTPS certificate and health check..."
attempt=0
until curl -fsS --max-time 8 "https://$DOMAIN/healthz" >/dev/null 2>&1; do
  attempt=$((attempt + 1))
  if [ "$attempt" -ge 30 ]; then
    docker compose -f deploy/compose.yaml logs --tail=80 caddy netprobe >&2 || true
    fail "HTTPS did not become healthy within 5 minutes. Check that ports 80/443 are allowed by your cloud firewall."
  fi
  sleep 10
done

info "Deployment complete: https://$DOMAIN"
info "Linux deep test: curl -fsSL https://$DOMAIN/install.sh | sh"
info "Windows deep test: irm https://$DOMAIN/install.ps1 | iex"
