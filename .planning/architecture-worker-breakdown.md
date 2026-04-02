# Architecture: Worker Breakdown Endpoint

> Source: README feature table ("Worker breakdown | -- | Yes") + existing WorkersTable component
> Date: 2026-03-31
> Mode: Feature (existing codebase)

## Correction: Tier-Gating Already Implemented

During research, it appeared that tier-gating on payments/hashrate was missing.
After reading the full handler code, this is **not the case**:

- `TierMiddleware` wraps the entire mux in `main.go:150`
- `handleMinerPayments` calls `TierFromContext()` + `EffectivePaymentLimit()` (routes.go:188-189)
- `handleMinerHashrate` calls `TierFromContext()` + `EffectiveHashrateHours()` (routes.go:228-229)

Free users are capped at 100 payments and 720 hours (30 days) of hashrate.
Paid users have no cap. The data retention pruner enforces the physical limit.

**No work needed for item 2.**

---

## Scope: Worker Breakdown (Item 1)

The README advertises "Worker breakdown" as a paid-tier feature. The data source
(`p2pool_shares.worker_name`) and the frontend component (`WorkersTable.tsx`)
both exist, but there is no backend query, no API endpoint, and no frontend wiring.

## File Tree

```
~ services/manager/internal/aggregator/aggregator.go      — add MinerWorker type + GetMinerWorkers()
~ services/manager/cmd/manager/routes.go                   — add GET /api/miner/{address}/workers
~ frontend/lib/api.ts                                      — add MinerWorker interface
~ frontend/app/miner/page.tsx                              — wire WorkersTable for paid subscribers
~ CLAUDE.md                                                — update endpoint list + implementation status
```

## Component Breakdown

### Feature: Worker Breakdown API
- Files: `aggregator.go`, `routes.go`
- Dependencies: `p2pool_shares` table with `worker_name` column (migration 001)
- Complexity: **low** — single aggregation query, follows existing handler pattern exactly

### Feature: Frontend Wiring
- Files: `api.ts`, `miner/page.tsx`
- Dependencies: Worker breakdown API endpoint
- Complexity: **low** — WorkersTable component already exists, just needs data fetch + conditional rendering

## Data Model

No schema changes. The `p2pool_shares` table already has:
- `miner_address VARCHAR(256)` — filter by miner
- `worker_name VARCHAR(128)` — group by worker
- `created_at TIMESTAMPTZ` — for "last share" timestamp
- Index: `idx_shares_miner_sidechain ON (miner_address, sidechain)` — covers the query

## Key Decisions

### Worker Data Source: Indexed `p2pool_shares` vs live P2Pool API
- **Chosen**: Query `p2pool_shares` grouped by `worker_name` — because the data
  is already indexed per-miner, respects retention policy (30d free / 15mo paid),
  and gives historical context (total shares per worker over time).
- **Rejected**: Live `GET /api/worker_stats` from P2Pool API — only shows the
  current PPLNS window, is pool-wide (not per-miner), and wouldn't respect
  subscription tier boundaries.

### Tier Gating: Hard gate (403) vs soft cap
- **Chosen**: `RequirePaid` middleware (hard 403 for free users) — consistent
  with the README ("-- | Yes") and with how tax-export is gated. The frontend
  conditionally renders the WorkersTable only when `subStatus.tier === 'paid'`.
- **Rejected**: Soft cap (show limited view for free users) — adds complexity
  for no clear benefit. The README explicitly marks this as paid-only.

## Build Phases

### Phase 1: Backend — Aggregator Query + Route
- **Goal**: Serve `GET /api/miner/{address}/workers` returning per-worker share counts
- **Files**: `aggregator.go`, `routes.go`
- **Dependencies**: none (existing schema + middleware)
- **End Conditions**:
  - [ ] `GET /api/miner/{address}/workers` returns JSON array of `{worker_name, shares, last_share_at}`
  - [ ] Endpoint returns 403 for free-tier callers (via `RequirePaid` middleware)
  - [ ] Existing tests pass: `go test -race ./...` in `services/manager/`

### Phase 2: Frontend — Wire WorkersTable into Miner Page
- **Goal**: Display worker breakdown on the miner dashboard for paid subscribers
- **Files**: `api.ts`, `miner/page.tsx`
- **Dependencies**: Phase 1 (endpoint must exist)
- **End Conditions**:
  - [ ] Paid subscribers see WorkersTable with worker names, share counts, and last share times
  - [ ] Free-tier users do not see the WorkersTable section
  - [ ] Existing frontend tests pass: `npm test` in `frontend/`
  - [ ] TypeScript compiles: `npx tsc --noEmit` in `frontend/`

### Phase 3: Documentation Update
- **Goal**: Update CLAUDE.md to reflect the new endpoint and corrected implementation status
- **Files**: `CLAUDE.md`
- **Dependencies**: Phase 1 + 2
- **End Conditions**:
  - [ ] `/api/miner/{address}/workers` listed in endpoint table
  - [ ] "Worker breakdown" no longer listed as unimplemented

## Phase Dependency Graph

Phase 1 → Phase 2 → Phase 3

## Risk Register

1. **worker_name is NULL for some shares**: P2Pool's `worker` field in the API may be
   empty when miners don't set a worker name. Mitigation: use `COALESCE(worker_name, 'default')`
   in the query, matching what `WorkersTable.tsx` already does on the frontend
   (`worker.worker_name || 'default'`).
2. **Query performance on large share tables**: Grouping all shares by worker for a miner
   could be slow if a miner has millions of shares. Mitigation: the existing index on
   `(miner_address, sidechain)` covers the WHERE clause; add a time window (last 30 days
   for free / uncapped for paid) to bound the scan. The 30-day pruning for free-tier
   data naturally limits the dataset.
