-- SideWatch v1: uncle tracking, software version, coinbase private key,
-- extended retention for paid subscribers.

-- Share-level uncle and software tracking.
ALTER TABLE p2pool_shares ADD COLUMN IF NOT EXISTS is_uncle BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE p2pool_shares ADD COLUMN IF NOT EXISTS software_id SMALLINT;
ALTER TABLE p2pool_shares ADD COLUMN IF NOT EXISTS software_version TEXT;

-- Coinbase private key on found blocks (public via P2Pool API, enables trustless audit).
ALTER TABLE p2pool_blocks ADD COLUMN IF NOT EXISTS coinbase_private_key TEXT;

-- Extended retention flag for paid subscribers.
ALTER TABLE subscriptions ADD COLUMN IF NOT EXISTS extended_retention BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE subscriptions ADD COLUMN IF NOT EXISTS retention_since TIMESTAMPTZ;

-- Index for uncle rate queries: quickly count uncle vs normal shares per miner.
CREATE INDEX IF NOT EXISTS idx_shares_uncle_miner
    ON p2pool_shares (miner_address, is_uncle, created_at);
