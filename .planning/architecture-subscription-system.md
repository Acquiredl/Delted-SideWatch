# Architecture: Subscription Verification System

> Source: User spec (monetization model for Sidewatch)
> Date: 2026-03-25
> Mode: Feature (existing codebase)

## File Tree

```
services/manager/
  + pkg/walletrpc/
  +   client.go                    ← monero-wallet-rpc JSON-RPC client (view-only)
  +   types.go                     ← request/response structs for wallet RPC
  +   client_test.go               ← table-driven tests with sample JSON responses
  + internal/subscription/
  +   types.go                     ← Subscription, SubPayment, Tier, APIKey domain types
  +   service.go                   ← CRUD: create subscription, generate API key, check tier
  +   scanner.go                   ← polls wallet-rpc for incoming transfers, activates subs
  +   middleware.go                ← HTTP middleware: extract tier from context, enforce gates
  +   service_test.go              ← unit tests for tier logic, API key hashing, grace period
  +   scanner_test.go              ← unit tests for payment matching, confirmation depth
  +   middleware_test.go           ← unit tests for tier enforcement on each endpoint pattern
  + pkg/db/migrations/
  +   003_subscriptions.sql        ← subscriptions, subscription_addresses, subscription_payments
  ~ cmd/manager/
  ~   config.go                    ← add WALLET_RPC_URL, SUBSCRIPTION_WALLET_ADDRESS, SUB_PRICE_USD
  ~   main.go                      ← wire subscription service + scanner into startup
  ~   routes.go                    ← add subscription endpoints, wrap gated routes with middleware
  ~ internal/aggregator/
  ~   aggregator.go                ← add tier-aware query variants (capped vs uncapped)
  ~ docker-compose.yml             ← add monero-wallet-rpc service (view-only)
  ~ .env.example                   ← add new env vars
```

## Component Breakdown

### Feature: On-Chain Subscription Payment Scanner
- Files: `pkg/walletrpc/*`, `internal/subscription/scanner.go`, `pkg/db/migrations/003_subscriptions.sql`
- Dependencies: monero-wallet-rpc container running with view-only wallet, Postgres
- Complexity: **high** — wallet RPC integration, subaddress assignment, confirmation depth buffer, price-based amount validation

### Feature: Subscription Service (CRUD + Tier Logic)
- Files: `internal/subscription/types.go`, `internal/subscription/service.go`
- Dependencies: Postgres schema (migration 003), Redis cache
- Complexity: **medium** — standard CRUD but with grace period logic, API key generation, and tier derivation

### Feature: Tier-Gated Middleware
- Files: `internal/subscription/middleware.go`, modified `aggregator.go`, modified `routes.go`
- Dependencies: Subscription service, Redis cache
- Complexity: **medium** — context injection pattern, per-endpoint limit adjustment, API key resolution

### Feature: Subscription API Endpoints
- Files: modified `routes.go`, modified `main.go`, modified `config.go`
- Dependencies: Subscription service, wallet RPC client
- Complexity: **low** — thin handlers following existing pattern, wiring into main startup

## Data Model

### subscriptions
- Fields: id (BIGSERIAL PK), miner_address (VARCHAR 256 UNIQUE), tier (VARCHAR 32), api_key_hash (VARCHAR 64 nullable), email (VARCHAR 256 nullable), expires_at (TIMESTAMPTZ nullable), grace_until (TIMESTAMPTZ nullable), created_at (TIMESTAMPTZ), updated_at (TIMESTAMPTZ)
- Indexes: UNIQUE (miner_address), (api_key_hash) WHERE api_key_hash IS NOT NULL, (expires_at) for expiry scanner
- Relationships: miner_address links to p2pool_shares, payments, miner_hashrate
- Notes: tier = 'free' | 'paid'. Free-tier rows are created lazily on first lookup. expires_at is NULL for free tier. grace_until = expires_at + 48 hours, computed on write.

### subscription_addresses
- Fields: id (BIGSERIAL PK), miner_address (VARCHAR 256 UNIQUE), subaddress (VARCHAR 256 UNIQUE), subaddress_index (INTEGER UNIQUE), created_at (TIMESTAMPTZ)
- Indexes: UNIQUE (miner_address), UNIQUE (subaddress), UNIQUE (subaddress_index)
- Relationships: miner_address → subscriptions
- Notes: Maps each miner to a unique wallet subaddress for payment identification. Subaddress index is the wallet-rpc account/address index used to generate it.

