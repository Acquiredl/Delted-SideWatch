#!/bin/bash
# validate-node.sh — Validate the dashboard against a live P2Pool node + monerod
#
# Runs a series of checks to verify the manager service is correctly
# polling, indexing, and serving data from real nodes (not mocknode).
#
# Usage:
#   ./validate-node.sh                        # defaults: manager=localhost:8081, p2pool=localhost:3333, monerod=localhost:18081
#   ./validate-node.sh --manager http://host:8081 --p2pool http://host:3333 --monerod http://host:18081
#
# Exit code 0 = all checks passed, non-zero = failures detected.
set -euo pipefail

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------
MANAGER_URL="${MANAGER_URL:-http://localhost:8081}"
P2POOL_URL="${P2POOL_URL:-http://localhost:3333}"
MONEROD_URL="${MONEROD_URL:-http://localhost:18081}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --manager)  MANAGER_URL="$2"; shift 2 ;;
    --p2pool)   P2POOL_URL="$2"; shift 2 ;;
    --monerod)  MONEROD_URL="$2"; shift 2 ;;
    --help|-h)
      echo "Usage: $0 [--manager URL] [--p2pool URL] [--monerod URL]"
      exit 0
      ;;
    *) echo "Unknown arg: $1"; exit 1 ;;
  esac
done

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------
PASS=0
FAIL=0
WARN=0

pass() { echo "  [PASS] $*"; PASS=$((PASS + 1)); }
fail() { echo "  [FAIL] $*"; FAIL=$((FAIL + 1)); }
warn() { echo "  [WARN] $*"; WARN=$((WARN + 1)); }
info() { echo "  [INFO] $*"; }
section() { echo ""; echo "=== $* ==="; }

# curl wrapper: returns body to stdout, HTTP status code to fd 3
fetch() {
  local url="$1"
  curl -sS --max-time 10 -w '\n%{http_code}' "$url" 2>/dev/null || echo -e "\n000"
}

fetch_json() {
  local url="$1"
  local response
  response=$(fetch "$url")
  local status
  status=$(echo "$response" | tail -1)
  local body
  body=$(echo "$response" | sed '$d')

  if [[ "$status" == "000" ]]; then
    echo ""
    return 1
  elif [[ "$status" -ge 200 && "$status" -lt 300 ]]; then
    echo "$body"
    return 0
  else
    echo ""
    return 1
  fi
}

rpc_call() {
  local url="$1"
  local method="$2"
  local params="${3:-{}}"
  curl -sS --max-time 10 -X POST "$url/json_rpc" \
    -H 'Content-Type: application/json' \
    -d "{\"jsonrpc\":\"2.0\",\"id\":\"0\",\"method\":\"$method\",\"params\":$params}" 2>/dev/null || echo ""
}

# ---------------------------------------------------------------------------
# 1. Monerod health
# ---------------------------------------------------------------------------
check_monerod() {
  section "Monerod ($MONEROD_URL)"

  local result
  result=$(rpc_call "$MONEROD_URL" "get_last_block_header")

  if [[ -z "$result" ]]; then
    fail "Cannot reach monerod at $MONEROD_URL"
    return
  fi

  local height
  height=$(echo "$result" | python3 -c "import sys,json; print(json.load(sys.stdin)['result']['block_header']['height'])" 2>/dev/null || echo "")

  if [[ -z "$height" ]]; then
    fail "Monerod responded but could not parse block height"
    return
  fi

  pass "Monerod reachable — current height: $height"

  # Check if synced (height should be recent — within ~10 blocks of real network)
  local timestamp
  timestamp=$(echo "$result" | python3 -c "import sys,json; print(json.load(sys.stdin)['result']['block_header']['timestamp'])" 2>/dev/null || echo "0")
  local now
  now=$(date +%s)
  local age=$(( now - timestamp ))

  if [[ "$age" -gt 7200 ]]; then
    warn "Latest block is ${age}s old — monerod may still be syncing"
  else
    pass "Monerod appears synced (latest block ${age}s ago)"
  fi
}

# ---------------------------------------------------------------------------
# 2. P2Pool health
# ---------------------------------------------------------------------------
check_p2pool() {
  section "P2Pool ($P2POOL_URL)"

  local stats
  stats=$(fetch_json "$P2POOL_URL/api/pool/stats")

  if [[ -z "$stats" ]]; then
    fail "Cannot reach P2Pool at $P2POOL_URL/api/pool/stats"
    return
  fi

  local hashrate miners
  hashrate=$(echo "$stats" | python3 -c "import sys,json; print(json.load(sys.stdin)['pool_statistics']['hash_rate'])" 2>/dev/null || echo "")
  miners=$(echo "$stats" | python3 -c "import sys,json; print(json.load(sys.stdin)['pool_statistics']['miners'])" 2>/dev/null || echo "")

  if [[ -z "$hashrate" ]]; then
    fail "P2Pool responded but could not parse pool stats"
    return
  fi

  pass "P2Pool reachable — hashrate: $hashrate H/s, miners: $miners"

  # Check shares endpoint
  local shares
  shares=$(fetch_json "$P2POOL_URL/api/shares")
  if [[ -n "$shares" ]]; then
    local share_count
    share_count=$(echo "$shares" | python3 -c "import sys,json; print(len(json.load(sys.stdin)))" 2>/dev/null || echo "0")
    pass "Shares endpoint OK — $share_count shares in PPLNS window"
  else
    fail "Shares endpoint failed"
  fi

  # Check found blocks
  local blocks
  blocks=$(fetch_json "$P2POOL_URL/api/found_blocks")
  if [[ -n "$blocks" ]]; then
    local block_count
    block_count=$(echo "$blocks" | python3 -c "import sys,json; print(len(json.load(sys.stdin)))" 2>/dev/null || echo "0")
    pass "Found blocks endpoint OK — $block_count blocks"
  else
    warn "Found blocks endpoint returned empty (may be normal for new nodes)"
  fi
}

