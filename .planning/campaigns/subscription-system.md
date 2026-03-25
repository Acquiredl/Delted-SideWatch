# Campaign: Subscription Verification System

> Status: completed
> Created: 2026-03-25
> Architecture: .planning/architecture-subscription-system.md
> Direction: Implement XMR subscription payment verification — wallet RPC client, payment scanner, tier middleware, API endpoints, Docker config.

## Phases

| # | Name | Type | Status | Dependencies |
|---|------|------|--------|-------------|
| 0 | Baseline | verify | completed | none |
| 1 | Schema + Types + Wallet RPC Client | build | completed | 0 |
| 2 | Subscription Payment Scanner | build | completed | 1 |
| 3 | Subscription Service + Tier Middleware | build | completed | 2 |
| 4 | API Endpoints + Wiring + Docker | build | completed | 3 |

## Phase End Conditions

### Phase 0: Baseline
- [ ] `cd services/manager && go build ./cmd/manager/` compiles (command_passes)
- [ ] `cd services/manager && go test ./...` passes (command_passes)
- [ ] `cd services/gateway && go build ./cmd/gateway/` compiles (command_passes)

### Phase 1: Schema + Types + Wallet RPC Client
- [ ] Migration 003 valid SQL (file_exists + manual)
- [ ] `go build ./pkg/walletrpc/` compiles (command_passes)
- [ ] `go vet ./pkg/walletrpc/` passes (command_passes)
- [ ] `go build ./internal/subscription/` compiles (command_passes)
- [ ] Wallet RPC client unit tests pass (command_passes)
- [ ] Existing tests still pass (command_passes)

### Phase 2: Subscription Payment Scanner
- [ ] `go build ./internal/subscription/` compiles (command_passes)
- [ ] `go vet ./internal/subscription/` passes (command_passes)
- [ ] Scanner unit tests pass (command_passes)
- [ ] Existing tests still pass (command_passes)

### Phase 3: Subscription Service + Tier Middleware
- [ ] `go build ./internal/subscription/` compiles (command_passes)
- [ ] Service + middleware unit tests pass (command_passes)
- [ ] Aggregator tier-aware methods compile (command_passes)
- [ ] Existing tests still pass (command_passes)

### Phase 4: API Endpoints + Wiring + Docker
- [ ] `cd services/manager && go build ./cmd/manager/` compiles (command_passes)
- [ ] `cd services/manager && go test ./...` passes (command_passes)
- [ ] `cd services/gateway && go build ./cmd/gateway/` compiles (command_passes)
- [ ] `docker compose config` validates (command_passes)
- [ ] .env.example contains new vars (file_exists)

## Active Context

All phases completed. Campaign done.

## Feature Ledger

1. `pkg/walletrpc/` — monero-wallet-rpc JSON-RPC client (CreateAddress, GetTransfers, GetAddress, GetHeight)
2. `pkg/db/migrations/003_subscriptions.sql` — subscriptions, subscription_addresses, subscription_payments tables
3. `internal/subscription/scanner.go` — polls wallet-rpc, matches subaddresses, 10-confirm depth, activates subscriptions
4. `internal/subscription/service.go` — CRUD, API key gen (SHA-256 hashed), Redis-cached tier checks
5. `internal/subscription/middleware.go` — TierMiddleware, RequirePaid gate, FreeTierLimits (720h hashrate, 100 payments)
6. `cmd/manager/routes.go` — 4 new endpoints: address, status, payments, api-key; tax-export gated behind RequirePaid
7. `cmd/manager/main.go` — wallet-rpc + subscription scanner wired into startup (graceful skip if WALLET_RPC_URL unset)
8. `docker-compose.yml` — wallet-rpc service added (view-only mode)
9. `aggregator.go` — GetMinerPayments now accepts maxAge parameter for tier-based filtering

## Decision Log

1. monero-wallet-rpc (view-only) for payment detection
2. Unique subaddresses per miner for payment linking
3. Manager-side middleware + Redis cache for tier checks
4. Random API keys, SHA-256 hashed, Redis-cached
5. USD-equivalent amount validation with 20% tolerance

## Review Queue

(empty)
