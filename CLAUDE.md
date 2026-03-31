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
├── DEPLOYMENT.md
├── SECURITY.md
├── Makefile
├── docker-compose.yml
├── docker-compose.dev.yml
├── docker-compose.test.yml
├── .env.example
├── .gitignore
├── .golangci.yml
│
├── .github/
│   ├── CODEOWNERS
│   ├── dependabot.yml
│   └── workflows/
│       ├── deploy.yml                 ← CD pipeline
│       └── security.yml              ← DevSecOps scanning
│
├── services/
│   ├── gateway/                       ← Go API gateway
│   │   ├── go.mod
│   │   ├── cmd/gateway/
│   │   │   ├── main.go
│   │   │   └── config.go
│   │   └── internal/
│   │       ├── auth/                  ← JWT middleware
│   │       │   ├── jwt.go
│   │       │   └── jwt_test.go
│   │       └── middleware/            ← rate limit, logger, requestID
│   │           ├── logger.go
│   │           ├── ratelimit.go
│   │           ├── ratelimit_test.go
│   │           └── requestid.go
│   │
│   ├── manager/                       ← Go pool manager (main build)
│   │   ├── go.mod
│   │   ├── cmd/manager/
│   │   │   ├── main.go
│   │   │   ├── routes.go
│   │   │   └── config.go
│   │   ├── internal/
│   │   │   ├── p2pool/                ← P2Pool sidechain poller + indexer
│   │   │   │   ├── client.go
│   │   │   │   ├── indexer.go
│   │   │   │   ├── indexer_integration_test.go
│   │   │   │   └── types.go
│   │   │   ├── scanner/               ← on-chain coinbase scanner
│   │   │   │   ├── scanner.go
│   │   │   │   ├── scanner_integration_test.go
│   │   │   │   ├── coinbase.go
│   │   │   │   ├── coinbase_test.go
│   │   │   │   └── priceoracle.go     ← CoinGecko XMR/USD + XMR/CAD
│   │   │   ├── aggregator/            ← builds miner stat views
│   │   │   │   ├── aggregator.go
│   │   │   │   ├── aggregator_integration_test.go
│   │   │   │   ├── timeseries.go
│   │   │   │   ├── timeseries_test.go
│   │   │   │   └── timeseries_integration_test.go
│   │   │   ├── subscription/          ← XMR subscription payment verification
│   │   │   │   ├── service.go
│   │   │   │   ├── service_test.go
│   │   │   │   ├── scanner.go
│   │   │   │   ├── scanner_test.go
│   │   │   │   ├── middleware.go
│   │   │   │   ├── middleware_test.go
│   │   │   │   └── types.go
│   │   │   ├── ws/                    ← WebSocket live hashrate push
│   │   │   │   ├── handler.go
│   │   │   │   ├── hub.go
│   │   │   │   └── hub_integration_test.go
│   │   │   ├── events/                ← ZMQ + polling block event listener
│   │   │   │   └── zmq.go
│   │   │   ├── metrics/               ← Prometheus metrics
│   │   │   │   └── metrics.go
│   │   │   ├── cache/                 ← Redis helpers
│   │   │   │   ├── cache.go
│   │   │   │   └── cache_integration_test.go
│   │   │   └── testhelper/            ← shared test utilities
│   │   │       └── testdb.go
│   │   └── pkg/
│   │       ├── db/                    ← pgx connection pool
│   │       │   ├── db.go
│   │       │   └── migrations/
│   │       │       ├── 001_initial.sql
│   │       │       ├── 002_payments.sql
│   │       │       └── 003_subscriptions.sql
│   │       ├── monerod/               ← Monero RPC client
│   │       │   ├── client.go
│   │       │   ├── client_test.go
│   │       │   └── types.go
│   │       ├── p2poolclient/          ← P2Pool API client (typed)
│   │       │   ├── client.go
│   │       │   ├── client_test.go
│   │       │   └── types.go
│   │       └── walletrpc/             ← view-only wallet RPC (subscription verification)
│   │           ├── client.go
│   │           ├── client_test.go
│   │           └── types.go
│   │
│   └── mocknode/                      ← fake P2Pool + monerod for local dev/testing
│       ├── go.mod
│       └── main.go
│
├── frontend/                          ← Next.js 14 (TypeScript)
│   ├── app/
│   │   ├── layout.tsx
│   │   ├── globals.css
│   │   ├── page.tsx                   ← pool stats home
│   │   ├── miner/page.tsx             ← miner dashboard (address lookup)
│   │   ├── blocks/page.tsx            ← block explorer
│   │   ├── sidechain/page.tsx         ← P2Pool sidechain viewer
│   │   ├── admin/page.tsx             ← JWT-protected admin panel
│   │   ├── subscribe/page.tsx         ← subscription management + payment
│   │   └── __tests__/                 ← page-level tests
│   ├── components/
│   │   ├── LiveStats.tsx              ← WebSocket pool hashrate
│   │   ├── HashrateChart.tsx          ← Recharts area chart
│   │   ├── BlocksTable.tsx
│   │   ├── PaymentsTable.tsx
│   │   ├── WorkersTable.tsx
│   │   ├── SidechainTable.tsx
│   │   ├── SubscriptionStatus.tsx     ← tier badge, expiry, benefits
│   │   ├── SubscriptionPayment.tsx    ← payment subaddress + history
│   │   ├── Navigation.tsx
│   │   ├── PrivacyNotice.tsx          ← coinbase transparency warning
│   │   └── __tests__/                 ← component-level tests
│   └── lib/
│       ├── api.ts                     ← typed fetch client
│       ├── useWebSocket.ts            ← live hashrate hook
│       └── __tests__/                 ← lib-level tests
│
├── config/
│   ├── nginx/nginx.conf
│   ├── alertmanager/alertmanager.yml
│   ├── prometheus/
│   │   ├── prometheus.yml
│   │   └── alerts/pool.yml
│   ├── grafana/provisioning/
│   │   ├── datasources/prometheus.yml
│   │   └── dashboards/
│   │       ├── dashboard.yml
│   │       ├── pool-overview.json
│   │       └── miner-detail.json
│   ├── loki/
│   │   ├── config.yml
│   │   └── promtail.yml
│   └── tor/torrc
│
└── infra/
    ├── docker/
    │   ├── gateway/Dockerfile[.dev]
    │   ├── manager/Dockerfile[.dev]
    │   ├── frontend/Dockerfile[.dev]
    │   ├── mocknode/Dockerfile
    │   └── tor/Dockerfile
    ├── scripts/
    │   ├── initdb.sql
    │   ├── pool-backup.sh
    │   ├── restore.sh
    │   ├── deploy.sh
    │   ├── provision.sh
    │   ├── setup-tls.sh
    │   ├── harden.sh
    │   ├── healthcheck.sh
    │   ├── install-services.sh
    │   └── generate-secrets.sh
    └── systemd/
        ├── p2pool-dashboard.service
        ├── p2pool-backup.service
        └── p2pool-backup.timer
