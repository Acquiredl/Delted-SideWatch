# Architecture: Shared Node Pool + Crowdfund Model

> Source: Roadmap items 5+6 (CLAUDE.md "Potential future work")
> Date: 2026-04-01 (revised)
> Mode: Multi-feature architecture (design only, not yet built)
> Prerequisites: Items 3+4 (main sidechain + production validation) must complete first
> Revision: v3 — shared node model, resolved open questions, removed self-hosting scope

## Overview

Two related features designed as a single cohesive system:

1. **Shared Node Pool** — Subscribers share managed P2Pool nodes (1 mini, 1 main) rather than getting dedicated instances. This is how P2Pool is designed to work — many miners share one node.
2. **Crowdfund Model** — Transparent community funding for infrastructure costs using a pay-what-you-want system (Model C).

Self-hosting support is out of scope. The codebase is open-source — anyone motivated enough to self-host can make it work without dedicated tooling.

### Why Shared Nodes (v2 Revision)

The v1 architecture proposed dedicated P2Pool instances per subscriber. This was wrong for three reasons:

1. **P2Pool is designed for sharing.** Miners connecting via stratum submit shares under their own wallet address. P2Pool's PPLNS system tracks per-miner-address, not per-node. Multiple miners on one P2Pool node each get paid independently.
2. **Dedicated nodes waste resources.** Each P2Pool mini instance uses ~256MB RAM + 0.5 CPU — multiplied by 10 subscribers, that's 2.5GB + 5 CPU for something one instance handles for 50-100 miners.
3. **Operational complexity collapses.** No compose template generation, no port pool management, no per-subscriber container lifecycle, no cleanup automation. The operator just runs 1-2 P2Pool nodes and monitors them.

The real value proposition is **managed infrastructure** — the subscriber doesn't run monerod (250GB+ disk, days to sync) or P2Pool. They just point XMRig at a URL.

---

## 1. Infrastructure Cost Analysis

### Real Costs (DigitalOcean, April 2026)

