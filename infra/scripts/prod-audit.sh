#!/bin/bash
# prod-audit.sh — Production hygiene audit for SideWatch VPS
# Run ON the VPS to check for residual data, misconfigurations, and stale state.
#
# Usage:
#   ./prod-audit.sh                 # full audit
#   ./prod-audit.sh --fix           # audit + apply safe automated fixes
#   ./prod-audit.sh --verbose       # show detailed output
set -euo pipefail

# ---------------------------------------------------------------------------
# Config
# ---------------------------------------------------------------------------
INSTALL_DIR="${INSTALL_DIR:-/opt/p2pool-dashboard}"
FIX=false
VERBOSE=false

while [[ $# -gt 0 ]]; do
  case "$1" in
    --fix)     FIX=true;     shift ;;
    --verbose) VERBOSE=true; shift ;;
    --dir)     INSTALL_DIR="$2"; shift 2 ;;
    *) echo "Unknown option: $1"; exit 1 ;;
  esac
done

cd "$INSTALL_DIR" || { echo "ERROR: $INSTALL_DIR not found"; exit 1; }

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
CYAN='\033[0;36m'
NC='\033[0m'
BOLD='\033[1m'

PASS=0; FAIL=0; WARN=0; FIXED=0

pass()  { PASS=$((PASS + 1));  echo -e "  ${GREEN}OK${NC}    $1"; }
fail()  { FAIL=$((FAIL + 1));  echo -e "  ${RED}FAIL${NC}  $1  — $2"; }
warn()  { WARN=$((WARN + 1));  echo -e "  ${YELLOW}WARN${NC}  $1  — $2"; }
fixed() { FIXED=$((FIXED + 1)); echo -e "  ${GREEN}FIX${NC}   $1"; }

section() {
  echo ""
  echo -e "${BOLD}${CYAN}[$1]${NC}"
}

# ---------------------------------------------------------------------------
# 1. Docker cleanup — residual images, containers, build cache
# ---------------------------------------------------------------------------
section "Docker Cleanup"

# Dangling images
dangling=$(docker images -f "dangling=true" -q 2>/dev/null | wc -l)
if [[ "$dangling" -gt 0 ]]; then
  warn "dangling images" "$dangling found"
  if [[ "$FIX" == true ]]; then
    docker image prune -f > /dev/null 2>&1
    fixed "removed $dangling dangling images"
  fi
else
  pass "no dangling images"
fi

# Stopped containers
stopped=$(docker ps -a -f "status=exited" -q 2>/dev/null | wc -l)
if [[ "$stopped" -gt 0 ]]; then
  warn "stopped containers" "$stopped found"
  if [[ "$FIX" == true ]]; then
    docker container prune -f > /dev/null 2>&1
    fixed "removed $stopped stopped containers"
  fi
else
  pass "no stopped containers"
fi

# Build cache size
cache_size=$(docker system df --format '{{.Size}}' 2>/dev/null | tail -1)
if [[ -n "$cache_size" ]]; then
  echo -e "  ${CYAN}INFO${NC}  Docker build cache: $cache_size"
fi

# Unused volumes
unused_vols=$(docker volume ls -f "dangling=true" -q 2>/dev/null | wc -l)
if [[ "$unused_vols" -gt 0 ]]; then
  warn "unused volumes" "$unused_vols found (review before removing)"
  if [[ "$VERBOSE" == true ]]; then
    docker volume ls -f "dangling=true" 2>/dev/null
  fi
else
  pass "no unused volumes"
fi

# ---------------------------------------------------------------------------
# 2. Secrets & permissions
# ---------------------------------------------------------------------------
section "Secrets & Permissions"

