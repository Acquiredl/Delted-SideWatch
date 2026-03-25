# Architecture: XMR P2Pool Dashboard

> Source: CLAUDE.md (serves as PRD — comprehensive spec with schema, API contracts, file tree)
> Date: 2026-03-24
> Mode: Greenfield

## File Tree

```
xmr-p2pool-dashboard/
├── CLAUDE.md
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
│   ├── gateway/
│   │   ├── go.mod
│   │   ├── go.sum
│   │   ├── cmd/gateway/
│   │   │   ├── main.go              ← entry point, wires auth + proxy + middleware
│   │   │   └── config.go            ← env-based config with mustGetEnv()
│   │   └── internal/
│   │       ├── auth/
│   │       │   └── jwt.go           ← JWT validation middleware (HS256)
│   │       ├── middleware/
│   │       │   ├── ratelimit.go     ← token-bucket per IP via Redis
│   │       │   ├── logger.go        ← slog request logging
│   │       │   └── requestid.go     ← X-Request-ID propagation
│   │       └── proxy/
│   │           └── proxy.go         ← httputil.ReverseProxy to manager
│   │
│   └── manager/
│       ├── go.mod
│       ├── go.sum
│       ├── cmd/manager/
│       │   ├── main.go              ← entry point, starts all subsystems
│       │   ├── routes.go            ← HTTP handler registration
│       │   └── config.go            ← env-based config
│       ├── internal/
│       │   ├── p2pool/
│       │   │   ├── client.go        ← HTTP client wrapping p2poolclient pkg
│       │   │   ├── indexer.go       ← 30s poll loop, upserts shares + blocks
│       │   │   └── types.go         ← internal domain types
│       │   ├── scanner/
│       │   │   ├── scanner.go       ← ZMQ listener + block processing loop
│       │   │   ├── coinbase.go      ← extracts outputs, matches miner addresses
│       │   │   └── priceoracle.go   ← CoinGecko XMR/USD + XMR/CAD
│       │   ├── aggregator/
│       │   │   ├── aggregator.go    ← miner stats queries
│       │   │   └── timeseries.go    ← 15-min bucket rollup from shares
│       │   ├── events/
│       │   │   └── zmq.go           ← ZMQ subscriber for monerod block events
│       │   ├── metrics/
│       │   │   └── metrics.go       ← Prometheus counters/gauges/histograms
│       │   └── cache/
│       │       └── cache.go         ← Redis get/set/invalidate helpers
│       └── pkg/
│           ├── db/
│           │   ├── db.go            ← pgx pool init + health check
│           │   └── migrations/
│           │       ├── 001_initial.sql    ← p2pool_shares, p2pool_blocks, miner_hashrate
│           │       └── 002_payments.sql   ← payments table
│           ├── monerod/
│           │   ├── client.go        ← JSON-RPC client (get_block, get_transactions, etc.)
│           │   └── types.go         ← RPC request/response structs
│           └── p2poolclient/
│               ├── client.go        ← typed HTTP client for P2Pool local API
│               └── types.go         ← PoolStats, Share, FoundBlock, WorkerStats structs
│
├── frontend/
│   ├── package.json
│   ├── tsconfig.json
│   ├── next.config.js
│   ├── tailwind.config.ts
│   ├── postcss.config.js
│   ├── app/
│   │   ├── layout.tsx               ← root layout with nav, dark theme
│   │   ├── globals.css              ← Tailwind base + custom vars
│   │   ├── page.tsx                 ← pool stats home (hashrate, miners, blocks)
│   │   ├── miner/page.tsx           ← miner dashboard (address lookup)
│   │   ├── blocks/page.tsx          ← block explorer table
│   │   ├── sidechain/page.tsx       ← P2Pool sidechain viewer
│   │   └── admin/page.tsx           ← JWT-protected admin panel
│   ├── components/
│   │   ├── LiveStats.tsx            ← WebSocket real-time pool hashrate
│   │   ├── HashrateChart.tsx        ← Recharts area chart (miner + pool)
│   │   ├── BlocksTable.tsx          ← paginated blocks found by P2Pool
│   │   ├── PaymentsTable.tsx        ← miner payment history
│   │   ├── WorkersTable.tsx         ← active workers for an address
│   │   ├── SidechainTable.tsx       ← recent sidechain shares
│   │   └── PrivacyNotice.tsx        ← coinbase transparency warning banner
│   └── lib/
│       ├── api.ts                   ← typed fetch client for gateway endpoints
│       └── useWebSocket.ts          ← reconnecting WS hook for live data
│
├── config/
│   ├── nginx/nginx.conf             ← TLS termination, rate limiting, proxy to gateway
│   ├── prometheus/
│   │   ├── prometheus.yml           ← scrape config for manager + gateway
│   │   └── alerts/pool.yml          ← alerting rules (hashrate drop, missed blocks)
│   ├── grafana/provisioning/
│   │   ├── datasources/
│   │   │   └── prometheus.yml       ← auto-provision Prometheus datasource
│   │   └── dashboards/
│   │       ├── dashboard.yml        ← provisioner config
│   │       └── pool-overview.json   ← pool + miner dashboard panels
│   └── loki/
│       ├── config.yml               ← Loki storage config
│       └── promtail.yml             ← log collection from Docker containers
│
└── infra/
    ├── docker/
    │   ├── gateway/Dockerfile        ← multi-stage Go build, non-root
    │   ├── gateway/Dockerfile.dev    ← air live-reload
    │   ├── manager/Dockerfile        ← multi-stage Go build, non-root
    │   └── manager/Dockerfile.dev    ← air live-reload
    └── scripts/
        ├── initdb.sql                ← creates manager_user role, grants
        └── pool-backup.sh            ← pg_dump cron script
```

