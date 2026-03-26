# XMR P2Pool Dashboard

**Mine Monero on P2Pool without running a node.** This dashboard gives you
everything you need to monitor your mining, track your payments, and understand
your earnings -- all without hosting a P2Pool node or a Monero full node on
your own hardware.

## Why P2Pool?

Traditional Monero mining pools hold your rewards and pay you on their schedule.
P2Pool is different: **every block reward goes directly to your wallet** via the
Monero blockchain. No pool operator can steal your funds, delay payments, or
skim fees. It's fully decentralized mining the way Monero was designed.

The catch? Running your own P2Pool node requires a full Monero node (~200 GB),
a P2Pool sidechain node, and the knowledge to keep them running 24/7.

**This project removes that barrier.** We host the P2Pool node infrastructure
and give you a clean dashboard to track everything. You point your miner at our
node, keep 100% of your rewards, and get full visibility into your mining
performance.

## What You Get

| Feature | Free Tier | Paid Tier (~$5/mo in XMR) |
|---|---|---|
| Real-time hashrate monitoring | Yes | Yes |
| Live pool stats via WebSocket | Yes | Yes |
| Block explorer (P2Pool-found blocks) | Yes | Yes |
| Sidechain share viewer | Yes | Yes |
| Payment tracking history | 30 days | Unlimited |
| Hashrate history | 30 days | Unlimited |
| Tax export (CSV with USD/CAD values) | -- | Yes |
| API key for integrations | -- | Yes |
| Tor (.onion) access | Yes | Yes |

**No account required.** Just enter your Monero wallet address to see your stats.
Upgrading to the paid tier is a single XMR payment -- no email, no signup, no KYC.

## Getting Started (For Miners)

1. **Configure XMRig** to point at the hosted P2Pool node (address provided on the dashboard)
2. **Visit the dashboard** and enter your Monero wallet address
3. **Watch your stats** -- hashrate, shares, payments, all in real-time

That's it. Your rewards arrive directly in your wallet via the Monero blockchain.
No withdrawals, no minimums, no waiting.

---

## Architecture

The stack is split into three Go/TypeScript services backed by PostgreSQL, Redis,
and a full observability pipeline:

```
XMRig --> P2Pool node
              |
              |  P2Pool local API + ZMQ block events
              v
        +-----------------------------+
        |       Go Manager            |  <-- core service
        |                             |
        |  P2Pool poller/indexer      |  polls sidechain every 30s
        |  Coinbase scanner           |  tracks on-chain payments
        |  Hashrate aggregator        |  15-min bucketed timeseries
        |  Subscription verifier      |  XMR payment detection
        |  WebSocket hub              |  live stats push
        |  Prometheus metrics         |
        +-------------+---------------+
                      |
              +-------v-------+     +---------------------+
              |  Go Gateway   |     |  PostgreSQL 15      |
              |  JWT auth     |     |  Redis 7            |
              |  Rate limiting|     |  Prometheus         |
              |  REST + WS    |     |  Grafana            |
              +-------+-------+     |  Loki + Promtail    |
                      |             |  Alertmanager       |
              +-------v-------+     +---------------------+
              |  Next.js 14   |
              |  Frontend     |
              +---------------+
```

### Tech Stack

| Layer | Technology |
|---|---|
| Backend | Go 1.22+, stdlib `net/http`, `log/slog` |
| Database | PostgreSQL 15 via `pgx/v5` (no ORM) |
| Cache | Redis 7 with LRU eviction |
| Frontend | Next.js 14, TypeScript, Recharts |
| Auth | JWT (gateway), API keys (subscription) |
| Metrics | Prometheus + Grafana dashboards |
| Logging | Loki + Promtail (structured JSON) |
| Alerting | Alertmanager (hashrate drops, indexer errors, stale blocks) |
| Reverse proxy | Nginx with TLS termination |
| Privacy | Tor hidden service (.onion) |
| Containers | Docker Compose, non-root images |

## Security

This is a **read-only monitoring service**. We never hold, transfer, or have
access to miner funds. All mining payments are handled natively by P2Pool
directly on the Monero blockchain.

### Infrastructure Hardening

- **Non-root containers** -- every Dockerfile uses a non-root `USER`
- **Docker secrets** -- all sensitive values (DB passwords, JWT keys) read from `/run/secrets/` with env fallback
- **Least-privilege database** -- dedicated `manager_user` role, no superuser at runtime
- **Dual-layer rate limiting** -- nginx (`limit_req_zone`) + Go gateway middleware
- **TLS everywhere** externally; plain HTTP only on the isolated Docker bridge network
- **No IP logging** -- miner address lookups never log the requesting IP (no IP-to-wallet correlation)
- **Tor hidden service** -- optional `.onion` access for maximum privacy

### CI/CD Security Pipeline

Every push and pull request triggers a multi-stage security pipeline that **must pass before code can be deployed**:

