# SideWatch — P2Pool Mini Observability Dashboard

**[sidewatch.org](https://sidewatch.org)**

SideWatch is a free, open-source observability dashboard for Monero miners on
**P2Pool mini**. It provides real-time hashrate monitoring, payment tracking,
share timelines, uncle rate analysis, and tax-ready CSV exports -- all without
requiring you to run your own node infrastructure.

Point your miner at SideWatch's hosted P2Pool node, keep 100% of your rewards,
and get full visibility into your mining performance.

---

## What Problem Does This Solve?

P2Pool is the gold standard for decentralized Monero mining: every block reward
goes directly to your wallet on-chain. No pool operator can skim fees, delay
payouts, or freeze your funds.

But running P2Pool mini yourself means:

- Syncing a **full Monero node** (~200 GB and growing)
- Running a **P2Pool sidechain node** alongside it
- Keeping both services online **24/7** with updates, monitoring, and backups
- Building your own tooling to understand your mining performance

Most hobby miners just want to mine and see their stats. SideWatch removes the
infrastructure burden and gives you a clean dashboard on top of the P2Pool
sidechain you're already mining on.

---

## Running P2Pool Locally vs. Using SideWatch

There are real tradeoffs. SideWatch doesn't replace self-hosting -- it's an
alternative for miners who want simplicity.

### Reasons to Run P2Pool Yourself

- **Full sovereignty** -- you control every component, no reliance on a third party
- **Network health** -- more independent P2Pool nodes strengthen the sidechain
- **Latency** -- your miner connects to localhost, lowest possible share submission latency
- **Custom configuration** -- tune P2Pool flags, sidechain selection, and node parameters exactly how you want
- **Privacy** -- your stratum connection never leaves your machine

### Reasons to Use SideWatch

- **No sync required** -- skip the 200 GB Monero blockchain download and multi-day initial sync
- **No maintenance** -- we handle node updates, monitoring, restarts, and disk management
- **Instant setup** -- point XMRig at the SideWatch stratum endpoint and you're mining in minutes
- **Built-in observability** -- hashrate history, share timelines, uncle rate tracking, and payment archive out of the box
- **Works on any hardware** -- mine from a laptop, a NAS, or a single-board computer without running heavy infrastructure alongside it
- **Tor access** -- `.onion` endpoint for miners who want maximum privacy (not yet implemented)

Both options give you the same P2Pool guarantees: zero fees, direct-to-wallet
payouts, fully decentralized. SideWatch just handles the infrastructure side.

---

## Features

### Free Tier

Every miner gets full access to the core dashboard at no cost:

- Real-time hashrate monitoring via WebSocket
- Live pool stats (hashrate, miners, sidechain difficulty)
- Block explorer with found block history, reward, and mining effort
- Sidechain metrics over time (pool hashrate, difficulty, miner count)
- Expected share time calculator based on your hashrate vs. sidechain difficulty
- Uncle rate monitoring with elevated rate warnings (>10%)
- Current PPLNS window vs. weekly active miners toggle
- Payment tracking (30-day rolling window)
- Hashrate history (30-day rolling window)
- Tor hidden service access (not yet implemented)

**No account required.** Just point XMRig at the stratum URL and start mining.
No wallet configuration needed in XMRig — the P2Pool node's wallet is pre-configured.
Enter the node's wallet address on the dashboard to see stats.

### Supporters

Supporters unlock extended data retention and power-user features for a
pay-what-you-want contribution (minimum ~$1/month in XMR):

- **15-month data retention** (vs. 30 days on free) for payments and hashrate
- **Tax export** -- CSV download with XMR/USD and XMR/CAD fiat values at time of payment
- **API key** -- integrate your mining data with your own tools or scripts
- **Priority support** via a dedicated channel (not yet implemented)

Payments are paid in XMR directly -- no email, no signup, no KYC. The
system detects your payment automatically using a view-only wallet (the spend
key is kept offline and never touches the server).

### Node Fund (Crowdfunding Model)

SideWatch runs on a transparent **crowdfunding model** rather than a traditional
subscription service. The monthly infrastructure goal (~$150) covers VPS costs,
bandwidth, and operator time. Here's how it works:

- **Supporter** ($1+/month in XMR) -- all subscriber features, listed on the
  supporters page (opt-out available, addresses are truncated)
- **Champion** ($5+/month in XMR) -- everything above, plus priority support
- Contributions above the minimum are pay-what-you-want
- The fund goal and current progress are displayed transparently on the dashboard
- If the fund exceeds its monthly goal, surplus covers future months or
  infrastructure upgrades (additional nodes, regions, main sidechain support)

This model keeps SideWatch aligned with its users: there are no ads, no data
sales, and no incentive to lock miners into proprietary features. The code is
open source under AGPL-3.0 -- anyone can audit exactly what runs on the server.

---

## Privacy

SideWatch is built for the audience that chose P2Pool specifically because they
value decentralization and privacy.

**What we store:** Hashrate history (15-minute buckets from local stratum
workers), payment amounts with fiat prices, found block records, and pool-level
sidechain metrics. Wallet addresses are stored as truncated prefixes only
(~32 characters, as provided by P2Pool's stratum API).

**What we do NOT store:** IP addresses, connection logs, or any data linking
your identity to your wallet address. For additional privacy, use a VPN.
Tor hidden service support is planned for a future release.

---

## Important: Do Not Connect to the Hosted Node

> **The hosted SideWatch stratum endpoint is not ready for public use.**
> P2Pool ties the wallet to the node, not to the miner. If you connect your
> XMRig to someone else's P2Pool node, all rewards go to their wallet — not
> yours. There are no per-miner payouts. This project was built under the
> mistaken assumption that P2Pool splits rewards between connected miners.
> It does not. Until the architecture is reworked to support per-user nodes,
> **do not connect to the hosted stratum endpoint**.

## Getting Started (Self-Hosted Only)

To use SideWatch, run your own instance with your own P2Pool node and wallet:

1. **Follow the [self-hosting guide](docs/SELF-HOSTING.md)** to set up monerod + P2Pool + SideWatch
2. **Set your wallet** via `MONERO_WALLET_ADDRESS` in `.env` — this is the wallet that receives all rewards
3. **Point your own XMRig rigs** at your node: `xmrig -o 127.0.0.1:3333`
4. **Visit the dashboard** to monitor hashrate, shares, payments, and uncle rate

---

## Tech Stack

| Layer | Technology |
|---|---|
| Backend | Go 1.25+, stdlib `net/http`, `log/slog` |
| Database | PostgreSQL 15 via `pgx/v5` (no ORM) |
| Cache | Redis 7 |
| Frontend | Next.js 14, TypeScript, Recharts |
| Auth | JWT (gateway), API keys (subscribers) |
| Metrics | Prometheus + Grafana |
| Logging | Loki + Promtail (structured JSON) |
| Alerting | Alertmanager |
| Containers | Docker Compose, non-root images |
| CI/CD | GitHub Actions with DevSecOps pipeline |
| Privacy | Tor hidden service (not yet implemented) |

### Architecture

```
XMRig --> P2Pool node (mini sidechain)
              |
              |  P2Pool data-api (JSON on tmpfs) + ZMQ block events
              v
        +---------------------------------+
        |         Go Manager              |
        |                                 |
        |  Sidechain poller/indexer       |  30s poll cycle
        |  Coinbase scanner               |  on-chain payment tracking
        |  Hashrate aggregator            |  15-min bucketed timeseries
        |  Uncle rate tracker             |  per-miner uncle detection
        |  Data retention pruner          |  30d free / 15mo paid
        |  Subscription verifier          |  XMR payment detection
        |  WebSocket hub                  |  live stats push
        |  Prometheus metrics             |
        +--------------+------------------+
                       |
               +-------v-------+     +---------------------+
               |  Go Gateway   |     |  PostgreSQL + Redis  |
               |  JWT + Rate   |     |  Prometheus + Grafana|
               |  Limiting     |     |  Loki + Alertmanager |
               +-------+-------+     +---------------------+
                       |
               +-------v-------+
               |  Next.js 14   |
               |  Dashboard    |
               +---------------+
```

---

## Security

SideWatch is a **read-only monitoring service**. We never hold, transfer, or
have access to miner funds. All payouts are handled natively by P2Pool on the
Monero blockchain.

- Non-root containers with least-privilege database roles
- All secrets via Docker secrets (`/run/secrets/`)
- Dual-layer rate limiting (nginx + Go gateway)
- TLS externally; isolated Docker network internally
- No IP-to-wallet correlation logging
- Subscription wallet is view-only (cannot spend)

See [SECURITY.md](docs/SECURITY.md) for full details.

### CI/CD Security Pipeline

Every push triggers a multi-stage pipeline that must pass before deployment:

| Stage | Tools |
|---|---|
| Static analysis | golangci-lint, TypeScript strict mode |
| Vulnerability scanning | govulncheck, npm audit, Trivy (Docker images) |
| Secret detection | Gitleaks (full git history) |
| Testing | Go race detector, Jest, Playwright E2E |
| Backup verification | PostgreSQL backup/restore round-trip |
| Dependency management | Dependabot (Go, npm, Actions, Docker) |

```
Push to main
  --> Go tests (race detector + vet)
  --> Frontend tests (Jest + TypeScript)
  --> Playwright E2E smoke tests
  --> Security scan (lint, vulncheck, audit, Trivy, Gitleaks)
  --> Build & push Docker images to GHCR
  --> Deploy to VPS via SSH
```

---

## Why I Built This

I started SideWatch as a solo project to solve a real problem I had as a P2Pool
miner: I wanted good observability without running infrastructure on my
own hardware. But honestly, the project became as much about the learning as
the product itself.I am actually a penetration tester at heart but needed 
more hands on experience with Cloud Infrastructure and server management. 

Building and operating SideWatch end-to-end -- from architecture to production
-- forced me to develop skills across the full stack:

**Infrastructure as Code (IaC)**
Everything runs in Docker Compose with reproducible provisioning scripts,
systemd service definitions, automated backup/restore, and TLS setup. The
infrastructure is version-controlled and deployable from a single command.
I had previous experience with docker so this was easier for me.

**AI-Assisted Development with Claude Code**
This entire codebase was built with heavy use of [Claude Code](https://docs.anthropic.com/en/docs/claude-code)
as a development partner. Architecture decisions, implementation, debugging,
security review, documentation -- Claude Code was involved at every stage. The
experience taught me how to collaborate effectively with AI tooling: how to
prompt for architecture tradeoffs, when to trust suggestions vs. verify, and
how to maintain code quality with AI-assisted velocity. This is the first project
I do from start to finish with claude code. 

**CI/CD with a DevSecOps Focus**
The pipeline isn't just "run tests and deploy." It includes static analysis,
dependency vulnerability scanning, container image scanning, secret detection,
E2E browser tests, and backup verification -- all gating deployment. Building
this taught me to think about how security tests are integrated in a DevSecOps
environment. I made many design mistakes along the way that I will certainly avoid
in my next projects.

**Go Backend Engineering**
Stdlib-only HTTP routing, structured logging with `slog`, PostgreSQL with raw
SQL (no ORM), Redis caching, WebSocket pub/sub, Prometheus instrumentation,
background job scheduling, and data retention management. Every dependency
choice was deliberate -- minimal surface area, maximum control. This was by far
my weakest gap in knowledge and Claude Code helped a lot.

**Full-Stack Observability**
Prometheus metrics, Grafana dashboards, Loki log aggregation, Alertmanager
rules -- the monitoring stack isn't a bolt-on, it's a core part of the system.
Building it gave me hands-on experience with the same observability patterns
used in production infrastructure. note to self* my metrics query's are still
a little jank. 

**Frontend Development (Next.js + TypeScript)**
Server-side rendering, WebSocket integration, responsive charting with Recharts,
and a full test suite across pages, components, and utility libraries.

**Cryptocurrency Domain Knowledge**
Monero RPC interfaces, P2Pool sidechain mechanics, coinbase transaction parsing,
PPLNS payout windows, uncle shares, and on-chain payment verification. This is
niche knowledge that required reading protocol documentation, node source code,
and hands-on experimentation with a live sidechain. I finally grasp these concepts
on a more fundamental level and "decently" explain them to others without being
confused myself.

**Security Engineering**
Non-root containers, secret management, rate limiting, JWT authentication, TLS
configuration, view-only wallet architecture, and a privacy-first design that
avoids storing anything that could link identities to wallet addresses. Applying
all this hardening allowed me to see what other production webapps can use to 
secure themselves and reduce their vulnerabilities. Even if the server gets 
breached besides destroying/defacing the website, they can't steal anything 
that isn't already public information. There are no funds to be grabbed from anyone.

The source code is available under AGPL-3.0 -- both because transparency builds
trust with privacy-conscious miners, and because it documents every decision I
made along the way.

---

## Deployment

See [DEPLOYMENT.md](docs/DEPLOYMENT.md) for the full VPS deployment guide.

## License

[AGPL-3.0](LICENSE)
