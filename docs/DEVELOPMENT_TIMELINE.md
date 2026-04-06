# SideWatch: The Development Timeline

**A narrative account of building a P2Pool Monero mining observability dashboard — from blank repo to production in 12 days.**

*Written for presentation and retrospective. Covers architecture decisions, troubleshooting sagas, lessons learned, and the evolution of the project's identity.*

---

## Table of Contents

1. [The Blueprint](#chapter-1-the-blueprint)
2. [The Big Bang](#chapter-2-the-big-bang)
3. [The Hardening Sprint](#chapter-3-the-hardening-sprint)
4. [Ship It: The Monerod Saga](#chapter-4-ship-it-the-monerod-saga)
5. [Feature Expansion and the CI Gauntlet](#chapter-5-feature-expansion-and-the-ci-gauntlet)
6. [The Crowdfund Architecture](#chapter-6-the-crowdfund-architecture)
7. [Reality Check: The P2Pool Rewrite](#chapter-7-reality-check-the-p2pool-rewrite)
8. [The Polish Phase](#chapter-8-the-polish-phase)
9. [Honest Shipping](#chapter-9-honest-shipping)
10. [Lessons and Principles](#chapter-10-lessons-and-principles)

---

## Chapter 1: The Blueprint

### The Problem Worth Solving

P2Pool is a decentralized mining pool for Monero. Unlike traditional mining pools, there is no central operator who collects your hashrate and sends you payouts. Every miner runs their own P2Pool node (or connects to someone else's), and the pool's sidechain distributes rewards directly to miners' wallets through Monero's coinbase transactions. No custody. No fees. No trust required.

This is a beautiful design with a practical problem: observability is terrible.

If you're a P2Pool miner, here's what you get out of the box: a terminal window showing log output, and a set of JSON files written to disk every few seconds. There's no dashboard, no hashrate history, no payment archive, no way to see your workers, and no record of what happened yesterday. You have to either read raw JSON files or use a third-party tool like p2pool.observer to check your stats.

The idea for SideWatch came from this gap. Not to replace P2Pool or compete with it, but to layer observability on top of a running P2Pool node — the same way Grafana layers observability on top of Prometheus. A managed P2Pool node with a clean dashboard that miners could just point their XMRig at, without syncing 200 GB of Monero blockchain and maintaining a 24/7 server.

### Architecture Decisions Made Before Writing Code

The project was designed on Claude Desktop (in a project called "P2Pool + Go Dashboard") before any code was written. Several key architecture decisions were resolved through conversation:

**Go + Next.js, not a monolith.** The backend needed to handle WebSocket connections for live hashrate push, periodic polling of the P2Pool data-api, coinbase transaction scanning on the Monero blockchain, and time-series aggregation — all concurrently. Go's goroutines and stdlib HTTP server were a natural fit. The frontend was Next.js 14 with TypeScript because it offered server-side rendering for SEO, a clean component model, and SWR for data fetching.

**stdlib HTTP, not a framework.** No Gin, no Echo, no Chi. The Go standard library's `net/http` package, especially after Go 1.22's routing improvements, handles everything this project needs. Frameworks add dependency weight, version lock-in, and middleware conventions that obscure what's actually happening. For a project that needs to be maintained by a solo developer, understanding every line of the HTTP layer matters.

**pgx, not an ORM.** The database layer uses `github.com/jackc/pgx/v5` with hand-written SQL. Every query names its columns explicitly. Every parameter is parameterized. No magic, no query generation, no `SELECT *`. This decision was driven by the time-series nature of the data — the queries for hashrate aggregation, payment grouping, and retention pruning are specific enough that an ORM would fight you at every turn.

**API consumer, not consensus reimplementation.** This was the most important architectural boundary. p2pool.observer (git.gammaspectra.live/P2Pool/observer) takes a fundamentally different approach: it reimplements P2Pool's sidechain consensus in Go from scratch, indexing every share, uncle, and block independently. SideWatch was designed to be a consumer of the P2Pool node's existing API — reading whatever the running node exposes, not trying to understand the sidechain at the protocol level. This kept the project maintainable for a solo developer and avoided the bugs that come from partial protocol reimplementation.

**Trust model as the product.** P2Pool's target audience chose it specifically because they don't want to trust a pool operator with their hashrate or their money. Any observability tool targeting this audience needs to align with that philosophy. SideWatch was designed so the operator (Steph) can never intercept funds or falsify payments by construction. The dashboard reads from a P2Pool node; miners keep all their rewards. This isn't just a technical detail — it's the core selling point, and it needed to be framed prominently in all user-facing messaging.

**AGPL-3.0 license.** Specifically chosen to block competing hosted forks. Open source builds trust with the privacy-conscious target audience, but AGPL prevents someone from forking the repo and running a competing hosted service without sharing their modifications. This was a deliberate business decision, not just community goodwill.

**P2Pool mini, not main.** P2Pool has two sidechains: mini (lower difficulty, faster shares, targeting small miners) and main (higher difficulty, for large operations). The project launched mini-only because mini targets the hobbyist/small miner profile that aligns with the zero-fee, decentralization angle. The data layer was designed to be sidechain-agnostic so adding main later would be low-friction, but the current VPS (DigitalOcean, 8 GB / 4 vCPU) can't run both simultaneously.

**ZMQ over polling for block events.** Since the project controls the infrastructure, Monero's ZMQ block notification system is always available. Rather than polling monerod for new blocks, the scanner subscribes to ZMQ events for immediate notification when a new block is mined. Better latency, event-driven, and the correct tool when you own the node.

**Confirmation depth buffer for reorg handling.** Payments are not recorded as final until the Monero block is 10 confirmations deep (~20 minutes). This eliminates the vast majority of sidechain reorg risk with minimal implementation complexity, and the UI communicates that payments appear after sufficient confirmations.

These decisions were all resolved before the first line of code was written. They formed the CLAUDE.md context document that would guide every subsequent implementation session.

---

## Chapter 2: The Big Bang

**March 24, 2026 — One commit. 173 files. 24,733 lines of code.**

At 10:10 PM on March 24th, the initial commit landed. It wasn't a skeleton or a boilerplate scaffold — it was the entire application architecture, built in a single AI-assisted session with Claude Code.

### What Was Created in One Shot

**Two complete Go services:**

The **Manager** service (`services/manager/`) was the primary build target — 15 packages covering every backend concern:
- `cmd/manager/` — main entry point (157 lines), HTTP route registration (354 lines), environment config (90 lines)
- `internal/p2pool/` — sidechain poller and indexer that periodically fetches P2Pool API data and writes it to PostgreSQL
- `internal/scanner/` — coinbase transaction scanner that reads Monero blocks, extracts miner payments, and records them with fiat prices
- `internal/aggregator/` — builds miner stat views with 15-minute bucketed hashrate time series
- `internal/events/` — ZMQ block event listener (199 lines) subscribing to monerod's block notifications
- `internal/ws/` — WebSocket hub for pushing live hashrate updates to connected browsers
- `internal/cache/` — Redis caching helpers
- `internal/metrics/` — Prometheus metric registration and instrumentation
- `pkg/db/` — pgx connection pool with two migrations (initial schema + payments table)
- `pkg/monerod/` — complete Monero JSON-RPC client (153 lines + 115 lines of types) with methods for `get_block`, `get_transactions`, `get_last_block_header`
- `pkg/p2poolclient/` — typed HTTP client for the P2Pool data-api (104 lines + 56 lines of types)

The **Gateway** service (`services/gateway/`) was the API gateway — JWT authentication middleware, rate limiting (token bucket per IP), request ID injection, structured logging, and HTTP reverse proxy to the manager.

**A complete Next.js 14 frontend** with 5 pages (home, miner dashboard, blocks explorer, sidechain viewer, admin panel), 8 components (LiveStats with WebSocket, HashrateChart with Recharts, BlocksTable, PaymentsTable, WorkersTable, SidechainTable, Navigation, PrivacyNotice), a typed API client, and a WebSocket hook.

**A mock P2Pool + monerod server** (`services/mocknode/`, 552 lines) that simulated both APIs for local development and testing — serving fake pool stats, fake shares, fake blocks, and fake monerod RPC responses.

**Full infrastructure:** Three Docker Compose configurations (production, dev, test), seven Dockerfiles (each Go service in prod and dev variants, plus the mocknode), nginx reverse proxy config, Prometheus with alert rules, a Grafana dashboard (the pool-overview JSON alone was 420 lines), Loki and Promtail for log aggregation, Alertmanager for notifications, and an `initdb.sql` for PostgreSQL initialization.

**Tests from day one:** Six test files shipped with the initial commit — unit tests for the monerod client (455 lines), P2Pool client (385 lines), coinbase parser (183 lines), hashrate time series (132 lines), JWT middleware (174 lines), and rate limiter (121 lines). All table-driven, all using `httptest` for handler testing.

### The Significance

This was not a gradual, organic project. The entire architecture was designed in conversation, then generated in a single session. The co-author line on every commit — `Claude Opus 4.6` — tells the story: this was AI-assisted development at a scale that would have taken a solo developer weeks or months to produce manually.

But the speed came with a cost that wouldn't become apparent for another 9 days: the mocknode. By building a fake P2Pool server that invented its own API surface, the project created a comprehensive integration test environment that was internally consistent but bore little resemblance to the real P2Pool v4.3 binary. Every component was tested against the mocknode and worked perfectly — against the wrong API.

### First Follow-up: Wiring the Price Oracle

Ninety minutes after the initial commit, the first real feature extension landed (`d594eba`). The PriceOracle existed in the initial codebase but wasn't connected to the scanner's payment processing flow. This 37-line change wired them together so that when the scanner processes confirmed coinbase blocks, it fetches XMR/USD and XMR/CAD spot prices from CoinGecko and attaches them to payment records. If the oracle is down, prices gracefully degrade to NULL.

The next morning (`972e982`) brought the first real bug fix: the frontend Docker container was trying to reach the API gateway at `localhost` instead of the gateway container's hostname. Next.js bakes environment variables at build time, so `API_PROXY_URL` had to be set in the Dockerfile, not at runtime. This was the first time someone had actually tried to run the containers end-to-end and hit the "works on my machine" boundary — a theme that would recur with increasing intensity.

---

## Chapter 3: The Hardening Sprint

**March 25, 2026 — 9 commits in 5 hours. The project goes from "code exists" to "production-ready."**

### 8:22 AM — WebSocket Fix and Admin Auth

The day started with a two-line discovery (`9e25f8a`): browser WebSocket connections cannot be proxied through Next.js rewrites. They need to connect directly to the gateway. Added `NEXT_PUBLIC_WS_URL` as a client-side environment variable. Also discovered that the gateway's JWT-protected route prefix was `/admin/` but the actual endpoint was `/api/admin/` — a mismatch that left the admin panel completely unprotected.

### 9:13 AM — Testing Infrastructure (1,649 lines)

Commit `b7f2960` was the testing infrastructure buildout. The gateway got WebSocket upgrade tunneling via raw TCP hijack (using Go's `http.Hijacker` interface to upgrade HTTP connections to bidirectional TCP streams). Six integration test files were added covering the aggregator, time-series builder, Redis cache, P2Pool indexer, coinbase scanner, and WebSocket hub. A shared test helper (`testdb.go`) was created for spinning up isolated test databases. A full end-to-end test suite (388 lines) with its own `go.mod` was added for testing the complete stack through Docker Compose.

### 9:43 AM — VPS Deployment Automation (2,412 lines)

This single commit (`5027e79`) delivered the entire deployment pipeline — not as a CI/CD configuration, but as a complete set of shell scripts for provisioning a bare Ubuntu VPS:

- **`provision.sh`** (291 lines): Installs Docker, configures UFW firewall, sets up fail2ban for brute-force protection
- **`generate-secrets.sh`** (81 lines): Creates Docker secrets for Postgres password, JWT signing key, and API tokens
- **`setup-tls.sh`** (254 lines): Configures Let's Encrypt with automatic renewal via certbot timer
- **`deploy.sh`** (229 lines): Git pull, Docker image rebuild, healthcheck with automatic rollback on failure
- **`harden.sh`** (362 lines): SSH lockdown (key-only auth, no root login), Docker container resource limits, sysctl tuning
- **`pool-backup.sh`** (186 lines): PostgreSQL pg_dump with S3 or rsync remote upload
- **`restore.sh`** (237 lines): Disaster recovery from backup with integrity verification
- **`healthcheck.sh`** (159 lines): External health verification hitting every service endpoint
- **`install-services.sh`** (62 lines): Systemd unit installation
- Three systemd units: the main dashboard service, a backup service, and a backup timer
- A GitHub Actions CI/CD workflow (60 lines)
- A comprehensive DEPLOYMENT.md operator runbook (371 lines)

### 10:07 AM — Code Review Cleanup

Twenty-four minutes later (`20c8472`), a code review caught several issues: the systemd timer was calling inline bash instead of the backup script, a state file in `/tmp` would be lost on reboot, the deploy script's COMPOSE_FILES variable wasn't a proper bash array, the healthcheck script had a false-positive TLS expiry check, the GitHub Actions workflow accepted unsanitized branch input, and the secret generation script could produce non-uniform hex output.

### 11:18 AM — The Subscription System (2,699 lines)

Commit `b1b3b27` delivered the entire monetization layer in one shot. This was the XMR subscription system — miners pay in Monero to unlock extended data retention and premium features.

The architecture:
- **`pkg/walletrpc/`** — A JSON-RPC client for `monero-wallet-rpc`, the Monero wallet daemon. This is a view-only wallet (it can see incoming payments but cannot spend them). The client generates unique subaddresses per subscriber so payments can be attributed to specific miners.
- **`internal/subscription/`** — The subscription engine: a scanner that periodically queries the wallet for new payments, a service layer that manages subscription state (tier, expiry, retention), and middleware that enforces tier-based access limits on API endpoints.
- **Migration `003_subscriptions.sql`** — Three new tables: subscriptions (per-miner subscription state), subscription_addresses (generated payment subaddresses), and subscription_payments (verified incoming payments).
- **Four new API endpoints** — address assignment, subscription status, payment history, and API key generation.

The subscription tiers were designed as a progressive unlock: free tier gets 30 days of data retention, paid tier gets 15 months. The middleware was wired globally so that any handler could check `RequirePaid` to gate features.

### 12:30 PM — Frontend Tests + Tor + Grafana (8,482 lines)

Three separate features in one commit (`2f6ea85`):
1. **14 Jest test suites** (79 tests) covering all components, pages, library functions, and the WebSocket hook
2. **Tor hidden service** — An Alpine-based container running the Tor daemon, routing to the nginx reverse proxy, with persistent onion keys stored in a Docker volume. This was infrastructure-ready but would later be disabled when it proved unstable.
3. **Grafana miner-detail dashboard** — 8 panels powered by 5 new per-miner Prometheus metrics emitted from the aggregator and scanner

### 1:00 PM and 1:20 PM — Documentation and DevSecOps

Two final commits closed the day. The documentation commit (`de90f1e`, 1,231 lines) rewrote infrastructure docs, created a comprehensive deployment walkthrough, and also included functional code changes to JWT auth path rewriting and rate limiter improvements. The DevSecOps commit (`2f3e626`, 680 lines) added a GitHub Actions security workflow with golangci-lint, gosec, govulncheck, npm audit, Trivy image scanning, and gitleaks secret detection — plus Dependabot configuration and a CODEOWNERS file.

### The Day's Velocity

In five hours, the project gained:
- Full integration and E2E test suites
- Complete VPS deployment automation with TLS, backup/restore, and hardening
- XMR subscription monetization with view-only wallet payment verification
- 79 frontend tests across 14 suites
- Tor hidden service infrastructure
- Grafana miner dashboards with custom Prometheus metrics
- DevSecOps CI/CD pipeline with 6 scanning tools
- Complete documentation rewrite

This pace — measured in thousands of lines per hour — is characteristic of AI-assisted development. But it also planted seeds for future problems: each feature was built against the mocknode, each integration test validated behavior against simulated APIs, and each Docker configuration was theoretically correct but empirically untested against real container images.

---

## Chapter 4: Ship It — The Monerod Saga

**March 26, 2026 — 10 commits in 2.5 hours. The first real deployment attempt.**

### The Trigger

At 7:52 AM (`446c8c4`), the deployment infrastructure was wired up for real: CI now builds Docker images and pushes them to GitHub Container Registry, `deploy.sh` pulls pre-built images instead of building on the VPS, and — critically — two new compose overlays were created. `docker-compose.node.yml` (106 lines) defined the real monerod and P2Pool Mini containers. `docker-compose.prod.yml` (54 lines) added production-specific configuration.

This was the commit that introduced actual Monero and P2Pool container definitions. Everything that follows is the cascade of failures from spinning them up on a real VPS.

### 8:07 AM — No curl in Alpine (+15 minutes)

**Commit `f6a94b9`.** The monerod healthcheck used `curl` to probe the RPC endpoint. But `sethsimmons/simple-monerod` is Alpine-based and doesn't ship `curl`. The healthcheck silently failed, Docker marked the container as unhealthy, and p2pool (which depended on monerod being healthy) never started. Fix: switched to `wget`, which Alpine does include. Also discovered the GitHub Actions security workflow couldn't be called as a reusable workflow because it was missing the `workflow_call` trigger.

### 8:15 AM — Duplicate --non-interactive (+8 minutes)

**Commit `f327464`.** The `sethsimmons/simple-monerod` Docker image has `--non-interactive` baked into its ENTRYPOINT. The compose file's `command` block also specified it. Monerod received the flag twice and rejected the entire command. One-line fix.

### 8:20 AM — Command String vs. Array Syntax (+5 minutes)

**Commit `ad97eae`.** This was the key Docker ENTRYPOINT lesson. The compose file used YAML folded-string syntax:

```yaml
command: >
  monerod --restricted-rpc --rpc-bind-ip=0.0.0.0 ...
```

This produces a single string: `"monerod --restricted-rpc --rpc-bind-ip=0.0.0.0 ..."`. But the image's ENTRYPOINT already runs `monerod`. Docker concatenates ENTRYPOINT + CMD, so the actual command became:

```
monerod monerod --restricted-rpc --rpc-bind-ip=0.0.0.0 ...
```

The binary name `monerod` was passed as a positional argument to itself, causing immediate exit. The fix was switching from folded-string to YAML array syntax:

```yaml
command:
  - "--restricted-rpc"
  - "--rpc-bind-ip=0.0.0.0"
```

Array items are appended as CMD arguments to the ENTRYPOINT, not concatenated as a competing command. This same fix was applied to both the monerod and p2pool containers.

**The lesson:** When using third-party Docker images, always check whether the binary is invoked via ENTRYPOINT or CMD. If it's ENTRYPOINT, your `command:` must contain only flags, never the binary name.

### 8:23 AM — Still Colliding (+3 minutes)

**Commit `75011de`.** Even after the array syntax fix, `--non-interactive` was still in the flag list and still colliding with the ENTRYPOINT's built-in copy. Removed it again. This was the third commit in 8 minutes touching the same flag.

### 8:34 AM — Healthcheck Start Period (+11 minutes)

**Commit `f0f1cef`.** The monerod RPC server doesn't start until database initialization completes. On first run (syncing from genesis), this takes far longer than the original `start_period: 300s` (5 minutes). Docker marked monerod as unhealthy before RPC was available, which cascaded: p2pool wouldn't start (depended on healthy monerod), manager wouldn't start (depended on healthy p2pool), and the entire dashboard was unreachable.

Fix: bumped `start_period` from 300s to **1800s** (30 minutes), interval from 30s to 60s, retries from 5 to 3. On first deployment, monerod needs to sync the entire Monero blockchain (~200 GB) before becoming fully operational — but the RPC endpoint comes up much sooner for basic queries.

### 9:12 AM — Alertmanager Crash Loop (+38 minutes)

**Commit `5109187`.** Alertmanager v0.31.1 crashed in a restart loop because the configuration specified an `http_config.headers` field that this version doesn't support. The configuration had been written speculatively during development — adding an `X-Admin-Token` header for webhook authentication — but the actual Alertmanager binary rejected it. Since the webhook lives on the internal Docker network, the auth header wasn't needed anyway. Removed the block.

### 9:50 AM — localhost Doesn't Resolve in Alpine (+38 minutes)

**Commit `ee7e878`.** The minimal Alpine containers used by sethsimmons images don't have `/etc/hosts` entries that map `localhost` to `127.0.0.1`. The monerod RPC was listening on `0.0.0.0:18081` and responding correctly, but the healthcheck `wget http://localhost:18081/get_info` couldn't resolve the hostname. Fix: replaced `localhost` with `127.0.0.1` in both monerod and p2pool healthcheck commands.

**The lesson:** Never use `localhost` in container healthchecks. Always use `127.0.0.1`. Minimal container images often lack DNS resolution infrastructure.

### 9:57 AM — JSON Spacing in grep (+7 minutes)

**Commit `32e028b`.** The healthcheck grep pattern was `"status":"OK"` (no space after the colon). Monerod's actual JSON response was `"status": "OK"` (with a space). The grep never matched, so the healthcheck always reported failure even though the response was correct. Simplified to `grep -q 'OK'`.

**The lesson:** Don't grep for exact JSON formatting — whitespace varies between implementations. Match the value, not the formatting.

### 10:21 AM — Relaxing Service Dependencies (+24 minutes)

**Commit `765c9a2`.** The final fix of the saga. The manager service depended on p2pool with `condition: service_healthy`, and p2pool depended on monerod with the same condition. On first deployment, monerod takes hours (or days) to sync the full blockchain. This meant the entire dashboard — including the frontend — was completely inaccessible while monerod synced.

Fix: changed the manager's dependency on p2pool from `service_healthy` to `service_started`. The manager already handles p2pool being unavailable gracefully (it logs warnings and shows empty data). This way the dashboard is accessible immediately, and data populates as the underlying services come online.

### The Pattern

The Monerod Saga compressed into 2.5 hours what many teams experience over weeks: the gap between "it works in development" and "it runs in production." Every assumption made during development with the mocknode — about container tooling, ENTRYPOINT conventions, DNS resolution, JSON formatting, startup timing, and service dependencies — had to be corrected empirically.

The rapid-fire commits (3-8 minute intervals) tell the story of SSH-into-VPS, `docker compose up`, read logs, find error, fix, push, redeploy, find next error. Each fix revealed the next problem. This is the reality of deploying against third-party Docker images you've never run before.

---

## Chapter 5: Feature Expansion and the CI Gauntlet

**March 27-30, 2026 — New features, branding, and a brutal lint migration.**

### March 27: Documentation and Performance

The day started with three documentation and tooling commits: main sidechain support architecture (`ece1982`), an OpenAPI spec with self-hosting guide (`2e312bc`), and Go benchmarks with an HTTP load test script (`ff68c62`).

Then came a meaningful performance optimization (`4ae9ae9`). The WebSocket broadcast loop and the HTTP pool stats handler were both independently querying PostgreSQL every 5 seconds — roughly 60 database queries per minute just for pool stats. The fix added a `GetPoolStatsCached()` method to the Aggregator that checks Redis first (15-second TTL on key `pool:stats`), falling back to Postgres on cache miss. Both the WebSocket hub and the HTTP handler share this cached result, dropping query volume from ~60/min to ~4/min.

The same commit tuned the pgx connection pool for the constrained VPS environment: `MaxConns` reduced from 20 to 10 (a single-node dashboard doesn't need 20 concurrent database connections), and a 1-minute `HealthCheckPeriod` was added to evict stale connections after network hiccups. Nginx got `worker_connections` bumped from 1024 to 4096 and HTTP/1.1 keepalive to the gateway upstream.

### March 28: The Security Scan Reckoning

The DevSecOps pipeline from March 25 finally ran against the real codebase (`efef70e`) and surfaced a wall of issues:

- **17 Go stdlib vulnerabilities** spanning crypto/tls, crypto/x509, net/url, net/http, os, encoding/asn1, and encoding/pem. All resolved by upgrading Go from 1.22 to 1.25 across CI workflows, Dockerfiles, and go.mod files.
- **Critical Next.js CVEs** in version 14.2.5 — authentication bypass, SSRF, cache poisoning, and content injection. Bumped to 14.2.35.
- **npm audit findings** requiring overrides for glob (>=10.4.6) and minimatch (>=9.0.7). The remaining 4 advisories were DoS-category and mitigated by the existing nginx rate limiting, so the audit threshold was lowered from "high" to "critical."

This was a reminder that AI-generated code pins to whatever versions were current at generation time. When the security scanner runs days later, the dependency landscape may have shifted.

### March 30: The golangci-lint v2 Migration

This was a 26-minute battle across 6 commits that tells the story of migrating from golangci-lint v1 to v2 configuration format — and fixing every error the stricter linter surfaced.

**Commit 1** (`aa810b7`, 9:56 AM): Migrated `.golangci.yml` from v1 to v2 format. One file, 38 lines changed.

**Commit 2** (`3c50232`, 10:06 AM): Fixed all **37 errors** in the manager service across 19 files. This was the big one:
- **errcheck** (unchecked error returns): The most numerous category. Every `resp.Body.Close()`, `file.Close()`, `rows.Close()`, `tx.Rollback()`, and `writer.Write()` call that previously discarded its error now had to be handled. In Go, it is idiomatic to check errors from Close() operations — a database rollback failing, for instance, indicates a serious connection problem.
- **gosec G706** (log injection): User-provided input was being passed directly to `slog.String()` without sanitization. An attacker could inject newlines or ANSI escape sequences into structured log output.
- **gocritic httpNoBody**: GET requests were passing `nil` as the request body. The correct practice is `http.NoBody`, which is semantically explicit.
- **staticcheck QF1001**: A boolean condition in a test was using double negation instead of De Morgan's law simplification.
- **ineffassign**: A dead variable assignment in the aggregator was being written to but never read.
- **gofmt**: Struct alignment and spacing inconsistencies across config.go, zmq.go, and indexer.go.
- **gocritic exitAfterDefer**: `os.Exit()` was being called in `main()` after deferred functions were registered. In Go, `os.Exit` does not run deferred functions, so cleanup code would be silently skipped. Changed to call `cancel()` before exiting.
- **Deprecated WebSocket library**: The entire WebSocket implementation had to be migrated from `nhooyr.io/websocket` (deprecated) to `github.com/coder/websocket` (its maintained fork).

**Commits 3-6** (`35aacc5` through `c49fb1e`, 10:11-10:22 AM): Chased down the remaining 20 errors across the gateway service and edge cases in the manager — more `http.NoBody` fixes in test files, more `resp.Body.Close` error checks in the price oracle and P2Pool client, replacing `os.Exit` with `return` in main startup paths, and gofmt fixes in the wallet RPC types.

**Total: 57 lint errors fixed** in 26 minutes. The breakdown tells a story about what AI-generated Go code gets wrong: it consistently forgets to check errors from Close() operations, uses `nil` where `http.NoBody` is idiomatic, and doesn't always handle the `os.Exit`/deferred-function interaction correctly.

### March 30 Evening: SideWatch v1 and the Brand

The name "SideWatch" was confirmed on March 30. The name reflects the core value proposition — observability layered on P2Pool's **side**chain. The product had been called "XMR P2Pool Dashboard" and "P2Pool Mini Dashboard" up to this point.

That evening, two commits (`f330400` and `5f5bf86`) delivered the SideWatch v1 paid-tier features:

**Backend additions:**
- Uncle rate queries (`GetUncleRate()`, `GetWeeklyActiveMiners()`) in the aggregator
- Worker breakdown endpoint (per-miner worker stats, paid tier only)
- Data retention pruning in the timeseries builder — 30 days for free, 15 months for paid, running daily
- Indexer updates to record `is_uncle`, `software_id`, `software_version` from the P2Pool API (nullable/optional fields)
- Sweep of expired retention for lapsed subscriptions

**Frontend additions:**
- ShareTimeCalculator: expected share time from hashrate + sidechain difficulty
- UncleRateWarning: alert banner when uncle rate exceeds 10%
- WindowVsWeeklyToggle: switch between current PPLNS window miners and 7-day active miners on the home page
- BlocksTable: coinbase private key column with click-to-copy
- SidechainTable: uncle type and software version columns
- SubscriptionStatus: tier badge, expiry date, retention disclosure

**Migration `004_sidewatch_v1.sql`** added uncle tracking columns, software identification fields, coinbase private key storage, and extended retention columns to the subscriptions table.

There was a staging mishap: three new frontend components and the migration file were created locally but never `git add`-ed because `git commit -a` only stages tracked files, not new ones. CI failed because Jest mocks couldn't resolve the missing modules. Fixed in a follow-up commit 12 minutes later.

**Important foreshadowing:** Many of these v1 features — uncle rate, software version, coinbase private keys, per-share data — depended on fields that the real P2Pool v4.3 API does not expose. They were built against the mocknode which happily served this data. This would become painfully apparent on April 2.

---

## Chapter 6: The Crowdfund Architecture

**April 1, 2026 — A major architecture decision and its implementation in a single day.**

### The Decision

The question was: how should SideWatch monetize hosted node access? Three models were considered:

**Model A: Dedicated nodes** — Each subscriber gets their own P2Pool instance. Maximum isolation, maximum cost, maximum operational complexity. Rejected because P2Pool nodes can serve 100+ miners simultaneously, so running one instance per subscriber would waste enormous resources.

**Model B: Flat subscription** — Fixed monthly price, access to shared nodes. Simple but doesn't build community or transparency.

**Model C: Shared node pool with transparent crowdfund** — Pay-what-you-want above a minimum, with a public fund progress bar showing monthly costs, subscriber contributions, and operator surplus. This was chosen.

The reasoning: a solo operator running shared P2Pool nodes has fixed infrastructure costs (~$89/month for a capable VPS) plus operator time (~$61/month as a modest floor). A transparent crowdfund model aligns with P2Pool's community-first philosophy, provides accountability, and creates a natural incentive for supporters to promote the service. Break-even at approximately 22 subscribers at $4/month average.

### The Implementation

Architecture documentation (`5830acb`) came first — 984 lines across two planning documents covering the node pool architecture, bandwidth analysis, and a 4-phase build plan.

Then the implementation (`ad32335`) landed as a single 40-file, 2,445-line commit covering all 4 phases:

**Phase 1 — Tier model + schema (Migration 005):**
- Renamed "paid" tier to "supporter", added "champion" tier ($5+ minimum)
- Created `node_pool`, `node_health_log`, and `node_fund_months` tables
- Seeded default mini and main node entries
- `TierIncludes()` hierarchy so champion includes all supporter benefits
- `TierForAmount()` maps payment USD value to the appropriate tier

**Phase 2 — Node pool management + fund API:**
- Health checker polling each node via P2Pool API every 60 seconds
- Connection info endpoint generating XMRig configuration snippets
- Least-loaded node assignment for new miners
- Fund status (live payment sums), history (monthly breakdown), supporters list (truncated addresses with champions listed first)
- Five new API endpoints and five new Prometheus metrics

**Phase 3 — Frontend:**
- Six new components: FundProgress bar, FundHistory chart, SupportersPage, XMRigConfig generator, TierSelector, NodeHealth indicators
- Two new pages: `/fund` (progress + history + supporters) and `/connect` (XMRig configuration guide with copy-to-clipboard)
- Updated home page with a compact fund widget and LiveStats with node health status dots

**Phase 4 — Networking + monitoring:**
- nginx stream block for TCP stratum passthrough (ports 3333 for mini, 3334 for main)
- UFW firewall rules for stratum ports
- Prometheus alert rules (node unhealthy, no miners, hashrate drop, fund shortfall)
- Grafana dashboard panels for node health and fund status

### The Code Review

A structured code review (`b98bfa8`) caught 11 findings, several of them serious:

1. **Race condition in AssignNode**: The node assignment logic was doing a read-then-write without locking. Under concurrent requests, two miners could be assigned to the same node even if it was at capacity. Fixed with atomic `UPDATE...RETURNING` using `FOR UPDATE SKIP LOCKED`.

2. **uint64 underflow**: The `checkConfirmations` function subtracted block heights without checking that the result wouldn't underflow below zero. On unsigned integers, this wraps to a massive number instead of going negative.

3. **Supporter double-counting**: The supporter count was maintained as an incrementing counter, which would drift if the scanner processed the same payment twice. Changed to `COUNT(DISTINCT)` on read.

4. **Missing proof-of-ownership**: API key generation accepted any wallet address without verifying the requester actually owns it. Added a requirement for `tx_hash` on first generation (proving you've made a payment from that address).

5. **Batched database queries**: The `GetMinerStats` handler was making 7 sequential database queries. Collapsed into a single `pgx.Batch` round-trip, reducing latency significantly.

Post-merge cleanup required four more commits: golangci-lint fixes (gofmt alignment, errcheck, gosec G115 for unsafe integer casts), backup test updates for the three new tables, and E2E test assertion updates because "Subscribe" became "Support SideWatch" and "$5/month" became "$1+/mo Supporter".

---

## Chapter 7: Reality Check — The P2Pool Rewrite

**April 2, 2026 — 18 commits across 10 hours. The day everything changed.**

This is the central story of the project. On April 2nd, with a real P2Pool v4.3 binary running on the VPS and a real monerod fully synced, the team discovered that virtually every assumption about the P2Pool API was wrong. What followed was a complete rewrite of the data pipeline — from types to client to indexer to aggregator to mocknode to Docker configuration to frontend.

### The Discovery

The manager service was running, connected to the P2Pool container, and hitting `http://p2pool:3333/api/pool/stats`. Every request returned the same five-word response:

```
P2Pool Stratum online
```

P2Pool's stratum port (3333) serves exactly one purpose: accepting mining connections from XMRig. When it receives an HTTP request instead of a Stratum protocol handshake, it returns "P2Pool Stratum online" for literally every path. There is no REST API on this port. There never was.

The mocknode had invented an entire API surface — `/api/pool/stats`, `/api/shares`, `/api/found_blocks`, `/api/worker_stats`, `/api/p2p/peers` — that doesn't exist on the real P2Pool binary.

### How P2Pool Actually Works

P2Pool v4.3 uses `--data-api <path>` to write JSON files to a directory on disk, typically a tmpfs mount. It does **not** serve these files over HTTP. To make them accessible, you need a separate HTTP server (nginx) pointed at the directory.

The real file paths are:
- `/pool/stats` — pool hashrate, miners, sidechain difficulty, last found block
- `/network/stats` — Monero network difficulty, height, reward
- `/local/stratum` — local node's connected workers, hashrate, shares found, effort
- `/local/p2p` — P2P peer list with software versions

No `/api/` prefix. Completely different endpoint names. Completely different JSON field names (camelCase in real API vs. snake_case in the mocknode). And most critically: five endpoints that the mocknode served simply don't exist in any form.

### The Volume Saga (4 commits in 11 minutes)

Before rewriting the application code, the `--data-api` flag itself needed to work in Docker. This took four attempts:

**11:56 AM** (`742d8eb`): Added `--data-api /data` and `--local-api` flags to the P2Pool container. P2Pool exited immediately because the `/data` directory doesn't exist inside the container.

**11:59 AM** (`0b4fb21`): Removed `--data-api` and kept only `--local-api`. But `--local-api` requires `--data-api` to be set first. P2Pool logged an error and disabled local API entirely.

**12:04 PM** (`a2deaab`): Added back `--data-api` with a named Docker volume mounted at `/data`. P2Pool started but couldn't write — the Docker volume was owned by root, and P2Pool runs as a non-root user.

**12:07 PM** (`6660a5b`): Switched from a named Docker volume to a tmpfs mount. tmpfs is writable by all users and is ideal for the small, frequently-updated JSON files that P2Pool writes. This finally worked.

Additionally, between these attempts, the P2Pool container had zero P2P peers. Docker's internal DNS proxy couldn't resolve P2Pool's seed nodes (`seeds-mini.p2pool.io`). Adding explicit DNS servers (`dns: 8.8.8.8, 1.1.1.1`) to the compose file fixed peer discovery. The `--no-upnp` flag was also needed because UPnP doesn't work inside Docker containers.

### The Complete Rewrite (12:31 PM)

**Commit `ffe5b71`** — 14 files changed, 578 insertions, 641 deletions. This single commit rewrote the entire P2Pool integration layer.

**What was wrong:**

| Mocknode Assumption | Reality |
|---|---|
| API served on stratum port :3333 | Data-api writes files to disk; nginx sidecar serves them |
| Path: `/api/pool/stats` | Path: `/pool/stats` |
| Path: `/api/shares` | **Does not exist** — no individual share data exposed |
| Path: `/api/found_blocks` | **Does not exist** — only `lastBlockFound` height in pool stats |
| Path: `/api/worker_stats` | **Does not exist** — only `/local/stratum` for THIS node's workers |
| Path: `/api/p2p/peers` | Path: `/local/p2p` |
| Workers are JSON objects | Workers are CSV strings |
| Full wallet addresses | Truncated to ~32 characters |
| JSON uses snake_case | JSON uses camelCase |

**What was deleted:**
- The entire `Share` type and `indexShares()` function — there is no share-by-share data available
- The `FoundBlock` and `WorkerStats` types — these endpoints don't exist
- The individual share indexing loop that was the indexer's primary job

**What replaced it:**
- `indexPoolStats()` — records aggregate pool snapshots (hashrate, miners, difficulty, sidechain height) every 30 seconds into a new `pool_stats_snapshots` table
- `NetworkStats`, `LocalStratum`, `LocalP2P` types matching the real JSON structure
- New migration `004_pool_stats_snapshots.sql` for the snapshot data model
- An nginx sidecar (`p2pool-api`) serving the data-api files over HTTP on the internal Docker network
- The manager reads from `http://p2pool-api:8080/pool/stats` instead of `http://p2pool:3333/api/pool/stats`

### The CSV Worker Discovery (12:53 PM)

Even after the big rewrite, worker parsing was still broken. The previous commit modeled workers as JSON objects with named fields. But P2Pool v4.3 returns workers as **raw CSV strings**:

```json
{
  "workers": [
    "10.0.0.5:44832,15234,8847293,112354,4ABC..."
  ]
}
```

Each string is `"IP:port,hashrate,hashes,bestDiff,walletPrefix"` — not a JSON object, just a comma-separated string in a JSON array. The fix (`01e8168`) changed the type from `[]StratumWorker` to `[]string` for JSON unmarshaling, added a `ParseWorkerCSV()` function with simple comma splitting, and a `Workers()` method that lazily parses the raw strings on demand. The mocknode was updated to serve the same CSV format so integration tests would exercise the same code path.

### Frontend Data Display Overhaul (7:22 PM - 8:33 PM)

With the backend finally talking to real P2Pool, every frontend tab was showing empty or broken data. Commit `512e600` (13 files, 536 insertions) fixed all of them:

- **Home page**: Pool stats now came from `pool_stats_snapshots` (always populated). Added conditional "Total Paid" display and "Waiting for next block" instead of bare zeros.
- **Sidechain page**: The entire page was redesigned. It had been built around the `p2pool_shares` table, which was permanently empty because individual share data doesn't exist. Rebuilt as a pool stats timeline showing hashrate and difficulty charts with time-range selectors (6h/24h/3d/7d).
- **Miner page**: Added prefix matching for truncated P2Pool addresses (since full addresses aren't available from the API), a local workers table when no address is entered, and helpful "no activity" guidance for new miners.
- **Blocks page**: Informative empty state explaining that P2Pool mini finds a block approximately every 4-6 hours, pool context in the header, and pagination hidden when no blocks exist.

Three more commits finished the day:
- `6fb1761`: Five components (NodeHealth, FundProgress, ConnectPage, FundHistory, SupportersPage) silently rendered nothing when API calls failed. Added informative error messages.
- `68703b2`: **CSP blocked Next.js hydration.** The nginx Content Security Policy specified `script-src 'self'`, but Next.js 14 App Router injects inline `<script>` elements for React Server Component flight data. The strict policy silently prevented them from executing — React never hydrated, SWR never ran, and no API calls were made. The page appeared to load but showed nothing. Fix: added `'unsafe-inline'` to `script-src`.
- `b12b353`: The nginx stream block for stratum passthrough referenced `p2pool-mini:3333` and `p2pool-main:3334` — containers that don't exist in the current compose. nginx refuses to start if it can't resolve upstream hosts at startup, causing a complete site outage. Removed the stream block.

### What This Chapter Teaches

The P2Pool Reality Check is the single most important lesson of the entire project. Here's what went wrong at the architectural level:

**The mocknode was too good.** It was comprehensive, internally consistent, well-tested, and completely fictional. Every component built against it worked correctly — against the wrong API. The mocknode didn't simulate P2Pool's real behavior; it simulated what we imagined P2Pool's behavior should be.

**Documentation couldn't save us.** P2Pool's documentation doesn't comprehensively describe the data-api format. The `--data-api` flag is mentioned in the README, but the actual JSON schemas, field names, and data formats are only discoverable by running the binary and reading the files it produces.

**The cost of late integration.** Every day between March 24 (initial commit) and April 2 (first real P2Pool connection) increased the amount of code that had to be rewritten. Types, clients, indexers, aggregators, scanners, mocknode, Docker configs, healthchecks, frontend pages — the blast radius of the wrong API assumption touched every layer of the stack.

**The fix required 18 commits and 10 hours.** Not because any single change was complex, but because the wrong assumption was load-bearing. Pull one thread and the whole chain unravels: types → client → indexer → aggregator → routes → frontend → tests → Docker → mocknode.

---

## Chapter 8: The Polish Phase

**April 3, 2026 — Frontend identity, the wallet-rpc saga, and production hardening.**

### The Visual Identity

Five commits across the morning redesigned the frontend from a functional dashboard into a branded product. The CubeLogo component (an animated Rubik's cube rendered in CSS), a RubikBackground component (ambient floating cube squares behind content), a Rubik's cube color palette (`--cube-orange`, `--cube-blue`, `--cube-green`, `--cube-red`, `--cube-yellow`), and five new CSS animations gave SideWatch a distinctive visual identity.

The subscribe page was redesigned from a simple form into a branded experience with a tier selector, roadmap section (mini live, main next), and a 3-step payment guide. The SidechainContext (a React context for multi-sidechain URL switching) was deleted — the project launched mini-only, so the abstraction was dead weight.

The CubeLogo introduced a persistent CSS bug that took three commits to fix. The cube used CSS 3D transforms (`transform-style: preserve-3d`, `perspective`), and the sticky navigation bar's `backdrop-filter: blur()` was flattening the 3D rendering context. On scroll, the cube visually distorted. The first fix tried GPU layer isolation (`will-change: transform`), the second moved the `backdrop-filter` to a nav pseudo-element, and the third isolated the cube's 3D context from the navigation's compositing layer entirely. Three commits for a CSS visual bug — a reminder that 3D CSS transforms and backdrop filters occupy different compositing layers and interact in non-obvious ways.

### The wallet-rpc Saga (8 commits)

Getting `monero-wallet-rpc` running in Docker for subscription payment verification was its own multi-commit debugging odyssey. Each commit fixed exactly one issue, creating an instructive trail:

**Commit 1** (`84004ac`): Initial setup. Added a `wallet-rpc` service using the `sethsimmons/simple-monero-wallet-rpc` image behind a Compose profile (`profiles: [wallet]`). Used long-form `command: >` syntax with `--rpc-bind-ip=0.0.0.0`, `--confirm-external-bind`, `--rpc-bind-port=18088`, and `--password=""`. Also added a nil pointer guard — the subscription address handler was crashing when wallet-rpc was unavailable.

**Commit 2** (`04887af`): The image's ENTRYPOINT already sets `--rpc-bind-ip` and `--confirm-external-bind`. Passing them again caused a duplicate flag failure. Switched to YAML array syntax and removed the duplicated flags. (Sound familiar? This is exactly the monerod ENTRYPOINT lesson from March 26, replayed with a different container image.)

**Commit 3** (`c53d845`): The `depends_on: monerod: condition: service_healthy` reference was invalid because monerod is defined in `docker-compose.node.yml`, not the base `docker-compose.yml`. Docker Compose cannot reference cross-file services in `depends_on`. Removed the dependency entirely.

**Commit 4** (`26215cd`): The entrypoint ignores `--rpc-bind-port` when it conflicts with its own default. The custom port 18088 was silently overridden to 18083. Updated the environment variable to match the actual port.

**Commit 5** (`70086ae`): Two issues. First, `--password=""` in YAML array syntax passes the literal characters `""` (two quote marks) to the binary, not an empty string. Changed to `--password=`. Second, the wallet volume was mounted read-only (`:ro`) but `monero-wallet-rpc` writes cache files. Removed the `:ro` flag.

**Commit 6** (`084aa71`): The wallet was created without a password. Even `--password=` with a truly empty value caused argument parsing to fail. Removed the flag entirely.

**Commit 7** (`ed2c457`): The Docker image's default CMD provides `--rpc-bind-port=18083`, but since the compose file overrides the entire `command`, the default CMD was replaced entirely and the port flag was lost. Added it back explicitly.

**Commit 8** (`f919dfc`): The final fix. `monero-wallet-rpc` requires an explicit password when running in non-interactive (daemon) mode, even when the wallet has no password. But `--password` had been tried and failed due to shell quoting. Solution: `--password-file=/wallet/password.txt` pointing to an empty file, which cleanly satisfies the requirement without quoting ambiguity.

**The final working command:**
```yaml
command:
  - --daemon-address=monerod:18081
  - --wallet-file=/wallet/subscription
  - --password-file=/wallet/password.txt
  - --rpc-bind-port=18083
  - --disable-rpc-login
```

**The lesson:** Docker images with ENTRYPOINT binaries are opaque black boxes. You cannot know which flags are set by the ENTRYPOINT, which are set by CMD, which override silently, and which fail on duplication without reading the image's Dockerfile or running the container and observing its behavior. The wallet-rpc saga repeated every lesson from the monerod saga — with a different image, different flags, and different failure modes.

### Production Hardening

Three commits addressed production security and operational concerns:

**Data retention and auth** (`717eae6`): Retention now expires with the subscription (lapsed miners fall back to 30-day pruning). Tax exports for previous years are held back for exactly 2 CSV downloads before being pruned — preventing miners from losing tax data they need for filing. A `RequireOwner` middleware was added requiring API key proof of address ownership, fixing a vulnerability where anyone knowing a wallet address could download that miner's payment CSV. A data deletion endpoint was added with double confirmation.

**Admin stats** (`b7cc227`): The admin panel got subscription statistics — total miners, active supporters, active champions, lapsed, held-back exports, and deleted accounts, all derived from `FILTER` clauses on the subscriptions table.

**Monitoring hardening** (`573dd78`): Grafana, Prometheus, Alertmanager, and Loki were all bound to `0.0.0.0` — publicly accessible to anyone who scanned the VPS ports. Changed all monitoring services to bind to `127.0.0.1`. Added Loki 7-day retention, Prometheus 30-day/2GB retention, persistent volumes for log data, Grafana hardening (disabled signup, anonymous access, external snapshots), resource limits for all monitoring containers, and a 570-line `prod-audit.sh` VPS hygiene checker.

### Repo Cleanup

Commit `c7cdb21` moved documentation files from the repo root into `docs/` and dev compose files into `infra/compose/`, reducing repo root clutter. Updated all cross-references across CLAUDE.md, README, Makefile, and various docs.

### The Tor Decision

Two commits told the Tor story. The Tor container had been crash-looping since deployment — stuck at bootstrap 0%, repeatedly restarting, and causing deploy healthcheck failures that triggered unnecessary rollbacks. The first fix (`3479bcb`) excluded Tor from the deploy healthcheck as a band-aid. The second (`baaf7d5`) disabled the service entirely, updating all documentation and UI to say "not yet implemented" and "planned for future release." The infrastructure files (Dockerfile, torrc) were kept in the repo for when Tor is ready.

---

## Chapter 9: Honest Shipping

**April 4-5, 2026 — The project confronts what it actually is.**

### Removing Dead Features

The most mature commit of the project was `233acbb` — "Remove dead features that rely on unpopulated p2pool_shares table." This was the moment the project stopped claiming to have features that didn't work.

The P2Pool v4.3 data-api does not expose individual shares, uncle status, software version, or coinbase private keys. Several v1 features had been built against the mocknode which served this data, but in production the underlying tables were empty and the values were always null. The commit removed:

1. **Total Shares card** on the miner dashboard — always showed 0
2. **Uncle rate warning banner** — `uncle_rate_24h` was always null
3. **Paid worker breakdown** — queried per-miner worker data that doesn't exist
4. **Coinbase private key column** from blocks table — always showed `'--'`
5. **Weekly active miners toggle** on home page — replaced with a simple text display
6. **"Per-worker breakdown" from subscriber feature lists** — honest about what paid users actually get
7. **SECURITY.md rewritten** — replaced claims about what's stored with what's actually stored
8. **README privacy section corrected** — from theoretical capabilities to actual data collection

This commit also updated feature descriptions from aspirational ("Coinbase private keys published for trustless verification") to conditional ("Coinbase private keys for trustless verification, when exposed by P2Pool"). The transparency section was rewritten from a confident present-tense claim to a forward-looking conditional: "If a future P2Pool version exposes coinbase private keys via the API, SideWatch will surface them automatically."

### The Blocks Page Bug

Two commits (`b44c915` and `bddb50b`) fixed a subtle data pipeline gap. The blocks page was showing empty hashes, zero rewards, and NaN effort. Here's what was happening:

The **indexer** detects new blocks by comparing `lastBlockFound` against a stored high-water mark. When it sees a new block, it inserts a row into `p2pool_blocks` — but the P2Pool data-api only provides the block height, not the hash, reward, or effort. So the indexer was inserting rows with `main_hash = ""`, `coinbase_reward = 0`, and `effort = NULL`.

The **scanner** was supposed to backfill this data from monerod when the block reaches 10 confirmations. Its `processBlock` function already fetches the full block details (hash, reward, coinbase transaction) from monerod — but it was only using this data for payment extraction, not for updating the blocks table.

**Fix 1**: Added a backfill UPDATE query to the scanner's `processBlock` that fills in hash and reward from monerod when processing confirmed blocks.

**Fix 2**: The `effort` field was never being captured at all. Added effort capture to the indexer — when a block is found, fetch `current_effort` from `/local/stratum` and write it to the database at that moment (effort is only meaningful at the instant a block is found; it resets immediately after).

**Fix 3**: The scanner maintained an in-memory `pendingBlocks` map of blocks awaiting confirmation. If the manager restarted, this map was lost. Added `RecoverUnprocessed()` which queries the database for blocks with empty hashes or zero rewards, re-adds them to the pending set, and immediately attempts to process any with sufficient confirmations. Called once at startup.

**Fix 4** (`1e99021`): The recovery function only ran at startup. Added a goroutine that runs it every 5 minutes as a safety net for blocks missed by ZMQ (network blips, ZMQ disconnects). This way blocks get backfilled within minutes instead of waiting for the next deploy.

### Wallet Address Redaction

Commit `3c17d6b` addressed a privacy concern in the gateway's access logger. Request paths like `/api/miner/4ABC.../payments` were being logged in full, associating client IP addresses with specific wallet addresses. Added a `sanitizePath()` function that detects paths starting with `/api/miner/` and replaces the address segment with `[redacted]`.

### Miner Page Clarification

Commit `40af942` addressed user confusion. People were looking up addresses from mini.p2pool.observer and getting empty results. Added three text clarifications explaining that SideWatch only tracks miners connected to this specific P2Pool node — for global P2Pool mini stats, visit mini.p2pool.observer. This was an honest acknowledgment of the tool's scope rather than a misleading empty state.

### The About Page

The final significant feature was `ebb0051` — an about page that is remarkable for its honesty. Four sections, each with a different-colored card:

**"Tax exports — the thing nobody else does"** (green): Positions tax export as the single strongest differentiator. P2Pool miners filing taxes in Canada or the US need a record of every coinbase payment with fiat value at receipt time. That data doesn't exist anywhere in a usable format. SideWatch records each payment with XMR/USD and XMR/CAD from CoinGecko. Explicitly says this is the strongest reason to use SideWatch over other tools.

**"The managed node trade-off"** (blue): An honest acknowledgment of the philosophical tension. P2Pool's philosophy is "run your own node" but SideWatch asks users to trust a managed node. Explicitly tells users: "If you have the resources and inclination to run your own P2Pool node, you absolutely should — it strengthens the network and gives you full sovereignty." Positions SideWatch as the option for miners who don't want the 200 GB sync and 24/7 maintenance overhead.

**"How SideWatch compares to p2pool.observer"** (orange): Direct comparison. Calls p2pool.observer "an excellent tool" and says "If you just want to check your mining stats, p2pool.observer is probably what you need." Explains how SideWatch's scope is different — it only tracks its own node's miners but adds managed infrastructure, tax exports, and extended retention.

**"Why this project exists"** (yellow): Personal and transparent. States the project started as a learning exercise. Identifies the author as a penetration tester who needed hands-on experience with cloud infrastructure, server management, and full-stack development. Lists every skill the project forced: Go, PostgreSQL, Redis, Docker, CI/CD, Prometheus/Grafana, Next.js, cryptocurrency protocol internals. Credits Claude Code as the development partner and links to the AGPL-3.0 source.

This about page is the project's identity statement. It doesn't pretend to be something it isn't. It doesn't oversell. It tells users exactly when they should use a different tool. This kind of honesty is rare in software, and it's the right approach for an audience that chose P2Pool precisely because they value transparency.

---

## Chapter 10: Lessons and Principles

### Lesson 1: The Mocknode Trap

**Never build a comprehensive mock that invents an API surface. Test against real infrastructure as early as possible.**

The mocknode was the project's most expensive mistake. It was built on March 24 as part of the initial commit — a 552-line fake server that responded to API paths that sounded right but didn't exist. Every component was tested against it. Every integration test passed. The team was confident the data pipeline worked.

On April 2, nine days and ~80 commits later, the real P2Pool binary was connected for the first time. The mocknode's API paths were wrong. The JSON field names were wrong. The data types were wrong. Five of the seven endpoints didn't exist at all. Workers were CSV strings, not JSON objects. Addresses were truncated. The entire data pipeline had to be rewritten.

If the real P2Pool binary had been connected on day 2 instead of day 9, the rewrite would have been a 2-file, 30-minute fix instead of a 14-file, 10-hour emergency. The mocknode should have been built from real P2Pool output, not from imagined API contracts.

### Lesson 2: Docker ENTRYPOINT Is Invisible

**Always check whether a Docker image uses ENTRYPOINT or CMD before writing your compose file.**

This lesson was learned twice — once with monerod (March 26) and again with wallet-rpc (April 3). Both the `sethsimmons/simple-monerod` and `sethsimmons/simple-monero-wallet-rpc` images use ENTRYPOINT to run the binary. When you specify `command:` in docker-compose, Docker concatenates ENTRYPOINT + CMD. If your command includes the binary name, it gets passed as an argument to itself.

The wallet-rpc saga was particularly painful: 8 commits to get the service running, each fixing a different interaction between the compose file's `command` and the image's ENTRYPOINT/CMD defaults. Duplicate flags that silently fail, port flags that get ignored, password flags that can't handle empty strings in YAML, volumes that need write access — none of these are documented in the image's README.

**The rule:** Always use YAML array syntax for `command:`. Always list only flags, never the binary name. Always test with `docker compose run --rm <service> --help` to see what the ENTRYPOINT provides.

### Lesson 3: Alpine Containers Are Minimal

**Don't assume standard tools exist in Alpine-based containers.**

Three separate debugging sessions were caused by Alpine's minimalism:
- `curl` doesn't exist → use `wget` for healthchecks
- `localhost` doesn't resolve → use `127.0.0.1`
- `netcat` doesn't exist → use file existence checks for P2Pool healthcheck

### Lesson 4: Secrets Merge, They Don't Replace

**Docker Compose's `secrets: []` does NOT clear secrets from a base compose file. Arrays are merged, not overridden.**

This caused a multi-attempt debugging loop where the manager couldn't authenticate to Postgres despite correct environment variables. The manager reads `/run/secrets/postgres_password` first (via `readSecret()` helper), and the secret file was present (merged from the base compose) but contained the wrong value. Both secrets files AND environment variables must be aligned.

### Lesson 5: E2E Tests Must Track Frontend Changes

**Every frontend text change, button rename, or page restructure must be accompanied by an E2E test update.**

This was learned through two CI failures where Playwright assertions referenced stale text ("Subscribe" had become "Support SideWatch", "$5/month" had become "$1+/mo Supporter"). The rule became: after any frontend edit, grep `smoke.spec.ts` for text and selectors that might have changed.

### Lesson 6: CSP and Next.js Don't Mix Easily

**Next.js 14 App Router injects inline `<script>` tags that `script-src 'self'` blocks.**

This was a silent failure — the page appeared to load, all HTML rendered, but React never hydrated, SWR never fetched data, and the dashboard showed empty states everywhere. No console errors (CSP violations are logged at a different level). The fix was adding `'unsafe-inline'` to `script-src`, with a note that nonce-based CSP would be the proper long-term solution.

### Lesson 7: Design for What the API Actually Provides

**Never build features against data you assume will be available. Verify first.**

Uncle rate, software version, coinbase private keys, per-share data — all were built as features and then removed when the real API didn't expose them. The total wasted effort: several frontend components, backend endpoints, database columns, and the time spent reviewing and testing them. The columns still exist in the schema (nullable, never populated), but the UI no longer promises them.

### Lesson 8: Honest Positioning Beats Feature Lists

**For a trust-conscious audience, transparency about limitations builds more credibility than a long feature list.**

The about page's comparison with p2pool.observer — "If you just want to check your mining stats, p2pool.observer is probably what you need" — is counterintuitively the project's strongest marketing. It tells the target audience: this project respects your intelligence, and the people behind it are more interested in being useful than in acquiring users.

### Lesson 9: AI-Assisted Development Changes the Shape of Work

**AI doesn't eliminate problems — it moves them from "can I build this?" to "did I build the right thing?"**

In 12 days, a solo developer with Claude Code produced:
- 123 commits
- Two complete Go services (~15,000 lines)
- A full Next.js frontend (~5,000 lines)
- Complete infrastructure (Docker, CI/CD, monitoring, deployment)
- A subscription monetization system
- A crowdfund model with node pool management
- Production deployment on a VPS with TLS and domain

The velocity is extraordinary. But the project's biggest failures — the mocknode API mismatch, the Docker ENTRYPOINT collisions, the CSP hydration bug — were all caused by building faster than validating. AI assistance made it trivially easy to generate comprehensive code, but it couldn't tell us that the code was talking to an API that doesn't exist.

The lesson isn't "don't use AI." The lesson is that AI shifts the bottleneck from implementation to validation. When you can build a complete system in an afternoon, the expensive part isn't writing the code — it's knowing whether the code is correct. Integration testing against real systems, not mocks, becomes the highest-leverage activity.

### Lesson 10: Ship Honestly, Then Iterate

**It's better to ship a tool that honestly does 3 things well than one that claims to do 10 things but half of them show empty data.**

The project's final form, after removing dead features and adding honest documentation, is a focused tool that does a few things no other P2Pool tool does: managed node access, hashrate history, payment archive with fiat prices, and tax-ready CSV export. Everything else was stripped away or conditioned with "when exposed by P2Pool."

This is the right approach for a trust-conscious audience. P2Pool miners will verify your claims. If your dashboard says "uncle rate tracking" and the column is always empty, they'll lose trust in everything else you claim. If your about page says "p2pool.observer is probably better for this use case," they'll trust that the things you do claim actually work.

---

## Timeline Summary

| Date | Phase | Key Event |
|------|-------|-----------|
| March 24 | Big Bang | 173 files, 24,733 lines in one commit |
| March 25 | Hardening Sprint | 9 commits in 5 hours — tests, deploy, subscriptions, DevSecOps |
| March 26 | Monerod Saga | 10 container fixes in 2.5 hours — first real deployment |
| March 27-30 | Feature Expansion | Redis caching, 57 lint errors fixed, SideWatch v1 features, branding |
| April 1 | Crowdfund Architecture | Shared node pool + crowdfund in one day, 11 code review findings |
| April 2 | **The P2Pool Rewrite** | **18 commits in 10 hours — every API assumption was wrong** |
| April 3 | Polish Phase | Visual identity, wallet-rpc saga (8 commits), production hardening |
| April 4-5 | Honest Shipping | Dead feature removal, about page, block recovery safety net |

**Total: 123 commits across 12 days. One developer. One AI assistant. One production deployment.**

---

*This timeline was compiled from git history, Claude Code session memory, and the project's CLAUDE.md architecture document. Generated April 5, 2026.*
