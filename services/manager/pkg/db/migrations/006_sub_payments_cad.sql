-- Add CAD price column to subscription_payments for Canadian tax reporting.
-- The scanner already fetches both USD and CAD from CoinGecko; this column
-- stores the CAD spot price at the time each subscription payment is confirmed.
ALTER TABLE subscription_payments
    ADD COLUMN IF NOT EXISTS xmr_cad_price NUMERIC(12,4);
