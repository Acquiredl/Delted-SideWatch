-- Subscription state: one row per miner, tracks tier and expiry.
CREATE TABLE IF NOT EXISTS subscriptions (
    id              BIGSERIAL PRIMARY KEY,
    miner_address   VARCHAR(256) NOT NULL UNIQUE,
    tier            VARCHAR(32) NOT NULL DEFAULT 'free',
    api_key_hash    VARCHAR(64),
    email           VARCHAR(256),
    expires_at      TIMESTAMPTZ,
    grace_until     TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_subscriptions_api_key_hash
    ON subscriptions (api_key_hash) WHERE api_key_hash IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_subscriptions_expires_at
    ON subscriptions (expires_at) WHERE expires_at IS NOT NULL;

-- Maps each miner to a unique wallet subaddress for payment identification.
CREATE TABLE IF NOT EXISTS subscription_addresses (
    id              BIGSERIAL PRIMARY KEY,
    miner_address   VARCHAR(256) NOT NULL UNIQUE,
    subaddress      VARCHAR(256) NOT NULL UNIQUE,
    subaddress_index INTEGER NOT NULL UNIQUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- On-chain subscription payments detected by the scanner.
CREATE TABLE IF NOT EXISTS subscription_payments (
    id              BIGSERIAL PRIMARY KEY,
    miner_address   VARCHAR(256) NOT NULL,
    tx_hash         VARCHAR(64) NOT NULL UNIQUE,
    amount          BIGINT NOT NULL,
    xmr_usd_price  NUMERIC(12,4),
    confirmed       BOOLEAN NOT NULL DEFAULT FALSE,
    main_height     BIGINT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_sub_payments_miner_created
    ON subscription_payments (miner_address, created_at);
CREATE INDEX IF NOT EXISTS idx_sub_payments_unconfirmed
    ON subscription_payments (confirmed) WHERE NOT confirmed;
