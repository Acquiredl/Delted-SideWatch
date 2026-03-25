#!/bin/bash
# deploy.sh — Pull latest code and deploy P2Pool Dashboard
# Usage: ./deploy.sh [--branch main] [--no-rollback]
#
# Pulls latest from git, rebuilds images, restarts services, verifies health.
# Automatically rolls back on healthcheck failure unless --no-rollback is set.
set -euo pipefail

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------
INSTALL_DIR="${INSTALL_DIR:-/opt/p2pool-dashboard}"
BRANCH="${BRANCH:-main}"
ROLLBACK=true
COMPOSE_ARGS=(-f docker-compose.yml)
HEALTH_RETRIES=30
HEALTH_INTERVAL=5

while [[ $# -gt 0 ]]; do
  case "$1" in
    --branch)      BRANCH="$2";  shift 2 ;;
    --no-rollback) ROLLBACK=false; shift ;;
    --dir)         INSTALL_DIR="$2"; shift 2 ;;
    *) echo "Unknown option: $1"; exit 1 ;;
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
# Pre-flight checks
# ---------------------------------------------------------------------------
preflight() {
  cd "$INSTALL_DIR" || fatal "Install directory not found: $INSTALL_DIR"

  command -v docker &>/dev/null   || fatal "docker not found"
  command -v git &>/dev/null      || fatal "git not found"
  [[ -f "docker-compose.yml" ]]  || fatal "Compose file not found: docker-compose.yml"
  [[ -d ".git" ]]                || fatal "Not a git repository: $INSTALL_DIR"

  # Auto-detect production overlay
  if [[ -f "docker-compose.prod.yml" ]]; then
    COMPOSE_ARGS+=(-f docker-compose.prod.yml)
    info "Production overlay detected — resource limits will be applied"
  fi

  info "Deploy target: $INSTALL_DIR (branch: $BRANCH)"
}

# ---------------------------------------------------------------------------
# 1. Record current state for rollback
# ---------------------------------------------------------------------------
PREVIOUS_COMMIT=""

save_rollback_point() {
  PREVIOUS_COMMIT=$(git rev-parse HEAD)
  info "Current commit: ${PREVIOUS_COMMIT:0:12}"
}

# ---------------------------------------------------------------------------
# 2. Pull latest code
# ---------------------------------------------------------------------------
pull_latest() {
  info "Fetching latest from origin/$BRANCH..."

  # Stash any local changes (shouldn't be any on a deploy target)
  if ! git diff --quiet 2>/dev/null; then
    warn "Uncommitted changes detected — stashing"
    git stash push -m "deploy-$(date +%Y%m%d_%H%M%S)"
  fi

  git fetch origin "$BRANCH"

  local LOCAL_HEAD
  local REMOTE_HEAD
  LOCAL_HEAD=$(git rev-parse HEAD)
  REMOTE_HEAD=$(git rev-parse "origin/$BRANCH")

  if [[ "$LOCAL_HEAD" == "$REMOTE_HEAD" ]]; then
    info "Already up to date at ${LOCAL_HEAD:0:12}"
    info "Redeploying anyway (use Ctrl+C to cancel)..."
  fi

  git checkout "$BRANCH" 2>/dev/null || git checkout -b "$BRANCH" "origin/$BRANCH"
  git reset --hard "origin/$BRANCH"

  NEW_COMMIT=$(git rev-parse HEAD)
  info "Updated to: ${NEW_COMMIT:0:12}"

  # Show what changed
  if [[ "$PREVIOUS_COMMIT" != "$NEW_COMMIT" ]]; then
    echo ""
    git log --oneline "${PREVIOUS_COMMIT}..${NEW_COMMIT}" 2>/dev/null | head -20
    echo ""
  fi
}

# ---------------------------------------------------------------------------
# 3. Build and restart
# ---------------------------------------------------------------------------
build_and_restart() {
  info "Building images..."
  docker compose "${COMPOSE_ARGS[@]}" build --parallel 2>&1 | tail -5

  info "Starting services..."
  docker compose "${COMPOSE_ARGS[@]}" up -d --remove-orphans 2>&1 | tail -10

  info "Removing unused images..."
  docker image prune -f > /dev/null 2>&1 || true
}

# ---------------------------------------------------------------------------
# 4. Healthcheck
# ---------------------------------------------------------------------------
check_health() {
  info "Waiting for services to become healthy..."

  local endpoints=(
    "http://localhost:8080/health|Gateway"
    "http://localhost:8081/health|Manager"
  )

  for entry in "${endpoints[@]}"; do
    local url="${entry%%|*}"
    local name="${entry##*|}"
    local healthy=false

    for i in $(seq 1 "$HEALTH_RETRIES"); do
      if curl -sf --max-time 5 "$url" > /dev/null 2>&1; then
        info "$name healthy ($url)"
        healthy=true
        break
      fi
      sleep "$HEALTH_INTERVAL"
    done

    if [[ "$healthy" != true ]]; then
      error "$name failed healthcheck after $((HEALTH_RETRIES * HEALTH_INTERVAL))s ($url)"
      return 1
    fi
  done

  # Check all containers are running (not restarting)
  local unhealthy
  unhealthy=$(docker compose "${COMPOSE_ARGS[@]}" ps --format json 2>/dev/null | \
    jq -r 'select(.State != "running") | .Name' 2>/dev/null || true)

  if [[ -n "$unhealthy" ]]; then
    error "Unhealthy containers:"
    echo "$unhealthy"
    return 1
  fi

  info "All services healthy"
  return 0
}