| Check | What It Does |
|---|---|
| **golangci-lint** | Static analysis on both Go services (manager + gateway) |
| **govulncheck** | Scans Go dependencies for known vulnerabilities (Go advisory DB) |
| **npm audit** | Checks frontend dependencies for HIGH+ severity CVEs |
| **Trivy** | Scans built Docker images for OS and library vulnerabilities (HIGH/CRITICAL, blocks on findings) |
| **Gitleaks** | Full git history scan for leaked secrets, tokens, and credentials |
| **Frontend tests** | Runs the full test suite + TypeScript type checking |
| **Go tests** | Runs all unit tests with race detector (`-race`) + `go vet` |
| **Dependabot** | Automated dependency update PRs for Go, npm, GitHub Actions, and Docker |

The deployment pipeline enforces a strict gate: **Go tests, frontend tests, and the full security scan must all pass** before Docker images are built and pushed to GHCR. Deployment to the VPS only happens after images are successfully built from `main`.

```
Push to main
  --> Test Go services (race detector + vet)
  --> Test frontend (Jest + TypeScript)
  --> Security scan (lint, vulncheck, npm audit, Trivy, Gitleaks)
  --> Build & push Docker images to GHCR
  --> Deploy to VPS via SSH
```

### Subscription Wallet

The optional paid tier uses a **view-only** wallet to detect incoming XMR payments.
This wallet **cannot spend** -- the full spend key is kept offline by the operator.
If the wallet is not configured, the subscription system is simply disabled and all
miners stay on the free tier. See [SECURITY.md](SECURITY.md) for full details.

## Observability

- **Prometheus** scrapes the manager every 10s with pre-built alert rules
- **Grafana** ships with two dashboards: Pool Overview and Miner Detail
- **Loki + Promtail** aggregate all container logs via Docker socket
- **Alertmanager** fires on hashrate drops, indexer errors, and stale blocks
- **External healthcheck** script for uptime monitoring with Discord/email alerts

## API Endpoints

### Core

| Method | Path | Description |
|---|---|---|
| `GET` | `/health` | Service health (postgres + redis) |
| `GET` | `/api/pool/stats` | Pool hashrate, miners, total hashes |
| `GET` | `/api/miner/{address}` | Per-miner stats |
| `GET` | `/api/miner/{address}/payments` | Payment history (paginated) |
| `GET` | `/api/miner/{address}/hashrate` | Hashrate timeseries (`?hours=24`, max 168 free / uncapped paid) |
| `GET` | `/api/miner/{address}/tax-export` | CSV with fiat values (paid tier) |
| `GET` | `/api/blocks` | P2Pool-found blocks (paginated) |
| `GET` | `/api/sidechain/shares` | Recent sidechain shares |
| `WS` | `/ws/pool/stats` | Live pool stats via WebSocket |

### Subscription

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/subscription/address/{address}` | Get or assign a payment subaddress |
| `GET` | `/api/subscription/status/{address}` | Tier status and expiry |
| `GET` | `/api/subscription/payments/{address}` | Subscription payment history |
| `POST` | `/api/subscription/api-key/{address}` | Generate API key (paid tier) |

Pagination: `?limit=50&offset=0`. Subscription requires `WALLET_RPC_URL` to be configured.

## Quick Start

```bash
# Clone
git clone https://github.com/acquiredl/xmr-p2pool-dashboard.git
cd xmr-p2pool-dashboard

# Configure
cp .env.example .env
# Edit .env with your P2Pool and monerod addresses

# Launch
docker compose up -d

# Verify
curl http://localhost:8081/health   # manager
curl http://localhost:8080/health   # gateway
```

The dashboard will be available at `https://localhost` via nginx.

## Development

```bash
# Start dev stack with hot reload
make dev

# Run tests
make test

# Run linter
make lint

# Run individual services
cd services/manager && go run ./cmd/manager/
cd services/gateway && go run ./cmd/gateway/
```

## Environment Variables

| Variable | Default | Description |
|---|---|---|
| `P2POOL_API_URL` | `http://p2pool:3333` | P2Pool node API endpoint |
| `P2POOL_SIDECHAIN` | `mini` | Sidechain to index (`mini` or `main`) |
| `MONEROD_URL` | `http://monerod:18081` | Monero daemon RPC |
| `MONEROD_ZMQ_URL` | `tcp://monerod:18083` | Monero daemon ZMQ |
| `POSTGRES_HOST` | `postgres` | PostgreSQL hostname |
| `POSTGRES_DB` | `p2pool_dashboard` | Database name |
| `POSTGRES_USER` | `manager_user` | Database user |
| `POSTGRES_PASSWORD` | (secret) | Database password |
| `REDIS_URL` | `redis:6379` | Redis address |
| `API_PORT` | `8081` | Manager API port |
| `METRICS_PORT` | `9090` | Prometheus metrics port |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `JWT_SECRET` | (secret) | Gateway JWT signing key |

Secrets can also be provided via Docker secrets at `/run/secrets/<name>`.

## Deployment

See [DEPLOYMENT.md](DEPLOYMENT.md) for the full VPS deployment guide covering
provisioning, TLS, systemd services, backups, monitoring, and CI/CD.

For the optional subscription system, see [docs/subscription-setup.md](docs/subscription-setup.md).

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/my-feature`)
3. Write tests for new functionality
4. Ensure `make lint` and `make test` pass
5. Submit a pull request with a clear description

Please read [SECURITY.md](SECURITY.md) before contributing security-sensitive changes.

## License

AGPL-3.0