## Component Breakdown

### Feature: P2Pool Sidechain Indexing
- Files: `pkg/p2poolclient/*`, `internal/p2pool/*`, `pkg/db/*`
- Dependencies: Database layer, P2Pool client
- Complexity: **medium** — straightforward polling but needs upsert-on-conflict logic and sidechain-agnostic design

### Feature: Coinbase Scanner
- Files: `pkg/monerod/*`, `internal/scanner/scanner.go`, `internal/scanner/coinbase.go`, `internal/events/zmq.go`
- Dependencies: Monerod client, database layer, ZMQ event listener
- Complexity: **high** — ZMQ integration, coinbase output parsing, 10-confirmation depth buffer, address matching

### Feature: Aggregation + Timeseries
- Files: `internal/aggregator/*`, `internal/scanner/priceoracle.go`
- Dependencies: Indexed shares in DB (from indexer), price data
- Complexity: **medium** — SQL-heavy bucketing, CoinGecko rate limiting

### Feature: REST API
- Files: `cmd/manager/routes.go`, `internal/cache/cache.go`, `internal/metrics/metrics.go`
- Dependencies: All internal packages (aggregator, indexer, scanner)
- Complexity: **medium** — many endpoints but each is a thin query + serialize layer

### Feature: API Gateway
- Files: `services/gateway/*`
- Dependencies: Manager API running
- Complexity: **low** — stdlib reverse proxy + JWT middleware + rate limiter

### Feature: Frontend Dashboard
- Files: `frontend/*`
- Dependencies: Gateway API accessible
- Complexity: **medium** — 5 pages, 7 components, WebSocket integration, Recharts

### Feature: Observability Stack
- Files: `config/*`, `internal/metrics/metrics.go`
- Dependencies: Running services to scrape
- Complexity: **low** — config files, no custom code beyond metric registration

## Data Model

### p2pool_shares
- Fields: id (BIGSERIAL PK), sidechain (VARCHAR 10), miner_address (VARCHAR 256), worker_name (VARCHAR 128 nullable), sidechain_height (BIGINT), difficulty (BIGINT), created_at (TIMESTAMPTZ)
- Indexes: (miner_address, sidechain), (sidechain_height), (created_at)
- Relationships: Miner address links to payments, miner_hashrate

### p2pool_blocks
- Fields: id (BIGSERIAL PK), main_height (BIGINT UNIQUE), main_hash (VARCHAR 64), sidechain_height (BIGINT), coinbase_reward (BIGINT), effort (NUMERIC 10,4), found_at (TIMESTAMPTZ)
- Indexes: (main_height UNIQUE), (found_at)
- Relationships: main_height/main_hash links to payments

### payments
- Fields: id (BIGSERIAL PK), miner_address (VARCHAR 256), amount (BIGINT), main_height (BIGINT), main_hash (VARCHAR 64), xmr_usd_price (NUMERIC 12,4), xmr_cad_price (NUMERIC 12,4), created_at (TIMESTAMPTZ)
- Indexes: (miner_address, created_at), (main_height)
- Relationships: miner_address → p2pool_shares; main_height → p2pool_blocks

### miner_hashrate
- Fields: id (BIGSERIAL PK), miner_address (VARCHAR 256), sidechain (VARCHAR 10), hashrate (BIGINT), bucket_time (TIMESTAMPTZ)
- Indexes: UNIQUE (miner_address, sidechain, bucket_time)
- Relationships: miner_address → p2pool_shares

