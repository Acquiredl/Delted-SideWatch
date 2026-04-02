CREATE TABLE IF NOT EXISTS pool_stats_snapshots (
    id                   BIGSERIAL PRIMARY KEY,
    sidechain            VARCHAR(10) NOT NULL,
    pool_hashrate        BIGINT NOT NULL,
    pool_miners          INT NOT NULL,
    sidechain_height     BIGINT NOT NULL,
    sidechain_difficulty BIGINT NOT NULL,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_pool_stats_snapshots_created ON pool_stats_snapshots (created_at);
CREATE INDEX IF NOT EXISTS idx_pool_stats_snapshots_sidechain ON pool_stats_snapshots (sidechain, created_at);
