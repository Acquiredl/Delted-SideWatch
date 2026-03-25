# Manager Service

The primary backend service. Polls the P2Pool sidechain API, scans monerod for
coinbase payments, builds miner hashrate timeseries, and serves all dashboard
data over a REST API and WebSocket.

## Key Exports

| Package | Description |
|---|---|
| `internal/p2pool` | P2Pool API client and sidechain share/block indexer |
| `internal/scanner` | Monitors monerod for new blocks, extracts coinbase outputs, records payments |
| `internal/aggregator` | Query layer for pool stats, miner stats, payments, and hashrate timeseries |
| `internal/events` | ZMQ block event listener — triggers the scanner on new Monero blocks |
| `internal/cache` | Redis cache wrapper with typed get/set and TTL |
| `internal/metrics` | Prometheus metric definitions (pool hashrate, miner count, HTTP latency) |
| `internal/ws` | WebSocket broadcast hub for live pool stats |
| `pkg/db` | pgx connection pool, health check, and forward-only SQL migrations |
| `pkg/monerod` | Monero JSON-RPC client (get_block, get_transactions, get_last_block_header) |
| `pkg/p2poolclient` | Typed HTTP client for the P2Pool local API |

## Architecture

On startup, `cmd/manager/main.go` wires all components and launches four
background goroutines:

1. **Indexer** — polls P2Pool API every 30s, upserts shares and found blocks
   into Postgres
2. **Timeseries builder** — aggregates raw shares into 15-minute hashrate
   buckets per miner
3. **Block listener + scanner** — listens for new Monero blocks via ZMQ, scans
   coinbase outputs to reconstruct per-miner payments
4. **WebSocket hub** — broadcasts pool stats to connected clients

The HTTP server exposes the REST API (routes registered in `cmd/manager/routes.go`)
and a metrics server on a separate port for Prometheus scraping.

## API Endpoints

| Method | Path | Description |
|---|---|---|
| `GET` | `/health` | Service + Postgres + Redis health check |
| `GET` | `/api/pool/stats` | Aggregated pool overview (cached 15s) |
| `GET` | `/api/miner/{address}` | Stats for a single miner |
| `GET` | `/api/miner/{address}/payments` | Paginated payment history (`?limit=&offset=`) |
| `GET` | `/api/miner/{address}/hashrate` | Hashrate timeseries (`?hours=`, max 168) |
| `GET` | `/api/miner/{address}/tax-export` | CSV download of all payments with fiat values |
| `GET` | `/api/blocks` | Paginated found blocks |
| `GET` | `/api/sidechain/shares` | Paginated sidechain shares |
| `WS`  | `/ws/pool/stats` | Live pool stats via WebSocket |

## Configuration

All via environment variables with Docker secrets fallback (`/run/secrets/<key>`).
See `.env.example` for the full list. Required vars (`mustGetEnv`) will panic if
missing:

- `POSTGRES_HOST`, `POSTGRES_DB`, `POSTGRES_USER`, `POSTGRES_PASSWORD` (required)
- `P2POOL_API_URL` (default: `http://p2pool:3333`)
- `P2POOL_SIDECHAIN` (default: `mini`)
- `MONEROD_URL` (default: `http://monerod:18081`)
- `MONEROD_ZMQ_URL` (default: `tcp://monerod:18083`)
- `REDIS_URL` (default: `redis:6379`)
- `API_PORT` (default: `8081`), `METRICS_PORT` (default: `9090`)

## Running

```bash
# Build
go build -o manager ./cmd/manager/

# Run directly (requires Postgres, Redis, P2Pool, monerod)
./manager

# Run tests
go test -race ./...
```

## Dependencies

- `pgx/v5` — PostgreSQL driver (no ORM)
- `go-redis/v9` — Redis client for caching and rate state
- `zmq4` — ZeroMQ bindings for block event subscription
- `prometheus/client_golang` — metrics exposition
- `nhooyr.io/websocket` — WebSocket server
