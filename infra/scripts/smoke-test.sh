#!/bin/bash
# smoke-test.sh — Production stack validation for SideWatch
# Run ON the VPS after deploy to verify all services are healthy.
#
# Usage:
#   ./smoke-test.sh                     # defaults: localhost, no verbose
#   ./smoke-test.sh --verbose           # show response bodies
#   ./smoke-test.sh --host 10.0.0.5     # test specific host
#
# Tests:
#   1. Container health (docker compose)
#   2. Monerod RPC (synced, chain tip)
#   3. P2Pool API (pool stats, shares, peers)
#   4. Manager health + all API endpoints
#   5. Gateway health
#   6. Frontend serving
#   7. Postgres connectivity (via manager /health)
#   8. Redis connectivity (via manager /health)
#   9. Indexer verification (shares in DB via API)
#  10. Manager logs check (recent errors)
set -euo pipefail

# ---------------------------------------------------------------------------
# Config
# ---------------------------------------------------------------------------
HOST="127.0.0.1"
VERBOSE=false
TIMEOUT=10

while [[ $# -gt 0 ]]; do
  case "$1" in
    --host)    HOST="$2";   shift 2 ;;
    --verbose) VERBOSE=true; shift ;;
    *) echo "Unknown option: $1"; exit 1 ;;
  esac
done

MANAGER="http://${HOST}:8081"
GATEWAY="http://${HOST}:8080"
FRONTEND="http://${HOST}:3001"
P2POOL="http://${HOST}:3333"
MONEROD="http://${HOST}:18081"
PROMETHEUS="http://${HOST}:9091"

PASS=0
FAIL=0
WARN=0
RESULTS=()

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color
BOLD='\033[1m'

pass() {
  PASS=$((PASS + 1))
  RESULTS+=("${GREEN}PASS${NC}  $1")
  echo -e "  ${GREEN}✓${NC} $1"
}

fail() {
  FAIL=$((FAIL + 1))
  RESULTS+=("${RED}FAIL${NC}  $1  — $2")
  echo -e "  ${RED}✗${NC} $1  — $2"
}

warn() {
  WARN=$((WARN + 1))
  RESULTS+=("${YELLOW}WARN${NC}  $1  — $2")
  echo -e "  ${YELLOW}!${NC} $1  — $2"
}

section() {
  echo ""
  echo -e "${BOLD}${CYAN}[$1]${NC}"
}

# curl wrapper: returns body to stdout, HTTP code to fd 3
http_get() {
  local url="$1"
  local body http_code
  body=$(curl -sf --max-time "$TIMEOUT" -w "\n%{http_code}" "$url" 2>/dev/null) || {
    echo ""
    return 1
  }
  http_code=$(echo "$body" | tail -1)
  body=$(echo "$body" | sed '$d')
  if [[ "$VERBOSE" == true && -n "$body" ]]; then
    echo -e "    ${CYAN}→${NC} $(echo "$body" | head -5)" >&2
  fi
  echo "$body"
  return 0
}

http_post_json() {
  local url="$1"
  local data="$2"
  curl -sf --max-time "$TIMEOUT" -X POST \
    -H "Content-Type: application/json" \
    -d "$data" "$url" 2>/dev/null || echo ""
}

# ---------------------------------------------------------------------------
# 1. Container health
# ---------------------------------------------------------------------------
section "Docker Containers"

if command -v docker &>/dev/null; then
  # Check each expected container
  EXPECTED_CONTAINERS=("manager" "gateway" "frontend" "nginx" "postgres" "redis" "p2pool" "monerod" "prometheus" "grafana" "tor")

  for name in "${EXPECTED_CONTAINERS[@]}"; do
    status=$(docker ps --filter "name=p2pool-dashboard-${name}" --format '{{.Status}}' 2>/dev/null | head -1)
    if [[ -z "$status" ]]; then
      fail "$name" "container not found"
    elif echo "$status" | grep -qi "unhealthy"; then
      warn "$name" "running but unhealthy — $status"
    elif echo "$status" | grep -qi "up"; then
      pass "$name — $status"
    else
      fail "$name" "$status"
    fi
  done
else
  warn "docker" "docker CLI not available — skipping container checks"
fi

# ---------------------------------------------------------------------------
# 2. Monerod RPC
# ---------------------------------------------------------------------------
section "Monerod"

