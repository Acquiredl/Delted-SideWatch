# Infra

Dockerfiles, deployment scripts, and systemd units for the P2Pool Dashboard.

## Dockerfiles

| File | Base | Description |
|---|---|---|
| `docker/manager/Dockerfile` | `golang:1.22-alpine` → `alpine:3.19` | Production manager binary (non-root `appuser:1000`) |
| `docker/manager/Dockerfile.dev` | `golang:1.22-alpine` + air | Development with hot reload |
| `docker/gateway/Dockerfile` | `golang:1.22-alpine` → `alpine:3.19` | Production gateway binary (non-root `appuser:1000`) |
| `docker/gateway/Dockerfile.dev` | `golang:1.22-alpine` + air | Development with hot reload |
| `docker/frontend/Dockerfile` | `node:18-alpine` (3-stage) | Production Next.js build (non-root `nextjs:1001`) |
| `docker/frontend/Dockerfile.dev` | `node:18-alpine` | Development with npm dev server |
| `docker/tor/Dockerfile` | `alpine:3.19` | Tor hidden service v3 (non-root `tor` user) |
| `docker/mocknode/Dockerfile` | `golang:1.22-alpine` | Mock P2Pool + monerod for testing |

All production images use Alpine with a non-root `USER` directive.

## Scripts

| Script | Description |
|---|---|
| `scripts/provision.sh` | VPS base setup — installs Docker, creates deploy user, configures UFW + fail2ban, tunes kernel |
| `scripts/harden.sh` | Security hardening — SSH key-only auth, Docker log rotation, container resource limits, auto-updates |
| `scripts/generate-secrets.sh` | Generates random secrets for Postgres, JWT, and Grafana in `./secrets/` |
| `scripts/setup-tls.sh` | Obtains Let's Encrypt certificates via certbot, configures auto-renewal |
| `scripts/deploy.sh` | Pull + rebuild + healthcheck deploy with automatic rollback on failure |
| `scripts/install-services.sh` | Installs systemd units for auto-start and scheduled backups |
| `scripts/pool-backup.sh` | Database backup (pg_dump) with optional S3/rsync remote upload |
| `scripts/restore.sh` | Restores database from local or remote backup |
| `scripts/healthcheck.sh` | External uptime monitor — checks endpoints, TLS expiry, sends Discord/email alerts |
| `scripts/initdb.sql` | Postgres initialization — grants schema permissions to `manager_user` |

## Systemd Units

| Unit | Description |
|---|---|
| `systemd/p2pool-dashboard.service` | Starts the Docker Compose stack on boot (oneshot + RemainAfterExit) |
| `systemd/p2pool-backup.service` | One-shot backup job, triggered by timer |
| `systemd/p2pool-backup.timer` | Daily backup at 04:00 UTC with 30-minute randomized jitter |

## Docker Compose Files

| File | Purpose |
|---|---|
| `docker-compose.yml` | Production stack — all 12 services including Tor |
| `docker-compose.dev.yml` | Dev overlay — hot reload, monitoring disabled, DB ports exposed |
| `docker-compose.test.yml` | Test overlay — mocknode replaces real P2Pool/monerod |

## Usage

```bash
# Production
docker compose up -d

# Development (hot reload, no monitoring)
make dev

# Run tests with mocknode
docker compose -f docker-compose.yml -f docker-compose.test.yml up --build

# Deploy to VPS (first time)
sudo bash infra/scripts/provision.sh --deploy-user deploy --ssh-port 22
sudo bash infra/scripts/harden.sh --ssh-port 22 --disable-root
bash infra/scripts/generate-secrets.sh
sudo bash infra/scripts/setup-tls.sh --domain pool.yourdomain.com
sudo bash infra/scripts/install-services.sh
sudo systemctl start p2pool-dashboard

# Update running deployment
bash infra/scripts/deploy.sh --branch main
```
