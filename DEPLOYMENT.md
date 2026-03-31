# SideWatch — VPS Deployment Guide

Operator runbook for deploying and maintaining SideWatch (P2Pool Dashboard) on an Ubuntu VPS.

---

## Requirements

| Resource | Minimum | Recommended |
|----------|---------|-------------|
| RAM | 4 GB | 8 GB |
| CPU | 2 vCPU | 4 vCPU |
| Storage | 50 GB SSD | 100 GB SSD |
| OS | Ubuntu 22.04 LTS | Ubuntu 24.04 LTS |
| Bandwidth | 1 TB/mo | Unmetered |

You also need:
- A domain name with DNS pointed to the VPS IP
- SSH key access to the VPS
- A running P2Pool node and monerod (can be on the same or separate VPS)

---

## First Deploy

### 1. Provision the server

SSH in as root and run:

```bash
# Download the repo
git clone https://github.com/acquiredl/xmr-p2pool-dashboard.git /opt/p2pool-dashboard
cd /opt/p2pool-dashboard

# Provision: installs Docker, creates deploy user, configures firewall
sudo bash infra/scripts/provision.sh --deploy-user deploy --ssh-port 22
```

This installs Docker, creates a `deploy` user, enables UFW (ports 22, 80, 443), configures fail2ban, and tunes kernel parameters.

### 2. Verify deploy user access

**Before proceeding**, open a new terminal and verify:

```bash
ssh deploy@<vps-ip>
docker ps  # should work without sudo
```

### 3. Harden the server

```bash
sudo bash infra/scripts/harden.sh --ssh-port 22 --disable-root
```

This disables root SSH login, enforces key-only auth, configures Docker log rotation, creates container resource limits (`docker-compose.prod.yml`), and enables automatic security updates.

**Verify SSH still works from another terminal before closing your session.**

### 4. Generate secrets

```bash
bash infra/scripts/generate-secrets.sh
```

Creates random passwords for Postgres, JWT, and Grafana in `./secrets/`.

### 5. Configure environment

```bash
cp .env.example .env
nano .env
```

Set at minimum:
- `P2POOL_API_URL` — your P2Pool node address
- `MONEROD_URL` — your monerod RPC address
- `MONEROD_ZMQ_URL` — your monerod ZMQ address

### 6. Set up TLS

```bash
sudo bash infra/scripts/setup-tls.sh --domain pool.yourdomain.com --email you@email.com
```

Obtains a Let's Encrypt certificate, symlinks it for nginx, and configures auto-renewal.

Use `--staging` first to test without hitting rate limits.

### 7. Install systemd services

```bash
sudo bash infra/scripts/install-services.sh
```

Enables:
- `p2pool-dashboard.service` — starts the stack on boot
- `p2pool-backup.timer` — daily database backup at 04:00 UTC

### 8. Start the stack

```bash
sudo systemctl start p2pool-dashboard
```

Verify:
```bash
sudo systemctl status p2pool-dashboard
docker compose ps
curl -k https://localhost/health
```

---

## Updating / Deploying New Code

### Manual deploy

```bash
cd /opt/p2pool-dashboard
bash infra/scripts/deploy.sh --branch main
```

The deploy script:
1. Records current commit for rollback
2. Pulls latest from `origin/main`
3. Rebuilds Docker images (uses cache)
4. Restarts services
5. Verifies health endpoints
6. Rolls back automatically if healthcheck fails

### Automated deploy via GitHub Actions

Push to `main` triggers the CI/CD pipeline (`.github/workflows/deploy.yml`).

The pipeline runs in two stages:
1. **Test** — `go test -race` and `go vet` for both manager and gateway services
2. **Deploy** — SSH into VPS and run `infra/scripts/deploy.sh` (only if tests pass)

Required GitHub secrets:

| Secret | Value |
|--------|-------|
| `VPS_HOST` | Your VPS IP or hostname |
| `VPS_USER` | `deploy` |
| `VPS_SSH_KEY` | Private SSH key for deploy user |
| `VPS_SSH_PORT` | `22` (or custom port) |

Set these in: Repository Settings > Secrets and variables > Actions.

The deploy script records the current commit before pulling, so if the
healthcheck fails after deploy, it automatically rolls back to the previous
working state.

---

## Backup & Restore

### Automatic backups

Daily at 04:00 UTC via systemd timer. Backups stored in `/opt/p2pool-dashboard/backups/`.

