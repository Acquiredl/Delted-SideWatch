# P2Pool Dashboard -- Complete Deployment Guide

A self-contained guide for deploying the XMR P2Pool Dashboard from scratch.
Covers prerequisites, infrastructure, configuration, launch, verification,
ongoing operations, and troubleshooting.

---

## Table of Contents

1. [Overview](#overview)
2. [Prerequisites](#prerequisites)
3. [Infrastructure Layout](#infrastructure-layout)
4. [Phase 1: Server Provisioning](#phase-1-server-provisioning)
5. [Phase 2: Security Hardening](#phase-2-security-hardening)
6. [Phase 3: Secrets and Configuration](#phase-3-secrets-and-configuration)
7. [Phase 4: TLS Certificates](#phase-4-tls-certificates)
8. [Phase 5: First Launch](#phase-5-first-launch)
9. [Phase 6: Verification](#phase-6-verification)
10. [Phase 7: Tor Hidden Service](#phase-7-tor-hidden-service)
11. [Phase 8: Subscription System (Optional)](#phase-8-subscription-system-optional)
12. [Ongoing Operations](#ongoing-operations)
13. [CI/CD Pipeline](#cicd-pipeline)
14. [Backup and Recovery](#backup-and-recovery)
15. [Monitoring and Alerting](#monitoring-and-alerting)
16. [Resource Limits and Scaling](#resource-limits-and-scaling)
17. [Network and Ports](#network-and-ports)
18. [Troubleshooting](#troubleshooting)

---

## Overview

The P2Pool Dashboard is a read-only monitoring service for P2Pool Monero miners.
It indexes sidechain shares and on-chain coinbase payments from a P2Pool node
and a Monero full node, then presents the data through a web dashboard.

**What the dashboard does:**
- Polls the P2Pool API every 30s for sidechain shares and found blocks
- Listens to monerod via ZMQ for new Monero blocks
- Scans coinbase transactions to reconstruct per-miner payments
- Aggregates miner hashrate into 15-minute timeseries buckets
- Serves a Next.js dashboard with real-time WebSocket updates

**What it does NOT do:**
- Hold, transfer, or have access to miner funds
- Run a Stratum server (P2Pool handles this)
- Require miners to create accounts or provide email

The production stack runs 12 Docker containers behind nginx with TLS.

---

## Prerequisites

### Hardware

| Resource | Minimum | Recommended |
|----------|---------|-------------|
| RAM | 4 GB | 8 GB |
| CPU | 2 vCPU | 4 vCPU |
| Storage | 50 GB SSD | 100 GB SSD |
| Bandwidth | 1 TB/month | Unmetered |

The 4 GB minimum comes from container resource limits totaling ~3.2 GB reserved.

### Software

- **OS**: Ubuntu 22.04 LTS or 24.04 LTS (provisioning script assumes Debian/Ubuntu)
- **Docker**: Installed by provisioning script (Docker Engine + Compose plugin)
- **Git**: For cloning the repository

### External Dependencies

- **A running P2Pool node** with its HTTP API accessible (default port 3333)
- **A running monerod** with JSON-RPC (port 18081) and ZMQ (port 18083) enabled
- **A domain name** with DNS A record pointed to the VPS IP
- **SSH key access** to the VPS

P2Pool and monerod can run on the same VPS, a separate VPS, or your home
network. The dashboard connects to them over HTTP/ZMQ -- they just need to be
reachable from the Docker network.

---

## Infrastructure Layout

```
Internet
  |
  +---> nginx:443 (TLS termination, rate limit 10 req/s)
  |       +---> /api/*  --> gateway:8080 --> manager:8081
  |       +---> /ws     --> gateway:8080 (WebSocket upgrade)
  |       +---> /health --> gateway:8080
  |       +---> /       --> frontend:3000
  |
  +---> tor:9050 --> nginx:80 (Tor hidden service)

Internal Docker Network (backend):
  +-- manager:8081      Go backend (polls P2Pool, scans monerod, REST API)
  +-- gateway:8080      Go proxy (JWT auth, rate limiting)
  +-- frontend:3000     Next.js dashboard
  +-- postgres:5432     PostgreSQL 15 (indexed data)
  +-- redis:6379        Redis 7 (cache, rate limit state)
  +-- prometheus:9090   Metrics scraping (10s interval)
  +-- alertmanager:9093 Alert routing
  +-- grafana:3000      Monitoring dashboards
  +-- loki:3100         Log aggregation
  +-- promtail:9080     Log shipping (Docker socket)
  +-- tor               Hidden service (optional .onion access)
```

---

## Phase 1: Server Provisioning

SSH into your VPS as root and clone the repository:

```bash
git clone https://github.com/acquiredl/xmr-p2pool-dashboard.git /opt/p2pool-dashboard
cd /opt/p2pool-dashboard
```

Run the provisioning script:

```bash
sudo bash infra/scripts/provision.sh --deploy-user deploy --ssh-port 22
```

This script:
- Installs Docker Engine and the Compose plugin
- Creates a `deploy` user with Docker group membership
- Configures UFW firewall (allows ports 22, 80, 443 only)
- Installs and configures fail2ban (SSH: 3 attempts = 24h ban)
- Tunes kernel parameters (TCP, file descriptors, swappiness)
- Configures Docker daemon (log rotation, DNS)

**Before proceeding**, open a new terminal and verify:

```bash
ssh deploy@<vps-ip>
docker ps   # should work without sudo
```

Do not close your root session until you confirm the deploy user works.

---

## Phase 2: Security Hardening

```bash
sudo bash infra/scripts/harden.sh --ssh-port 22 --disable-root
```

This script:
- Disables root SSH login and password authentication
- Enforces SSH key-only auth (max 3 attempts)
- Disables X11 forwarding
- Configures Docker log rotation (10 MB x 3 files)
- Creates `docker-compose.prod.yml` with container memory and CPU limits
- Enables automatic security updates via `unattended-upgrades`

**Verify SSH still works from another terminal before closing your session.**
If you lock yourself out, you'll need console access from your VPS provider.

---

## Phase 3: Secrets and Configuration

### Generate secrets

```bash
bash infra/scripts/generate-secrets.sh
```

Creates three files in `./secrets/` (mode 600):
- `postgres_password` -- PostgreSQL authentication
- `jwt_secret` -- JWT token signing for the gateway
- `grafana_admin_password` -- Grafana admin login

### Configure environment

```bash
cp .env.example .env
nano .env
```

**Required settings** (everything else has sensible defaults):

| Variable | Description | Example |
|----------|-------------|---------|
| `P2POOL_API_URL` | Your P2Pool node's API endpoint | `http://10.0.0.5:3333` |
| `MONEROD_URL` | Your monerod JSON-RPC endpoint | `http://10.0.0.5:18081` |
| `MONEROD_ZMQ_URL` | Your monerod ZMQ endpoint | `tcp://10.0.0.5:18083` |

If P2Pool and monerod run on the same VPS, use the Docker host IP
(`172.17.0.1`) or add them to the Docker network.

**Optional settings:**

| Variable | Default | Description |
|----------|---------|-------------|
| `P2POOL_SIDECHAIN` | `mini` | `mini` or `main` |
| `POSTGRES_HOST` | `postgres` | Usually leave as default |
| `POSTGRES_DB` | `p2pool_dashboard` | Database name |
| `REDIS_URL` | `redis:6379` | Redis address |
| `API_PORT` | `8081` | Manager API port |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |

---

## Phase 4: TLS Certificates

```bash
sudo bash infra/scripts/setup-tls.sh --domain pool.yourdomain.com --email you@email.com
```

This obtains a Let's Encrypt certificate, symlinks it for nginx, and configures
a systemd timer for automatic renewal.

**Test first** with `--staging` to avoid hitting Let's Encrypt rate limits:

```bash
sudo bash infra/scripts/setup-tls.sh --domain pool.yourdomain.com --email you@email.com --staging
```

Once the staging cert works, run again without `--staging` for the real cert.

---

## Phase 5: First Launch

### Install systemd services

```bash
sudo bash infra/scripts/install-services.sh
```

This installs three systemd units:
- `p2pool-dashboard.service` -- starts the Docker stack on boot
- `p2pool-backup.timer` -- daily database backup at 04:00 UTC
- `p2pool-backup.service` -- the backup job itself

### Start the stack

```bash
sudo systemctl start p2pool-dashboard
```

The first start pulls and builds all Docker images. This takes a few minutes.

---

## Phase 6: Verification

```bash
# Check systemd status
sudo systemctl status p2pool-dashboard

# Check all containers are running
docker compose ps

# Test health endpoints
curl -k https://localhost/health
curl http://localhost:8081/health   # manager direct
curl http://localhost:8080/health   # gateway direct

# Test the API
curl -k https://localhost/api/pool/stats

# Check manager logs for successful P2Pool polling
docker compose logs manager --tail 20

# Check the frontend loads
curl -k -s https://localhost | head -20
```

**Expected startup timeline:**
1. PostgreSQL starts first (healthcheck: `pg_isready`)
2. Redis starts (healthcheck: `redis-cli ping`)
3. Manager starts after Postgres + Redis are healthy, runs migrations, begins polling
4. Gateway starts after manager is healthy
5. Frontend serves the Next.js build
6. Nginx starts after gateway + frontend are healthy
7. Monitoring stack (Prometheus, Grafana, Loki) starts independently

If the manager logs show P2Pool API errors, verify `P2POOL_API_URL` is
reachable from inside the Docker network:

```bash
docker compose exec manager wget -qO- http://<your-p2pool-host>:3333/api/pool/stats
```

---

## Phase 7: Tor Hidden Service

Tor starts automatically with the production stack. On first boot it generates
a v3 `.onion` address.

```bash
# Get the .onion hostname
make tor-hostname

# Check Tor container logs
docker compose logs tor
```

The `.onion` address is persisted in a Docker volume. Share it with users who
prefer Tor access -- it provides the same dashboard over HTTP through the
hidden service.

To disable Tor entirely, add a compose override:

```yaml
# docker-compose.override.yml
services:
  tor:
    deploy:
      replicas: 0
```

---

## Phase 8: Subscription System (Optional)

The XMR subscription system lets miners pay a small amount to unlock paid-tier
features (extended history, unlimited records, CSV tax export). It uses a
**view-only** wallet that can detect incoming payments but cannot spend funds.

This is entirely optional. If not configured, all miners stay on the free tier
and the dashboard works normally.

See [subscription-setup.md](subscription-setup.md) for the full setup guide covering:
1. Creating a view-only wallet
2. Deploying wallet files
3. Adding the `wallet-rpc` service
4. Configuring environment variables
5. Verification

---

## Ongoing Operations

### Updating the dashboard

**Manual deploy:**

```bash
cd /opt/p2pool-dashboard
bash infra/scripts/deploy.sh --branch main
```

The deploy script:
1. Records current commit (rollback point)
2. Pulls latest code from the specified branch
3. Rebuilds Docker images (uses layer cache)
4. Restarts changed services
5. Checks health endpoints (gateway:8080/health, manager:8081/health)
6. Rolls back automatically if healthcheck fails

**Automated deploy via CI/CD:** see [CI/CD Pipeline](#cicd-pipeline) below.

### Restarting services

```bash
# Restart the entire stack
sudo systemctl restart p2pool-dashboard

# Restart a single service
docker compose restart manager

# View real-time logs
docker compose logs -f manager gateway
```

### Database access

```bash
# Interactive SQL shell
docker compose exec postgres psql -U manager_user -d p2pool_dashboard

# Quick query
docker compose exec postgres psql -U manager_user -d p2pool_dashboard \
  -c "SELECT COUNT(*) FROM p2pool_shares;"
```

---

## CI/CD Pipeline

The GitHub Actions workflow (`.github/workflows/deploy.yml`) automates
testing and deployment on push to `main`.

### Pipeline stages

1. **Test** -- runs `go test -race ./...` and `go vet` for both manager and
   gateway services
2. **Deploy** -- SSH into the VPS and execute `infra/scripts/deploy.sh`
   (only runs if tests pass)

### Setup

Add these secrets in GitHub: Repository Settings > Secrets and variables > Actions.

| Secret | Value |
|--------|-------|
| `VPS_HOST` | VPS IP or hostname |
| `VPS_USER` | `deploy` |
| `VPS_SSH_KEY` | Private SSH key (full PEM content) |
| `VPS_SSH_PORT` | `22` (or your custom SSH port) |

### How it works

On push to `main`:
1. Tests run on GitHub's CI runners
2. If tests pass, the action SSHs into the VPS as the deploy user
3. It runs `deploy.sh` which pulls, rebuilds, and healthchecks
4. If the healthcheck fails, the deploy script rolls back to the previous commit

You can also trigger a manual deploy from the GitHub Actions tab.

---

## Backup and Recovery

### Automatic backups

The systemd timer runs `pool-backup.sh` daily at 04:00 UTC (with up to 30
minutes of randomized jitter to reduce load spikes).

```bash
# Check timer status
systemctl list-timers p2pool-backup.timer

# Trigger a backup manually
sudo systemctl start p2pool-backup.service

# View backup logs
journalctl -u p2pool-backup.service --since today
```

Backups are stored in `/opt/p2pool-dashboard/backups/` with 7-day retention.

### Remote backups

Configure S3-compatible remote backup in `.env`:

```bash
BACKUP_REMOTE_URL=s3://your-bucket/p2pool-backups
AWS_ACCESS_KEY_ID=your-key
AWS_SECRET_ACCESS_KEY=your-secret
AWS_DEFAULT_REGION=us-east-1

# For non-AWS S3 (Backblaze B2, MinIO, etc.):
# S3_ENDPOINT_URL=https://s3.us-west-000.backblazeb2.com
```

Rsync is also supported:

```bash
BACKUP_REMOTE_URL=rsync://user@backup-host:/backups/p2pool
```

### Restoring from backup

```bash
# Restore the most recent local backup
bash infra/scripts/restore.sh

# Restore a specific file
bash infra/scripts/restore.sh backups/p2pool_20260325_040012.dump

# Download and restore from S3
bash infra/scripts/restore.sh --from-remote
```

**Warning:** Restore drops and recreates the database. You will be prompted
to confirm before any data is destroyed.

### What to back up beyond the database

| Item | Location | Backup strategy |
|------|----------|----------------|
| Database | `backups/` | Automated via timer |
| Secrets | `secrets/` | Copy to secure offline storage |
| Environment | `.env` | Copy to secure offline storage |
| TLS certificates | `/etc/letsencrypt/` | Auto-renewed; backup optional |
| Tor hidden service key | Docker volume `tor-keys` | Back up if you want to preserve the .onion address |
| Subscription wallet | `secrets/wallet/` | Can be regenerated from spend key |

---

## Monitoring and Alerting

### Grafana

Access via SSH tunnel (not exposed externally):

```bash
ssh -L 3000:localhost:3000 deploy@<vps-ip>
# Then open http://localhost:3000
```

Default credentials: `admin` / (value in `secrets/grafana_admin_password`).

**Pre-configured dashboards:**

- **Pool Overview** -- pool hashrate, active miners, blocks found, payments
  recorded, pending blocks, API request rate, API latency (p50/p95/p99),
  indexer poll duration, indexer error rate
- **Miner Detail** -- per-miner hashrate, current hashrate, active shares,
  total paid, payment count, last payment, pool share percentage, share rate.
  Select a miner address from the dropdown.

### Prometheus

Access via SSH tunnel:

```bash
ssh -L 9091:localhost:9091 deploy@<vps-ip>
# Then open http://localhost:9091
```

### Alert rules

Five alerts are configured in `config/prometheus/alerts/pool.yml`:

| Alert | Severity | Trigger |
|-------|----------|---------|
| Pool hashrate drop >50% | warning | 10 minutes sustained |
| No blocks found in 24h | warning | 5 minute evaluation |
| Indexer error rate >0.1/s | critical | Immediate |
| API p95 latency >2s | warning | 5 minute evaluation |
| >20 pending blocks | warning | 5 minute evaluation |

### Alert routing

Alerts route to the manager webhook by default. To add Discord notifications:

1. Edit `config/alertmanager/alertmanager.yml`
2. Uncomment the `discord` receiver section
3. Set your Discord webhook URL
4. Restart alertmanager: `docker compose restart alertmanager`

Email routing is also available (commented out, requires SMTP config).

### External health monitoring

Run on a **separate machine** (not the dashboard VPS) for independent monitoring:

```bash
# One-time check
bash infra/scripts/healthcheck.sh --url https://pool.yourdomain.com

# With Discord alerts on failure
bash infra/scripts/healthcheck.sh \
  --url https://pool.yourdomain.com \
  --webhook https://discord.com/api/webhooks/...

# Add to cron for continuous monitoring (every 5 minutes)
*/5 * * * * /path/to/healthcheck.sh --url https://pool.yourdomain.com --webhook https://discord.com/api/webhooks/...
```

The healthcheck script verifies:
- Gateway health endpoint responds
- API returns valid data
- Frontend serves HTML
- TLS certificate has >14 days until expiry

---

## Resource Limits and Scaling

When `harden.sh` creates `docker-compose.prod.yml`, container resource limits
are enforced:

| Service | Memory Limit | Memory Reserve | CPU Limit |
|---------|-------------|----------------|-----------|
| Manager | 512 MB | 256 MB | 1.0 |
| Gateway | 256 MB | 128 MB | 0.5 |
| Postgres | 1 GB | 512 MB | 1.0 |
| Redis | 300 MB | 64 MB | 0.5 |
| Frontend | 512 MB | 256 MB | 0.5 |
| Nginx | 128 MB | 32 MB | 0.25 |
| Prometheus | 512 MB | 256 MB | 0.5 |
| Grafana | 512 MB | 128 MB | 0.5 |
| Loki | 256 MB | 128 MB | 0.5 |
| Promtail | 128 MB | 64 MB | 0.25 |
| Alertmanager | 128 MB | 64 MB | 0.25 |

**Total: ~3.2 GB reserved, ~4.2 GB limit.** This is why 4 GB RAM is the
minimum requirement.

To adjust limits, edit `docker-compose.prod.yml` and restart the stack.

### Scaling notes

- The dashboard is designed for a single P2Pool node. For multi-node setups,
  run separate dashboard instances.
- PostgreSQL is the primary bottleneck at scale. For large pools, consider
  tuning `shared_buffers`, `work_mem`, and adding indexes for common queries.
- Redis cache TTLs (15s for pool stats) can be tuned via environment variables.
- The indexer polls every 30s. This is sufficient for P2Pool mini's ~10s block
  time. Reduce the interval only if you need sub-minute data freshness.

---

## Network and Ports

| Port | Service | External? | Notes |
|------|---------|-----------|-------|
| 22 | SSH | Yes | Configurable via provision.sh |
| 80 | nginx HTTP | Yes | Redirects to 443, serves ACME challenges |
| 443 | nginx HTTPS | Yes | TLS termination, main entry point |
| 8080 | Gateway | Internal | JWT auth, rate limiting, reverse proxy |
| 8081 | Manager API | Internal | REST API + WebSocket |
| 9090 | Manager metrics | Internal | Prometheus scrape target |
| 9091 | Prometheus | Internal | Access via SSH tunnel |
| 9093 | Alertmanager | Internal | Alert routing |
| 3000 | Grafana | Internal | Access via SSH tunnel |
| 3001 | Frontend | Internal | Proxied through nginx |
| 3100 | Loki | Internal | Log aggregation |

All internal ports are isolated within the Docker bridge network and are not
exposed to the internet. Use SSH tunnels to access Grafana and Prometheus:

```bash
# Grafana
ssh -L 3000:localhost:3000 deploy@<vps-ip>

# Prometheus
ssh -L 9091:localhost:9091 deploy@<vps-ip>
```

---

## Troubleshooting

### Stack won't start

```bash
# Check systemd status
journalctl -u p2pool-dashboard -n 50

# Check container status
docker compose ps

# Check individual service logs
docker compose logs manager --tail 50
docker compose logs postgres --tail 50
docker compose logs gateway --tail 50
```

### Manager panics on startup

The manager uses `mustGetEnv()` for required variables. If a required env var
or Docker secret is missing, it panics with a clear message. Check:

```bash
docker compose logs manager | head -20
```

Common causes:
- Missing `secrets/postgres_password` -- run `generate-secrets.sh`
- Missing `.env` file -- copy from `.env.example`
- Wrong `POSTGRES_HOST` -- should be `postgres` (the Docker service name)

### PostgreSQL connection errors

```bash
# Check if postgres is running and healthy
docker compose exec postgres pg_isready -U manager_user

# Verify the password secret is mounted
docker compose exec manager cat /run/secrets/postgres_password

# Check postgres logs for auth errors
docker compose logs postgres --tail 20
```

### P2Pool API unreachable

```bash
# Test from inside the Docker network
docker compose exec manager wget -qO- http://<P2POOL_API_URL>/api/pool/stats

# Check if the P2Pool node is running
curl http://<p2pool-host>:3333/api/pool/stats
```

If P2Pool runs on the same host, use the Docker host IP (`172.17.0.1`) not
`localhost`.

### TLS certificate issues

```bash
# Check certificate status
sudo certbot certificates

# Force renewal
sudo certbot renew --force-renewal

# Check nginx can read certs
docker compose exec nginx ls -la /etc/nginx/certs/

# Check nginx logs
docker compose logs nginx --tail 20
```

### WebSocket not connecting

The gateway proxies WebSocket connections at `/ws`. Check:

```bash
# Verify nginx passes Upgrade headers
docker compose logs nginx | grep -i upgrade

# Test WebSocket connectivity
curl -i -N \
  -H "Connection: Upgrade" \
  -H "Upgrade: websocket" \
  -H "Sec-WebSocket-Version: 13" \
  -H "Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==" \
  https://pool.yourdomain.com/ws/pool/stats
```

### Stale data

The indexer polls P2Pool every 30 seconds. The aggregator caches pool stats
in Redis for 15 seconds. If data appears stale:

```bash
# Check indexer is running
docker compose logs manager | grep "indexer"

# Check Redis cache
docker compose exec redis redis-cli TTL "pool:stats"

# Check last indexed share
docker compose exec postgres psql -U manager_user -d p2pool_dashboard \
  -c "SELECT MAX(created_at) FROM p2pool_shares;"
```

### Out of disk space

```bash
# Check disk usage
df -h

# Check Docker disk usage
docker system df

# Clean unused Docker resources (images, containers, networks)
docker system prune

# Clean old backups (keeps last 3 days)
find /opt/p2pool-dashboard/backups -name "*.dump" -mtime +3 -delete
```

### High memory usage

```bash
# Check container resource usage
docker stats --no-stream

# If a container is OOM-killed
docker compose logs <service> --tail 100
dmesg | grep -i oom

# Adjust limits in docker-compose.prod.yml
nano docker-compose.prod.yml
docker compose up -d
```

### Subscription scanner not working

```bash
# Check if WALLET_RPC_URL is set
docker compose exec manager printenv WALLET_RPC_URL

# Check scanner logs
docker compose logs manager | grep "subscription"

# Verify wallet-rpc is responding
curl -s http://localhost:18088/json_rpc \
  -d '{"jsonrpc":"2.0","id":"0","method":"get_height"}' \
  -H 'Content-Type: application/json'
```

If the wallet is still syncing, the scanner will log errors until sync
completes. This is expected on first deploy.