# ---------------------------------------------------------------------------
# 3. Manager health
# ---------------------------------------------------------------------------
check_manager() {
  section "Manager ($MANAGER_URL)"

  local health
  health=$(fetch_json "$MANAGER_URL/health")

  if [[ -z "$health" ]]; then
    fail "Cannot reach manager at $MANAGER_URL/health"
    return
  fi

  local status pg_status redis_status
  status=$(echo "$health" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('status',''))" 2>/dev/null || echo "")
  pg_status=$(echo "$health" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('postgres',''))" 2>/dev/null || echo "")
  redis_status=$(echo "$health" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('redis',''))" 2>/dev/null || echo "")

  if [[ "$status" == "ok" ]]; then
    pass "Manager healthy — postgres: $pg_status, redis: $redis_status"
  else
    fail "Manager unhealthy — status: $status, postgres: $pg_status, redis: $redis_status"
  fi
}

# ---------------------------------------------------------------------------
# 4. Manager API endpoints
# ---------------------------------------------------------------------------
check_manager_api() {
  section "Manager API Endpoints"

  # Pool stats
  local pool_stats
  pool_stats=$(fetch_json "$MANAGER_URL/api/pool/stats")
  if [[ -n "$pool_stats" ]]; then
    local total_miners sidechain
    total_miners=$(echo "$pool_stats" | python3 -c "import sys,json; print(json.load(sys.stdin).get('total_miners',0))" 2>/dev/null || echo "0")
    sidechain=$(echo "$pool_stats" | python3 -c "import sys,json; print(json.load(sys.stdin).get('sidechain','unknown'))" 2>/dev/null || echo "unknown")
    pass "GET /api/pool/stats — miners: $total_miners, sidechain: $sidechain"
  else
    fail "GET /api/pool/stats failed"
  fi

  # Blocks
  local blocks
  blocks=$(fetch_json "$MANAGER_URL/api/blocks?limit=5&offset=0")
  if [[ -n "$blocks" ]]; then
    local block_count
    block_count=$(echo "$blocks" | python3 -c "import sys,json; print(len(json.load(sys.stdin)))" 2>/dev/null || echo "0")
    pass "GET /api/blocks — returned $block_count blocks"
  else
    warn "GET /api/blocks returned empty (may be normal if no blocks found yet)"
  fi

  # Sidechain shares
  local shares
  shares=$(fetch_json "$MANAGER_URL/api/sidechain/shares?limit=10&offset=0")
  if [[ -n "$shares" ]]; then
    local share_count
    share_count=$(echo "$shares" | python3 -c "import sys,json; print(len(json.load(sys.stdin)))" 2>/dev/null || echo "0")
    pass "GET /api/sidechain/shares — returned $share_count shares"
  else
    warn "GET /api/sidechain/shares returned empty (indexer may still be warming up)"
  fi
}

# ---------------------------------------------------------------------------
# 5. Data flow: P2Pool → Manager pipeline
# ---------------------------------------------------------------------------
check_data_flow() {
  section "Data Flow Validation"

  # Compare P2Pool direct stats with what the manager serves
  local p2pool_miners manager_miners
  p2pool_miners=$(fetch_json "$P2POOL_URL/api/pool/stats" | python3 -c "import sys,json; print(json.load(sys.stdin)['pool_statistics']['miners'])" 2>/dev/null || echo "")
  manager_miners=$(fetch_json "$MANAGER_URL/api/pool/stats" | python3 -c "import sys,json; print(json.load(sys.stdin).get('total_miners',0))" 2>/dev/null || echo "")

  if [[ -z "$p2pool_miners" || -z "$manager_miners" ]]; then
    warn "Could not compare P2Pool vs manager data (one or both unreachable)"
    return
  fi

  if [[ "$p2pool_miners" == "$manager_miners" ]]; then
    pass "Miner count matches: P2Pool=$p2pool_miners, Manager=$manager_miners"
  else
    info "Miner count differs: P2Pool=$p2pool_miners, Manager=$manager_miners (may lag by one poll cycle)"
  fi
}

# ---------------------------------------------------------------------------
# 6. Metrics endpoint
# ---------------------------------------------------------------------------
check_metrics() {
  section "Prometheus Metrics"

  local metrics
  metrics=$(fetch "$MANAGER_URL:9090/metrics" 2>/dev/null || fetch "http://localhost:9090/metrics" 2>/dev/null || echo "")
  local status
  status=$(echo "$metrics" | tail -1)

  if [[ "$status" == "200" ]]; then
    pass "Metrics endpoint reachable"
  else
    warn "Metrics endpoint not reachable (may be on a different port)"
  fi
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
main() {
  echo "============================================"
  echo "  P2Pool Dashboard — Production Validation"
  echo "============================================"
  echo ""
  echo "  Manager:  $MANAGER_URL"
  echo "  P2Pool:   $P2POOL_URL"
  echo "  Monerod:  $MONEROD_URL"

  check_monerod
  check_p2pool
  check_manager
  check_manager_api
  check_data_flow
  check_metrics

  section "Summary"
  echo "  Passed: $PASS"
  echo "  Failed: $FAIL"
  echo "  Warnings: $WARN"
  echo ""

  if [[ "$FAIL" -gt 0 ]]; then
    echo "  RESULT: FAILED ($FAIL check(s) failed)"
    exit 1
  elif [[ "$WARN" -gt 0 ]]; then
    echo "  RESULT: PASSED with warnings"
    exit 0
  else
    echo "  RESULT: ALL CHECKS PASSED"
    exit 0
  fi
}

main "$@"