## Key Decisions

### ZMQ Library: `github.com/go-zeromq/zmq4` (pure Go)
- **Chosen**: Pure Go ZMQ4 — because it has no CGO dependency, simplifies Docker builds, and works on all platforms without libzmq. Adequate for subscribing to monerod block notifications (low throughput).
- **Rejected**: `pebbe/zmq4` (CGO binding) — requires libzmq-dev in build container, complicates cross-compilation, overkill for our simple SUB socket use case.

### Frontend Data Fetching: SWR
- **Chosen**: `swr` — because it's lightweight (~4KB), pairs naturally with Next.js (same Vercel ecosystem), and the stale-while-revalidate pattern is perfect for dashboard polling. No need for mutation/cache invalidation complexity.
- **Rejected**: `@tanstack/react-query` — more powerful but heavier, mutation features unused since the frontend is read-only. Adds concepts (query keys, hydration) that over-complicate a simple dashboard.
- **Rejected**: Plain `fetch` in useEffect — no caching, no deduplication, manual loading/error state.

### Frontend Styling: Tailwind CSS
- **Chosen**: Tailwind CSS — already standard for Next.js projects, utility-first approach keeps components self-contained, dark mode built-in.
- **Rejected**: CSS Modules — more setup, less velocity for a dashboard with many similar table/card components.

### Manager-to-Gateway Communication: Plain HTTP on Docker network
- **Chosen**: Direct HTTP — because both services run on the same Docker network, TLS termination happens at nginx, and gRPC adds protobuf complexity for what are simple REST queries.
- **Rejected**: gRPC — protobuf schema maintenance overhead, no streaming need between gateway and manager.

### Confirmation Depth Strategy: 10-block buffer (from CLAUDE.md decision)
- **Chosen**: Don't record payments as final until Monero block is 10 confirmations deep. Scanner tracks pending blocks and promotes them after depth threshold.
- **Rejected**: Reorg detection + rollback — complex state machine, race conditions, not worth it when depth buffer eliminates 99%+ of reorg risk.

## Build Phases

### Phase 1: Project Foundation
- **Goal**: Scaffold the complete directory structure, Go modules, Docker infrastructure, and database layer
- **Files**:
  - `services/manager/go.mod`, `services/gateway/go.mod`
  - `services/manager/cmd/manager/config.go`
  - `services/manager/pkg/db/db.go`
  - `services/manager/pkg/db/migrations/001_initial.sql`
  - `services/manager/pkg/db/migrations/002_payments.sql`
  - `infra/scripts/initdb.sql`
  - `infra/docker/manager/Dockerfile`, `infra/docker/manager/Dockerfile.dev`
  - `infra/docker/gateway/Dockerfile`, `infra/docker/gateway/Dockerfile.dev`
  - `docker-compose.yml`, `docker-compose.dev.yml`
  - `Makefile`
  - `.env.example`, `.gitignore`, `.golangci.yml`
- **Dependencies**: none
- **End Conditions**:
  - [ ] `cd services/manager && go build ./cmd/manager/` compiles with zero errors
  - [ ] `cd services/gateway && go build ./cmd/gateway/` compiles with zero errors
  - [ ] SQL migrations are valid (parseable, no syntax errors)
  - [ ] `docker compose config` validates without errors
  - [ ] `.env.example` contains all environment variables from CLAUDE.md

### Phase 2: External Clients
- **Goal**: Implement typed HTTP clients for P2Pool local API and monerod JSON-RPC
- **Files**:
  - `services/manager/pkg/p2poolclient/types.go`
  - `services/manager/pkg/p2poolclient/client.go`
  - `services/manager/pkg/monerod/types.go`
  - `services/manager/pkg/monerod/client.go`
- **Dependencies**: Phase 1 (go.mod must exist)
- **End Conditions**:
  - [ ] `go build ./pkg/p2poolclient/` compiles
  - [ ] `go build ./pkg/monerod/` compiles
  - [ ] `go vet ./pkg/...` passes
  - [ ] P2Pool client covers: GetPoolStats, GetShares, GetFoundBlocks, GetWorkerStats, GetPeers
  - [ ] Monerod client covers: GetLastBlockHeader, GetBlockByHeight, GetBlock, GetTransactions
  - [ ] All methods return typed structs, not raw JSON
  - [ ] All errors wrapped with `fmt.Errorf("context: %w", err)`
  - [ ] Unit tests exist for response parsing (table-driven with sample JSON)