```

---

## Go Conventions

- **Module path:** `github.com/acquiredl/xmr-p2pool-dashboard`
- **Go version:** 1.25+
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

**Core tables** (defined in `001_initial.sql`):
- `p2pool_shares` — indexed sidechain shares (mini/main)
- `p2pool_blocks` — P2Pool-found main chain blocks
- `miner_hashrate` — 15-min bucketed hashrate timeseries per miner

**Payment tables** (defined in `002_payments.sql`):
- `payments` — on-chain coinbase payments with fiat prices (atomic units, XMR/USD + XMR/CAD)

**Subscription tables** (defined in `003_subscriptions.sql`):
- Subscription tiers and XMR payment verification

See the migration files in `services/manager/pkg/db/migrations/` for full DDL.

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

## Implementation Status

All originally planned components have been implemented:

**Backend (Go) — complete:**
- `pkg/p2poolclient/` — typed HTTP client for P2Pool local API
- `pkg/monerod/` — JSON-RPC client for monerod
- `pkg/walletrpc/` — view-only wallet RPC client (subscription verification)
- `internal/p2pool/` — sidechain poller + indexer (30s poll, upserts shares/blocks)
- `internal/scanner/` — coinbase scanner + price oracle (CoinGecko, historical backfill)
- `internal/aggregator/` — 15-min bucketed hashrate timeseries
- `internal/subscription/` — XMR subscription payment verification system
- `internal/ws/` — WebSocket hub for live hashrate push
- `internal/cache/` — Redis caching layer
- `internal/metrics/` — Prometheus instrumentation
- `cmd/manager/routes.go` — all HTTP handlers implemented (673 lines)
- 3 DB migrations (initial schema, payments, subscriptions)
- Gateway: JWT auth, rate limiting, WebSocket proxy

**Frontend (Next.js 14) — complete:**
- All 6 pages: home, miner dashboard, blocks, sidechain, admin, subscribe
- All 10 components including Navigation, SubscriptionStatus, SubscriptionPayment
- Subscription page: tier display, payment subaddress, payment history, API key generation
- Miner page: subscription tier badge + upgrade CTA for free-tier users
- Typed API client + WebSocket hook
- Full test suite (17 test files)

**Infrastructure — complete:**
- Docker: 5 services (manager, gateway, frontend, mocknode, tor) with dev variants
- Compose: prod, dev, and test configurations
- Monitoring: Prometheus + alerts, Grafana (pool-overview + miner-detail), Loki, Alertmanager
- Deployment: VPS provisioning, systemd units, TLS, backup/restore, hardening scripts
- CI/CD: GitHub Actions (deploy + security scanning + frontend tests), Dependabot, CODEOWNERS
- Tor hidden service
- Alertmanager webhook authenticated via Bearer token (credentials_file from Docker secret)

**Test coverage:** 16 Go test files (unit + integration), 17 frontend test files, mocknode for local E2E

**Potential future work:**
- Live validation against a production P2Pool node (currently tested against mocknode only)
- Main sidechain support (data layer is sidechain-agnostic, currently mini only)

---

## Common Tasks for Claude Code

**Add a new API endpoint:**
1. Add route in `services/manager/cmd/manager/routes.go`
2. Implement handler — either in `internal/aggregator/` or a new `internal/` package
3. Add corresponding types to `frontend/lib/api.ts`
4. Wire up frontend page or component

**Add a new DB table:**
1. Create migration in `services/manager/pkg/db/migrations/`
2. Number it sequentially (next: `004_name.sql`)
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
- Spending wallet RPC (view-only wallet RPC exists for subscription verification — never custodying miner funds)
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
View-only wallet verification implemented (`internal/subscription/`, `pkg/walletrpc/`). Users send XMR to a known address; the system verifies payments via view-only wallet RPC. Manual email-based txid verification remains available as fallback.
Tor hidden service
Add it. One extra Docker container, no code changes, strong trust signal for the target audience. Offered as an opt-in alternative URL, not the primary one. Document it in the README.

---

## Citadel Harness

This project uses the [Citadel](https://github.com/SethGammon/Citadel) agent
orchestration harness. Configuration is in `.claude/harness.json`.

- 25 skills registered — run `/do --list` to see all
- Hooks configured in `.claude/settings.json`
- Campaign/fleet state in `.planning/`