Check timer status:
```bash
systemctl list-timers p2pool-backup.timer
```

Manual backup:
```bash
sudo systemctl start p2pool-backup.service
```

### Remote backups (S3)

Add to `.env`:
```bash
BACKUP_REMOTE_URL=s3://your-bucket/p2pool-backups
AWS_ACCESS_KEY_ID=your-key
AWS_SECRET_ACCESS_KEY=your-secret
AWS_DEFAULT_REGION=us-east-1
# For non-AWS S3 (Backblaze B2, MinIO):
# S3_ENDPOINT_URL=https://s3.us-west-000.backblazeb2.com
```

### Restore from backup

```bash
# Restore most recent local backup
bash infra/scripts/restore.sh

# Restore specific file
bash infra/scripts/restore.sh backups/p2pool_20260325_040012.dump

# Download and restore from S3
bash infra/scripts/restore.sh --from-remote
```

**Warning:** Restore drops and recreates the database. You will be prompted to confirm.

---

## Tor Hidden Service (Optional)

The Tor container starts automatically with the production stack. On first boot
it generates a v3 `.onion` address.

```bash
# Get the .onion hostname
make tor-hostname

# Or directly:
docker compose exec tor cat /var/lib/tor/hidden_service/hostname
```

The hidden service routes HTTP traffic through nginx, providing the same
dashboard over Tor. No code changes are needed — just share the `.onion` URL
with users who prefer Tor access.

To disable Tor, set `replicas: 0` for the tor service in a compose override.

---

## Subscription Wallet (Optional)

The XMR subscription system is entirely optional. To enable it:

1. Create a view-only wallet (see [docs/subscription-setup.md](docs/subscription-setup.md))
2. Deploy the wallet files to `secrets/wallet/`
3. Add `WALLET_RPC_URL=http://wallet-rpc:18088` to `.env`
4. Add the `wallet-rpc` service to your compose file (see subscription-setup.md)
5. Restart the stack

If `WALLET_RPC_URL` is not set, subscription endpoints return free-tier
defaults and the scanner does not start. The rest of the dashboard works
normally.

---

## SideWatch v1 Features (Migration 004)

Migration `004_sidewatch_v1.sql` runs automatically on startup and adds:
- Uncle tracking + software version columns on shares
- Coinbase private key column on found blocks
- Extended retention flags on subscriptions

**Data retention pruning** runs daily as part of the timeseries builder:
- Free-tier miners: data older than 30 days is deleted
- Paid-tier miners: data older than 15 months is deleted

No additional configuration is needed. The new P2Pool API fields (`uncle`,
`software_id`, `software_version`, `coinbase_private_key`) are optional —
if your P2Pool node version doesn't expose them, the columns remain NULL
and the dashboard degrades gracefully.

---

## Monitoring

### Grafana

Access at `https://pool.yourdomain.com:3000` (or via SSH tunnel).

Default credentials: `admin` / (value in `secrets/grafana_admin_password`).

Pre-configured dashboards:
- **Pool Overview** — hashrate, active miners, blocks found, API metrics, indexer health
- **Miner Detail** — per-miner hashrate, shares, payments (select miner address from dropdown)

### Prometheus

Access at `https://pool.yourdomain.com:9091` (or via SSH tunnel).

Alerts configured in `config/prometheus/alerts/pool.yml`:
- Pool hashrate drop > 50%
- No blocks found in 24h
- Indexer error rate > 0.1/s
- API p95 latency > 2s
- Too many pending blocks

### Alertmanager

Alerts route to the manager webhook by default. To enable Discord notifications, edit `config/alertmanager/alertmanager.yml` and uncomment the Discord webhook receiver.

### External health monitoring

Run on a **separate machine** (not the VPS):

```bash
# One-time check
bash infra/scripts/healthcheck.sh --url https://pool.yourdomain.com

# With Discord alerts
bash infra/scripts/healthcheck.sh \
  --url https://pool.yourdomain.com \
  --webhook https://discord.com/api/webhooks/...

# Cron (every 5 minutes)
*/5 * * * * /path/to/healthcheck.sh --url https://pool.yourdomain.com --webhook https://discord.com/api/webhooks/...
```

Checks: gateway health, API, frontend, TLS cert expiry (< 14 days warning).

---

## Ports

