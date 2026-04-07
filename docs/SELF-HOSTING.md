# Self-Hosting Guide

Run your own P2Pool dashboard with a full Monero node and P2Pool sidechain node.

## Hardware Requirements

| Component | Minimum | Recommended |
|---|---|---|
| CPU | 2 cores | 4+ cores |
| RAM | 8 GB | 16 GB |
| Disk | 200 GB SSD | 500 GB NVMe |
| Bandwidth | 500 GB/month | Unlimited |

Monerod (pruned) uses ~70 GB of disk and grows ~1 GB/month. A full (non-pruned)
node needs ~200 GB. P2Pool and the dashboard add negligible disk usage.

## Quick Start

### 1. Clone and configure

```bash
git clone https://github.com/Acquiredl/Delted-SideWatch.git
cd Delted-SideWatch
cp .env.example .env
```

Edit `.env` with your values:

```bash
# REQUIRED: Your Monero wallet address for P2Pool payouts
MONERO_WALLET_ADDRESS=4YourMoneroAddressHere...

# REQUIRED: Change these from defaults
POSTGRES_PASSWORD=$(openssl rand -hex 16)
ADMIN_TOKEN=$(openssl rand -hex 32)
JWT_SECRET=$(openssl rand -hex 32)

# Sidechain (mini for hobbyist miners, main for large miners)
P2POOL_SIDECHAIN=mini
NEXT_PUBLIC_SIDECHAIN=mini
```

### 2. Start the stack

```bash
docker compose -f docker-compose.yml -f docker-compose.node.yml up -d
```

This starts: monerod, P2Pool, the dashboard manager, gateway, frontend,
Postgres, Redis, nginx, Prometheus, and Grafana.

### 3. Wait for monerod to sync

Initial blockchain sync takes **12-24 hours** (pruned mode). Monitor progress:

```bash
# Watch sync progress
docker compose logs -f monerod

# Check sync status via RPC
curl -s http://localhost:18081/get_info | python3 -c "
import sys, json
d = json.load(sys.stdin)
pct = d['height'] / d['target_height'] * 100 if d.get('target_height') else 0
print(f\"Height: {d['height']} / {d.get('target_height', '?')} ({pct:.1f}%)\")
"
```

P2Pool starts automatically once monerod is healthy (synced and responsive).

### 4. Point your miner

Once P2Pool is running, point XMRig at it. The wallet address is configured on
the P2Pool node (via `MONERO_WALLET_ADDRESS` in `.env`), **not** in XMRig:

```bash
xmrig -o 127.0.0.1:3333
```

Or from another machine on your network:

```bash
xmrig -o YOUR_SERVER_IP:3333
```

The `-u` flag in XMRig is optional and only used to set custom difficulty
(e.g., `-u x+10000`). It does not control which wallet receives payouts —
that is always determined by the P2Pool node's `--wallet` flag.

### 5. Access the dashboard

- **Dashboard**: http://localhost:3000
- **API**: http://localhost:8080/api/pool/stats
- **Grafana**: http://localhost:3001 (admin/admin on first login)
- **Prometheus**: http://localhost:9091

### 6. Validate

Run the validation script to confirm everything is working:

```bash
bash infra/scripts/validate-node.sh
```

## Choosing Mini vs Main Sidechain

| | Mini | Main |
|---|---|---|
| Target audience | Hobbyist miners (<100 KH/s) | Large miners (>100 KH/s) |
| Share difficulty | ~300 MH | ~10 GH+ |
| PPLNS window | 2160 shares | Larger |
| P2P port | 37888 | 37889 |
| Default | Yes | No |

### Switching to main sidechain

1. Edit `.env`:
   ```
   P2POOL_SIDECHAIN=main
   NEXT_PUBLIC_SIDECHAIN=main
   ```

2. Edit `docker-compose.node.yml` — in the `p2pool` service:
   - Remove the `"--mini"` argument
   - Change `"0.0.0.0:37888"` to `"0.0.0.0:37889"`
   - Change ports from `37888:37888` to `37889:37889`

3. Restart the stack:
   ```bash
   docker compose -f docker-compose.yml -f docker-compose.node.yml up -d
   ```

## Firewall / Port Forwarding

| Port | Service | Required? |
|---|---|---|
| 18080 | Monerod P2P | Yes — needed for blockchain sync |
| 18081 | Monerod RPC | No — only needed if exposing RPC externally |
| 37888 | P2Pool P2P (mini) | Yes — needed for sidechain sync |
| 37889 | P2Pool P2P (main) | Yes (if using main) |
| 3333 | P2Pool Stratum | Yes — miners connect here |
| 80/443 | Nginx (dashboard) | Yes — if serving the dashboard publicly |

If running behind NAT, forward at minimum: **18080** (monerod P2P) and
**37888** (P2Pool P2P) for the node to participate in both networks.

## Backups

Database backups run automatically via the systemd timer (if installed) or
manually:

```bash
bash infra/scripts/pool-backup.sh
```

Backups are stored in `/opt/p2pool-dashboard/backups/` with 7-day retention.
Configure `BACKUP_REMOTE_URL` in `.env` for S3 or rsync offsite backup.

To restore:

```bash
bash infra/scripts/restore.sh --yes latest.dump
```

## TLS / HTTPS

For public-facing deployments:

```bash
bash infra/scripts/setup-tls.sh your-domain.com
```

This configures Let's Encrypt certificates with automatic renewal via nginx.

## Tor Hidden Service (Not Yet Implemented)

Tor hidden service support is planned for a future release. The infrastructure
files exist in the repo but the service is currently disabled.

## Monitoring

- **Grafana dashboards**: Pool overview and per-miner detail are pre-provisioned
- **Prometheus alerts**: Configured in `config/prometheus/alerts/pool.yml`
- **Alertmanager**: Sends alerts to the dashboard webhook; configure email/Slack
  in `config/alertmanager/alertmanager.yml`
- **Loki + Promtail**: Centralized log aggregation, queryable via Grafana

## Updating

```bash
git pull
docker compose -f docker-compose.yml -f docker-compose.node.yml up -d --build
```

Or if using pre-built images from GHCR:

```bash
docker compose -f docker-compose.yml -f docker-compose.prod.yml -f docker-compose.node.yml pull
docker compose -f docker-compose.yml -f docker-compose.prod.yml -f docker-compose.node.yml up -d
```

## Troubleshooting

**Monerod won't sync**: Ensure port 18080 is open. Check `docker compose logs monerod`
for peer connection errors.

**P2Pool won't start**: It depends on monerod being healthy (fully synced).
Check `docker compose ps` — P2Pool should be "waiting" until monerod passes
its health check.

**Dashboard shows no data**: The manager polls P2Pool every 30 seconds. Wait
1-2 minutes after P2Pool starts, then check `docker compose logs manager` for
polling errors.

**High memory usage**: Monerod is the primary consumer. The `docker-compose.node.yml`
sets a 4 GB limit. If you need more, adjust the `deploy.resources.limits.memory`
value.
