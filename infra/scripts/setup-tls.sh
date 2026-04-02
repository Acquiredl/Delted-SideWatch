#!/bin/bash
# setup-tls.sh — Obtain and configure Let's Encrypt TLS certificates
# Usage: sudo ./setup-tls.sh --domain sidewatch.org [--email admin@sidewatch.org]
#
# Two modes:
#   --init       Generate a self-signed bootstrap cert so nginx can start before
#                a real cert is obtained. Run this FIRST on a fresh deploy.
#   (default)    Obtain a real Let's Encrypt certificate via certbot standalone.
#
# Requires: openssl (for --init), certbot (for real certs), Docker.
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
INIT_ONLY=false

while [[ $# -gt 0 ]]; do
  case "$1" in
    --domain)  DOMAIN="$2";  shift 2 ;;
    --email)   EMAIL="$2";   shift 2 ;;
    --staging) STAGING=true;  shift ;;
    --init)    INIT_ONLY=true; shift ;;
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
  [[ -n "$DOMAIN" ]] || error "Domain required: --domain sidewatch.org"

  if [[ -z "$EMAIL" ]]; then
    EMAIL="admin@$DOMAIN"
    warn "No email provided, using: $EMAIL"
  fi
}

# ---------------------------------------------------------------------------
# 1. Generate self-signed bootstrap cert (--init mode)
# ---------------------------------------------------------------------------
generate_bootstrap_cert() {
  mkdir -p "$CERTS_DIR"

  if [[ -f "$CERTS_DIR/fullchain.pem" && -f "$CERTS_DIR/privkey.pem" ]]; then
    info "Certificates already exist in $CERTS_DIR — skipping bootstrap"
    info "To force regeneration, remove existing certs first"
    return
  fi

  info "Generating self-signed bootstrap certificate for $DOMAIN..."
  openssl req -x509 -nodes -newkey rsa:2048 -days 1 \
    -keyout "$CERTS_DIR/privkey.pem" \
    -out "$CERTS_DIR/fullchain.pem" \
    -subj "/CN=$DOMAIN" 2>/dev/null

  info "Bootstrap cert created — nginx can now start on port 443"
  info "Run this script again WITHOUT --init to obtain a real Let's Encrypt cert"
}

# ---------------------------------------------------------------------------
# 2. Install certbot
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
# 3. Obtain certificate via standalone (stops nginx temporarily)
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
    -d "www.$DOMAIN"
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
    cd "$INSTALL_DIR" && docker compose stop nginx 2>/dev/null || \
      docker stop "$(docker ps -qf name=nginx)" 2>/dev/null || true
    nginx_was_running=true
  fi

  # Obtain the certificate
  certbot "${certbot_args[@]}"

  info "Certificate obtained successfully"

  # Copy certs to project directory (restart nginx after)
  copy_certificates

  # Restart nginx if it was running
  if [[ "$nginx_was_running" == true ]]; then
    info "Restarting nginx container..."
    cd "$INSTALL_DIR" && docker compose start nginx 2>/dev/null || true
  fi
}

# ---------------------------------------------------------------------------
# 4. Copy certs to project directory (NOT symlinks — Docker can't follow them)
# ---------------------------------------------------------------------------
copy_certificates() {
  local le_dir="/etc/letsencrypt/live/$DOMAIN"

  if [[ ! -d "$le_dir" ]]; then
    error "Certificate directory not found: $le_dir"
  fi

  info "Copying certificates to $CERTS_DIR..."

  # Copy actual files (resolve symlinks) — Docker bind mounts need real files
  cp -L "$le_dir/fullchain.pem" "$CERTS_DIR/fullchain.pem"
  cp -L "$le_dir/privkey.pem"   "$CERTS_DIR/privkey.pem"

  # Restrict permissions
  chmod 644 "$CERTS_DIR/fullchain.pem"
  chmod 600 "$CERTS_DIR/privkey.pem"

  info "Certificates copied:"
  info "  $CERTS_DIR/fullchain.pem"
  info "  $CERTS_DIR/privkey.pem"
}

# ---------------------------------------------------------------------------
# 5. Configure auto-renewal with cert copy + nginx reload
# ---------------------------------------------------------------------------
configure_renewal() {
  info "Configuring auto-renewal..."

  # Create renewal hook that copies new certs and reloads nginx
  local hook_dir="/etc/letsencrypt/renewal-hooks/deploy"
  mkdir -p "$hook_dir"

  cat > "$hook_dir/copy-and-reload.sh" <<HOOK
#!/bin/bash
# Post-renewal: copy new certs to project dir and reload nginx
CERTS_DIR="$CERTS_DIR"
LE_DIR="/etc/letsencrypt/live/$DOMAIN"

cp -L "\$LE_DIR/fullchain.pem" "\$CERTS_DIR/fullchain.pem"
cp -L "\$LE_DIR/privkey.pem"   "\$CERTS_DIR/privkey.pem"
chmod 644 "\$CERTS_DIR/fullchain.pem"
chmod 600 "\$CERTS_DIR/privkey.pem"

# Reload nginx inside the container
docker exec \$(docker ps -qf name=nginx) nginx -s reload 2>/dev/null || \
  cd "$INSTALL_DIR" && docker compose restart nginx 2>/dev/null || true
HOOK
  chmod +x "$hook_dir/copy-and-reload.sh"

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
# Main
# ---------------------------------------------------------------------------
main() {
  require_root
  validate_inputs

  echo "============================================"
  echo "  SideWatch — TLS Setup"
  echo "============================================"
  echo ""
  echo "  Domain:  $DOMAIN"
  echo "  Email:   $EMAIL"
  echo "  Mode:    $(if [[ "$INIT_ONLY" == true ]]; then echo "bootstrap (self-signed)"; else echo "Let's Encrypt"; fi)"
  echo "  Staging: $STAGING"
  echo ""

  if [[ "$INIT_ONLY" == true ]]; then
    generate_bootstrap_cert
    echo ""
    echo "  Bootstrap complete. Start nginx, then re-run without --init."
    exit 0
  fi

  install_certbot
  obtain_certificate
  configure_renewal

  echo ""
  echo "============================================"
  echo "  TLS setup complete!"
  echo "============================================"
  echo ""
  echo "  Certificate: /etc/letsencrypt/live/$DOMAIN/"
  echo "  Copied to:   $CERTS_DIR/"
  echo "  Auto-renew:  active (certbot timer + copy hook)"
  echo ""
  echo "  Nginx is serving HTTPS for $DOMAIN."
  echo "  Ensure DNS A record points to this server."
  echo ""
}

main "$@"
