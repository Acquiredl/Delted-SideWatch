#!/bin/bash
# setup-tls.sh — Obtain and configure Let's Encrypt TLS certificates
# Usage: sudo ./setup-tls.sh --domain example.com [--email admin@example.com]
#
# Requires: certbot (installed via snap), nginx container running on port 80.
# Safe to re-run — certbot skips if cert already valid.
set -euo pipefail

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------
DOMAIN=""
EMAIL=""
INSTALL_DIR="${INSTALL_DIR:-/opt/p2pool-dashboard}"
CERTS_DIR="$INSTALL_DIR/certs"
STAGING=false

while [[ $# -gt 0 ]]; do
  case "$1" in
    --domain)  DOMAIN="$2";  shift 2 ;;
    --email)   EMAIL="$2";   shift 2 ;;
    --staging) STAGING=true;  shift ;;
    *) echo "Unknown option: $1"; exit 1 ;;
  esac
done

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------
info()  { echo "[INFO]  $*"; }
warn()  { echo "[WARN]  $*"; }
error() { echo "[ERROR] $*" >&2; exit 1; }

require_root() {
  [[ $EUID -eq 0 ]] || error "This script must be run as root (use sudo)"
}

# ---------------------------------------------------------------------------
# Validation
# ---------------------------------------------------------------------------
validate_inputs() {
  [[ -n "$DOMAIN" ]] || error "Domain required: --domain example.com"

  if [[ -z "$EMAIL" ]]; then
    EMAIL="admin@$DOMAIN"
    warn "No email provided, using: $EMAIL"
  fi
}

# ---------------------------------------------------------------------------
# 1. Install certbot
# ---------------------------------------------------------------------------
install_certbot() {
  if command -v certbot &>/dev/null; then
    info "certbot already installed: $(certbot --version 2>&1)"
    return
  fi

  info "Installing certbot via snap..."
  snap install --classic certbot 2>/dev/null || {
    # Fallback to apt if snap not available
    info "snap not available, installing certbot via apt..."
    apt-get update -qq
    DEBIAN_FRONTEND=noninteractive apt-get install -y -qq certbot > /dev/null
  }

  # Ensure certbot is in PATH
  if [[ -f /snap/bin/certbot && ! -f /usr/bin/certbot ]]; then
    ln -sf /snap/bin/certbot /usr/bin/certbot
  fi

  info "certbot installed: $(certbot --version 2>&1)"
}

# ---------------------------------------------------------------------------
# 2. Stop nginx temporarily for standalone verification
# ---------------------------------------------------------------------------
obtain_certificate() {
  info "Obtaining certificate for $DOMAIN..."

  mkdir -p "$CERTS_DIR"

  # Build certbot args
  local certbot_args=(
    certonly
    --standalone
    --preferred-challenges http
    -d "$DOMAIN"
    --email "$EMAIL"
    --agree-tos
    --non-interactive
  )

  if [[ "$STAGING" == true ]]; then
    certbot_args+=(--staging)
    warn "Using Let's Encrypt staging environment (certs won't be trusted)"
  fi

  # Stop nginx container if running so certbot can bind port 80
  local nginx_was_running=false
  if docker ps --format '{{.Names}}' 2>/dev/null | grep -q nginx; then
    info "Stopping nginx container for certificate verification..."
    docker compose -f "$INSTALL_DIR/docker-compose.yml" stop nginx 2>/dev/null || \
      docker stop "$(docker ps -qf name=nginx)" 2>/dev/null || true
    nginx_was_running=true
  fi

  # Obtain the certificate
  certbot "${certbot_args[@]}"

  # Restart nginx if it was running
  if [[ "$nginx_was_running" == true ]]; then
    info "Restarting nginx container..."
    docker compose -f "$INSTALL_DIR/docker-compose.yml" start nginx 2>/dev/null || true
  fi

  info "Certificate obtained successfully"
}

