#!/bin/bash
# load-test.sh — HTTP load test for the P2Pool Dashboard API
#
# Uses vegeta (preferred) or curl-based serial testing as fallback.
# Install vegeta: go install github.com/tsenart/vegeta@latest
#
# Usage:
#   ./load-test.sh                          # default: localhost:8081, 50 req/s, 30s
#   ./load-test.sh --target http://host:8081 --rate 100 --duration 60s
set -euo pipefail

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------
TARGET="${TARGET:-http://localhost:8081}"
RATE="${RATE:-50}"
DURATION="${DURATION:-30s}"
TEST_ADDRESS="4AdUndXHHZ6cfufTMvppY6JwXNouMBzSkbLYfpAV5Usx3skQNBjb3JcW38T4FcvKFZXS5gTb2o4oFk5HfVrp9p2pGjHnLNq"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --target)   TARGET="$2"; shift 2 ;;
    --rate)     RATE="$2"; shift 2 ;;
    --duration) DURATION="$2"; shift 2 ;;
    --help|-h)
      echo "Usage: $0 [--target URL] [--rate RPS] [--duration TIME]"
      exit 0
      ;;
    *) echo "Unknown arg: $1"; exit 1 ;;
  esac
done

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------
info() { echo "[load-test] $*"; }

# ---------------------------------------------------------------------------
# Health check
# ---------------------------------------------------------------------------
info "Checking target: $TARGET"
if ! curl -sf --max-time 5 "$TARGET/health" > /dev/null 2>&1; then
  echo "ERROR: Cannot reach $TARGET/health — is the service running?"
  exit 1
fi
info "Target healthy"

# ---------------------------------------------------------------------------
# Vegeta load test
# ---------------------------------------------------------------------------
run_vegeta() {
  info "Using vegeta — rate: ${RATE}/s, duration: $DURATION"

  local targets_file
  targets_file=$(mktemp)

  cat > "$targets_file" <<EOF
GET ${TARGET}/health
GET ${TARGET}/api/pool/stats
GET ${TARGET}/api/blocks?limit=50&offset=0
GET ${TARGET}/api/sidechain/shares?limit=100&offset=0
GET ${TARGET}/api/miner/${TEST_ADDRESS}
GET ${TARGET}/api/miner/${TEST_ADDRESS}/payments?limit=50&offset=0
GET ${TARGET}/api/miner/${TEST_ADDRESS}/hashrate?hours=24
GET ${TARGET}/api/subscription/status/${TEST_ADDRESS}
EOF

  info "Targets:"
  cat "$targets_file" | sed 's/^/  /'
  echo ""

  vegeta attack \
    -targets="$targets_file" \
    -rate="${RATE}/1s" \
    -duration="$DURATION" \
    | vegeta report

  echo ""
  info "Latency histogram:"
  vegeta attack \
    -targets="$targets_file" \
    -rate="${RATE}/1s" \
    -duration="10s" \
    | vegeta report -type='hist[0,5ms,10ms,25ms,50ms,100ms,250ms,500ms,1s]'

  rm -f "$targets_file"
}

# ---------------------------------------------------------------------------
# Curl-based fallback
# ---------------------------------------------------------------------------
run_curl_bench() {
  info "vegeta not found — using curl serial benchmark"
  info "Install vegeta for proper load testing: go install github.com/tsenart/vegeta@latest"
  echo ""

  local endpoints=(
    "/health"
    "/api/pool/stats"
    "/api/blocks?limit=50&offset=0"
    "/api/sidechain/shares?limit=100&offset=0"
    "/api/miner/${TEST_ADDRESS}"
    "/api/miner/${TEST_ADDRESS}/payments?limit=50&offset=0"
    "/api/miner/${TEST_ADDRESS}/hashrate?hours=24"
    "/api/subscription/status/${TEST_ADDRESS}"
  )

  local iterations=100
  info "Running $iterations requests per endpoint (serial)..."
  echo ""

  printf "%-55s %8s %8s %8s %8s\n" "Endpoint" "Min" "Avg" "Max" "Errors"
  printf "%-55s %8s %8s %8s %8s\n" "-------" "---" "---" "---" "------"

  for ep in "${endpoints[@]}"; do
    local total_ms=0
    local min_ms=999999
    local max_ms=0
    local errors=0

    for ((i=0; i<iterations; i++)); do
      local result
      result=$(curl -sf -o /dev/null -w '%{time_total}:%{http_code}' --max-time 5 "${TARGET}${ep}" 2>/dev/null || echo "5.0:000")

      local time_s="${result%%:*}"
      local status="${result##*:}"
      local time_ms
      time_ms=$(echo "$time_s" | awk '{printf "%.0f", $1 * 1000}')

      total_ms=$((total_ms + time_ms))
      [[ "$time_ms" -lt "$min_ms" ]] && min_ms="$time_ms"
      [[ "$time_ms" -gt "$max_ms" ]] && max_ms="$time_ms"
      [[ "$status" != "200" ]] && errors=$((errors + 1))
    done

    local avg_ms=$((total_ms / iterations))
    printf "%-55s %6dms %6dms %6dms %8d\n" "$ep" "$min_ms" "$avg_ms" "$max_ms" "$errors"
  done
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
if command -v vegeta &>/dev/null; then
  run_vegeta
else
  run_curl_bench
fi

echo ""
info "Done"