# ---------------------------------------------------------------------------
# 5. Smoke tests
# ---------------------------------------------------------------------------
smoke_test() {
  info "Running post-deploy smoke tests..."

  # Check Manager /api/pool/stats returns total_hashrate
  local pool_stats
  pool_stats=$(curl -sf --max-time 10 "http://localhost:8081/api/pool/stats" 2>/dev/null || true)
  if echo "$pool_stats" | grep -q '"total_hashrate"'; then
    info "Smoke: /api/pool/stats contains total_hashrate — OK"
  else
    error "Smoke: /api/pool/stats missing total_hashrate or unreachable"
    return 1
  fi

  # Check Manager /health returns status ok
  local manager_health
  manager_health=$(curl -sf --max-time 10 "http://localhost:8081/health" 2>/dev/null || true)
  if echo "$manager_health" | grep -q '"status":"ok"'; then
    info "Smoke: Manager /health status ok — OK"
  else
    error "Smoke: Manager /health missing status ok or unreachable"
    return 1
  fi

  # Check Gateway /health returns status ok
  local gateway_health
  gateway_health=$(curl -sf --max-time 10 "http://localhost:8080/health" 2>/dev/null || true)
  if echo "$gateway_health" | grep -q '"status":"ok"'; then
    info "Smoke: Gateway /health status ok — OK"
  else
    error "Smoke: Gateway /health missing status ok or unreachable"
    return 1
  fi

  info "All smoke tests passed"
  return 0
}

# ---------------------------------------------------------------------------
# 6. Discord deploy notification
# ---------------------------------------------------------------------------
notify_discord() {
  local status="$1"
  local commit="$2"
  local branch="$3"
  local duration="$4"

  # Silently skip if webhook URL is not configured
  if [[ -z "${DISCORD_WEBHOOK_URL:-}" ]]; then
    return 0
  fi

  local color
  if [[ "$status" == "success" ]]; then
    color=65280   # 0x00ff00 green
  else
    color=16711680 # 0xff0000 red
  fi

  local short_commit="${commit:0:12}"

  local payload
  payload=$(cat <<EOJSON
{
  "embeds": [{
    "title": "P2Pool Dashboard Deploy",
    "color": $color,
    "fields": [
      { "name": "Status",   "value": "$status",        "inline": true },
      { "name": "Branch",   "value": "$branch",        "inline": true },
      { "name": "Commit",   "value": "$short_commit",  "inline": true },
      { "name": "Duration", "value": "${duration}s",   "inline": true }
    ]
  }]
}
EOJSON
)

  if curl -sf --max-time 10 -H "Content-Type: application/json" \
       -d "$payload" "$DISCORD_WEBHOOK_URL" > /dev/null 2>&1; then
    info "Discord notification sent ($status)"
  else
    warn "Discord notification failed — deploy continues"
  fi

  return 0
}

# ---------------------------------------------------------------------------
# 7. Rollback
# ---------------------------------------------------------------------------
rollback() {
  if [[ "$ROLLBACK" != true ]]; then
    error "Deploy failed — rollback disabled, manual intervention required"
    exit 1
  fi

  if [[ -z "$PREVIOUS_COMMIT" ]]; then
    fatal "No rollback point — cannot recover"
  fi

  warn "Rolling back to ${PREVIOUS_COMMIT:0:12}..."

  git reset --hard "$PREVIOUS_COMMIT"

  docker compose "${COMPOSE_ARGS[@]}" build --parallel 2>&1 | tail -5
  docker compose "${COMPOSE_ARGS[@]}" up -d --remove-orphans 2>&1 | tail -5

  # Verify rollback health
  sleep 10
  if check_health; then
    warn "Rollback successful — running on ${PREVIOUS_COMMIT:0:12}"
    warn "Investigate the failed deploy before retrying"
  else
    fatal "Rollback also failed — manual intervention required"
  fi

  exit 1
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
main() {
  local start_time
  start_time=$(date +%s)

  echo "============================================"
  echo "  P2Pool Dashboard — Deploy"
  echo "============================================"
  echo ""

  preflight
  save_rollback_point
  pull_latest
  build_and_restart

  if ! check_health; then
    local elapsed=$(( $(date +%s) - start_time ))
    error "Healthcheck failed — initiating rollback"
    notify_discord "failure" "$NEW_COMMIT" "$BRANCH" "$elapsed"
    rollback
  fi

  if ! smoke_test; then
    local elapsed=$(( $(date +%s) - start_time ))
    error "Smoke tests failed — initiating rollback"
    notify_discord "failure" "$NEW_COMMIT" "$BRANCH" "$elapsed"
    rollback
  fi

  local elapsed=$(( $(date +%s) - start_time ))
  echo ""
  echo "============================================"
  echo "  Deploy complete! (${elapsed}s)"
  echo "============================================"
  echo ""
  docker compose "${COMPOSE_ARGS[@]}" ps --format "table {{.Name}}\t{{.Status}}\t{{.Ports}}" 2>/dev/null || \
    docker compose "${COMPOSE_ARGS[@]}" ps
  echo ""

  notify_discord "success" "$NEW_COMMIT" "$BRANCH" "$elapsed"
}

main "$@"
