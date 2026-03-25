# XMR P2Pool Dashboard — Claude Code Context

## Project Goal

A Go + Next.js dashboard for P2Pool Monero miners. NOT a traditional mining pool.
There is no wallet custody, no payout processing, no Miningcore. This service reads
from a P2Pool node and a Monero full node, indexes sidechain and on-chain data, and
serves it to miners via a clean dashboard.

Miners keep all their rewards. We never touch their money.

---

## Architecture

```
XMRig → P2Pool node ──────────────────────────────────────────────┐
              │                                                     │
              │  P2Pool local API (localhost:3333/api/)             │
              │  ZMQ block events (via monerod)                     │
              ▼                                                     │
        ┌─────────────────────────────┐                            │
        │       Go Manager            │  ← PRIMARY BUILD TARGET    │
        │                             │                            │
        │  /internal/p2pool/     ←────┼── polls P2Pool API         │
        │  /internal/scanner/    ←────┼── scans monerod coinbase   │
        │  /internal/aggregator/ ←────┼── builds timeseries        │
        │  /internal/metrics/         │                            │
        │  /pkg/monerod/         ←────┼── monerod RPC client       │
        │  /pkg/p2poolclient/    ←────┼── P2Pool API client        │
        │  /pkg/db/                   │                            │
        │  /pkg/cache/                │                            │
        └──────────────┬──────────────┘                            │
                       │                                           │
              ┌────────▼────────┐     ┌──────────────────────┐    │
              │  Go Gateway     │     │   PostgreSQL 15       │    │
              │                 │     │   Redis 7             │    │
              │  JWT auth       │     │   Prometheus          │    │
              │  Rate limiting  │     │   Grafana             │    │
              │  REST proxy     │     │   Loki                │    │
              │  WS proxy       │     └──────────────────────┘    │
              └────────┬────────┘                                  │
                       │                                           │
              ┌────────▼────────┐                                  │
              │  Next.js 14     │◄──────────────────────────────── ┘
              │  Frontend       │
              └─────────────────┘
```

---

## Repo Structure

