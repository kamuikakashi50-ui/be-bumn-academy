DROP INDEX IF EXISTS orders_payment_provider_idx;

ALTER TABLE orders
    DROP COLUMN IF EXISTS payment_provider,
    DROP COLUMN IF EXISTS payment_status,
    DROP COLUMN IF EXISTS snap_token,
    DROP COLUMN IF EXISTS snap_redirect_url;
