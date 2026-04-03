# Architecture: Multi-Sidechain Support (Mini + Main)

> Date: 2026-04-02
> Mode: Feature (existing codebase)
> Branch: `feature/multi-sidechain`
> Merge target: `dev` (when demand justifies VPS upgrade)

## Context

SideWatch currently runs a single P2Pool sidechain (mini). This architecture
adds support for running **mini and main simultaneously** on one VPS, with
the system designed so adding a third sidechain later is trivial.

**Important:** P2Pool only supports two sidechains — mini and main. There is
no "nano" sidechain. If one is ever created, this architecture accommodates
it with zero code changes (config-only).

## Deployment Strategy

**Initial launch: mini only.** The current DigitalOcean droplet (8 GB / 4 vCPU)
cannot run both sidechains. This feature branch stays parked until there is
user demand for main sidechain support, at which point:

1. Upgrade droplet to 16 GB / 8 vCPU ($48→$96/mo on DO, or equivalent)
2. Merge `feature/multi-sidechain` into `dev`
3. Deploy with `P2POOL_SIDECHAINS=mini:...,main:...`

Until then, `dev` and `main` branches run mini-only as today.

## File Tree

New and modified files only. `~` = modified, `+` = new.

```
~ services/manager/cmd/manager/config.go              (multi-sidechain config parsing)
~ services/manager/cmd/manager/main.go                 (spawn N indexers + N timeseries builders)
~ services/manager/cmd/manager/routes.go               (add ?sidechain= query param to endpoints)
~ services/manager/internal/p2pool/client.go           (no change — already sidechain-parameterized)
~ services/manager/internal/p2pool/indexer.go          (no change — already takes Service per sidechain)
~ services/manager/internal/aggregator/aggregator.go   (accept sidechain param per query, not constructor)
~ services/manager/internal/aggregator/timeseries.go   (no change — already takes sidechain param)
~ services/manager/internal/ws/hub.go                  (broadcast per-sidechain stats)
~ services/manager/internal/ws/handler.go              (accept ?sidechain= on WS upgrade)
~ docker-compose.node.yml                              (add p2pool-main service, shared monerod)
~ docker-compose.test.yml                              (mocknode serves both sidechains)
~ config/nginx/nginx.conf                              (stratum ports: 3333 mini, 3334 main)
~ config/prometheus/prometheus.yml                     (scrape targets unchanged — single manager)
~ .env.example                                         (new multi-sidechain env vars)
~ frontend/lib/api.ts                                  (add sidechain query param to fetch calls)
~ frontend/app/layout.tsx                              (SidechainProvider context)
~ frontend/app/page.tsx                                (use sidechain context)
~ frontend/app/miner/page.tsx                          (use sidechain context)
~ frontend/app/blocks/page.tsx                         (use sidechain context)
~ frontend/app/sidechain/page.tsx                      (use sidechain context)
~ frontend/components/Navigation.tsx                   (add sidechain selector dropdown)
~ frontend/components/LiveStats.tsx                    (use sidechain from context)
~ frontend/components/SidechainTable.tsx               (use sidechain from context)
~ frontend/components/ShareTimeCalculator.tsx          (use sidechain from context)
~ frontend/lib/useWebSocket.ts                         (pass sidechain to WS URL)
+ frontend/lib/SidechainContext.tsx                    (React context for active sidechain)
+ services/mocknode/sidechain.go                       (mocknode multi-sidechain responses)
~ infra/scripts/provision.sh                           (update resource recommendations in comments)
```

## Component Breakdown

### Feature: Multi-Sidechain Backend Config
- Files: `config.go`, `main.go`
- Dependencies: None
- Complexity: Medium
- Design: Parse `P2POOL_SIDECHAINS=mini=http://p2pool-mini:3333,main=http://p2pool-main:3333`
  into a slice of `SidechainConfig{Name, APIURL}`. For each entry, create a
  separate `p2poolclient.Client`, `p2pool.Service`, `p2pool.Indexer`, and
  `aggregator.TimeseriesBuilder`. All share the same DB pool, Redis, and monerod.

### Feature: Multi-Poller Indexing
- Files: `main.go`, `indexer.go` (no changes needed to indexer itself)
- Dependencies: Multi-sidechain config
- Complexity: Low
- Design: `main.go` loops over sidechain configs, spawning one indexer goroutine
  per sidechain. Each indexer already tags data with its sidechain string.
  The DB schema already has `sidechain VARCHAR(10)` on all tables.