### subscription_payments
- Fields: id (BIGSERIAL PK), miner_address (VARCHAR 256), tx_hash (VARCHAR 64 UNIQUE), amount (BIGINT atomic units), xmr_usd_price (NUMERIC 12,4 nullable), confirmed (BOOLEAN DEFAULT FALSE), main_height (BIGINT nullable), created_at (TIMESTAMPTZ)
- Indexes: UNIQUE (tx_hash), (miner_address, created_at), (confirmed) WHERE NOT confirmed
- Relationships: miner_address → subscriptions
- Notes: Payments start as unconfirmed. Scanner promotes to confirmed after 10 confirmations. Only confirmed payments trigger subscription activation.

## Key Decisions

### Payment Detection: monero-wallet-rpc (view-only mode)
- **Chosen**: Run monero-wallet-rpc with a view-only wallet as an additional Docker service. It generates unique subaddresses per miner and detects incoming payments automatically.
  - Battle-tested Monero payment detection — no custom crypto needed
  - View-only wallet cannot spend funds (security guarantee)
  - Built-in subaddress generation maps 1:1 to miners
  - Consistent with CLAUDE.md roadmap: "Automated wallet RPC verification is a future milestone"
  - One additional Docker container, minimal ops overhead
- **Rejected**: Pure Go output scanning — would require implementing Monero's ed25519 key derivation, Keccak-256 Hs() function, and subaddress cryptography. High risk of subtle bugs in security-critical code. The crypto correctness burden is not justified when wallet-rpc exists.
- **Rejected**: TX proof submission — requires manual step from miner (submit txid + tx key). Poor UX, not automated. Suitable for MVP but the spec requires on-chain scanning.
- **Note**: The "No wallet RPC" rule in CLAUDE.md refers to miner fund custody ("we never touch miner funds"). The subscription wallet is the operator's own revenue address. A view-only instance detects payments but can never spend. This is architecturally distinct from custodying miner funds.

### Miner-to-Payment Linking: Unique Subaddresses
- **Chosen**: Each miner receives a unique subaddress of the subscription wallet. When XMR arrives at that subaddress, the system knows who paid. This is the standard Monero merchant pattern (used by BTCPay Server, Monero Payment Gateway, etc.).
  - Privacy-preserving: no payment IDs, no integrated addresses (both deprecated)
  - Deterministic: same miner always gets same subaddress (idempotent)
  - Scalable: wallet-rpc supports millions of subaddresses
- **Rejected**: Payment IDs — deprecated in Monero, poor privacy, wallets warn against them.
- **Rejected**: Amount-based matching — unreliable, race conditions with concurrent payments.

### Subscription Check Location: Manager-side middleware + Redis cache
- **Chosen**: Subscription middleware runs in the manager's handler chain. Subscription status is cached in Redis with 30-second TTL. The gateway stays thin (only JWT + rate limit + proxy).
  - Manager already owns all data and business logic
  - Avoids giving the gateway direct DB access
  - Redis cache eliminates per-request DB hits (existing cache.Store pattern)
  - Gateway remains a pure proxy — separation of concerns preserved
- **Rejected**: Gateway-side subscription check — would require the gateway to understand subscription logic, query DB or Redis with subscription-specific keys. Increases gateway complexity for marginal latency gain (one extra proxy hop is ~1ms on Docker network).

### API Key Design: Random token, SHA-256 hashed
- **Chosen**: Generate 32 random bytes via crypto/rand, hex-encode to 64-char token, SHA-256 hash for DB storage, cache key→tier mapping in Redis.
  - Standard API key pattern, supports revocation (delete from DB)
  - SHA-256 hash means DB compromise doesn't leak usable keys
  - Redis cache for O(1) lookup on every request
- **Rejected**: JWT-based API keys — can't be revoked without a blocklist, contain claims that could leak tier info.
- **Rejected**: HMAC-derived keys — server secret compromise = all keys compromised, no individual revocation.

### Amount Validation: USD-equivalent at time of receipt
- **Chosen**: When a payment arrives, compute USD value using the existing PriceOracle. Accept if amount >= $5 USD equivalent with 20% tolerance (i.e., $4 minimum). This handles price fluctuation between when the miner sees the price and when the tx confirms.
  - Reuses existing PriceOracle infrastructure
  - Tolerance window prevents failed subscriptions from minor price swings
  - Simple to understand for miners: "send ~$5 in XMR"
- **Rejected**: Fixed XMR amount — requires manual adjustment as XMR price changes, poor UX.

## Build Phases

