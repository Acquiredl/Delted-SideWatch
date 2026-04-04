-- Data retention export: held-back year data for lapsed subscribers
-- and support for the "delete all my data" nuke endpoint.

-- When a subscription lapses across a year boundary, the previous year's
-- payment data is preserved for tax export. The miner gets 2 downloads
-- (or can manually delete via nuke), then the data is pruned.
ALTER TABLE subscriptions ADD COLUMN IF NOT EXISTS tax_exports_remaining SMALLINT;
ALTER TABLE subscriptions ADD COLUMN IF NOT EXISTS held_year SMALLINT;

-- Track when a miner requests full data deletion.
-- Soft-delete: the nuke handler sets this, the next prune cycle does the actual DELETE.
ALTER TABLE subscriptions ADD COLUMN IF NOT EXISTS data_deleted_at TIMESTAMPTZ;

-- Index for prune queries: quickly find miners with held-back data.
CREATE INDEX IF NOT EXISTS idx_subscriptions_held_year
    ON subscriptions (held_year) WHERE held_year IS NOT NULL;
