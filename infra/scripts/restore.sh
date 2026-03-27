#!/bin/bash
# restore.sh — Restore P2Pool Dashboard database from backup
# Usage: ./restore.sh [DUMP_FILE]
#        ./restore.sh --from-remote [REMOTE_FILENAME]
#
# Without arguments, restores the most recent local backup.
# With --from-remote, downloads from BACKUP_REMOTE_URL first.
#
# WARNING: This drops and recreates the database. All current data is replaced.
set -euo pipefail

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------
BACKUP_DIR="${BACKUP_DIR:-/opt/p2pool-dashboard/backups}"
REMOTE_URL="${BACKUP_REMOTE_URL:-}"
INSTALL_DIR="${INSTALL_DIR:-/opt/p2pool-dashboard}"
FROM_REMOTE=false
SKIP_CONFIRM=false
TARGET_FILE=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --from-remote) FROM_REMOTE=true; TARGET_FILE="${2:-}"; shift; [[ -n "$TARGET_FILE" ]] && shift ;;
    --dir)         INSTALL_DIR="$2"; shift 2 ;;
    --backup-dir)  BACKUP_DIR="$2"; shift 2 ;;
    --yes|-y)      SKIP_CONFIRM=true; shift ;;
    --help|-h)
      echo "Usage: $0 [DUMP_FILE | --from-remote [FILENAME]]"
      echo ""
      echo "  No args:              restore most recent local backup"
      echo "  DUMP_FILE:            restore specific local file"
      echo "  --from-remote:        download + restore latest remote backup"
      echo "  --from-remote FILE:   download + restore specific remote file"
      echo "  --yes, -y:            skip confirmation prompt"
      exit 0
      ;;
    *)
      TARGET_FILE="$1"; shift ;;
  esac
done

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------
info()  { echo "[$(date +%H:%M:%S)] [INFO]  $*"; }
warn()  { echo "[$(date +%H:%M:%S)] [WARN]  $*"; }
error() { echo "[$(date +%H:%M:%S)] [ERROR] $*" >&2; }
fatal() { error "$*"; exit 1; }

# ---------------------------------------------------------------------------
# 1. Locate or download the backup file
# ---------------------------------------------------------------------------
resolve_dump_file() {
  if [[ "$FROM_REMOTE" == true ]]; then
    download_from_remote
    return
  fi

  if [[ -n "$TARGET_FILE" ]]; then
    # Specific file provided
    if [[ -f "$TARGET_FILE" ]]; then
      DUMP_PATH="$TARGET_FILE"
    elif [[ -f "$BACKUP_DIR/$TARGET_FILE" ]]; then
      DUMP_PATH="$BACKUP_DIR/$TARGET_FILE"
    else
      fatal "Backup file not found: $TARGET_FILE"
    fi
  else
    # Find most recent local backup
    DUMP_PATH=$(find "$BACKUP_DIR" -name "*.dump" -type f -printf '%T@ %p\n' 2>/dev/null | \
      sort -rn | head -1 | cut -d' ' -f2-)

    if [[ -z "$DUMP_PATH" ]]; then
      fatal "No backup files found in $BACKUP_DIR"
    fi
  fi

  info "Restore source: $DUMP_PATH"
}

download_from_remote() {
  [[ -n "$REMOTE_URL" ]] || fatal "BACKUP_REMOTE_URL not set"

  mkdir -p "$BACKUP_DIR"

  case "$REMOTE_URL" in
    s3://*)
      local s3_args=()
      if [[ -n "${S3_ENDPOINT_URL:-}" ]]; then
        s3_args+=(--endpoint-url "$S3_ENDPOINT_URL")
      fi

      if [[ -n "$TARGET_FILE" ]]; then
        # Download specific file
        DUMP_PATH="$BACKUP_DIR/$TARGET_FILE"
        aws s3 cp "${s3_args[@]}" "${REMOTE_URL%/}/$TARGET_FILE" "$DUMP_PATH"
      else
        # Find and download most recent
        TARGET_FILE=$(aws s3 ls "${s3_args[@]}" "${REMOTE_URL%/}/" 2>/dev/null | \
          grep '\.dump$' | sort -r | head -1 | awk '{print $4}')

        [[ -n "$TARGET_FILE" ]] || fatal "No backups found at $REMOTE_URL"

        DUMP_PATH="$BACKUP_DIR/$TARGET_FILE"
        aws s3 cp "${s3_args[@]}" "${REMOTE_URL%/}/$TARGET_FILE" "$DUMP_PATH"
      fi
      ;;

    rsync://*)
      local rsync_target="${REMOTE_URL#rsync://}"
      if [[ -n "$TARGET_FILE" ]]; then
        DUMP_PATH="$BACKUP_DIR/$TARGET_FILE"
        rsync -az "$rsync_target/$TARGET_FILE" "$DUMP_PATH"
      else
        fatal "Specify a filename with --from-remote for rsync sources"
      fi
      ;;

    *)
      fatal "Unknown remote URL scheme: $REMOTE_URL"
      ;;
  esac

  info "Downloaded: $DUMP_PATH"
}