# Check secrets directory
if [[ -d "$INSTALL_DIR/secrets" ]]; then
  secrets_perm=$(stat -c '%a' "$INSTALL_DIR/secrets" 2>/dev/null || stat -f '%A' "$INSTALL_DIR/secrets" 2>/dev/null)
  if [[ "$secrets_perm" == "700" ]]; then
    pass "secrets/ directory permissions: 700"
  else
    fail "secrets/ permissions" "got $secrets_perm, expected 700"
    if [[ "$FIX" == true ]]; then
      chmod 700 "$INSTALL_DIR/secrets"
      fixed "set secrets/ to 700"
    fi
  fi

  # Check each secret file
  for secret_file in postgres_password jwt_secret grafana_admin_password admin_token; do
    if [[ -f "$INSTALL_DIR/secrets/$secret_file" ]]; then
      file_perm=$(stat -c '%a' "$INSTALL_DIR/secrets/$secret_file" 2>/dev/null || stat -f '%A' "$INSTALL_DIR/secrets/$secret_file" 2>/dev/null)
      if [[ "$file_perm" == "600" ]]; then
        pass "$secret_file: permissions 600"
      else
        fail "$secret_file" "permissions $file_perm, expected 600"
        if [[ "$FIX" == true ]]; then
          chmod 600 "$INSTALL_DIR/secrets/$secret_file"
          fixed "set $secret_file to 600"
        fi
      fi
    else
      fail "$secret_file" "file missing — run generate-secrets.sh"
    fi
  done
else
  fail "secrets directory" "not found"
fi

# Check .env permissions
if [[ -f "$INSTALL_DIR/.env" ]]; then
  env_perm=$(stat -c '%a' "$INSTALL_DIR/.env" 2>/dev/null || stat -f '%A' "$INSTALL_DIR/.env" 2>/dev/null)
  if [[ "$env_perm" == "600" ]]; then
    pass ".env file permissions: 600"
  else
    warn ".env permissions" "got $env_perm, expected 600"
    if [[ "$FIX" == true ]]; then
      chmod 600 "$INSTALL_DIR/.env"
      fixed "set .env to 600"
    fi
  fi
else
  warn ".env" "file not found"
fi

# Check .env not tracked by git
if git ls-files --error-unmatch .env > /dev/null 2>&1; then
  fail ".env in git" "CRITICAL: .env is tracked by git — remove immediately"
else
  pass ".env not tracked by git"
fi

# Check secrets not tracked by git
if git ls-files --error-unmatch secrets/ > /dev/null 2>&1; then
  fail "secrets in git" "CRITICAL: secrets/ is tracked by git"
else
  pass "secrets/ not tracked by git"
fi

# ---------------------------------------------------------------------------
# 3. Environment variable audit
# ---------------------------------------------------------------------------
section "Environment Config"