```
xmr-p2pool-dashboard/
├── CLAUDE.md                          ← you are here
├── README.md
├── CHEATSHEET.md
├── SECURITY.md
├── Makefile
├── docker-compose.yml
├── docker-compose.dev.yml
├── .env.example
├── .gitignore
├── .golangci.yml
│
├── services/
│   ├── gateway/                       ← Go API gateway
│   │   ├── go.mod
│   │   ├── cmd/gateway/
│   │   │   ├── main.go
│   │   │   └── config.go
│   │   └── internal/
│   │       ├── auth/                  ← JWT middleware
│   │       ├── middleware/            ← rate limit, logger, requestID
│   │       └── proxy/                ← reverse proxy to manager
│   │
│   └── manager/                       ← Go pool manager (main build)
│       ├── go.mod
│       ├── cmd/manager/
│       │   ├── main.go
│       │   ├── routes.go
│       │   └── config.go
│       ├── internal/
│       │   ├── p2pool/                ← P2Pool sidechain poller + indexer
│       │   │   ├── client.go          ← HTTP client for P2Pool local API
│       │   │   ├── indexer.go         ← indexes shares, blocks into Postgres
│       │   │   └── types.go           ← P2Pool API response types
│       │   ├── scanner/               ← on-chain coinbase scanner
│       │   │   ├── scanner.go         ← watches monerod for new blocks
│       │   │   ├── coinbase.go        ← extracts + matches coinbase outputs
│       │   │   └── priceoracle.go     ← fetches XMR/USD + XMR/CAD spot price
│       │   ├── aggregator/            ← builds miner stat views
│       │   │   ├── aggregator.go
│       │   │   └── timeseries.go      ← rolling hashrate timeseries per miner
│       │   ├── events/                ← ZMQ + polling block event listener
│       │   │   └── zmq.go
│       │   ├── metrics/               ← Prometheus metrics
│       │   │   └── metrics.go
│       │   └── cache/                 ← Redis helpers
│       │       └── cache.go
│       └── pkg/
│           ├── db/                    ← pgx connection pool
│           │   ├── db.go
│           │   └── migrations/        ← SQL migration files
│           │       ├── 001_initial.sql
│           │       └── 002_payments.sql
│           ├── monerod/               ← Monero RPC client
│           │   ├── client.go
│           │   └── types.go
│           └── p2poolclient/          ← P2Pool API client (typed)
│               ├── client.go
│               └── types.go
│
├── frontend/                          ← Next.js 14 (TypeScript)
│   ├── app/
│   │   ├── layout.tsx
│   │   ├── globals.css
│   │   ├── page.tsx                   ← pool stats home
│   │   ├── miner/page.tsx             ← miner dashboard (address lookup)
│   │   ├── blocks/page.tsx            ← block explorer
│   │   ├── sidechain/page.tsx         ← P2Pool sidechain viewer
│   │   └── admin/page.tsx             ← JWT-protected admin panel
│   ├── components/
│   │   ├── LiveStats.tsx              ← WebSocket pool hashrate
│   │   ├── HashrateChart.tsx          ← Recharts area chart
│   │   ├── BlocksTable.tsx
│   │   ├── PaymentsTable.tsx
│   │   ├── WorkersTable.tsx
│   │   ├── SidechainTable.tsx
│   │   └── PrivacyNotice.tsx          ← coinbase transparency warning
│   └── lib/
│       ├── api.ts                     ← typed fetch client
│       └── useWebSocket.ts            ← live hashrate hook
│
├── config/
│   ├── nginx/nginx.conf
│   ├── prometheus/
│   │   ├── prometheus.yml
│   │   └── alerts/pool.yml
│   ├── grafana/provisioning/
│   │   ├── datasources/
│   │   └── dashboards/
│   └── loki/
│       ├── config.yml
│       └── promtail.yml
│
└── infra/
    ├── docker/
    │   ├── gateway/Dockerfile
    │   ├── gateway/Dockerfile.dev
    │   ├── manager/Dockerfile
    │   └── manager/Dockerfile.dev
    └── scripts/
        ├── initdb.sql
        └── pool-backup.sh
```

---

## Go Conventions

- **Module path:** `github.com/acquiredl/xmr-p2pool-dashboard`
- **Go version:** 1.22+
- **Logging:** `log/slog` with JSON handler — no third-party logging libs
- **HTTP:** `net/http` stdlib only — no Gin, Echo, Chi
- **Database:** `github.com/jackc/pgx/v5` — no ORM, always named columns, always parameterized queries
- **Redis:** `github.com/redis/go-redis/v9`
- **Metrics:** `github.com/prometheus/client_golang`
- **Errors:** always wrap — `fmt.Errorf("doing thing: %w", err)` — never swallow
- **Config:** env vars only — `mustGetEnv()` panics on missing required vars
- **Secrets:** read from `/run/secrets/<name>` first, env var fallback
- **Tests:** table-driven where appropriate, `httptest` for handlers, skip integration tests if dependency unavailable
- **No global state** — pass dependencies explicitly via struct fields

---

## Database Schema Conventions

- Minecore does NOT exist here — this project owns its entire schema
- All tables use `snake_case`
- All tables have `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`
- Use `BIGSERIAL` for surrogate PKs, not UUID (performance)
- Timeseries data uses `created_at` as the primary time index
- Always `EXPLAIN ANALYZE` new queries against realistic data before committing
- Migrations in `pkg/db/migrations/` — numbered, forward-only, no ORM migrations

**Core tables to implement:**