# ---------------------------------------------------------------------------
# 2. Verify the backup
# ---------------------------------------------------------------------------
verify_dump() {
  info "Verifying backup integrity..."

  if ! pg_restore --list "$DUMP_PATH" > /dev/null 2>&1; then
    # Try via docker if pg_restore not installed locally
    if docker compose -f "$INSTALL_DIR/docker-compose.yml" \
        exec -T postgres pg_restore --list /tmp/restore_check.dump > /dev/null 2>&1; then
      info "Backup verified (via container)"
    else
      fatal "Backup appears corrupt — aborting restore"
    fi
  else
    info "Backup verified"
  fi
}

# ---------------------------------------------------------------------------
# 3. Restore
# ---------------------------------------------------------------------------
restore_database() {
  local db_user="${POSTGRES_USER:-manager_user}"
  local db_name="${POSTGRES_DB:-p2pool_dashboard}"

  echo ""
  warn "This will DROP and RECREATE the database: $db_name"
  warn "All current data will be replaced with the backup."
  echo ""
  if [[ "$SKIP_CONFIRM" != true ]]; then
    read -rp "Continue? [y/N] " confirm
    [[ "$confirm" =~ ^[Yy]$ ]] || { info "Restore cancelled."; exit 0; }
  fi

  info "Restoring from: $(basename "$DUMP_PATH")"

  # Copy dump into postgres container
  local container_id
  container_id=$(docker compose -f "$INSTALL_DIR/docker-compose.yml" ps -q postgres 2>/dev/null)

  if [[ -z "$container_id" ]]; then
    fatal "Postgres container not running. Start the stack first: make deploy"
  fi

  docker cp "$DUMP_PATH" "$container_id:/tmp/restore.dump"

  # Drop and recreate the database
  info "Dropping existing database..."
  docker compose -f "$INSTALL_DIR/docker-compose.yml" exec -T postgres \
    psql -U "$db_user" -d postgres -c "
      SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '$db_name' AND pid <> pg_backend_pid();
      DROP DATABASE IF EXISTS $db_name;
      CREATE DATABASE $db_name OWNER $db_user;
    "

  # Restore
  info "Restoring database..."
  docker compose -f "$INSTALL_DIR/docker-compose.yml" exec -T postgres \
    pg_restore -U "$db_user" -d "$db_name" --no-owner --no-privileges /tmp/restore.dump

  # Cleanup
  docker compose -f "$INSTALL_DIR/docker-compose.yml" exec -T postgres rm -f /tmp/restore.dump

  info "Database restored successfully"
}

# ---------------------------------------------------------------------------
# 4. Post-restore verification
# ---------------------------------------------------------------------------
post_verify() {
  info "Post-restore verification..."

  local table_count
  table_count=$(docker compose -f "$INSTALL_DIR/docker-compose.yml" exec -T postgres \
    psql -U "${POSTGRES_USER:-manager_user}" -d "${POSTGRES_DB:-p2pool_dashboard}" -tAc \
    "SELECT count(*) FROM information_schema.tables WHERE table_schema = 'public';")

  info "Tables in database: $table_count"

  # Show row counts for key tables
  for table in p2pool_shares p2pool_blocks payments miner_hashrate; do
    local count
    count=$(docker compose -f "$INSTALL_DIR/docker-compose.yml" exec -T postgres \
      psql -U "${POSTGRES_USER:-manager_user}" -d "${POSTGRES_DB:-p2pool_dashboard}" -tAc \
      "SELECT count(*) FROM $table;" 2>/dev/null || echo "N/A")
    info "  $table: $count rows"
  done
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
main() {
  echo "============================================"
  echo "  P2Pool Dashboard — Database Restore"
  echo "============================================"
  echo ""

  resolve_dump_file
  verify_dump
  restore_database
  post_verify

  echo ""
  echo "============================================"
  echo "  Restore complete!"
  echo "============================================"
  echo ""
  echo "  You may want to restart the manager service:"
  echo "  docker compose -f $INSTALL_DIR/docker-compose.yml restart manager"
  echo ""
}

main "$@"
