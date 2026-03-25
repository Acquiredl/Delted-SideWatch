#!/bin/bash
# pool-backup.sh — Database backup with optional remote upload
# Usage: ./pool-backup.sh
#
# Runs pg_dump, verifies the backup, optionally uploads to S3-compatible
# storage or rsync target, and cleans old local backups.
#
# Environment variables:
#   POSTGRES_HOST           (default: postgres)
#   POSTGRES_USER           (default: manager_user)
#   POSTGRES_DB             (default: p2pool_dashboard)
#   BACKUP_DIR              (default: /backups)
#   BACKUP_RETENTION_DAYS   (default: 7)
#   BACKUP_REMOTE_URL       (optional) s3://bucket/path or rsync://host:/path
#   AWS_ACCESS_KEY_ID       (required if using S3)
#   AWS_SECRET_ACCESS_KEY   (required if using S3)
#   AWS_DEFAULT_REGION      (default: us-east-1)
#   S3_ENDPOINT_URL         (optional, for non-AWS S3-compatible stores)
set -euo pipefail

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_DIR="${BACKUP_DIR:-/backups}"
RETENTION_DAYS="${BACKUP_RETENTION_DAYS:-7}"
REMOTE_URL="${BACKUP_REMOTE_URL:-}"
DUMP_FILE="p2pool_${TIMESTAMP}.dump"
DUMP_PATH="$BACKUP_DIR/$DUMP_FILE"

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------
info()  { echo "[$(date +%H:%M:%S)] [INFO]  $*"; }
warn()  { echo "[$(date +%H:%M:%S)] [WARN]  $*"; }
error() { echo "[$(date +%H:%M:%S)] [ERROR] $*" >&2; }

# ---------------------------------------------------------------------------
# 1. Dump database
# ---------------------------------------------------------------------------
dump_database() {
  mkdir -p "$BACKUP_DIR"

  info "Starting pg_dump..."
  pg_dump \
    -h "${POSTGRES_HOST:-postgres}" \
    -U "${POSTGRES_USER:-manager_user}" \
    -d "${POSTGRES_DB:-p2pool_dashboard}" \
    --format=custom \
    -f "$DUMP_PATH"

  local size
  size=$(du -h "$DUMP_PATH" | cut -f1)
  info "Dump complete: $DUMP_FILE ($size)"
}

# ---------------------------------------------------------------------------
# 2. Verify backup integrity
# ---------------------------------------------------------------------------
verify_backup() {
  info "Verifying backup integrity..."

  if ! pg_restore --list "$DUMP_PATH" > /dev/null 2>&1; then
    error "Backup verification failed — dump may be corrupt: $DUMP_PATH"
    rm -f "$DUMP_PATH"
    exit 1
  fi

  info "Backup verified (pg_restore --list OK)"
}

# ---------------------------------------------------------------------------
# 3. Upload to remote storage
# ---------------------------------------------------------------------------
upload_remote() {
  [[ -n "$REMOTE_URL" ]] || return 0

  info "Uploading to remote: $REMOTE_URL"

  case "$REMOTE_URL" in
    s3://*)
      # S3-compatible storage (AWS, Backblaze B2, MinIO, etc.)
      local s3_args=()
      if [[ -n "${S3_ENDPOINT_URL:-}" ]]; then
        s3_args+=(--endpoint-url "$S3_ENDPOINT_URL")
      fi

      if ! command -v aws &>/dev/null; then
        warn "aws CLI not found — skipping S3 upload"
        warn "Install: apt-get install awscli  or  pip install awscli"
        return 1
      fi

      aws s3 cp "${s3_args[@]}" "$DUMP_PATH" "${REMOTE_URL%/}/$DUMP_FILE"
      info "Uploaded to S3: ${REMOTE_URL%/}/$DUMP_FILE"

      # Clean old remote backups
      clean_remote_s3
      ;;

    rsync://*)
      # rsync to remote host
      local rsync_target="${REMOTE_URL#rsync://}"
      rsync -az --progress "$DUMP_PATH" "$rsync_target/$DUMP_FILE"
      info "Synced to: $rsync_target/$DUMP_FILE"
      ;;

    /*)
      # Local path (e.g., mounted NFS/external drive)
      mkdir -p "$REMOTE_URL"
      cp "$DUMP_PATH" "${REMOTE_URL%/}/$DUMP_FILE"
      find "$REMOTE_URL" -name "*.dump" -mtime +"$RETENTION_DAYS" -delete
      info "Copied to: ${REMOTE_URL%/}/$DUMP_FILE"
      ;;

    *)
      warn "Unknown remote URL scheme: $REMOTE_URL — skipping upload"
      return 1
      ;;
  esac
}

# ---------------------------------------------------------------------------
# Clean old S3 backups
# ---------------------------------------------------------------------------
clean_remote_s3() {
  local s3_args=()
  if [[ -n "${S3_ENDPOINT_URL:-}" ]]; then
    s3_args+=(--endpoint-url "$S3_ENDPOINT_URL")
  fi

  local cutoff_date
  cutoff_date=$(date -d "-${RETENTION_DAYS} days" +%Y%m%d 2>/dev/null || \
                date -v-"${RETENTION_DAYS}"d +%Y%m%d 2>/dev/null || echo "")

  if [[ -z "$cutoff_date" ]]; then
    warn "Could not compute cutoff date — skipping remote cleanup"
    return
  fi

  info "Cleaning remote backups older than $RETENTION_DAYS days..."

  aws s3 ls "${s3_args[@]}" "${REMOTE_URL%/}/" 2>/dev/null | \
    grep '\.dump$' | \
    while read -r _ _ _ filename; do
      # Extract date from filename: p2pool_YYYYMMDD_HHMMSS.dump
      local file_date
      file_date=$(echo "$filename" | grep -oP '\d{8}' | head -1)
      if [[ -n "$file_date" && "$file_date" < "$cutoff_date" ]]; then
        aws s3 rm "${s3_args[@]}" "${REMOTE_URL%/}/$filename"
        info "Removed old remote backup: $filename"
      fi
    done
}

# ---------------------------------------------------------------------------
# 4. Clean old local backups
# ---------------------------------------------------------------------------
clean_local() {
  local deleted
  deleted=$(find "$BACKUP_DIR" -name "*.dump" -mtime +"$RETENTION_DAYS" -print -delete 2>/dev/null | wc -l)

  if [[ "$deleted" -gt 0 ]]; then
    info "Cleaned $deleted local backup(s) older than $RETENTION_DAYS days"
  fi
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
main() {
  info "=== P2Pool Backup ==="
  dump_database
  verify_backup
  upload_remote || warn "Remote upload failed — local backup retained"
  clean_local
  info "=== Backup complete: $DUMP_FILE ==="
}

main "$@"