if [[ -f "$INSTALL_DIR/.env" ]]; then
  # Check LOG_LEVEL is not debug
  log_level=$(grep -E '^LOG_LEVEL=' "$INSTALL_DIR/.env" 2>/dev/null | cut -d= -f2 | tr -d '"' || echo "")
  if [[ -z "$log_level" || "$log_level" == "info" ]]; then
    pass "LOG_LEVEL: ${log_level:-info (default)}"
  elif [[ "$log_level" == "debug" ]]; then
    fail "LOG_LEVEL" "set to 'debug' in production — exposes sensitive data"
    if [[ "$FIX" == true ]]; then
      sed -i 's/^LOG_LEVEL=debug/LOG_LEVEL=info/' "$INSTALL_DIR/.env"
      fixed "set LOG_LEVEL=info"
    fi
  else
    pass "LOG_LEVEL: $log_level"
  fi

  # Check ADMIN_TOKEN is set
  admin_token=$(grep -E '^ADMIN_TOKEN=' "$INSTALL_DIR/.env" 2>/dev/null | cut -d= -f2 | tr -d '"' || echo "")
  if [[ -n "$admin_token" && ${#admin_token} -ge 32 ]]; then
    pass "ADMIN_TOKEN set (${#admin_token} chars)"
  elif [[ -n "$admin_token" ]]; then
    warn "ADMIN_TOKEN" "only ${#admin_token} chars — recommend 48+"
  else
    fail "ADMIN_TOKEN" "not set in .env"
  fi

  # Verify ADMIN_TOKEN matches secrets/admin_token
  if [[ -f "$INSTALL_DIR/secrets/admin_token" && -n "$admin_token" ]]; then
    secret_token=$(cat "$INSTALL_DIR/secrets/admin_token" 2>/dev/null | tr -d '\n')
    if [[ "$admin_token" == "$secret_token" ]]; then
      pass "ADMIN_TOKEN matches secrets/admin_token"
    else
      fail "ADMIN_TOKEN mismatch" ".env and secrets/admin_token have different values"
    fi
  fi
fi

# ---------------------------------------------------------------------------
# 4. Docker log rotation
# ---------------------------------------------------------------------------
section "Docker Daemon Config"

if [[ -f /etc/docker/daemon.json ]]; then
  log_driver=$(jq -r '."log-driver" // "not set"' /etc/docker/daemon.json 2>/dev/null)
  max_size=$(jq -r '."log-opts"."max-size" // "not set"' /etc/docker/daemon.json 2>/dev/null)
  max_file=$(jq -r '."log-opts"."max-file" // "not set"' /etc/docker/daemon.json 2>/dev/null)

  if [[ "$log_driver" == "json-file" && "$max_size" != "not set" ]]; then
    pass "log rotation: $log_driver, max-size=$max_size, max-file=$max_file"
  else
    warn "log rotation" "log-driver=$log_driver, max-size=$max_size — may need configuration"
  fi

  no_priv=$(jq -r '."no-new-privileges" // "not set"' /etc/docker/daemon.json 2>/dev/null)
  if [[ "$no_priv" == "true" ]]; then
    pass "no-new-privileges: enabled"
  else
    warn "no-new-privileges" "not enabled in daemon.json"
  fi
else
  fail "daemon.json" "not found — run harden.sh"
fi

# ---------------------------------------------------------------------------
# 5. Monitoring port exposure
# ---------------------------------------------------------------------------
section "Port Exposure"

check_port_binding() {
  local port="$1"
  local service="$2"

  # Check if port is bound to 0.0.0.0 (publicly accessible)
  if ss -tlnp 2>/dev/null | grep -q "0.0.0.0:$port"; then
    fail "$service (port $port)" "bound to 0.0.0.0 — publicly accessible"
  elif ss -tlnp 2>/dev/null | grep -q "127.0.0.1:$port"; then
    pass "$service (port $port): localhost only"
  elif ss -tlnp 2>/dev/null | grep -q ":$port"; then
    warn "$service (port $port)" "check binding — may be exposed"
  else
    echo -e "  ${CYAN}INFO${NC}  $service (port $port): not listening"
  fi
}

check_port_binding 3000 "Grafana"
check_port_binding 9091 "Prometheus"
check_port_binding 9093 "Alertmanager"
check_port_binding 3100 "Loki"
check_port_binding 9090 "Manager metrics"

# These SHOULD be publicly accessible
for pub_port in 80 443 8080 8081 3333; do
  if ss -tlnp 2>/dev/null | grep -q ":$pub_port"; then
    echo -e "  ${CYAN}INFO${NC}  Port $pub_port: listening (expected)"
  fi
done

# ---------------------------------------------------------------------------
# 6. Firewall
# ---------------------------------------------------------------------------
section "Firewall"

if command -v ufw &>/dev/null; then
  ufw_status=$(ufw status 2>/dev/null | head -1)
  if echo "$ufw_status" | grep -qi "active"; then
    pass "UFW: active"

    # Check that monitoring ports are NOT in the allow list
    for blocked_port in 3000 3100 9090 9091 9093; do
      if ufw status 2>/dev/null | grep -q "$blocked_port.*ALLOW"; then
        warn "UFW port $blocked_port" "allowed through firewall — monitoring port should be blocked"
      fi
    done
  else
    fail "UFW" "not active"
  fi
else
  warn "UFW" "not installed"
fi

if command -v fail2ban-client &>/dev/null; then
  if fail2ban-client status sshd > /dev/null 2>&1; then
    banned=$(fail2ban-client status sshd 2>/dev/null | grep "Currently banned" | awk '{print $NF}')
    total=$(fail2ban-client status sshd 2>/dev/null | grep "Total banned" | awk '{print $NF}')
    pass "fail2ban SSH: $banned currently banned, $total total"
  else
    warn "fail2ban" "sshd jail not running"
  fi
else
  warn "fail2ban" "not installed"
fi

# ---------------------------------------------------------------------------
# 7. Database health
# ---------------------------------------------------------------------------
section "Database"

# Table sizes
db_output=$(docker compose exec -T postgres psql -U manager_user -d p2pool_dashboard -t -A -c "
  SELECT relname, pg_size_pretty(pg_total_relation_size(relid)), n_live_tup
  FROM pg_stat_user_tables ORDER BY pg_total_relation_size(relid) DESC;
" 2>/dev/null || echo "FAILED")

if [[ "$db_output" != "FAILED" && -n "$db_output" ]]; then
  pass "database accessible"
  if [[ "$VERBOSE" == true ]]; then
    echo -e "    ${CYAN}Table sizes:${NC}"
    echo "$db_output" | while IFS='|' read -r table size rows; do
      printf "      %-30s %10s  %s rows\n" "$table" "$size" "$rows"
    done
  fi
else
  fail "database" "could not connect"
fi

# Dead tuples (needs vacuum?)
dead_tuples=$(docker compose exec -T postgres psql -U manager_user -d p2pool_dashboard -t -A -c "
  SELECT relname, n_dead_tup FROM pg_stat_user_tables WHERE n_dead_tup > 1000 ORDER BY n_dead_tup DESC;
" 2>/dev/null || echo "")

if [[ -n "$dead_tuples" ]]; then
  warn "dead tuples" "tables with >1000 dead rows:"
  echo "$dead_tuples" | while IFS='|' read -r table dead; do
    echo -e "    $table: $dead dead tuples"
  done
  if [[ "$FIX" == true ]]; then
    docker compose exec -T postgres psql -U manager_user -d p2pool_dashboard -c "VACUUM ANALYZE;" > /dev/null 2>&1
    fixed "ran VACUUM ANALYZE"
  fi
else
  pass "no significant dead tuple accumulation"
fi

# Check for test/orphan subscription data
sub_count=$(docker compose exec -T postgres psql -U manager_user -d p2pool_dashboard -t -A -c "
  SELECT COUNT(*) FROM subscriptions;
" 2>/dev/null || echo "0")
echo -e "  ${CYAN}INFO${NC}  subscriptions table: $sub_count rows (review manually if unexpected)"

# ---------------------------------------------------------------------------
# 8. Redis health
# ---------------------------------------------------------------------------
section "Redis"

redis_info=$(docker compose exec -T redis redis-cli INFO memory 2>/dev/null || echo "FAILED")
if [[ "$redis_info" != "FAILED" ]]; then
  used_mem=$(echo "$redis_info" | grep "used_memory_human:" | cut -d: -f2 | tr -d '\r')
  max_mem=$(echo "$redis_info" | grep "maxmemory_human:" | cut -d: -f2 | tr -d '\r')
  evicted=$(echo "$redis_info" | grep "evicted_keys:" | cut -d: -f2 | tr -d '\r' || echo "0")
  pass "redis: used=$used_mem, max=$max_mem, evicted=$evicted"

  db_size=$(docker compose exec -T redis redis-cli DBSIZE 2>/dev/null | tr -d '\r')
  echo -e "  ${CYAN}INFO${NC}  $db_size"
else
  fail "redis" "could not connect"
fi

# ---------------------------------------------------------------------------
# 9. Disk space
# ---------------------------------------------------------------------------
section "Disk Space"

# Overall disk
root_usage=$(df -h / 2>/dev/null | awk 'NR==2{print $5}' | tr -d '%')
root_avail=$(df -h / 2>/dev/null | awk 'NR==2{print $4}')
if [[ -n "$root_usage" ]]; then
  if [[ "$root_usage" -lt 70 ]]; then
    pass "root filesystem: ${root_usage}% used ($root_avail available)"
  elif [[ "$root_usage" -lt 85 ]]; then
    warn "root filesystem" "${root_usage}% used ($root_avail available)"
  else
    fail "root filesystem" "${root_usage}% used — critically low ($root_avail available)"
  fi
fi

# Docker volumes
echo -e "  ${CYAN}INFO${NC}  Docker disk usage:"
docker system df 2>/dev/null | sed 's/^/    /'

# ---------------------------------------------------------------------------
# 10. Backup status
# ---------------------------------------------------------------------------
section "Backups"

if systemctl is-active p2pool-backup.timer > /dev/null 2>&1; then
  next_run=$(systemctl list-timers p2pool-backup.timer --no-pager 2>/dev/null | grep backup | awk '{print $1, $2}')
  pass "backup timer active — next: $next_run"
elif systemctl is-enabled p2pool-backup.timer > /dev/null 2>&1; then
  warn "backup timer" "enabled but not active"
else
  warn "backup timer" "not installed — run install-services.sh"
fi

# Check backup directory
backup_dir="$INSTALL_DIR/backups"
if [[ -d "$backup_dir" ]]; then
  backup_count=$(find "$backup_dir" -name "*.dump" -type f 2>/dev/null | wc -l)
  if [[ "$backup_count" -gt 0 ]]; then
    latest=$(find "$backup_dir" -name "*.dump" -type f -printf '%T@ %p\n' 2>/dev/null | sort -rn | head -1 | awk '{print $2}')
    latest_age=$(( ( $(date +%s) - $(stat -c %Y "$latest" 2>/dev/null || echo 0) ) / 3600 ))
    if [[ "$latest_age" -lt 48 ]]; then
      pass "latest backup: $(basename "$latest") (${latest_age}h ago)"
    else
      warn "latest backup" "$(basename "$latest") is ${latest_age}h old"
    fi
  else
    warn "backups" "no .dump files found in $backup_dir"
  fi
else
  warn "backup directory" "$backup_dir not found"
fi

# ---------------------------------------------------------------------------
# 11. TLS & auto-renewal
# ---------------------------------------------------------------------------
section "TLS"

if [[ -f "$INSTALL_DIR/certs/fullchain.pem" ]]; then
  expiry=$(openssl x509 -enddate -noout -in "$INSTALL_DIR/certs/fullchain.pem" 2>/dev/null | cut -d= -f2)
  if [[ -n "$expiry" ]]; then
    expiry_epoch=$(date -d "$expiry" +%s 2>/dev/null || echo "0")
    now_epoch=$(date +%s)
    days_left=$(( (expiry_epoch - now_epoch) / 86400 ))

    if [[ "$days_left" -gt 30 ]]; then
      pass "TLS cert: $days_left days remaining (expires: $expiry)"
    elif [[ "$days_left" -gt 7 ]]; then
      warn "TLS cert" "only $days_left days remaining — check auto-renewal"
    else
      fail "TLS cert" "expires in $days_left days — renew immediately"
    fi
  fi
else
  warn "TLS cert" "not found at $INSTALL_DIR/certs/fullchain.pem"
fi

# Check certbot timer
if systemctl is-active certbot.timer > /dev/null 2>&1 || \
   systemctl is-active snap.certbot.renew.timer > /dev/null 2>&1; then
  pass "certbot auto-renewal timer active"
else
  warn "certbot timer" "not active — TLS certs may not auto-renew"
fi

# ---------------------------------------------------------------------------
# 12. Monitoring stack health
# ---------------------------------------------------------------------------
section "Monitoring Stack"

# Prometheus targets
prom_targets=$(curl -sf --max-time 5 "http://127.0.0.1:9091/api/v1/targets" 2>/dev/null)
if [[ -n "$prom_targets" ]]; then
  up_count=$(echo "$prom_targets" | python3 -c "
import sys, json
d = json.load(sys.stdin)
targets = d.get('data', {}).get('activeTargets', [])
up = sum(1 for t in targets if t.get('health') == 'up')
total = len(targets)
print(f'{up}/{total}')
" 2>/dev/null || echo "parse error")
  pass "Prometheus targets: $up_count up"
else
  warn "Prometheus" "not responding on 127.0.0.1:9091"
fi

# Prometheus storage
prom_storage=$(curl -sf --max-time 5 "http://127.0.0.1:9091/api/v1/status/tsdb" 2>/dev/null)
if [[ -n "$prom_storage" && "$VERBOSE" == true ]]; then
  echo -e "    ${CYAN}TSDB status available — use --verbose for details${NC}"
fi

# Grafana health
grafana_health=$(curl -sf --max-time 5 "http://127.0.0.1:3000/api/health" 2>/dev/null)
if [[ -n "$grafana_health" ]]; then
  pass "Grafana responding"
else
  warn "Grafana" "not responding on 127.0.0.1:3000"
fi

# Grafana datasources
grafana_pw=""
if [[ -f "$INSTALL_DIR/secrets/grafana_admin_password" ]]; then
  grafana_pw=$(cat "$INSTALL_DIR/secrets/grafana_admin_password" 2>/dev/null | tr -d '\n')
fi
if [[ -n "$grafana_pw" ]]; then
  ds_count=$(curl -sf --max-time 5 -u "admin:$grafana_pw" "http://127.0.0.1:3000/api/datasources" 2>/dev/null | python3 -c "import sys,json; print(len(json.load(sys.stdin)))" 2>/dev/null || echo "0")
  if [[ "$ds_count" -gt 0 ]]; then
    pass "Grafana datasources: $ds_count configured"
  else
    warn "Grafana datasources" "none found"
  fi

  # Check dashboards
  dash_count=$(curl -sf --max-time 5 -u "admin:$grafana_pw" "http://127.0.0.1:3000/api/search" 2>/dev/null | python3 -c "import sys,json; print(len(json.load(sys.stdin)))" 2>/dev/null || echo "0")
  pass "Grafana dashboards: $dash_count provisioned"
fi

# Alertmanager
am_status=$(curl -sf --max-time 5 "http://127.0.0.1:9093/api/v2/status" 2>/dev/null)
if [[ -n "$am_status" ]]; then
  pass "Alertmanager responding"
else
  warn "Alertmanager" "not responding on 127.0.0.1:9093"
fi

# Loki
loki_ready=$(curl -sf --max-time 5 "http://127.0.0.1:3100/ready" 2>/dev/null)
if [[ "$loki_ready" == "ready" ]]; then
  pass "Loki ready"
else
  warn "Loki" "not ready on 127.0.0.1:3100"
fi

# ---------------------------------------------------------------------------
# 13. SSH & auto-updates
# ---------------------------------------------------------------------------
section "System Security"

# SSH config
if [[ -f /etc/ssh/sshd_config.d/99-p2pool-hardening.conf ]]; then
  pass "SSH hardening config present"
  pw_auth=$(grep "PasswordAuthentication" /etc/ssh/sshd_config.d/99-p2pool-hardening.conf 2>/dev/null | awk '{print $2}')
  if [[ "$pw_auth" == "no" ]]; then
    pass "SSH: password auth disabled"
  else
    warn "SSH" "password auth may be enabled"
  fi
else
  warn "SSH hardening" "config not found — run harden.sh"
fi

# Auto-updates
if [[ -f /etc/apt/apt.conf.d/50unattended-upgrades ]]; then
  pass "unattended-upgrades configured"
else
  warn "auto-updates" "unattended-upgrades not configured"
fi

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------
echo ""
echo -e "${BOLD}═══════════════════════════════════════════${NC}"
echo -e "${BOLD}  PRODUCTION AUDIT SUMMARY${NC}"
echo -e "${BOLD}═══════════════════════════════════════════${NC}"
echo -e "  ${GREEN}PASS: $PASS${NC}   ${RED}FAIL: $FAIL${NC}   ${YELLOW}WARN: $WARN${NC}   ${GREEN}FIXED: $FIXED${NC}"
echo ""

if [[ "$FAIL" -gt 0 ]]; then
  echo -e "${RED}Critical issues found. Address FAIL items before serving traffic.${NC}"
  exit 1
elif [[ "$WARN" -gt 0 ]]; then
  echo -e "${YELLOW}No critical issues, but review warnings.${NC}"
  if [[ "$FIX" != true ]]; then
    echo -e "Run with ${BOLD}--fix${NC} to auto-remediate safe issues."
  fi
  exit 0
else
  echo -e "${GREEN}All checks passed. Production hygiene is good.${NC}"
  exit 0
fi