monerod_info=$(http_post_json "${MONEROD}/json_rpc" '{"jsonrpc":"2.0","id":"0","method":"get_info"}')
if [[ -n "$monerod_info" ]]; then
  synced=$(echo "$monerod_info" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('result',{}).get('synchronized', False))" 2>/dev/null || echo "unknown")
  height=$(echo "$monerod_info" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('result',{}).get('height', 'unknown'))" 2>/dev/null || echo "unknown")
  target_height=$(echo "$monerod_info" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('result',{}).get('target_height', 'unknown'))" 2>/dev/null || echo "unknown")

  if [[ "$synced" == "True" ]]; then
    pass "monerod synced — height $height"
  else
    fail "monerod sync" "not synced — height $height / target $target_height"
  fi

  # Check last block header for chain tip freshness
  last_header=$(http_post_json "${MONEROD}/json_rpc" '{"jsonrpc":"2.0","id":"0","method":"get_last_block_header"}')
  if [[ -n "$last_header" ]]; then
    block_ts=$(echo "$last_header" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('result',{}).get('block_header',{}).get('timestamp', 0))" 2>/dev/null || echo "0")
    now_ts=$(date +%s)
    if [[ "$block_ts" -gt 0 ]]; then
      age=$(( now_ts - block_ts ))
      if [[ "$age" -lt 300 ]]; then
        pass "chain tip fresh — ${age}s old"
      elif [[ "$age" -lt 600 ]]; then
        warn "chain tip" "block is ${age}s old (>5 min)"
      else
        fail "chain tip" "block is ${age}s old (stale)"
      fi
    fi
  fi
else
  fail "monerod RPC" "no response from ${MONEROD}/json_rpc"
fi

# ---------------------------------------------------------------------------
# 3. P2Pool API
# ---------------------------------------------------------------------------
section "P2Pool"

p2pool_stats=$(http_get "${P2POOL}/api/pool/stats")
if [[ -n "$p2pool_stats" ]]; then
  hash_rate=$(echo "$p2pool_stats" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('pool_statistics',{}).get('hashRate', 'N/A'))" 2>/dev/null || echo "N/A")
  miners=$(echo "$p2pool_stats" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('pool_statistics',{}).get('miners', 'N/A'))" 2>/dev/null || echo "N/A")
  pass "pool stats — hashrate: $hash_rate, miners: $miners"
else
  fail "P2Pool /api/pool/stats" "no response — this is why the container is unhealthy"
fi

p2pool_shares=$(http_get "${P2POOL}/api/shares")
if [[ -n "$p2pool_shares" ]]; then
  share_count=$(echo "$p2pool_shares" | python3 -c "import sys,json; d=json.load(sys.stdin); print(len(d) if isinstance(d,list) else 'not-a-list')" 2>/dev/null || echo "parse-error")
  pass "shares endpoint — $share_count shares in PPLNS window"
else
  warn "P2Pool /api/shares" "empty or no response (normal if no miners)"
fi

p2pool_blocks=$(http_get "${P2POOL}/api/found_blocks")
if [[ -n "$p2pool_blocks" ]]; then
  block_count=$(echo "$p2pool_blocks" | python3 -c "import sys,json; d=json.load(sys.stdin); print(len(d) if isinstance(d,list) else 'not-a-list')" 2>/dev/null || echo "parse-error")
  pass "found blocks — $block_count blocks"
else
  warn "P2Pool /api/found_blocks" "empty or no response (normal if pool hasn't found blocks)"
fi

p2pool_peers=$(http_get "${P2POOL}/api/p2p/peers")
if [[ -n "$p2pool_peers" ]]; then
  peer_count=$(echo "$p2pool_peers" | python3 -c "import sys,json; d=json.load(sys.stdin); print(len(d) if isinstance(d,list) else 'not-a-list')" 2>/dev/null || echo "parse-error")
  if [[ "$peer_count" != "0" && "$peer_count" != "parse-error" ]]; then
    pass "P2P peers — $peer_count connected"
  else
    warn "P2P peers" "no peers connected — check firewall for port 37888"
  fi
else
  warn "P2Pool /api/p2p/peers" "no response"
fi

# ---------------------------------------------------------------------------
# 4. Manager API
# ---------------------------------------------------------------------------
section "Manager API"