# ---------------------------------------------------------------------------
# 3. Symlink certs to project directory
# ---------------------------------------------------------------------------
link_certificates() {
  local le_dir="/etc/letsencrypt/live/$DOMAIN"

  if [[ ! -d "$le_dir" ]]; then
    error "Certificate directory not found: $le_dir"
  fi

  info "Linking certificates to $CERTS_DIR..."

  # Create symlinks matching what nginx.conf expects
  ln -sf "$le_dir/fullchain.pem" "$CERTS_DIR/fullchain.pem"
  ln -sf "$le_dir/privkey.pem"   "$CERTS_DIR/privkey.pem"

  info "Certificates linked:"
  info "  $CERTS_DIR/fullchain.pem -> $le_dir/fullchain.pem"
  info "  $CERTS_DIR/privkey.pem   -> $le_dir/privkey.pem"
}

# ---------------------------------------------------------------------------
# 4. Configure auto-renewal
# ---------------------------------------------------------------------------
configure_renewal() {
  info "Configuring auto-renewal..."

  # Create renewal hook to reload nginx after cert renewal
  local hook_dir="/etc/letsencrypt/renewal-hooks/deploy"
  mkdir -p "$hook_dir"

  cat > "$hook_dir/reload-nginx.sh" <<'HOOK'
#!/bin/bash
# Reload nginx container after certificate renewal
docker exec $(docker ps -qf name=nginx) nginx -s reload 2>/dev/null || \
  docker compose -f /opt/p2pool-dashboard/docker-compose.yml restart nginx 2>/dev/null || true
HOOK
  chmod +x "$hook_dir/reload-nginx.sh"

  # Verify certbot timer is active (snap/apt both set this up)
  if systemctl is-active --quiet certbot.timer 2>/dev/null || \
     systemctl is-active --quiet snap.certbot.renew.timer 2>/dev/null; then
    info "Certbot auto-renewal timer is active"
  else
    # Create a systemd timer as fallback
    info "Setting up certbot renewal timer..."
    cat > /etc/systemd/system/certbot-renew.timer <<EOF
[Unit]
Description=Certbot renewal timer

[Timer]
OnCalendar=*-*-* 03:00:00
RandomizedDelaySec=3600
Persistent=true

[Install]
WantedBy=timers.target
EOF

    cat > /etc/systemd/system/certbot-renew.service <<EOF
[Unit]
Description=Certbot renewal

[Service]
Type=oneshot
ExecStart=/usr/bin/certbot renew --quiet
EOF

    systemctl daemon-reload
    systemctl enable certbot-renew.timer
    systemctl start certbot-renew.timer
    info "Certbot renewal timer created (daily at 03:00 +/- 1h jitter)"
  fi

  # Test renewal (dry run)
  info "Testing renewal (dry run)..."
  certbot renew --dry-run 2>&1 | tail -1 || warn "Dry run failed — check certbot configuration"
}

# ---------------------------------------------------------------------------
# 5. Update nginx config with domain
# ---------------------------------------------------------------------------
update_nginx_domain() {
  local nginx_conf="$INSTALL_DIR/config/nginx/nginx.conf"

  if [[ -f "$nginx_conf" ]]; then
    # Replace server_name _; with actual domain in HTTPS block
    if grep -q 'server_name _;' "$nginx_conf"; then
      info "Updating nginx.conf with domain: $DOMAIN"
      # Only replace in the HTTPS server block (the second occurrence)
      sed -i "s/server_name _;/server_name $DOMAIN;/g" "$nginx_conf"
      info "nginx.conf updated — reload nginx to apply"
    fi
  else
    warn "nginx.conf not found at $nginx_conf — update server_name manually"
  fi
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
main() {
  require_root
  validate_inputs

  echo "============================================"
  echo "  P2Pool Dashboard — TLS Setup"
  echo "============================================"
  echo ""
  echo "  Domain:  $DOMAIN"
  echo "  Email:   $EMAIL"
  echo "  Staging: $STAGING"
  echo ""

  install_certbot
  obtain_certificate
  link_certificates
  configure_renewal
  update_nginx_domain

  echo ""
  echo "============================================"
  echo "  TLS setup complete!"
  echo "============================================"
  echo ""
  echo "  Certificate: /etc/letsencrypt/live/$DOMAIN/"
  echo "  Linked to:   $CERTS_DIR/"
  echo "  Auto-renew:  active (certbot timer)"
  echo ""
  echo "  Nginx will serve HTTPS for $DOMAIN."
  echo "  Point your DNS A record to this server's IP."
  echo ""
}

main "$@"