```sql
-- Indexed P2Pool sidechain shares
CREATE TABLE p2pool_shares (
    id              BIGSERIAL PRIMARY KEY,
    sidechain       VARCHAR(10) NOT NULL,  -- 'mini' or 'main'
    miner_address   VARCHAR(256) NOT NULL,
    worker_name     VARCHAR(128),
    sidechain_height BIGINT NOT NULL,
    difficulty      BIGINT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Blocks found by P2Pool (main chain blocks)
CREATE TABLE p2pool_blocks (
    id              BIGSERIAL PRIMARY KEY,
    main_height     BIGINT NOT NULL UNIQUE,
    main_hash       VARCHAR(64) NOT NULL,
    sidechain_height BIGINT NOT NULL,
    coinbase_reward BIGINT NOT NULL,  -- atomic units
    effort          NUMERIC(10,4),
    found_at        TIMESTAMPTZ NOT NULL
);

-- On-chain coinbase payments reconstructed by scanner
CREATE TABLE payments (
    id              BIGSERIAL PRIMARY KEY,
    miner_address   VARCHAR(256) NOT NULL,
    amount          BIGINT NOT NULL,       -- atomic units (1 XMR = 1e12)
    main_height     BIGINT NOT NULL,
    main_hash       VARCHAR(64) NOT NULL,
    xmr_usd_price   NUMERIC(12,4),
    xmr_cad_price   NUMERIC(12,4),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Miner hashrate timeseries (aggregated, not raw shares)
CREATE TABLE miner_hashrate (
    id              BIGSERIAL PRIMARY KEY,
    miner_address   VARCHAR(256) NOT NULL,
    sidechain       VARCHAR(10) NOT NULL,
    hashrate        BIGINT NOT NULL,       -- H/s
    bucket_time     TIMESTAMPTZ NOT NULL,  -- truncated to 15-min buckets
    UNIQUE (miner_address, sidechain, bucket_time)
);
```

---

## P2Pool API — Key Endpoints

All on `http://p2pool:3333` (internal Docker network):

```
GET /api/pool/stats
  → { pool_statistics: { hash_rate_short, miners, total_hashes, ... } }

GET /api/shares
  → [ { id, shares, timestamp, ... } ]  -- current PPLNS window

GET /api/found_blocks
  → [ { height, hash, timestamp, reward, effort, ... } ]

GET /api/worker_stats
  → { <address>: { shares, hashes, ... } }

GET /api/p2p/peers
  → [ { id, addr, ... } ]
```

The P2Pool API returns data for the **current PPLNS window** only. Historical
data must be reconstructed from the sidechain or your own indexed database.

---

## Monerod RPC — Key Methods

All via JSON-RPC at `http://monerod:18081/json_rpc`:

```
get_block_template       — for mining (not needed here)
get_last_block_header    — current chain tip
get_block_header_by_height
get_block                — full block including coinbase tx
get_transactions         — fetch specific txs by hash
```

For coinbase scanning:
1. `get_last_block_header` → get current height
2. `get_block` with height → get coinbase tx hash
3. `get_transactions` with coinbase hash → get outputs
4. Match output addresses against known P2Pool miner addresses

---

## Security Rules

- No wallet RPC anywhere in this project — we never touch miner funds
- All secrets via Docker secrets (`/run/secrets/`) with env fallback
- Non-root USER in every Dockerfile
- Postgres: `manager_user` role with least privilege — owns its own schema
- No IP logging associated with address lookups
- Rate limiting: nginx layer + Go gateway layer
- TLS everywhere externally; plain HTTP on Docker internal network

---

## What Has Been Designed (but needs implementation)

Every file in the repo has its structure and key signatures defined.
The following components need their core logic filled in:

**High priority — implement first:**
1. `pkg/p2poolclient/client.go` — HTTP client for P2Pool local API with all typed response structs
2. `pkg/monerod/client.go` — JSON-RPC client for monerod (get_block, get_transactions, get_last_block_header)
3. `internal/p2pool/indexer.go` — polls P2Pool API every 30s, upserts shares + blocks into Postgres
4. `internal/scanner/scanner.go` + `coinbase.go` — listens for new blocks via ZMQ, scans coinbase outputs, records payments
5. `pkg/db/migrations/001_initial.sql` — full schema as defined above