# Health endpoint (includes postgres + redis checks)
manager_health=$(http_get "${MANAGER}/health")
if [[ -n "$manager_health" ]]; then
  status=$(echo "$manager_health" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('status','unknown'))" 2>/dev/null || echo "unknown")
  pg_status=$(echo "$manager_health" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('postgres','unknown'))" 2>/dev/null || echo "unknown")
  redis_status=$(echo "$manager_health" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('redis','unknown'))" 2>/dev/null || echo "unknown")

  if [[ "$status" == "ok" ]]; then
    pass "health — postgres: $pg_status, redis: $redis_status"
  else
    fail "health" "status=$status, postgres=$pg_status, redis=$redis_status"
  fi
else
  fail "manager health" "no response from ${MANAGER}/health"
fi

# Pool stats (aggregated from DB)
pool_stats=$(http_get "${MANAGER}/api/pool/stats")
if [[ -n "$pool_stats" ]]; then
  total_miners=$(echo "$pool_stats" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('total_miners', 'N/A'))" 2>/dev/null || echo "N/A")
  total_hashrate=$(echo "$pool_stats" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('total_hashrate', 'N/A'))" 2>/dev/null || echo "N/A")
  blocks_found=$(echo "$pool_stats" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('blocks_found', 'N/A'))" 2>/dev/null || echo "N/A")
  sidechain=$(echo "$pool_stats" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('sidechain', 'N/A'))" 2>/dev/null || echo "N/A")
  pass "pool stats — miners: $total_miners, hashrate: $total_hashrate, blocks: $blocks_found, chain: $sidechain"
else
  fail "manager /api/pool/stats" "no response"
fi

# Blocks endpoint
blocks=$(http_get "${MANAGER}/api/blocks?limit=5")
if [[ -n "$blocks" ]]; then
  count=$(echo "$blocks" | python3 -c "import sys,json; d=json.load(sys.stdin); print(len(d) if isinstance(d,list) else 'not-a-list')" 2>/dev/null || echo "parse-error")
  pass "blocks — $count returned"
else
  warn "manager /api/blocks" "empty or no response"
fi

# Sidechain shares endpoint
shares=$(http_get "${MANAGER}/api/sidechain/shares?limit=5")
if [[ -n "$shares" ]]; then
  count=$(echo "$shares" | python3 -c "import sys,json; d=json.load(sys.stdin); print(len(d) if isinstance(d,list) else 'not-a-list')" 2>/dev/null || echo "parse-error")
  if [[ "$count" != "0" && "$count" != "parse-error" ]]; then
    pass "sidechain shares — $count returned (indexer is working!)"
  else
    warn "sidechain shares" "0 shares in DB — indexer may not have indexed yet"
  fi
else
  warn "manager /api/sidechain/shares" "empty response"
fi

# Metrics endpoint
metrics_resp=$(curl -sf --max-time "$TIMEOUT" "${MANAGER/8081/9090}/metrics" 2>/dev/null | head -5)
if [[ -n "$metrics_resp" ]]; then
  pass "Prometheus metrics endpoint"
else
  warn "metrics" "no response on :9090/metrics"
fi

# ---------------------------------------------------------------------------
# 5. Gateway
# ---------------------------------------------------------------------------
section "Gateway"

gw_health=$(http_get "${GATEWAY}/health")
if [[ -n "$gw_health" ]]; then
  pass "gateway health"
else
  fail "gateway health" "no response from ${GATEWAY}/health"
fi

# ---------------------------------------------------------------------------
# 6. Frontend
# ---------------------------------------------------------------------------
section "Frontend"

fe_resp=$(curl -sf --max-time "$TIMEOUT" -o /dev/null -w "%{http_code}" "${FRONTEND}/" 2>/dev/null || echo "000")
if [[ "$fe_resp" == "200" ]]; then
  pass "frontend serving (HTTP $fe_resp)"
else
  fail "frontend" "HTTP $fe_resp from ${FRONTEND}/"
fi

# ---------------------------------------------------------------------------
# 7. Nginx (external entry point)
# ---------------------------------------------------------------------------
section "Nginx"

