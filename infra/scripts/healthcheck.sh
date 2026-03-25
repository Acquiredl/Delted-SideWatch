#!/bin/bash
# healthcheck.sh — External health monitor for P2Pool Dashboard
# Usage: ./healthcheck.sh [--url https://pool.example.com] [--webhook DISCORD_URL]
#
# Designed to run on a SEPARATE machine (not the VPS itself) via cron.
# Checks HTTP endpoints and sends alerts via Discord/Slack webhook or email.
#
# Cron example (every 5 minutes):
#   */5 * * * * /path/to/healthcheck.sh --url https://pool.example.com --webhook https://discord.com/api/webhooks/...
set -euo pipefail

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------
BASE_URL="${HEALTH_CHECK_URL:-}"
WEBHOOK_URL="${HEALTH_WEBHOOK_URL:-}"
ALERT_EMAIL="${HEALTH_ALERT_EMAIL:-}"
TIMEOUT=10
STATE_FILE="/tmp/p2pool-health-state"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --url)     BASE_URL="$2";    shift 2 ;;
    --webhook) WEBHOOK_URL="$2"; shift 2 ;;
    --email)   ALERT_EMAIL="$2"; shift 2 ;;
    *) echo "Unknown option: $1"; exit 1 ;;
  esac
done

[[ -n "$BASE_URL" ]] || { echo "Usage: $0 --url https://pool.example.com"; exit 1; }

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------
FAILURES=()
TIMESTAMP=$(date -u +"%Y-%m-%d %H:%M:%S UTC")

check_endpoint() {
  local name="$1"
  local url="$2"
  local expected_code="${3:-200}"

  local http_code
  http_code=$(curl -sf -o /dev/null -w "%{http_code}" --max-time "$TIMEOUT" "$url" 2>/dev/null || echo "000")

  if [[ "$http_code" == "$expected_code" ]]; then
    return 0
  else
    FAILURES+=("$name: HTTP $http_code (expected $expected_code)")
    return 1
  fi
}

# ---------------------------------------------------------------------------
# Check endpoints
# ---------------------------------------------------------------------------
check_endpoint "Gateway Health" "${BASE_URL}/health"
check_endpoint "API Pool Stats" "${BASE_URL}/api/pool/stats"
check_endpoint "Frontend"       "${BASE_URL}/"

# ---------------------------------------------------------------------------
# TLS certificate expiry check
# ---------------------------------------------------------------------------
check_tls_expiry() {
  local domain
  domain=$(echo "$BASE_URL" | sed 's|https://||;s|/.*||')

  local expiry
  expiry=$(echo | openssl s_client -servername "$domain" -connect "$domain:443" 2>/dev/null | \
    openssl x509 -noout -enddate 2>/dev/null | cut -d= -f2)

  if [[ -n "$expiry" ]]; then
    local expiry_epoch
    local now_epoch
    expiry_epoch=$(date -d "$expiry" +%s 2>/dev/null || date -jf "%b %d %T %Y %Z" "$expiry" +%s 2>/dev/null || echo "0")
    now_epoch=$(date +%s)

    if [[ "$expiry_epoch" -eq 0 ]]; then
      FAILURES+=("TLS cert expiry check failed — could not parse date: $expiry")
      return
    fi

    local days_left=$(( (expiry_epoch - now_epoch) / 86400 ))

    if [[ "$days_left" -lt 14 ]]; then
      FAILURES+=("TLS cert expires in $days_left days ($expiry)")
    fi
  fi
}

# Only check TLS for HTTPS URLs
if [[ "$BASE_URL" == https://* ]]; then
  check_tls_expiry
fi

# ---------------------------------------------------------------------------
# Alert on failure
# ---------------------------------------------------------------------------
send_alert() {
  local message="$1"

  # Discord / Slack webhook
  if [[ -n "$WEBHOOK_URL" ]]; then
    local payload
    payload=$(jq -n --arg content "$message" '{"content": $content}')
    curl -sf -X POST -H "Content-Type: application/json" -d "$payload" "$WEBHOOK_URL" > /dev/null 2>&1 || true
  fi

  # Email (requires mailutils or sendmail)
  if [[ -n "$ALERT_EMAIL" ]] && command -v mail &>/dev/null; then
    echo "$message" | mail -s "P2Pool Dashboard Alert" "$ALERT_EMAIL" 2>/dev/null || true
  fi

  echo "$message"
}

send_recovery() {
  local message="[RECOVERED] P2Pool Dashboard is back online at $TIMESTAMP"

  if [[ -n "$WEBHOOK_URL" ]]; then
    local payload
    payload=$(jq -n --arg content "$message" '{"content": $content}')
    curl -sf -X POST -H "Content-Type: application/json" -d "$payload" "$WEBHOOK_URL" > /dev/null 2>&1 || true
  fi

  if [[ -n "$ALERT_EMAIL" ]] && command -v mail &>/dev/null; then
    echo "$message" | mail -s "P2Pool Dashboard Recovered" "$ALERT_EMAIL" 2>/dev/null || true
  fi

  echo "$message"
}

# ---------------------------------------------------------------------------
# State tracking (avoid spam)
# ---------------------------------------------------------------------------
WAS_DOWN=false
if [[ -f "$STATE_FILE" ]]; then
  WAS_DOWN=true
fi

if [[ ${#FAILURES[@]} -gt 0 ]]; then
  # Build failure message
  ALERT_MSG="[ALERT] P2Pool Dashboard issues at $TIMESTAMP"$'\n'
  for f in "${FAILURES[@]}"; do
    ALERT_MSG+="  - $f"$'\n'
  done
  ALERT_MSG+="URL: $BASE_URL"

  if [[ "$WAS_DOWN" != true ]]; then
    # First failure — send alert and create state file
    send_alert "$ALERT_MSG"
    echo "$TIMESTAMP" > "$STATE_FILE"
  fi
  # If already down, don't spam — alert was already sent

  exit 1
else
  if [[ "$WAS_DOWN" == true ]]; then
    # Was down, now recovered — send recovery
    send_recovery
    rm -f "$STATE_FILE"
  fi

  exit 0
fi