### Feature: API Sidechain Filtering
- Files: `routes.go`, `aggregator.go`
- Dependencies: Multi-sidechain config
- Complexity: Medium
- Design: All data endpoints accept `?sidechain=mini` query param.
  Default behavior: returns data for the first configured sidechain (mini).
  New endpoint `GET /api/sidechains` returns list of active sidechains.
  Aggregator methods change from using a stored sidechain string to accepting
  it as a parameter per call.

### Feature: WebSocket Per-Sidechain Broadcast
- Files: `hub.go`, `handler.go`, `useWebSocket.ts`
- Dependencies: Multi-sidechain config
- Complexity: Medium
- Design: WS upgrade URL becomes `/ws/pool/stats?sidechain=mini`. Hub maintains
  one broadcast channel per sidechain. Frontend WebSocket hook passes the
  active sidechain from context.

### Feature: Frontend Sidechain Selector
- Files: `SidechainContext.tsx`, `Navigation.tsx`, `api.ts`, all pages
- Dependencies: API sidechain filtering
- Complexity: Medium
- Design: React context holds `activeSidechain` state, initialized from
  `?sidechain=` URL query param (fallback: first in server-provided list).
  Navigation gets a dropdown selector. All `useSWR` calls include the
  sidechain in the fetch URL. Changing sidechain updates the URL query
  param for shareability.

### Feature: Docker Multi-Node Infrastructure
- Files: `docker-compose.node.yml`, `nginx.conf`, `.env.example`
- Dependencies: None (parallel with backend work)
- Complexity: Low
- Design: One shared monerod, two P2Pool services (p2pool-mini, p2pool-main).
  Nginx streams stratum: port 3333 → mini, port 3334 → main. Manager
  depends_on both P2Pool services.

### Feature: Mocknode Multi-Sidechain
- Files: `mocknode/sidechain.go`, `docker-compose.test.yml`
- Dependencies: None (parallel with backend work)
- Complexity: Low
- Design: Mocknode listens on two ports (3333 for mini, 3334 for main) or
  uses path-based routing. Returns different sidechain difficulty values
  per sidechain for realistic testing.

## Data Model

No schema changes needed. All tables already have `sidechain VARCHAR(10)`:

| Table | Sidechain Column | Status |
|-------|-----------------|--------|
| `p2pool_shares` | `sidechain VARCHAR(10) NOT NULL` | Exists |
| `p2pool_blocks` | `sidechain VARCHAR(10) NOT NULL` | Exists |
| `miner_hashrate` | `sidechain VARCHAR(10) NOT NULL` | Exists |
| `payments` | (linked via block) | No change |

All queries already filter `WHERE sidechain = $1`. No migration needed.

## Key Decisions

### 1. Single Manager with N Pollers (not multiple manager instances)
- **Chosen**: Single manager binary, one indexer goroutine per sidechain — because
  it uses half the RAM (~512MB vs ~1GB), enables native cross-sidechain queries,
  and is one service to operate/monitor. The DB is already sidechain-tagged so
  multiple pollers writing to the same tables is safe.