### Phase 3: Core Indexing Pipeline
- **Goal**: Implement the P2Pool sidechain indexer and the on-chain coinbase scanner — the two data pipelines that populate the database
- **Files**:
  - `services/manager/internal/events/zmq.go`
  - `services/manager/internal/p2pool/types.go`
  - `services/manager/internal/p2pool/client.go`
  - `services/manager/internal/p2pool/indexer.go`
  - `services/manager/internal/scanner/scanner.go`
  - `services/manager/internal/scanner/coinbase.go`
  - `services/manager/internal/cache/cache.go`
- **Dependencies**: Phase 1 (DB layer), Phase 2 (external clients)
- **End Conditions**:
  - [ ] `go build ./internal/...` compiles
  - [ ] `go vet ./internal/...` passes
  - [ ] Indexer polls P2Pool API on configurable interval (default 30s)
  - [ ] Indexer upserts shares with ON CONFLICT handling (idempotent)
  - [ ] Indexer upserts found blocks with ON CONFLICT on main_height
  - [ ] Scanner subscribes to ZMQ block events from monerod
  - [ ] Scanner implements 10-confirmation depth buffer before recording payments
  - [ ] Coinbase parser extracts outputs and matches against known miner addresses
  - [ ] Redis cache helpers implement get/set with TTL and invalidation
  - [ ] Unit tests for coinbase output parsing logic

### Phase 4: Aggregation + Price Oracle
- **Goal**: Build the timeseries aggregation layer and price data fetching
- **Files**:
  - `services/manager/internal/aggregator/aggregator.go`
  - `services/manager/internal/aggregator/timeseries.go`
  - `services/manager/internal/scanner/priceoracle.go`
- **Dependencies**: Phase 3 (indexed data must exist in DB)
- **End Conditions**:
  - [ ] `go build ./internal/aggregator/` compiles
  - [ ] Timeseries rollup buckets shares into 15-min intervals per miner
  - [ ] Aggregator provides: GetMinerStats, GetPoolStats, GetMinerHashrateHistory
  - [ ] Price oracle fetches XMR/USD and XMR/CAD from CoinGecko
  - [ ] Price oracle respects rate limits (caches for ≥60s)
  - [ ] Unit tests for bucket time truncation logic

### Phase 5: REST API + Metrics
- **Goal**: Wire up all HTTP handlers, Prometheus metrics, and the manager entry point
- **Files**:
  - `services/manager/cmd/manager/routes.go`
  - `services/manager/cmd/manager/main.go`
  - `services/manager/internal/metrics/metrics.go`
- **Dependencies**: Phase 4 (aggregator provides query layer)
- **End Conditions**:
  - [ ] `go build ./cmd/manager/` compiles
  - [ ] `go test ./...` passes for all manager packages
  - [ ] GET /health returns 200
  - [ ] GET /api/pool/stats returns pool statistics
  - [ ] GET /api/miner/:address returns miner stats
  - [ ] GET /api/miner/:address/payments returns payment history
  - [ ] GET /api/miner/:address/hashrate returns timeseries data
  - [ ] GET /api/blocks returns found blocks (paginated)
  - [ ] GET /api/sidechain/shares returns recent shares (paginated)
  - [ ] GET /api/miner/:address/tax-export returns CSV
  - [ ] Prometheus metrics endpoint on configured port
  - [ ] All handlers use parameterized SQL queries (no string interpolation)

### Phase 6: API Gateway
- **Goal**: Implement the gateway service with JWT auth, rate limiting, and reverse proxy
- **Files**:
  - `services/gateway/cmd/gateway/main.go`
  - `services/gateway/cmd/gateway/config.go`
  - `services/gateway/internal/auth/jwt.go`
  - `services/gateway/internal/middleware/ratelimit.go`
  - `services/gateway/internal/middleware/logger.go`
  - `services/gateway/internal/middleware/requestid.go`
  - `services/gateway/internal/proxy/proxy.go`