### Phase 0: Baseline
- **Goal**: Verify the existing codebase builds and tests pass before making changes
- **Files**: none (read-only verification)
- **Dependencies**: none
- **End Conditions**:
  - [ ] `cd services/manager && go build ./cmd/manager/` compiles
  - [ ] `cd services/manager && go test ./...` passes (skip integration if deps unavailable)
  - [ ] `cd services/gateway && go build ./cmd/gateway/` compiles
  - [ ] No new typecheck errors in any existing package

### Phase 1: Schema + Domain Types + Wallet RPC Client
- **Goal**: Create the database tables, Go domain types, and the wallet RPC client package
- **Files**:
  - `services/manager/pkg/db/migrations/003_subscriptions.sql`
  - `services/manager/pkg/walletrpc/types.go`
  - `services/manager/pkg/walletrpc/client.go`
  - `services/manager/pkg/walletrpc/client_test.go`
  - `services/manager/internal/subscription/types.go`
- **Dependencies**: Phase 0 (baseline verified)
- **End Conditions**:
  - [ ] Migration 003 is valid SQL (parseable, IF NOT EXISTS, correct types)
  - [ ] `go build ./pkg/walletrpc/` compiles
  - [ ] `go vet ./pkg/walletrpc/` passes
  - [ ] Wallet RPC client covers: CreateAddress, GetTransfers, GetAddress, GetHeight
  - [ ] All methods return typed structs with JSON tags
  - [ ] Unit tests pass for wallet RPC response parsing (table-driven, sample JSON)
  - [ ] `go build ./internal/subscription/` compiles (types only at this point)
  - [ ] Existing tests still pass

### Phase 2: Subscription Payment Scanner
- **Goal**: Implement the background scanner that polls wallet-rpc for incoming transfers, matches them to miners via subaddress mapping, waits for confirmations, and activates subscriptions
- **Files**:
  - `services/manager/internal/subscription/scanner.go`
  - `services/manager/internal/subscription/scanner_test.go`
- **Dependencies**: Phase 1 (wallet RPC client, schema, types)
- **End Conditions**:
  - [ ] `go build ./internal/subscription/` compiles
  - [ ] `go vet ./internal/subscription/` passes
  - [ ] Scanner polls wallet-rpc on configurable interval (default 60s)
  - [ ] Scanner assigns unique subaddresses to miners via wallet-rpc CreateAddress
  - [ ] Scanner implements 10-confirmation depth buffer (mirrors coinbase scanner pattern)
  - [ ] Unconfirmed payments recorded immediately, promoted to confirmed at depth
  - [ ] Confirmed payment >= $4 USD equivalent activates/extends 30-day subscription
  - [ ] Grace period (48h) computed and stored on activation
  - [ ] Unit tests for: confirmation promotion, amount validation, grace period calculation
  - [ ] Existing tests still pass

### Phase 3: Subscription Service + Tier Middleware
- **Goal**: Implement the subscription CRUD service, API key generation, Redis caching, and the HTTP middleware that enforces tier-based access control
- **Files**:
  - `services/manager/internal/subscription/service.go`
  - `services/manager/internal/subscription/middleware.go`
  - `services/manager/internal/subscription/service_test.go`
  - `services/manager/internal/subscription/middleware_test.go`
  - `~ services/manager/internal/aggregator/aggregator.go` (add tier-aware methods)
- **Dependencies**: Phase 2 (scanner populates subscription data)
- **End Conditions**:
  - [ ] `go build ./internal/subscription/` compiles
  - [ ] Service methods: GetOrCreateSubscription, GetSubscriptionByAddress, GetSubscriptionByAPIKey, GenerateAPIKey, CheckTier
  - [ ] API key generated with crypto/rand (32 bytes), stored as SHA-256 hash
  - [ ] Subscription status cached in Redis with 30s TTL (key: `sub:{address}`)
  - [ ] API key → address mapping cached in Redis (key: `apikey:{hash_prefix}`)
  - [ ] Middleware extracts tier from: (a) API key in X-API-Key header, or (b) miner address in path
  - [ ] Middleware injects tier into request context
  - [ ] Aggregator gains tier-aware methods: GetMinerPayments accepts maxAge filter, GetMinerHashrate accepts maxHours override
  - [ ] Free tier: hashrate capped at 720 hours (30 days), payments capped at 100, tax-export returns 403
  - [ ] Paid tier: no caps on hashrate/payments, tax-export allowed
  - [ ] Grace period: paid features available until grace_until (expires_at + 48h)
  - [ ] Unit tests for: API key hash/verify cycle, tier resolution, middleware rejection, grace period edge cases
  - [ ] Existing tests still pass