- **Rejected**: Separate manager instance per sidechain — doubles RAM and
  operational complexity. Cross-sidechain stats (e.g. "total hashrate across
  all sidechains") would require external aggregation.

### 2. Context Selector with Query Param (not URL-based routing)
- **Chosen**: React context + `?sidechain=mini` query param — because it
  preserves all existing routes, low migration cost, single-dashboard UX
  matches the product model. Query param makes URLs shareable.
- **Rejected**: URL-based routing (`/mini/blocks`, `/main/blocks`) — requires
  restructuring every route and component. Feels like separate apps rather
  than one dashboard. Higher complexity for minimal benefit.

### 3. Shared Monerod (not one per sidechain)
- **Chosen**: All P2Pool instances share one monerod — because monerod is
  sidechain-agnostic. It provides block templates and ZMQ events identically
  regardless of which P2Pool sidechain connects. Saves 4GB RAM and 2 CPUs.
- **Rejected**: Separate monerod per sidechain — would cost 8-12GB extra RAM
  for zero benefit. P2Pool nodes already share a single monerod in the
  reference architecture.

### 4. Config Format: Structured Env Var (not separate env vars per sidechain)
- **Chosen**: `P2POOL_SIDECHAINS=mini=http://p2pool-mini:3333,main=http://p2pool-main:3333`
  — single env var, scales to N sidechains, backwards-compatible (falls back
  to `P2POOL_API_URL` + `P2POOL_SIDECHAIN` if `P2POOL_SIDECHAINS` is unset).
- **Rejected**: `P2POOL_MINI_URL`, `P2POOL_MAIN_URL`, etc. — doesn't scale,
  requires code changes for each new sidechain.

## VPS Resource Budget

All P2Pool instances share ONE monerod. This is the critical optimization.

| Component | RAM (limit) | CPU (limit) | Notes |
|-----------|-------------|-------------|-------|
| monerod (shared) | 4 GB | 2.0 | Unchanged — sidechain-agnostic |
| p2pool-mini | 1 GB | 1.0 | Unchanged |
| p2pool-main | 1.5 GB | 1.0 | Main has more peers, slightly more RAM |
| manager | 512 MB | 1.0 | +2 goroutines, negligible increase |
| gateway | 256 MB | 0.5 | Unchanged |
| frontend | 256 MB | 0.5 | Unchanged |
| postgres | 1 GB | 1.0 | ~2x write volume, same limits |
| redis | 256 MB | 0.25 | ~2x cache keys, same limits |
| nginx | 128 MB | 0.25 | +1 stratum upstream |
| monitoring stack | 512 MB | 0.5 | Unchanged |
| **Total** | **~9.4 GB** | **~8.0** | |

### VPS Sizing

| Tier | Spec | Fits? |
|------|------|-------|
| 8 GB / 4 vCPU | Current setup (mini only) | Yes (tight) |
| 16 GB / 8 vCPU | Mini + main | **Minimum** — leaves ~6 GB headroom |
| 32 GB / 8 vCPU | Mini + main + growth | **Recommended** — comfortable for peak load, chain growth, and potential 3rd sidechain |

### Disk

No change. One monerod (pruned ~70 GB, grows ~1 GB/month) serves both sidechains.
P2Pool sidechain data is negligible (<1 GB per sidechain).

### Postgres Write Volume

Doubles (two indexers polling at 30s intervals). Current write rate is low
(~120 rows/min for shares). At 240 rows/min with indexes, Postgres 15 handles
this trivially within the 1 GB memory limit.

## Build Phases

### Phase 0: Baseline
- **Goal**: Record current test/build state before changes
- **Files**: None
- **Dependencies**: None
- **End Conditions**:
  - [ ] `cd services/manager && go test -race ./...` passes (record count)
  - [ ] `cd services/gateway && go test -race ./...` passes
  - [ ] `cd frontend && npm test` passes (record count)
  - [ ] `docker compose -f docker-compose.yml -f docker-compose.test.yml up -d` starts clean

### Phase 1: Multi-Sidechain Backend Config + Pollers
- **Goal**: Manager reads N sidechain configs and spawns one indexer per sidechain
- **Files**: `config.go`, `main.go`
- **Dependencies**: None
- **End Conditions**:
  - [ ] `P2POOL_SIDECHAINS` env var parsed into `[]SidechainConfig`
  - [ ] Falls back to `P2POOL_API_URL` + `P2POOL_SIDECHAIN` when `P2POOL_SIDECHAINS` is unset
  - [ ] One `p2pool.Indexer` goroutine running per configured sidechain
  - [ ] One `aggregator.TimeseriesBuilder` goroutine per sidechain
  - [ ] Startup log shows all configured sidechains
  - [ ] `go test -race ./...` passes with no new failures
  - [ ] Manager starts successfully with both old (single) and new (multi) config formats

### Phase 2: API Sidechain Filtering
- **Goal**: All data endpoints accept `?sidechain=` and a new `/api/sidechains` lists them
- **Files**: `routes.go`, `aggregator.go`
- **Dependencies**: Phase 1
- **End Conditions**:
  - [ ] `GET /api/sidechains` returns `["mini","main"]`
  - [ ] `GET /api/pool/stats?sidechain=mini` returns mini stats only
  - [ ] `GET /api/pool/stats?sidechain=main` returns main stats only
  - [ ] `GET /api/pool/stats` (no param) returns first configured sidechain (backwards-compatible)
  - [ ] All data endpoints (`/api/miner/*`, `/api/blocks`, `/api/sidechain/*`) accept `?sidechain=`
  - [ ] `go test -race ./...` passes

### Phase 3: WebSocket Per-Sidechain Broadcast
- **Goal**: WS clients subscribe to a specific sidechain's live stats
- **Files**: `hub.go`, `handler.go`
- **Dependencies**: Phase 1
- **End Conditions**:
  - [ ] `/ws/pool/stats?sidechain=mini` receives only mini stats
  - [ ] `/ws/pool/stats?sidechain=main` receives only main stats
  - [ ] `/ws/pool/stats` (no param) defaults to first sidechain
  - [ ] Hub integration test passes with multi-sidechain broadcast

### Phase 4: Docker Infrastructure
- **Goal**: docker-compose.node.yml runs mini + main P2Pool on shared monerod
- **Files**: `docker-compose.node.yml`, `nginx.conf`, `.env.example`
- **Dependencies**: None (parallel with Phases 1-3)
- **End Conditions**:
  - [ ] `docker compose -f docker-compose.yml -f docker-compose.node.yml config` validates
  - [ ] p2pool-mini starts on port 3333 (stratum) / 37888 (P2P)
  - [ ] p2pool-main starts on port 3334 (stratum) / 37889 (P2P)
  - [ ] Both P2Pool instances connect to shared monerod
  - [ ] nginx streams stratum traffic to correct P2Pool by port
  - [ ] Manager `P2POOL_SIDECHAINS` env wired to both P2Pool API URLs
  - [ ] Resource limits set: p2pool-main at 1.5G / 1.0 CPU

### Phase 5: Frontend Sidechain Selector
- **Goal**: Users can switch between sidechains in the dashboard
- **Files**: `SidechainContext.tsx`, `Navigation.tsx`, `api.ts`, `useWebSocket.ts`, all pages
- **Dependencies**: Phase 2 (API filtering), Phase 3 (WS filtering)
- **End Conditions**:
  - [ ] Sidechain dropdown in Navigation shows available sidechains (fetched from `/api/sidechains`)
  - [ ] Selecting a sidechain updates `?sidechain=` in URL
  - [ ] All data components re-fetch with new sidechain
  - [ ] WebSocket reconnects to new sidechain's stream
  - [ ] Page refresh preserves selected sidechain from URL param
  - [ ] Default is "mini" when no param present
  - [ ] `npm test` passes with no new failures

### Phase 6: Mocknode + Testing
- **Goal**: E2E test coverage for multi-sidechain flow
- **Files**: `mocknode/sidechain.go`, `docker-compose.test.yml`, integration tests
- **Dependencies**: Phases 1-5
- **End Conditions**:
  - [ ] Mocknode serves distinct data per sidechain (different difficulties, different share sets)
  - [ ] Integration tests verify indexer writes for both sidechains
  - [ ] Integration tests verify API returns correct data per sidechain filter
  - [ ] `docker compose -f docker-compose.yml -f docker-compose.test.yml up` runs clean
  - [ ] All existing tests still pass (zero regressions)

## Phase Dependency Graph

```
Phase 0 (Baseline)
    ├── Phase 1 (Backend Config) ──→ Phase 2 (API Filtering) ──→ Phase 5 (Frontend)
    │                             ──→ Phase 3 (WebSocket)     ──→ Phase 5 (Frontend)
    └── Phase 4 (Docker Infra)   ─────────────────────────────→ Phase 6 (Testing)
                                                                     ↑
Phase 1 + 2 + 3 + 4 + 5 ────────────────────────────────────────────┘
```

Phases 1 and 4 can run in parallel.
Phases 2 and 3 can run in parallel (both depend on 1).
Phase 5 depends on 2 and 3.
Phase 6 depends on all previous phases.

## Risk Register

1. **P2Pool main requires significantly more RAM than estimated**: Main sidechain
   has higher difficulty and more peers. Mitigation: set p2pool-main memory limit
   to 1.5G initially, monitor with Prometheus, increase to 2G if needed. The 16GB
   VPS has headroom.

2. **Backwards-compatibility break in config**: Existing deployments use
   `P2POOL_API_URL` + `P2POOL_SIDECHAIN`. Mitigation: fallback logic — if
   `P2POOL_SIDECHAINS` is unset, construct single-entry list from legacy vars.
   Zero breaking changes for existing deployments.

3. **Regression in existing functionality**: Two pollers writing to the same
   tables could cause unexpected interactions. Mitigation: Phase 0 baseline
   recording, all phases require "existing tests pass" as end condition, and
   the DB schema already isolates by sidechain column with appropriate unique
   constraints.

4. **VPS undersized for production load**: Resource estimates are based on
   Docker limits, not actual peak usage. Mitigation: deploy on 16GB VPS first,
   monitor for 48h via Grafana dashboards before committing to the spec. Upgrade
   path to 32GB is a single-command resize on DigitalOcean.