| Component | Spec | Monthly Cost |
|-----------|------|-------------|
| VPS (General Purpose) | 8GB RAM / 4 vCPU | $63 |
| Block storage (monerod full node) | 250GB SSD volume | $25 |
| Domain + TLS (Let's Encrypt) | - | ~$1 |
| **Total infrastructure** | | **~$89/mo** |

Pruned node alternative: 100GB volume ($10/mo) → total **~$74/mo**. Full node is recommended for reliability.

### What This VPS Runs

| Service | RAM | CPU | Notes |
|---------|-----|-----|-------|
| monerod | ~3.5GB | 1-2 cores | Largest consumer by far |
| P2Pool mini | ~256MB | 0.5 core | Lightweight |
| P2Pool main | ~512MB | 0.5 core | Slightly heavier than mini |
| SideWatch stack | ~2GB | 1 core | manager, gateway, frontend, postgres, redis |
| Monitoring | ~512MB | 0.25 core | prometheus, grafana, loki |
| **Total** | **~7GB** | **~3.5 cores** | Fits 8GB/4vCPU droplet |

### Capacity Per Node

P2Pool is peer-to-peer mining software. A single P2Pool instance handles:
- **Stratum connections**: 50-100+ concurrent miners without issue (P2Pool's stratum server is lightweight — each connection is just a TCP socket receiving work and submitting nonces)
- **Share processing**: Handled by the sidechain, not local compute
- **Block template requests**: One call to monerod per new block (~2 min on Monero), regardless of miner count

**Bottom line: one P2Pool mini node + one P2Pool main node serves 100+ subscribers on a single $89/mo VPS.**

---

## 2. Pricing Models

Three models analyzed. Each assumes the same $89/mo infrastructure cost and targets 30-100 subscribers at scale.

### Model A: Cost-Share (Community Cooperative)

**Philosophy:** Split infrastructure costs evenly. Operator donates time. Maximum affordability.

| Subscribers | Cost per subscriber | Operator revenue | Notes |
|-------------|--------------------|-----------------|----|
| 10 | $8.90/mo | $0 | Early stage — expensive |
| 20 | $4.45/mo | $0 | Approaching viability |
| 50 | $1.78/mo | $0 | Very affordable |
| 100 | $0.89/mo | $0 | Essentially free |

**Implementation:** Fixed minimum contribution (e.g., $2/mo) that decreases as subscriber count grows. Variable pricing shown transparently.

| Pros | Cons |
|------|------|
| Cheapest for users | Zero operator compensation |
| Very transparent | Variable pricing confuses users |
| Strong Monero ethos alignment | Volunteer burnout risk |
| Encourages community ownership | Free-rider problem (why pay if others cover it?) |

**Verdict:** Good for building community trust early, unsustainable long-term unless the operator is intrinsically motivated. Best as a launch strategy that transitions to Model C.

---

### Model B: SaaS Premium (Maximum Profit)

**Philosophy:** Price for value delivered. Subscribers pay for the convenience of not running infrastructure.

| Tier | Price | What you get |
|------|-------|-------------|
| Free | $0 | Dashboard with 30-day retention, basic stats |
| Premium | $10/mo | Extended retention (15mo), tax export, worker breakdown, all features |

| Subscribers | Revenue | Infra cost | Net profit | Operator hourly (10h/mo) |
|-------------|---------|-----------|-----------|------------------------|
| 10 | $100 | $89 | $11 | $1.10/hr |
| 20 | $200 | $89 | $111 | $11.10/hr |
| 50 | $500 | $89 | $411 | $41.10/hr |
| 100 | $1,000 | $89 | $911 | $91.10/hr |

**Implementation:** Simple fixed pricing. One paid tier. Standard subscription model.

| Pros | Cons |
|------|------|
| Predictable revenue | High price relative to "just run P2Pool yourself" |
| Sustainable business model | $10/mo is a hard sell for hobby miners doing $5-20/mo in XMR |
| Simple to implement | Lower adoption rate |
| Covers operator time | Feels extractive vs. P2Pool's ethos |

**Verdict:** Maximizes per-subscriber profit but risks low adoption. P2Pool miners are privacy-focused, cost-conscious hobbyists — $10/mo for a dashboard is a tough sell when the mining itself might only earn $10-30/mo.

---

### Model C: Crowdfund + Tiers (Balanced) — RECOMMENDED

**Philosophy:** Transparent funding goal with tiered contributions. Low barrier to entry, community-driven, sustainable for the operator.

#### The Node Fund

A publicly visible funding tracker on the SideWatch dashboard:

```
┌─────────────────────────────────────────────────────────┐
│  Node Fund — April 2026                                 │
│  ████████████████████████░░░░░░░░  73% funded           │
│  $109 / $150 goal                                       │
│  37 supporters this month                               │
│                                                         │
│  Infrastructure: $89/mo  |  Operator: $61/mo            │
│  ↳ monerod, P2Pool mini+main, monitoring, backups       │
└─────────────────────────────────────────────────────────┘
```

The monthly goal is set by the operator (default: $150/mo — covers infra + modest compensation). The breakdown is transparent: subscribers see exactly where money goes.

#### Tiers

| Tier | Slug | Suggested Price | Minimum | What you get |
|------|------|----------------|---------|-------------|
| Free | `free` | $0 | $0 | Dashboard, 30-day data, shared node access |
| Supporter | `supporter` | $3/mo | $1/mo | 15-month retention, tax export, worker breakdown, all features |
| Champion | `champion` | $7/mo | $5/mo | Everything in Supporter + name on supporters page + priority in future features |

**"Pay what you want" above the minimum.** A supporter can pay $3, $5, or $20. A champion can pay $7 or $50. The minimum guarantees the tier; anything above is voluntary support.

#### Revenue Projections

| Scenario | Supporters | Champions | Avg contribution | Revenue | Net after infra |
|----------|-----------|-----------|-----------------|---------|----------------|
| Early (month 1-3) | 10 | 2 | $4.00 | $48 | -$41 (operator subsidizes) |
| Growing (month 4-8) | 25 | 5 | $4.50 | $135 | $46 |
| Healthy (month 9+) | 40 | 10 | $4.50 | $225 | $136 |
| Thriving | 60 | 15 | $5.00 | $375 | $286 |

**Break-even: ~22 subscribers at $4/mo average.** This is achievable within a few months for a well-run P2Pool mini node.

| Pros | Cons |
|------|------|
| Low barrier ($1 minimum) | Needs critical mass (~22 subs to break even) |
| Transparency builds trust | Revenue varies month to month |
| Aligned with P2Pool/Monero ethos | More complex than flat pricing |
| Social proof (funding bar) | "Pay what you want" can be gamed |
| Scales gracefully | Early months may lose money |
| Champion tier rewards whales | Requires funding UI/tracking system |

**Verdict:** Best fit for the P2Pool community. The transparent funding model creates buy-in and community ownership. The low minimum ($1) removes friction. The funding goal bar creates urgency without being aggressive. Early losses are manageable ($41/mo worst case) and decrease as adoption grows.

#### Why Model C Wins

The target audience is hobbyist Monero miners who chose P2Pool specifically because it's decentralized and trustless. These users:
- Are allergic to opaque subscription pricing (Model B)
- Appreciate transparency about where money goes
- Will contribute more than the minimum if they trust the operator
- Are motivated by community participation (funding bar, supporters list)
- Have XMR they're mining anyway — small monthly contributions are natural

The crowdfund model turns subscribers into stakeholders. They're not buying a service — they're funding community infrastructure.

---

## 3. Database Schema Changes

### Migration `005_node_fund.sql`

```sql
-- Rename 'paid' tier to 'supporter' for clarity.
-- Champion is a higher contribution tier with the same feature set.
UPDATE subscriptions SET tier = 'supporter' WHERE tier = 'paid';

-- Node fund: tracks the shared P2Pool node pool and funding.
CREATE TABLE IF NOT EXISTS node_pool (
    id              BIGSERIAL PRIMARY KEY,
    name            VARCHAR(128) NOT NULL,            -- 'mini' or 'main'
    sidechain       VARCHAR(16) NOT NULL,             -- 'mini' or 'main'
    status          VARCHAR(32) NOT NULL DEFAULT 'running',
    stratum_host    VARCHAR(256),                     -- public hostname
    stratum_port    INTEGER,                          -- public stratum port
    api_url         VARCHAR(256),                     -- internal P2Pool API URL
    p2p_port        INTEGER,
    subscriber_count INTEGER NOT NULL DEFAULT 0,      -- cached count, updated periodically
    last_health_at  TIMESTAMPTZ,
    health_status   VARCHAR(32) DEFAULT 'unknown',    -- healthy, unhealthy, syncing, unknown
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Seed the default shared nodes.
INSERT INTO node_pool (name, sidechain, status, stratum_port, api_url, p2p_port)
VALUES
    ('SideWatch Mini', 'mini', 'running', 3333, 'http://p2pool-mini:3333', 37889),
    ('SideWatch Main', 'main', 'stopped', 3334, 'http://p2pool-main:3334', 37888)
ON CONFLICT DO NOTHING;

-- Node health log: periodic health check results.
CREATE TABLE IF NOT EXISTS node_health_log (
    id              BIGSERIAL PRIMARY KEY,
    node_pool_id    BIGINT NOT NULL REFERENCES node_pool(id),
    status          VARCHAR(32) NOT NULL,
    hashrate        BIGINT,                           -- Node's total reported hashrate
    peers           INTEGER,                          -- P2Pool peer count
    miners          INTEGER,                          -- Connected miners count
    uptime_secs     BIGINT,
    details         JSONB,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_node_health_log_pool_time
    ON node_health_log (node_pool_id, created_at DESC);

-- Prune old health logs (keep 30 days).
-- Handled by the existing timeseries pruning job.

-- Node fund: monthly funding tracking.
CREATE TABLE IF NOT EXISTS node_fund_months (
    id              BIGSERIAL PRIMARY KEY,
    month           DATE NOT NULL UNIQUE,             -- first day of month (2026-04-01)
    goal_usd        NUMERIC(10,2) NOT NULL,           -- monthly funding goal
    infra_cost_usd  NUMERIC(10,2) NOT NULL,           -- infrastructure cost portion
    total_funded_usd NUMERIC(10,2) NOT NULL DEFAULT 0,-- total contributions this month
    supporter_count INTEGER NOT NULL DEFAULT 0,        -- unique contributors
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Fund contributions: links subscription payments to monthly fund totals.
-- This is a VIEW over subscription_payments, not a separate table.
-- Each subscription payment in a given month counts toward that month's fund.

-- Subscriber node assignment: which shared node a subscriber uses.
ALTER TABLE subscriptions ADD COLUMN IF NOT EXISTS node_pool_id BIGINT REFERENCES node_pool(id);
ALTER TABLE subscriptions ADD COLUMN IF NOT EXISTS contribution_amount BIGINT DEFAULT 0;
    -- Last payment amount in atomic XMR, for display purposes

-- Index for fund queries.
CREATE INDEX IF NOT EXISTS idx_sub_payments_month
    ON subscription_payments (created_at)
    WHERE confirmed = TRUE;
```

### Key Schema Decisions

**No per-subscriber node tables.** The entire per-subscriber provisioning infrastructure from v1 (node_instances, node_port_pool, node_resource_limits) is eliminated. Subscribers are assigned to a shared `node_pool` entry.

**Fund tracking is derived, not duplicated.** `node_fund_months.total_funded_usd` is updated by the subscription scanner when payments are confirmed, using the existing XMR/USD price at time of payment. No separate "fund_contributions" table — the existing `subscription_payments` table already has all the data.

**Node pool is operator-managed.** Rows in `node_pool` are created by the operator (or seeded by migration), not by subscribers. The operator decides when to add a second mini node, a main node, etc.

---

## 4. Tier Model

### Current State

| Tier | Price | Data Retention | Features |
|------|-------|---------------|----------|
| Free | $0 | 30 days | Basic dashboard, capped hashrate/payments |
| Paid | ~$5/mo XMR | 15 months | Tax export, worker breakdown, uncapped data |

### Proposed Tiers

| Tier | Slug | Minimum | Suggested | Features |
|------|------|---------|-----------|----------|
| Free | `free` | $0 | $0 | 30-day retention, shared node, capped API |
| Supporter | `supporter` | ~$1/mo | ~$3/mo | 15-month retention, all features, name on fund page |
| Champion | `champion` | ~$5/mo | ~$7/mo | Everything in Supporter + highlighted on supporters page + early access to new features |

**All tiers use the same shared P2Pool node.** There is no node-access tier because the shared node is available to everyone (including free users who point XMRig at it directly). The tiers differentiate on **dashboard features and data retention**, not infrastructure access.

This is the correct model because:
- P2Pool stratum is publicly accessible by design (no auth on stratum)
- You can't meaningfully gate "node access" when the stratum port is open
- The value SideWatch adds is the **observability layer**, not the node itself
- The crowdfund model funds the node's existence; tiers reward contributors

### Tier Hierarchy

```go
const (
    TierFree      Tier = "free"
    TierSupporter Tier = "supporter"
    TierChampion  Tier = "champion"
)

func TierIncludes(actual Tier, required Tier) bool {
    hierarchy := map[Tier]int{
        TierFree:      0,
        TierSupporter: 1,
        TierChampion:  2,
    }
    return hierarchy[actual] >= hierarchy[required]
}
```

Champion has the same feature set as Supporter (both get all features). The distinction is:
- Minimum payment threshold ($5 vs $1)
- Highlighted on supporters page
- Priority support channel (e.g., dedicated Matrix/IRC room or direct contact)

### Migration from Current "Paid" Tier

Existing `tier = 'paid'` rows become `tier = 'supporter'`. The minimum payment threshold maps naturally: current $5/mo paid tier → Champion-level contribution. Existing subscribers are upgraded to Champion as a thank-you for early adoption.

---

## 5. Node Fund System

### How It Works

1. **Operator sets a monthly goal** (default: $150 — covers $89 infra + $61 operator time)
2. **Every subscription payment** contributes to the current month's fund
3. **The dashboard shows** a funding progress bar, contributor count, and cost breakdown
4. **Month rolls over** on the 1st — new fund month, fresh goal, counter resets

### Fund Calculation

```go
// FundStatus returns the current month's funding state.
func (s *Service) GetFundStatus(ctx context.Context) (*FundStatus, error) {
    month := time.Now().UTC().Truncate(24*time.Hour).Format("2006-01-01") // first of month
    
    // Sum confirmed payments this month, converted to USD at payment time.
    var funded float64
    var supporters int
    err := s.pool.QueryRow(ctx,
        `SELECT COALESCE(SUM(
            (amount::NUMERIC / 1e12) * COALESCE(xmr_usd_price, 0)
         ), 0),
         COUNT(DISTINCT miner_address)
         FROM subscription_payments
         WHERE confirmed = TRUE
           AND created_at >= date_trunc('month', NOW())
           AND created_at < date_trunc('month', NOW()) + INTERVAL '1 month'`,
    ).Scan(&funded, &supporters)
    // ...
}
```

### Fund API Endpoint

```
GET /api/fund/status
```

```json
{
  "month": "2026-04",
  "goal_usd": 150.00,
  "funded_usd": 109.50,
  "percent_funded": 73,
  "infra_cost_usd": 89.00,
  "supporter_count": 37,
  "node_count": 2,
  "nodes": [
    { "name": "SideWatch Mini", "sidechain": "mini", "status": "healthy", "miners": 24 },
    { "name": "SideWatch Main", "sidechain": "main", "status": "healthy", "miners": 13 }
  ]
}
```

### Supporters Page

A public page showing current fund status and (opt-in) contributor list:

```
This month's supporters (37):
  ★ 4A8x...3fKq  — Champion
  ★ 48Rn...7gVp  — Champion
    4B2x...9aKm  — Supporter
    4C7y...2bLn  — Supporter
    ...and 33 more
```

Contributors are identified by truncated miner address (first 4 + last 4 chars). No real names, no emails — pure Monero address attribution. Opt-out by default; subscribers choose to appear.

### Transparency Guarantee

The fund page always shows:
- Exact infrastructure cost ($89/mo)
- Operator's stated goal ($150/mo)
- How surplus is used ("Operator compensation for monitoring, maintenance, and development")
- Historical funding by month (chart)

This transparency is a feature, not a liability. P2Pool users chose decentralization because they value honesty.

---

## 6. Shared Node Architecture

### What Changes From v1

| v1 (Dedicated) | v2 (Shared) |
|----------------|-------------|
| 1 P2Pool instance per subscriber | 1-2 P2Pool instances total |
| Compose template generator | Operator manages nodes manually |
| Per-subscriber port allocation | Fixed stratum ports (3333 mini, 3334 main) |
| NodeManager with Provision/Deprovision | NodePool with HealthCheck only |
| 10 subscribers per VPS | 100+ subscribers per VPS |
| $15-25/mo per subscriber | $1-7/mo per subscriber |
| 7-phase build plan | 5-phase build plan |
| ~15 new files | ~8 new files |

### Architecture

```
                                ┌─────────────────────────┐
                                │   NodePool (Go)         │
                                │   internal/nodepool/    │
                                │                         │
                                │ - HealthCheck()         │
                                │ - GetConnectionInfo()   │
                                │ - GetFundStatus()       │
                                │ - AssignNode()          │
                                └────────┬────────────────┘
                                         │
                          ┌──────────────┼──────────────┐
                          │              │              │
                    ┌─────▼─────┐  ┌─────▼─────┐  ┌────▼────┐
                    │ P2Pool    │  │ P2Pool    │  │ Shared  │
                    │ mini      │  │ main      │  │ monerod │
                    │ :3333     │  │ :3334     │  │ :18081  │
                    └─────┬─────┘  └─────┬─────┘  └────┬────┘
                          │              │              │
                          └──────────────┼──────────────┘
                                         │
                              All on same Docker network
                              All on same VPS ($89/mo)
```

### Subscriber Connection Flow

```
Subscriber's XMRig
        │
        ▼
sidewatch.example.com:3333    (or :3334 for main)
        │
        ▼
nginx (TLS termination for HTTPS, TCP passthrough for stratum)
        │
        ▼
P2Pool mini container (:3333)
        │
        ▼
Shared monerod (:18081)
```

The subscriber configures XMRig with:
```json
{
    "url": "sidewatch.example.com:3333",
    "user": "YOUR_WALLET_ADDRESS",
    "pass": "x"
}
```

That's it. No provisioning, no waiting, no account creation. Point and mine.

### Adding Capacity

When the operator decides to add nodes (e.g., geographic regions, or load balancing):

1. Spin up a new VPS with monerod + P2Pool
2. Add a row to `node_pool`
3. Update nginx to route to the new node
4. The dashboard automatically shows the new node in the health display

This is a manual, operator-driven process — appropriate for a solo operator scaling from 1 to 2-3 VPS instances.

---

## 7. API Changes

### New Endpoints

```
GET  /api/fund/status              → Current month funding status + node health
GET  /api/fund/history             → Monthly funding history (chart data)
GET  /api/fund/supporters          → Opt-in supporter list (truncated addresses)
GET  /api/nodes/status             → Health status of all shared nodes
GET  /api/nodes/connection-info    → Stratum URLs + XMRig config for each node
PUT  /api/admin/fund/goal          → Update monthly funding goal (admin only)
POST /api/admin/nodes              → Add a new node pool entry (admin only)
PUT  /api/admin/nodes/{id}         → Update node config (admin only)
```

### Connection Info Response

```json
{
  "nodes": [
    {
      "name": "SideWatch Mini",
      "sidechain": "mini",
      "status": "healthy",
      "stratum_url": "sidewatch.example.com:3333",
      "xmrig_config": {
        "url": "sidewatch.example.com:3333",
        "user": "YOUR_WALLET_ADDRESS",
        "pass": "x"
      }
    },
    {
      "name": "SideWatch Main",
      "sidechain": "main",
      "status": "healthy",
      "stratum_url": "sidewatch.example.com:3334",
      "xmrig_config": {
        "url": "sidewatch.example.com:3334",
        "user": "YOUR_WALLET_ADDRESS",
        "pass": "x"
      }
    }
  ],
  "onion_url": "sidewatch...onion:3333"
}
```

### Health Endpoint Extension

```json
{
  "status": "ok",
  "mode": "managed",
  "postgres": "ok",
  "redis": "ok",
  "nodes": {
    "mini": "healthy",
    "main": "healthy"
  },
  "fund": {
    "month": "2026-04",
    "percent_funded": 73
  }
}
```

---

## 8. Frontend Changes

### New Components

| Component | Purpose |
|-----------|---------|
| `FundProgress.tsx` | Funding progress bar + cost breakdown (shown on home page) |
| `FundHistory.tsx` | Monthly funding history chart |
| `SupportersPage.tsx` | Opt-in list of contributors with tier badges |
| `XMRigConfig.tsx` | Copy-paste XMRig configuration for each shared node |
| `TierSelector.tsx` | Tier comparison with "pay what you want" slider |
| `NodeHealth.tsx` | Shared node health status (uptime, peers, hashrate) |

### Modified Components

| Component | Change |
|-----------|--------|
| `SubscriptionStatus.tsx` | Show Supporter/Champion tier, contribution amount, fund impact |
| `SubscriptionPayment.tsx` | "Pay what you want" with minimum, suggested amount, and tier thresholds |
| `Navigation.tsx` | Add "Fund" and "Connect" links |
| `LiveStats.tsx` | Show node health indicator (green dot) |
| `page.tsx` (home) | Add FundProgress widget to home page |

### New Pages

| Page | Purpose |
|------|---------|
| `app/fund/page.tsx` | Fund status, history, supporters list |
| `app/connect/page.tsx` | XMRig connection guide for each shared node |


---

## 9. File Tree (New + Modified)

```
services/manager/
  + internal/nodepool/
  +   pool.go                        ← NodePool: health checks, connection info, assignment
  +   pool_test.go                   ← Unit tests
  +   types.go                       ← NodePoolEntry, NodeHealth, FundStatus types
  + internal/fund/
  +   service.go                     ← Fund tracking: status, history, supporters
  +   service_test.go                ← Unit tests for fund calculations
  +   types.go                       ← FundMonth, FundStatus, Supporter types
  + pkg/db/migrations/
  +   005_node_fund.sql              ← Schema for node pool, health, fund tracking
  ~ internal/subscription/
  ~   types.go                       ← Add TierSupporter, TierChampion constants
  ~   service.go                     ← Update CheckTier, add contribution tracking
  ~   middleware.go                  ← Tier hierarchy update for Supporter/Champion
  ~   middleware_test.go             ← Tests for new tiers
  ~   scanner.go                     ← Update fund totals on confirmed payment
  ~ cmd/manager/
  ~   config.go                      ← Add FUND_GOAL_USD, node pool config
  ~   main.go                        ← Conditionally start fund/nodepool, mode gating
  ~   routes.go                      ← Add fund, node, connection-info endpoints
  ~ internal/metrics/
  ~   metrics.go                     ← Add node health + fund metrics

~ config/nginx/nginx.conf             ← Add TCP stream for stratum ports (3333, 3334)
~ config/prometheus/alerts/nodes.yml  ← Shared node alert rules

frontend/
  + app/fund/page.tsx                 ← Fund status + supporters page
  + app/connect/page.tsx              ← XMRig connection guide
  + components/FundProgress.tsx       ← Funding progress bar
  + components/FundHistory.tsx        ← Monthly chart
  + components/SupportersPage.tsx     ← Contributor list
  + components/XMRigConfig.tsx        ← Connection config helper
  + components/TierSelector.tsx       ← Tier comparison + pay-what-you-want
  + components/NodeHealth.tsx         ← Shared node health display
  ~ components/SubscriptionStatus.tsx ← Supporter/Champion tier display
  ~ components/SubscriptionPayment.tsx← Pay-what-you-want UI
  ~ components/Navigation.tsx         ← Add Fund + Connect nav links
  ~ components/LiveStats.tsx          ← Node health indicator
  ~ app/page.tsx                      ← FundProgress widget on home
  ~ app/subscribe/page.tsx            ← Integrate TierSelector
  ~ lib/api.ts                        ← Add fund + node API types
```

---

## 10. Phased Build Plan

### Phase 1: Tier Model + Fund Schema
**Goal:** Expand tiers to Supporter/Champion, add node pool + fund tables, update middleware.

**Files:**
- `+ services/manager/pkg/db/migrations/005_node_fund.sql`
- `~ services/manager/internal/subscription/types.go` — new tier constants, TierIncludes()
- `~ services/manager/internal/subscription/service.go` — update CheckTier for hierarchy
- `~ services/manager/internal/subscription/middleware.go` — use TierIncludes()
- `~ services/manager/internal/subscription/middleware_test.go` — test all tier combos
- `~ services/manager/internal/subscription/scanner.go` — update fund totals on confirmed payment
- `~ services/manager/cmd/manager/routes.go` — update RequirePaid to RequireTier(TierSupporter)
- `~ frontend/lib/api.ts` — update tier types
- `+ frontend/components/TierSelector.tsx` — tier comparison + pay-what-you-want
- `~ frontend/app/subscribe/page.tsx` — integrate TierSelector

**Dependencies:** None (builds on existing subscription system)
**End Conditions:**
- [ ] Migration 005 applies cleanly (`paid` → `supporter`)
- [ ] `TierIncludes(TierChampion, TierSupporter)` returns true
- [ ] Existing tier-gated features work with `supporter`
- [ ] Subscription scanner updates `node_fund_months` on confirmed payment
- [ ] Champion tier requires $5+ minimum, Supporter requires $1+
- [ ] Frontend shows tier comparison with pay-what-you-want slider
- [ ] All existing tests pass

**Complexity:** **M**

---

### Phase 2: Node Pool + Fund Dashboard
**Goal:** Build node health monitoring for shared nodes and the public funding dashboard.

**Files:**
- `+ services/manager/internal/nodepool/types.go`
- `+ services/manager/internal/nodepool/pool.go` — health checks, connection info
- `+ services/manager/internal/nodepool/pool_test.go`
- `+ services/manager/internal/fund/types.go`
- `+ services/manager/internal/fund/service.go` — fund status, history, supporters
- `+ services/manager/internal/fund/service_test.go`
- `~ services/manager/cmd/manager/config.go` — add FUND_GOAL_USD
- `~ services/manager/cmd/manager/main.go` — start node health checker
- `~ services/manager/cmd/manager/routes.go` — add fund + node endpoints
- `~ services/manager/internal/metrics/metrics.go` — node health + fund metrics

**Dependencies:** Phase 1 (schema must exist)
**End Conditions:**
- [ ] `GET /api/fund/status` returns funding progress
- [ ] `GET /api/nodes/status` returns health of shared nodes
- [ ] `GET /api/nodes/connection-info` returns XMRig config
- [ ] Health checker polls each node's P2Pool API every 60s
- [ ] `node_health_log` records health check results
- [ ] Fund status correctly sums confirmed payments for current month
- [ ] All tests pass

**Complexity:** **M**

---

### Phase 3: Frontend — Fund + Connect Pages
**Goal:** Build the funding dashboard, supporters page, and XMRig connection guide.

**Files:**
- `+ frontend/app/fund/page.tsx` — fund status page
- `+ frontend/app/connect/page.tsx` — connection guide
- `+ frontend/components/FundProgress.tsx` — progress bar widget
- `+ frontend/components/FundHistory.tsx` — monthly chart
- `+ frontend/components/SupportersPage.tsx` — contributor list
- `+ frontend/components/XMRigConfig.tsx` — connection helper
- `+ frontend/components/NodeHealth.tsx` — node status display
- `~ frontend/components/SubscriptionStatus.tsx` — show tier + contribution
- `~ frontend/components/SubscriptionPayment.tsx` — pay-what-you-want UI
- `~ frontend/components/Navigation.tsx` — add Fund + Connect links
- `~ frontend/components/LiveStats.tsx` — node health indicator
- `~ frontend/app/page.tsx` — add FundProgress to home page
- `~ frontend/lib/api.ts` — add fund + node API types

**Dependencies:** Phase 2 (API endpoints must exist)
**End Conditions:**
- [ ] Fund page shows progress bar, cost breakdown, supporter count
- [ ] Fund history shows monthly chart
- [ ] Supporters page shows opt-in contributor list (opt-out by default)
- [ ] Connect page shows XMRig config with copy-to-clipboard
- [ ] Home page shows funding widget
- [ ] Node health shown in LiveStats area
- [ ] Pay-what-you-want subscription flow works
- [ ] Frontend tests pass, TypeScript compiles

**Complexity:** **M**

---

### Phase 4: Networking + Monitoring
**Goal:** Configure stratum port exposure, alerts for shared nodes, and Grafana dashboards.

**Files:**
- `~ config/nginx/nginx.conf` — add TCP stream block for ports 3333, 3334
- `~ infra/scripts/provision.sh` — add stratum ports to UFW
- `+ config/prometheus/alerts/nodes.yml` — shared node alert rules
- `+ config/grafana/provisioning/dashboards/node-fund.json` — fund + node dashboard
- `~ config/prometheus/prometheus.yml` — scrape node metrics

**Dependencies:** Phase 3 (fund + node UI must be in place for full E2E testing)
**End Conditions:**
- [ ] XMRig can connect to `sidewatch.example.com:3333` and submit shares
- [ ] UFW allows stratum ports (3333, 3334)
- [ ] AlertManager fires on node unhealthy for 5m
- [ ] Grafana shows fund progress + node health
- [ ] Manual E2E: connect XMRig → mine → see shares on dashboard → contribute to fund

**Complexity:** **S**

---

## Phase Dependency Graph

```
Phase 1 (Tiers + Fund Schema)
    │
    ▼
Phase 2 (Node Pool + Fund API)
    │
    ▼
Phase 3 (Frontend)
    │
    ▼
Phase 4 (Networking + Monitoring)
```

All sequential. Total complexity: M + M + M + S = **4 phases, no L-complexity work.** Dramatically simpler than v1 (which was 7 phases with an L-complexity provisioning core).

---

## 11. Risk Register

| # | Risk | Impact | Likelihood | Mitigation |
|---|------|--------|-----------|------------|
| 1 | **Underfunding** — not enough subscribers to cover infra costs | Medium | High (early) | Operator accepts early losses as investment. Break-even at ~22 subs × $4. Fund transparency motivates contributions. Model A (cost-share) as fallback. |
| 2 | **Free-rider problem** — miners use the node but don't contribute | Low | High | This is fine. Free users are potential future supporters. The dashboard features (retention, tax export) are the real gate. The node runs regardless. |
| 3 | **monerod goes down** — all miners lose blockchain access | High | Medium | Monitoring alerts within 5m. Docker restart policies auto-recover. P2Pool reconnects automatically. This is the same risk every P2Pool node operator faces. |
| 4 | **Stratum abuse** — someone points a botnet at the stratum port | Medium | Low | P2Pool's stratum has no auth — this is true of all P2Pool nodes. Rate limiting at nginx layer. Resource limits via Docker. Worst case: restart the container. |
| 5 | **Price volatility** — XMR/USD moves between payment and fund accounting | Low | Medium | Fund amounts are recorded in USD at time of payment confirmation (using existing PriceOracle). The fund goal is in USD. Minor variance is acceptable. |
| 6 | **Solo operator burden** | Medium | Medium | Shared model is dramatically less work than v1. No per-subscriber provisioning. Health checks are automated. Grafana gives at-a-glance status. Expected: 2-5 hours/mo of active management. |
| 7 | **Competitor offering free dashboard** | Medium | Low | SideWatch differentiates on: integrated fund model, Tor support, privacy focus, open-source codebase. Community trust is the moat, not features. |
| 8 | **Too many free users overwhelming the node** | None | Very Low | **Not a real risk.** P2Pool uses libuv and handles thousands of concurrent stratum connections. Each miner connection is ~50MB/day (~1.5GB/mo) of bandwidth. The VPS bottleneck is monerod (3.5GB RAM, 250GB disk), not P2Pool — monerod workload is identical whether 5 or 500 miners connect (one block template at a time). The 8GB/4vCPU General Purpose droplet includes 4TB/mo transfer. At ~50MB/day per stratum miner + ~100-500GB/mo for monerod P2P + ~50GB/mo for dashboard HTTP traffic, you'd need **800+ concurrent stratum miners** to approach the 4TB transfer cap. By that point, the fund is well-capitalized from even a 10% conversion rate. Free users should never be capped — they are the onboarding funnel for new P2Pool miners and the conversion pipeline to supporters. |

---

## 12. Bandwidth Analysis

### Per-Miner Stratum Bandwidth

Stratum V1 (used by P2Pool) is a lightweight JSON-RPC protocol. Typical per-miner usage:

| Metric | Value |
|--------|-------|
| Per job notification (node → miner) | ~250-400 bytes |
| Per share submission (miner → node) | ~200-300 bytes |
| Jobs per block (~2 min on Monero) | ~1 per block |
| Shares per miner per day (varies by hashrate) | ~100-1000 |
| **Total per miner per day** | **~30-70 MB** |
| **Total per miner per month** | **~1-2 GB** |

Note: actual usage depends on miner hashrate (more shares = more bandwidth) and P2Pool's difficulty adjustment.

### Total Bandwidth Budget (DigitalOcean)

The 8GB/4vCPU General Purpose droplet includes **4TB/mo outbound transfer**. Overage: $0.01/GB. Inbound is free.

| Source | Monthly estimate | Notes |
|--------|-----------------|-------|
| monerod P2P (block relay, tx relay) | 100-500 GB | Configurable via `--out-peers`, `--in-peers`. Default ~200GB. Can limit to 100GB with `--out-peers 8 --in-peers 16` |
| P2Pool sidechain P2P | 5-20 GB | Lightweight — sidechain blocks are small |
| Stratum (50 miners) | 50-100 GB | ~1-2 GB/miner/mo |
| Stratum (200 miners) | 200-400 GB | Still well within budget |
| Stratum (500 miners) | 500-1000 GB | Approaching need for attention |
| Dashboard HTTP/WS traffic | 20-50 GB | API calls, WebSocket, frontend assets |
| Monitoring (Prometheus, Loki) | 5-10 GB | Internal mostly, minimal external |
| **Total (200 miners scenario)** | **~400-800 GB** | **20-40% of 4TB cap** |

### When Bandwidth Becomes a Cost Factor

| Miners | Est. total bandwidth | % of 4TB cap | Overage cost |
|--------|---------------------|-------------|-------------|
| 50 | ~300 GB | 7% | $0 |
| 200 | ~600 GB | 15% | $0 |
| 500 | ~1.2 TB | 30% | $0 |
| 1000 | ~2.2 TB | 55% | $0 |
| 2000 | ~4.2 TB | 105% | ~$2/mo |

**Bandwidth is not a meaningful cost risk.** Even at 2,000 miners, the overage is ~$2/mo. The monerod P2P traffic is the largest component and is independent of miner count. You'd hit CPU/RAM limits long before bandwidth limits.

### Cost-Reduction Levers (if needed at scale)

1. `monerod --out-peers 8 --in-peers 16` — reduces P2P bandwidth from ~500GB to ~100GB
2. CDN for frontend assets (Cloudflare free tier) — eliminates ~20-50GB
3. Reduce Prometheus retention — less Loki/Prometheus storage traffic

---

## 13. Resolved Decisions

These were open questions in v2, now resolved:

1. **Supporters list privacy** — Opt-out by default. Truncated miner addresses are shown on the supporters page unless the subscriber requests removal. Miner addresses are already public on the sidechain.

2. **Fund surplus handling** — Operator keeps surplus. The funding goal already transparently discloses operator compensation. Surplus is earned by growing the community.

3. **Champion perks** — Priority support channel (dedicated Matrix/IRC room or direct operator contact). No other perks for now — add more when the community asks.

4. **Multi-VPS scaling** — `node_pool` table supports multiple entries. Frontend shows all nodes. Subscribers auto-assigned to least-loaded node, can switch manually.

5. **monerod ZMQ sharing** — Confirmed: ZMQ PUB/SUB allows multiple subscribers. Both P2Pool mini and main share one monerod ZMQ endpoint. No proxy needed.

6. **Pricing model** — Model C (crowdfund) at launch. System supports switching to flat SaaS pricing later by adjusting config, but crowdfund is the default.

7. **Self-hosting** — Out of scope. Codebase is open-source; self-hosters can figure it out. No dedicated tooling, compose files, or mode flags.