| Port | Service | External? |
|------|---------|-----------|
| 80 | nginx (HTTP → HTTPS redirect) | Yes |
| 443 | nginx (HTTPS) | Yes |
| 22 | SSH | Yes |
| 8080 | Gateway | Internal only |
| 8081 | Manager API | Internal only |
| 3000 | Grafana | Internal only* |
| 3001 | Frontend (Next.js) | Internal only |
| 9090 | Manager metrics | Internal only |
| 9091 | Prometheus | Internal only* |
| 9093 | Alertmanager | Internal only |
| 3100 | Loki | Internal only |

*Access via SSH tunnel: `ssh -L 3000:localhost:3000 deploy@vps-ip`

---

## Resource Limits

When `docker-compose.prod.yml` is present (created by `harden.sh`), container resource limits are enforced:

| Service | Memory Limit | CPU Limit |
|---------|-------------|-----------|
| Manager | 512 MB | 1.0 |
| Gateway | 256 MB | 0.5 |
| Postgres | 1 GB | 1.0 |
| Redis | 300 MB | 0.5 |
| Frontend | 512 MB | 0.5 |
| Nginx | 128 MB | 0.25 |
| Prometheus | 512 MB | 0.5 |
| Grafana | 512 MB | 0.5 |
| Loki | 256 MB | 0.5 |
| Promtail | 128 MB | 0.25 |
| Alertmanager | 128 MB | 0.25 |

**Total: ~3.2 GB reserved, ~4.2 GB limit.** This is why 4 GB RAM is the minimum.

---

## Troubleshooting

### Stack won't start

```bash
# Check systemd logs
journalctl -u p2pool-dashboard -n 50

# Check individual containers
docker compose ps
docker compose logs manager --tail 50
docker compose logs postgres --tail 50
```

### Postgres connection errors

```bash
# Check if postgres is healthy
docker compose exec postgres pg_isready -U manager_user

# Check secrets are mounted
docker compose exec manager cat /run/secrets/postgres_password
```

### TLS certificate issues

```bash
# Check certificate status
certbot certificates

# Force renewal
sudo certbot renew --force-renewal

# Check nginx can read certs
docker compose exec nginx ls -la /etc/nginx/certs/
```

### Out of disk space

```bash
# Check disk usage
df -h
ncdu /opt/p2pool-dashboard

# Clean Docker
docker system prune -a --volumes  # WARNING: removes unused volumes too

# Clean old backups manually
find /opt/p2pool-dashboard/backups -name "*.dump" -mtime +3 -delete
```

### High memory usage

```bash
# Check container resource usage
docker stats --no-stream

# If a container is OOM-killed, check logs
docker compose logs <service> --tail 100

# Adjust limits in docker-compose.prod.yml
```

---

## Docker Secrets

Three secrets are stored in `./secrets/` (mode 600) and mounted as Docker secrets:

| Secret | Used by | Purpose |
|--------|---------|---------|
| `postgres_password` | manager, postgres | Database authentication |
| `jwt_secret` | gateway, manager | JWT token signing |
| `grafana_admin_password` | grafana | Grafana admin login |

Generate all three with `bash infra/scripts/generate-secrets.sh`.

---

## File Reference

```
infra/
├── scripts/
│   ├── provision.sh          # Server setup (Docker, UFW, fail2ban)
│   ├── harden.sh             # SSH + Docker + resource hardening
│   ├── generate-secrets.sh   # Random secret generation
│   ├── setup-tls.sh          # Let's Encrypt certificate setup
│   ├── install-services.sh   # Systemd unit installation
│   ├── deploy.sh             # Pull + rebuild + healthcheck deploy
│   ├── pool-backup.sh        # Database backup with remote upload
│   ├── restore.sh            # Database restore from backup
│   ├── healthcheck.sh        # External uptime monitor
│   └── initdb.sql            # Postgres init
├── systemd/
│   ├── p2pool-dashboard.service  # Stack auto-start on boot
│   ├── p2pool-backup.service     # Backup job
│   └── p2pool-backup.timer       # Daily backup schedule
└── docker/
    ├── manager/Dockerfile[.dev]   # Go manager service
    ├── gateway/Dockerfile[.dev]   # Go gateway service
    ├── frontend/Dockerfile[.dev]  # Next.js frontend
    ├── tor/Dockerfile             # Tor hidden service
    └── mocknode/Dockerfile        # Mock P2Pool/monerod for tests
```
