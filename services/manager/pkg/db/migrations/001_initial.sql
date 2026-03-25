CREATE TABLE IF NOT EXISTS p2pool_shares (
    id              BIGSERIAL PRIMARY KEY,
    sidechain       VARCHAR(10) NOT NULL,
    miner_address   VARCHAR(256) NOT NULL,
    worker_name     VARCHAR(128),
    sidechain_height BIGINT NOT NULL,
    difficulty      BIGINT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_shares_miner_sidechain ON p2pool_shares (miner_address, sidechain);
CREATE INDEX IF NOT EXISTS idx_shares_sidechain_height ON p2pool_shares (sidechain_height);
CREATE INDEX IF NOT EXISTS idx_shares_created_at ON p2pool_shares (created_at);

CREATE TABLE IF NOT EXISTS p2pool_blocks (
    id              BIGSERIAL PRIMARY KEY,
    main_height     BIGINT NOT NULL UNIQUE,
    main_hash       VARCHAR(64) NOT NULL,
    sidechain_height BIGINT NOT NULL,
    coinbase_reward BIGINT NOT NULL,
    effort          NUMERIC(10,4),
    found_at        TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_blocks_found_at ON p2pool_blocks (found_at);

CREATE TABLE IF NOT EXISTS miner_hashrate (
    id              BIGSERIAL PRIMARY KEY,
    miner_address   VARCHAR(256) NOT NULL,
    sidechain       VARCHAR(10) NOT NULL,
    hashrate        BIGINT NOT NULL,
    bucket_time     TIMESTAMPTZ NOT NULL,
    UNIQUE (miner_address, sidechain, bucket_time)
);
