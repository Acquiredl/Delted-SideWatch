#!/bin/bash
# generate-secrets.sh — Generate random secrets for P2Pool Dashboard
# Usage: ./generate-secrets.sh [--dir /path/to/secrets]
#
# Creates Docker-secrets-compatible files. Safe to re-run — will NOT
# overwrite existing secrets unless --force is passed.
set -euo pipefail

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------
SECRETS_DIR="${SECRETS_DIR:-./secrets}"
FORCE=false

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dir)   SECRETS_DIR="$2"; shift 2 ;;
    --force) FORCE=true;       shift ;;
    *) echo "Unknown option: $1"; exit 1 ;;
  esac
done

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------
info()  { echo "[INFO]  $*"; }
warn()  { echo "[WARN]  $*"; }

generate_secret() {
  openssl rand -hex 24
}

write_secret() {
  local name="$1"
  local file="$SECRETS_DIR/$name"

  if [[ -f "$file" && "$FORCE" != true ]]; then
    warn "Secret '$name' already exists — skipping (use --force to overwrite)"
    return
  fi

  generate_secret > "$file"
  chmod 600 "$file"
  info "Generated: $file"
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
main() {
  echo "============================================"
  echo "  P2Pool Dashboard — Secret Generation"
  echo "============================================"
  echo ""

  mkdir -p "$SECRETS_DIR"
  chmod 700 "$SECRETS_DIR"

  # These match the secrets defined in docker-compose.yml
  write_secret "postgres_password"
  write_secret "jwt_secret"
  write_secret "grafana_admin_password"
  write_secret "admin_token"

  echo ""
  echo "Secrets directory: $SECRETS_DIR"
  echo ""
  echo "Files created:"
  ls -la "$SECRETS_DIR"/ 2>/dev/null || true
  echo ""
  echo "These files are referenced by docker-compose.yml:"
  echo "  secrets:"
  echo "    postgres_password:"
  echo "      file: $SECRETS_DIR/postgres_password"
  echo "    jwt_secret:"
  echo "      file: $SECRETS_DIR/jwt_secret"
  echo "    grafana_admin_password:"
  echo "      file: $SECRETS_DIR/grafana_admin_password"
  echo "    admin_token:"
  echo "      file: $SECRETS_DIR/admin_token"
  echo ""
  echo "IMPORTANT: Set ADMIN_TOKEN in .env to the same value as secrets/admin_token:"
  echo "  ADMIN_TOKEN=\$(cat $SECRETS_DIR/admin_token)"
  echo ""
}

main "$@"
