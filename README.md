# XMR P2Pool Dashboard

A read-only monitoring dashboard for P2Pool Monero miners. This is **not** a
traditional mining pool -- there is no wallet custody, no payout processing, and
no operator fees. The service reads from a P2Pool node and a Monero full node,
indexes sidechain and on-chain data, and presents it to miners via a clean web
dashboard. Miners keep all their rewards. We never touch their money.

## Architecture

```
XMRig --> P2Pool node
              |
              |  P2Pool local API (localhost:3333/api/)
              |  ZMQ block events (via monerod)
              v
        +-----------------------------+
        |       Go Manager            |
        |                             |
        |  internal/p2pool/     <-- polls P2Pool API
        |  internal/scanner/    <-- scans monerod coinbase
        |  internal/aggregator/ <-- builds timeseries
        |  pkg/monerod/         <-- monerod RPC client
        |  pkg/p2poolclient/    <-- P2Pool API client
        |  pkg/db/              <-- PostgreSQL via pgx
        +-------------+---------------+
                      |
              +-------v-------+     +---------------------+
              |  Go Gateway   |     |  PostgreSQL 15      |
              |  JWT auth     |     |  Redis 7            |
              |  Rate limiting|     |  Prometheus         |
              |  REST proxy   |     |  Grafana            |
              |  WS proxy     |     |  Loki               |
              +-------+-------+     +---------------------+
                      |
              +-------v-------+
              |  Next.js 14   |
              |  Frontend     |
              +---------------+
```

## Quick Start

```bash
# 1. Clone the repository
git clone https://github.com/acquiredl/xmr-p2pool-dashboard.git
cd xmr-p2pool-dashboard

# 2. Copy and configure environment
cp .env.example .env
# Edit .env with your P2Pool node and monerod addresses

# 3. Start all services
docker compose up -d

# 4. Verify health
curl http://localhost:8081/health   # manager
curl http://localhost:8080/health   # gateway
```

The dashboard will be available at `https://localhost` (via nginx).

## Development Setup

```bash
# Start development stack with hot reload
make dev

# Run tests
make test

# Run linter
make lint

# Run just the Go services
cd services/manager && go run ./cmd/manager/
cd services/gateway && go run ./cmd/gateway/
```

## Environment Variables

| Variable | Default | Description |
|---|---|---|
| `P2POOL_API_URL` | `http://p2pool:3333` | P2Pool node API endpoint |
| `P2POOL_SIDECHAIN` | `mini` | Sidechain to index (`mini` or `main`) |
| `MONEROD_URL` | `http://monerod:18081` | Monero daemon RPC endpoint |
| `MONEROD_ZMQ_URL` | `tcp://monerod:18083` | Monero daemon ZMQ endpoint |
| `POSTGRES_HOST` | `postgres` | PostgreSQL hostname |
| `POSTGRES_DB` | `p2pool_dashboard` | PostgreSQL database name |
| `POSTGRES_USER` | `manager_user` | PostgreSQL username |
| `POSTGRES_PASSWORD` | (secret) | PostgreSQL password |
| `REDIS_URL` | `redis:6379` | Redis connection address |
| `API_PORT` | `8081` | Manager API listen port |
| `METRICS_PORT` | `9090` | Prometheus metrics port |
| `LOG_LEVEL` | `info` | Log level (`debug`, `info`, `warn`, `error`) |
| `JWT_SECRET` | (secret) | JWT signing key for gateway auth |

Secrets can also be provided via Docker secrets at `/run/secrets/<name>`.

## API Endpoints

### Core

| Method | Path | Description |
|---|---|---|
| `GET` | `/health` | Service health check (postgres + redis) |
| `GET` | `/api/pool/stats` | Aggregated pool statistics |
| `GET` | `/api/miner/{address}` | Stats for a specific miner |
| `GET` | `/api/miner/{address}/payments` | Paginated payment history |
| `GET` | `/api/miner/{address}/hashrate` | Hashrate timeseries (default 24h) |
| `GET` | `/api/miner/{address}/tax-export` | CSV export of payments with fiat values (paid tier) |
| `GET` | `/api/blocks` | Paginated list of found blocks |
| `GET` | `/api/sidechain/shares` | Recent sidechain shares |
| `WS` | `/ws/pool/stats` | Live pool stats via WebSocket |

### Subscription (optional)

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/subscription/address/{address}` | Get or assign a payment subaddress |
| `GET` | `/api/subscription/status/{address}` | Tier status and expiry |
| `GET` | `/api/subscription/payments/{address}` | Subscription payment history |
| `POST` | `/api/subscription/api-key/{address}` | Generate API key (paid tier only) |

Subscription endpoints require `WALLET_RPC_URL` to be configured. Without it,
all miners remain on the free tier. See [docs/subscription-setup.md](docs/subscription-setup.md).

**Query parameters for paginated endpoints:** `?limit=50&offset=0`

**Hashrate endpoint:** `?hours=24` (max 168)

## Security Model

- **No miner wallet RPC** -- the service never handles miner funds
- Optional view-only wallet for operator subscription payments (cannot spend)
- All secrets via Docker secrets with environment variable fallback
- Non-root `USER` in every Dockerfile
- PostgreSQL uses a least-privilege `manager_user` role
- No IP addresses logged in association with address lookups
- Rate limiting at both nginx and Go gateway layers
- TLS termination at nginx; plain HTTP on the internal Docker network
- Tor hidden service available for `.onion` access
- See [SECURITY.md](SECURITY.md) for full details

## Observability

- **Prometheus** scrapes the manager at `:9090/metrics` every 10 seconds
- **Grafana** ships with pre-built dashboards (Pool Overview + Miner Detail)
- **Loki + Promtail** aggregate container logs via Docker socket
- **Alertmanager** rules fire on hashrate drops, indexer errors, and stale blocks
- **External healthcheck** script for uptime monitoring with Discord/email alerts

## Deployment

See [DEPLOYMENT.md](DEPLOYMENT.md) for the full VPS deployment guide covering
provisioning, TLS, systemd services, backups, monitoring, and CI/CD.

For the optional XMR subscription system, see [docs/subscription-setup.md](docs/subscription-setup.md).

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/my-feature`)
3. Write tests for new functionality
4. Ensure `make lint` and `make test` pass
5. Submit a pull request with a clear description

Please read [SECURITY.md](SECURITY.md) before contributing security-sensitive changes.

## License

AGPL-3.0
