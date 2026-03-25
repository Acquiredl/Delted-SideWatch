CREATE TABLE IF NOT EXISTS payments (
    id              BIGSERIAL PRIMARY KEY,
    miner_address   VARCHAR(256) NOT NULL,
    amount          BIGINT NOT NULL,
    main_height     BIGINT NOT NULL,
    main_hash       VARCHAR(64) NOT NULL,
    xmr_usd_price   NUMERIC(12,4),
    xmr_cad_price   NUMERIC(12,4),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_payments_miner_created ON payments (miner_address, created_at);
CREATE INDEX IF NOT EXISTS idx_payments_main_height ON payments (main_height);