### Phase 4: API Endpoints + Wiring + Docker
- **Goal**: Add subscription API endpoints, wire everything into main.go, update config, add wallet-rpc to Docker, update .env.example
- **Files**:
  - `~ services/manager/cmd/manager/routes.go` (add subscription routes, wrap gated routes)
  - `~ services/manager/cmd/manager/main.go` (create + start subscription scanner)
  - `~ services/manager/cmd/manager/config.go` (add wallet RPC + subscription config)
  - `~ docker-compose.yml` (add monero-wallet-rpc service)
  - `~ .env.example` (add new env vars)
- **Dependencies**: Phase 3 (service + middleware exist)
- **End Conditions**:
  - [ ] `cd services/manager && go build ./cmd/manager/` compiles
  - [ ] `cd services/manager && go test ./...` passes
  - [ ] `cd services/gateway && go build ./cmd/gateway/` compiles
  - [ ] `docker compose config` validates
  - [ ] GET /api/subscription/address/{miner_address} returns payment subaddress
  - [ ] GET /api/subscription/status/{miner_address} returns tier + expiry + grace info
  - [ ] GET /api/subscription/payments/{miner_address} returns subscription payment history
  - [ ] POST /api/subscription/api-key/{miner_address} generates and returns API key (paid tier only)
  - [ ] Existing endpoints wrapped with tier middleware (tax-export gated, payments/hashrate limited)
  - [ ] Config includes: WALLET_RPC_URL, SUBSCRIPTION_MIN_USD, SUBSCRIPTION_DURATION_DAYS, SUBSCRIPTION_GRACE_HOURS
  - [ ] Subscription scanner starts as background goroutine in main.go
  - [ ] docker-compose.yml includes monero-wallet-rpc with view-only wallet volume
  - [ ] .env.example documents all new variables
  - [ ] No regressions in existing functionality

## Phase Dependency Graph

```
Phase 0 (Baseline) → Phase 1 (Schema + Types + WalletRPC)
                            → Phase 2 (Payment Scanner)
                                  → Phase 3 (Service + Middleware)
                                        → Phase 4 (Endpoints + Wiring + Docker)
```

All phases are strictly sequential. Each builds on the prior.

## Risk Register

1. **monero-wallet-rpc availability**: If wallet-rpc is down, new subaddress assignments and payment detection stop. **Mitigation**: Scanner logs errors and retries on next poll interval. Existing subscriptions remain active from cached/DB state. Health endpoint reports wallet-rpc status. No subscription features degrade catastrophically — they just pause until wallet-rpc recovers.

2. **XMR price fluctuation during payment window**: Miner sees "$5 = 0.03 XMR" on the UI but by the time their tx confirms (10 blocks, ~20 min), the price may have moved. **Mitigation**: 20% tolerance on the minimum amount ($4 USD equivalent accepted). The displayed amount includes a note: "Price may vary slightly due to market fluctuation." Overpayments are credited normally (no refund complexity).

3. **Subaddress index exhaustion / collision**: Subaddress indexes are integers starting from 0. If the mapping table gets corrupted or out of sync with wallet-rpc, new assignments could collide. **Mitigation**: UNIQUE constraint on subaddress_index in Postgres. Subaddress assignment is atomic: wallet-rpc CreateAddress → DB insert in a transaction. On conflict, re-fetch the latest index from wallet-rpc.

4. **Regression in existing free-tier functionality**: Adding middleware to existing routes could break currently-working endpoints. **Mitigation**: Phase 0 baseline recording. Every phase end condition includes "existing tests still pass." Middleware defaults to free tier (permissive) on any cache/DB failure — never blocks a request due to subscription system errors.

## Deployment Strategy

- **Platform**: Same Docker Compose stack (self-managed VPS)
- **New service**: `monero-wallet-rpc` container added to docker-compose.yml
  - Image: `sethsimmons/simple-monero-wallet-rpc` (or official `monero` image)
  - Volume: view-only wallet file at `/wallet/subscription-viewonly`
  - Connects to existing monerod on Docker network
  - Exposes RPC on port 18088 (internal only, not published)
- **New secret**: `subscription_wallet_viewkey` via Docker secrets
- **New env vars**: `WALLET_RPC_URL`, `SUBSCRIPTION_MIN_USD`, `SUBSCRIPTION_DURATION_DAYS`, `SUBSCRIPTION_GRACE_HOURS`
- **Pre-deploy**: Operator must create the subscription wallet, export view-only wallet file, place it in the wallet volume
- **Migration**: 003_subscriptions.sql runs automatically via existing db.Migrate() on startup
- **Rollback**: Drop the 3 new tables, remove wallet-rpc container. No changes to existing tables.
