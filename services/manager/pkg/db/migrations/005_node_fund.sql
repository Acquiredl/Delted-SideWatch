-- Phase 1: Shared node pool, crowdfund model, tier expansion.
-- Renames 'paid' tier to 'supporter', adds 'champion' tier support.

-- Rename 'paid' tier to 'supporter' for clarity.
-- Champion is a higher contribution tier with the same feature set.
UPDATE subscriptions SET tier = 'supporter' WHERE tier = 'paid';

-- Node fund: tracks the shared P2Pool node pool and funding.
CREATE TABLE IF NOT EXISTS node_pool (
    id              BIGSERIAL PRIMARY KEY,
    name            VARCHAR(128) NOT NULL,
    sidechain       VARCHAR(16) NOT NULL,
    status          VARCHAR(32) NOT NULL DEFAULT 'running',
    stratum_host    VARCHAR(256),
    stratum_port    INTEGER,
    api_url         VARCHAR(256),
    p2p_port        INTEGER,
    subscriber_count INTEGER NOT NULL DEFAULT 0,
    last_health_at  TIMESTAMPTZ,
    health_status   VARCHAR(32) DEFAULT 'unknown',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (name, sidechain)
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
    hashrate        BIGINT,
    peers           INTEGER,
    miners          INTEGER,
    uptime_secs     BIGINT,
    details         JSONB,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_node_health_log_pool_time
    ON node_health_log (node_pool_id, created_at DESC);

-- Node fund: monthly funding tracking.
CREATE TABLE IF NOT EXISTS node_fund_months (
    id              BIGSERIAL PRIMARY KEY,
    month           DATE NOT NULL UNIQUE,
    goal_usd        NUMERIC(10,2) NOT NULL,
    infra_cost_usd  NUMERIC(10,2) NOT NULL,
    total_funded_usd NUMERIC(10,2) NOT NULL DEFAULT 0,
    supporter_count INTEGER NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Subscriber node assignment: which shared node a subscriber uses.
ALTER TABLE subscriptions ADD COLUMN IF NOT EXISTS node_pool_id BIGINT REFERENCES node_pool(id);
ALTER TABLE subscriptions ADD COLUMN IF NOT EXISTS contribution_amount BIGINT DEFAULT 0;

-- Index for fund queries: sum confirmed payments by month.
CREATE INDEX IF NOT EXISTS idx_sub_payments_month
    ON subscription_payments (created_at)
    WHERE confirmed = TRUE;
