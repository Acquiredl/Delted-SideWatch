#!/bin/bash
set -euo pipefail
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_DIR="/backups"
mkdir -p "$BACKUP_DIR"
pg_dump -h "${POSTGRES_HOST:-postgres}" -U "${POSTGRES_USER:-manager_user}" \
  -d "${POSTGRES_DB:-p2pool_dashboard}" \
  --format=custom \
  -f "$BACKUP_DIR/p2pool_${TIMESTAMP}.dump"
# Keep last 7 days
find "$BACKUP_DIR" -name "*.dump" -mtime +7 -delete
echo "Backup complete: p2pool_${TIMESTAMP}.dump"
