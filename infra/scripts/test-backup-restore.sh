#!/bin/bash
# test-backup-restore.sh — CI test for backup/restore round-trip
#
# Validates that pg_dump + pg_restore preserves schema and data.
# Designed to run in GitHub Actions with a Postgres service container.
#
# Required env vars:
#   PGHOST, PGPORT, PGUSER, PGPASSWORD, PGDATABASE
set -euo pipefail

MIGRATIONS_DIR="${MIGRATIONS_DIR:-services/manager/pkg/db/migrations}"
BACKUP_FILE="/tmp/ci_backup_test.dump"

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------
info()  { echo "[CI-TEST] [INFO]  $*"; }
error() { echo "[CI-TEST] [ERROR] $*" >&2; }
fatal() { error "$*"; exit 1; }

run_sql() { psql -v ON_ERROR_STOP=1 -tAc "$1"; }
run_sql_file() { psql -v ON_ERROR_STOP=1 -f "$1"; }

# ---------------------------------------------------------------------------
# 1. Apply migrations
# ---------------------------------------------------------------------------
apply_migrations() {
  info "Applying migrations..."
  for f in "$MIGRATIONS_DIR"/*.sql; do
    info "  $(basename "$f")"
    run_sql_file "$f"
  done
  info "Migrations applied"
}

# ---------------------------------------------------------------------------
# 2. Insert seed data
# ---------------------------------------------------------------------------
seed_data() {
  info "Inserting seed data..."
  psql -v ON_ERROR_STOP=1 <<'SQL'
INSERT INTO p2pool_shares (sidechain, miner_address, sidechain_height, difficulty)
VALUES
  ('mini', '4ADDRESS1aaa', 100, 50000),
  ('mini', '4ADDRESS2bbb', 101, 60000),
  ('mini', '4ADDRESS1aaa', 102, 55000);

INSERT INTO p2pool_blocks (main_height, main_hash, sidechain_height, coinbase_reward, effort, found_at)
VALUES
  (3000000, 'abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890', 100, 600000000000, 85.5000, '2026-01-15 12:00:00+00'),
  (3000010, 'fedcba0987654321fedcba0987654321fedcba0987654321fedcba0987654321', 110, 600000000000, 120.3000, '2026-01-15 14:00:00+00');

INSERT INTO miner_hashrate (miner_address, sidechain, hashrate, bucket_time)
VALUES
  ('4ADDRESS1aaa', 'mini', 15000, '2026-01-15 12:00:00+00'),
  ('4ADDRESS2bbb', 'mini', 22000, '2026-01-15 12:00:00+00');

INSERT INTO payments (miner_address, amount, main_height, main_hash)
VALUES
  ('4ADDRESS1aaa', 300000000000, 3000000, 'abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890');

INSERT INTO subscriptions (miner_address, tier)
VALUES
  ('4ADDRESS1aaa', 'pro');

INSERT INTO subscription_addresses (miner_address, subaddress, subaddress_index)
VALUES
  ('4ADDRESS1aaa', '8SUBADDR1xxx', 1);

INSERT INTO subscription_payments (miner_address, tx_hash, amount, confirmed)
VALUES
  ('4ADDRESS1aaa', 'tx1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcd', 50000000000, true);

INSERT INTO pool_stats_snapshots (sidechain, pool_hashrate, pool_miners, sidechain_height, sidechain_difficulty)
VALUES
  ('mini', 5000000, 42, 500000, 120000);
SQL
  info "Seed data inserted"
}

# ---------------------------------------------------------------------------
# 3. Capture row counts before backup
# ---------------------------------------------------------------------------
declare -A BEFORE_COUNTS

capture_counts() {
  local label="$1"
  info "Capturing row counts ($label)..."
  for table in p2pool_shares p2pool_blocks miner_hashrate payments subscriptions subscription_addresses subscription_payments pool_stats_snapshots; do
    local count
    count=$(run_sql "SELECT count(*) FROM $table;")
    if [[ "$label" == "before" ]]; then
      BEFORE_COUNTS[$table]="$count"
    fi
    info "  $table: $count"
    echo "${label}_${table}=${count}" >> /tmp/ci_counts.env
  done
}

# ---------------------------------------------------------------------------
# 4. Backup
# ---------------------------------------------------------------------------
do_backup() {
  info "Running pg_dump..."
  pg_dump --format=custom -f "$BACKUP_FILE"

  local size
  size=$(stat --printf='%s' "$BACKUP_FILE" 2>/dev/null || stat -f '%z' "$BACKUP_FILE" 2>/dev/null)
  info "Backup created: $(( size / 1024 )) KB"

  info "Verifying backup with pg_restore --list..."
  pg_restore --list "$BACKUP_FILE" > /dev/null
  info "Backup verified"
}

# ---------------------------------------------------------------------------
# 5. Drop and restore
# ---------------------------------------------------------------------------
do_restore() {
  info "Dropping all tables..."
  psql -v ON_ERROR_STOP=1 <<'SQL'
DROP TABLE IF EXISTS pool_stats_snapshots CASCADE;
DROP TABLE IF EXISTS subscription_payments CASCADE;
DROP TABLE IF EXISTS subscription_addresses CASCADE;
DROP TABLE IF EXISTS subscriptions CASCADE;
DROP TABLE IF EXISTS payments CASCADE;
DROP TABLE IF EXISTS miner_hashrate CASCADE;
DROP TABLE IF EXISTS p2pool_blocks CASCADE;
DROP TABLE IF EXISTS p2pool_shares CASCADE;
SQL

  # Confirm tables are gone
  local table_count
  table_count=$(run_sql "SELECT count(*) FROM information_schema.tables WHERE table_schema = 'public';")
  [[ "$table_count" -eq 0 ]] || fatal "Expected 0 tables after drop, got $table_count"
  info "All tables dropped"

  info "Restoring from backup..."
  pg_restore --no-owner --no-privileges -d "$PGDATABASE" "$BACKUP_FILE"
  info "Restore complete"
}

# ---------------------------------------------------------------------------
# 6. Verify
# ---------------------------------------------------------------------------
verify_restore() {
  info "Verifying restored data..."
  local failures=0

  # Load before counts from env file
  source /tmp/ci_counts.env

  for table in p2pool_shares p2pool_blocks miner_hashrate payments subscriptions subscription_addresses subscription_payments pool_stats_snapshots; do
    local after_count before_count
    after_count=$(run_sql "SELECT count(*) FROM $table;")
    before_count_var="before_${table}"
    before_count="${!before_count_var}"

    if [[ "$after_count" != "$before_count" ]]; then
      error "MISMATCH: $table — before=$before_count, after=$after_count"
      failures=$((failures + 1))
    else
      info "  OK: $table — $after_count rows"
    fi
  done

  # Spot-check a specific value
  local check_tier
  check_tier=$(run_sql "SELECT tier FROM subscriptions WHERE miner_address = '4ADDRESS1aaa';")
  if [[ "$check_tier" != "pro" ]]; then
    error "MISMATCH: subscriptions.tier — expected 'pro', got '$check_tier'"
    failures=$((failures + 1))
  else
    info "  OK: spot-check subscription tier = pro"
  fi

  local check_reward
  check_reward=$(run_sql "SELECT coinbase_reward FROM p2pool_blocks WHERE main_height = 3000000;")
  if [[ "$check_reward" != "600000000000" ]]; then
    error "MISMATCH: p2pool_blocks.coinbase_reward — expected 600000000000, got '$check_reward'"
    failures=$((failures + 1))
  else
    info "  OK: spot-check block reward = 600000000000"
  fi

  if [[ "$failures" -gt 0 ]]; then
    fatal "$failures verification failure(s) — backup/restore round-trip FAILED"
  fi

  info "All verifications passed"
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
main() {
  info "=== Backup/Restore CI Test ==="

  apply_migrations
  seed_data
  capture_counts "before"
  do_backup
  do_restore
  capture_counts "after"
  verify_restore

  rm -f "$BACKUP_FILE" /tmp/ci_counts.env
  info "=== PASSED ==="
}

main "$@"