- **Dependencies**: Phase 5 (manager API must exist to proxy to)
- **End Conditions**:
  - [ ] `cd services/gateway && go build ./cmd/gateway/` compiles
  - [ ] `go test ./...` passes for all gateway packages
  - [ ] GET /health returns 200 (gateway's own health)
  - [ ] Unauthenticated requests to /admin/* return 401
  - [ ] Valid JWT passes through to manager
  - [ ] Rate limiter returns 429 after threshold exceeded
  - [ ] X-Request-ID header propagated on all requests
  - [ ] Request logging via slog with JSON handler

### Phase 7: Frontend Dashboard
- **Goal**: Build the complete Next.js frontend with all pages and components
- **Files**: All files under `frontend/`
- **Dependencies**: Phase 5 (API endpoints to fetch from)
- **End Conditions**:
  - [ ] `cd frontend && npm run build` succeeds
  - [ ] `npx tsc --noEmit` passes with zero errors
  - [ ] Home page renders pool stats (hashrate, miners, recent blocks)
  - [ ] Miner page accepts address input and displays stats, payments, hashrate chart
  - [ ] Blocks page displays paginated block explorer
  - [ ] Sidechain page displays recent shares
  - [ ] Admin page protected by JWT (redirect to login if no token)
  - [ ] WebSocket hook connects and receives live hashrate updates
  - [ ] HashrateChart renders area chart with Recharts
  - [ ] PrivacyNotice component displayed on miner lookup pages
  - [ ] All API calls use typed client from lib/api.ts
  - [ ] Responsive layout (mobile-friendly tables)

### Phase 8: Observability + Infrastructure
- **Goal**: Configure monitoring, logging, nginx, and finalize Docker setup
- **Files**:
  - `config/nginx/nginx.conf`
  - `config/prometheus/prometheus.yml`
  - `config/prometheus/alerts/pool.yml`
  - `config/grafana/provisioning/datasources/prometheus.yml`
  - `config/grafana/provisioning/dashboards/dashboard.yml`
  - `config/grafana/provisioning/dashboards/pool-overview.json`
  - `config/loki/config.yml`
  - `config/loki/promtail.yml`
  - `infra/scripts/pool-backup.sh`
  - `README.md`
  - `CHEATSHEET.md`
  - `SECURITY.md`
- **Dependencies**: Phase 5, Phase 6 (services must exist to scrape/proxy)
- **End Conditions**:
  - [ ] `docker compose config` validates
  - [ ] nginx proxies to gateway on port 443 (TLS) and 80 (redirect)
  - [ ] Prometheus scrapes manager metrics endpoint
  - [ ] Grafana auto-provisions Prometheus datasource and pool dashboard
  - [ ] Alert rules fire on: pool hashrate drops >50%, no blocks found in 24h
  - [ ] Promtail collects logs from all Docker containers
  - [ ] Backup script performs pg_dump to timestamped file
  - [ ] README documents setup, architecture, and environment variables
  - [ ] SECURITY.md documents security model and responsible disclosure

## Phase Dependency Graph

```
Phase 1 (Foundation) → Phase 2 (Clients) → Phase 3 (Indexing) → Phase 4 (Aggregation)
                                                                        ↓
                                                                 Phase 5 (API)
                                                                   ↓       ↓
                                                          Phase 6 (GW)  Phase 7 (Frontend)
                                                                   ↓       ↓
                                                                Phase 8 (Infra)
                                                           [Phase 6 + 7 parallel]
```

- Phases 1→2→3→4→5 are strictly sequential (each depends on the prior)
- Phases 6 and 7 can run **in parallel** after Phase 5
- Phase 8 runs after Phases 6 and 7 complete

## Risk Register

1. **ZMQ integration complexity**: Pure Go ZMQ4 library may have edge cases with monerod's ZMQ implementation. **Mitigation**: Implement a polling fallback (check `get_last_block_header` every 5s) that activates if ZMQ connection fails. Scanner should work with either event source.

2. **P2Pool API undocumented behavior**: P2Pool's local API is not formally documented — response shapes may vary between versions. **Mitigation**: Parse defensively with zero-value defaults for missing fields. Log warnings (not errors) on unexpected shapes. Pin P2Pool version in docker-compose.

3. **Coinbase output matching accuracy**: Monero's stealth addresses mean outputs are not directly linkable to addresses without the view key. P2Pool publishes miner addresses in the sidechain, but coinbase output matching depends on P2Pool's deterministic output construction. **Mitigation**: Cross-reference P2Pool's `/api/found_blocks` reward data with on-chain coinbase amounts. Use P2Pool's own address→output mapping rather than attempting independent derivation.

4. **CoinGecko rate limiting**: Free tier allows ~10-30 req/min. **Mitigation**: Cache prices aggressively (minimum 60s TTL), batch historical lookups, and gracefully degrade (null price fields) if rate limited.

## Deployment Strategy

- **Platform**: Docker Compose on self-managed infrastructure (VPS/dedicated server)
- **Method**: `docker compose up -d` with pull-based updates
- **Environment variables**: All listed in `.env.example` — secrets via Docker secrets with env fallback
- **Pre-deploy checks**: `go vet`, `go test -race`, `npm run build`, `tsc --noEmit`
- **Tor**: Separate `tor` container in docker-compose.yml with hidden service pointing to nginx