**Medium priority:**
6. `internal/aggregator/timeseries.go` — buckets raw shares into 15-min hashrate timeseries per miner
7. `internal/scanner/priceoracle.go` — CoinGecko API client for historical XMR/USD + XMR/CAD
8. All HTTP handlers in `cmd/manager/routes.go` — replace `not implemented` stubs
9. `frontend/components/SidechainTable.tsx` — display recent P2Pool sidechain shares
10. `frontend/app/sidechain/page.tsx` — sidechain health page

**Lower priority:**
11. Tax export endpoint — CSV with per-payment XMR amount + fiat value at time of receipt
12. WebSocket handler in manager for live hashrate push
13. Alertmanager webhook config
14. Grafana dashboard JSON for miner-level panels

---

## Common Tasks for Claude Code

**Add a new API endpoint:**
1. Add route in `services/manager/cmd/manager/routes.go`
2. Implement handler — either in `internal/aggregator/` or a new `internal/` package
3. Add corresponding types to `frontend/lib/api.ts`
4. Wire up frontend page or component

**Add a new DB table:**
1. Create migration in `services/manager/pkg/db/migrations/`
2. Number it sequentially (`003_name.sql`)
3. Add indexes for expected query patterns
4. Update relevant internal package to use new table

**Add a Prometheus metric:**
1. Register in `internal/metrics/metrics.go`
2. Instrument the relevant code path
3. Add a panel to `config/grafana/provisioning/dashboards/pool-overview.json`

**Run tests:**
```bash
cd services/manager && go test -race ./...
cd services/gateway && go test -race ./...
```

**Start dev stack:**
```bash
make dev
```

**Check all services healthy:**
```bash
docker compose ps
curl http://localhost:8081/health
curl http://localhost:8080/health
```

---

## Environment Variables (see .env.example for full list)

```
P2POOL_API_URL      http://p2pool:3333
P2POOL_SIDECHAIN    mini          # or 'main'
MONEROD_URL         http://monerod:18081
MONEROD_ZMQ_URL     tcp://monerod:18083
POSTGRES_HOST       postgres
POSTGRES_DB         p2pool_dashboard
POSTGRES_USER       manager_user
REDIS_URL           redis:6379
API_PORT            8081
METRICS_PORT        9090
LOG_LEVEL           info
```

---

## Do Not Implement Without Discussion

- Custom Stratum server (not needed — P2Pool handles this)
- Wallet RPC integration (not needed — no operator fund custody)
- Cross-address correlation or clustering features
- Long-term (>90 day) retention of per-address data
- Any feature that requires miners to create accounts or provide email

## Resolved Architecture Decisions
P2Pool mini vs main
Start with mini only. Mini targets the hobbyist/small miner profile that aligns with the project's zero-fee, decentralization angle. Data layer should be designed sidechain-agnostic so adding main later is low-friction.
Node hosting model
Run our own P2Pool node. The dashboard is a hosted service — users just visit it, no setup required. Self-hosting support is a potential future feature if there's demand. Aligns with the managed node hosting monetization tier.
Monerod block event delivery
ZMQ over polling. We control the infrastructure so ZMQ is always available. Better latency, event-driven, and the correct tool for this use case.
Sidechain reorg handling
Confirmation depth buffer (Option B). Payments are not recorded as final until the Monero block is 10 confirmations deep (~20 minutes). Eliminates the vast majority of reorg risk with minimal implementation complexity. UI should communicate that payments appear after sufficient confirmations.
XMR subscription payment flow
Manual verification at launch. Users send XMR and email their txid (txid only in the body). Email parsing can be automated with AI when volume justifies it. Automated wallet RPC verification is a future milestone once paying users validate the feature.
Tor hidden service
Add it. One extra Docker container, no code changes, strong trust signal for the target audience. Offered as an opt-in alternative URL, not the primary one. Document it in the README.

---

## Citadel Harness

This project uses the [Citadel](https://github.com/SethGammon/Citadel) agent
orchestration harness. Configuration is in `.claude/harness.json`.

- 25 skills registered — run `/do --list` to see all
- Hooks configured in `.claude/settings.json`
- Campaign/fleet state in `.planning/`