nginx_http=$(curl -sf --max-time "$TIMEOUT" -o /dev/null -w "%{http_code}" "http://${HOST}/" 2>/dev/null || echo "000")
if [[ "$nginx_http" == "200" || "$nginx_http" == "301" || "$nginx_http" == "302" ]]; then
  pass "nginx HTTP — $nginx_http"
else
  warn "nginx HTTP" "HTTP $nginx_http (may need TLS or config)"
fi

# ---------------------------------------------------------------------------
# 8. Monitoring stack
# ---------------------------------------------------------------------------
section "Monitoring"

prom_resp=$(http_get "${PROMETHEUS}/api/v1/status/config" 2>/dev/null)
if [[ -n "$prom_resp" ]]; then
  pass "Prometheus responding"
else
  warn "Prometheus" "no response on :9091"
fi

grafana_resp=$(curl -sf --max-time "$TIMEOUT" -o /dev/null -w "%{http_code}" "http://${HOST}:3000/api/health" 2>/dev/null || echo "000")
if [[ "$grafana_resp" == "200" ]]; then
  pass "Grafana health"
else
  warn "Grafana" "HTTP $grafana_resp"
fi

# ---------------------------------------------------------------------------
# 9. Manager logs — recent errors
# ---------------------------------------------------------------------------
section "Manager Logs (last 50 lines)"

if command -v docker &>/dev/null; then
  error_count=$(docker logs --tail 50 p2pool-dashboard-manager-1 2>&1 | grep -c '"level":"ERROR"' 2>/dev/null || echo "0")
  warn_count=$(docker logs --tail 50 p2pool-dashboard-manager-1 2>&1 | grep -c '"level":"WARN"' 2>/dev/null || echo "0")

  if [[ "$error_count" -gt 0 ]]; then
    warn "manager errors" "$error_count ERROR entries in last 50 log lines"
    if [[ "$VERBOSE" == true ]]; then
      echo -e "    ${RED}Recent errors:${NC}"
      docker logs --tail 50 p2pool-dashboard-manager-1 2>&1 | grep '"level":"ERROR"' | tail -3
    fi
  else
    pass "no errors in recent manager logs"
  fi

  if [[ "$warn_count" -gt 0 && "$VERBOSE" == true ]]; then
    echo -e "    ${YELLOW}Recent warnings:${NC}"
    docker logs --tail 50 p2pool-dashboard-manager-1 2>&1 | grep '"level":"WARN"' | tail -3
  fi
else
  warn "manager logs" "docker CLI not available"
fi

# ---------------------------------------------------------------------------
# 10. P2Pool logs — check for sync issues
# ---------------------------------------------------------------------------
section "P2Pool Logs (last 20 lines)"

if command -v docker &>/dev/null; then
  p2pool_errors=$(docker logs --tail 20 p2pool-dashboard-p2pool-1 2>&1 | grep -ciE "error|failed|panic" 2>/dev/null || echo "0")

  if [[ "$p2pool_errors" -gt 0 ]]; then
    warn "p2pool logs" "$p2pool_errors error/warning lines"
    if [[ "$VERBOSE" == true ]]; then
      docker logs --tail 20 p2pool-dashboard-p2pool-1 2>&1 | grep -iE "error|failed|panic" | tail -5
    fi
  else
    pass "no errors in recent p2pool logs"
  fi

  # Show last few lines for context
  if [[ "$VERBOSE" == true ]]; then
    echo -e "    ${CYAN}Last 5 lines:${NC}"
    docker logs --tail 5 p2pool-dashboard-p2pool-1 2>&1 | sed 's/^/    /'
  fi
fi

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------
echo ""
echo -e "${BOLD}═══════════════════════════════════════════${NC}"
echo -e "${BOLD}  SMOKE TEST SUMMARY${NC}"
echo -e "${BOLD}═══════════════════════════════════════════${NC}"
echo -e "  ${GREEN}PASS: $PASS${NC}   ${RED}FAIL: $FAIL${NC}   ${YELLOW}WARN: $WARN${NC}"
echo ""

if [[ "$FAIL" -gt 0 ]]; then
  echo -e "${RED}Some tests failed. Review the output above.${NC}"
  exit 1
elif [[ "$WARN" -gt 0 ]]; then
  echo -e "${YELLOW}All critical tests passed, but there are warnings.${NC}"
  exit 0
else
  echo -e "${GREEN}All tests passed!${NC}"
  exit 0
fi
