-- Kolom pembayaran Midtrans pada tabel orders.
--
--   payment_provider   : jalur pembayaran yang dipakai ("midtrans" / kosong = manual)
--   payment_status     : status mentah dari penyedia (pending, settlement, expire, …)
--   snap_token         : token Snap untuk snap.js
--   snap_redirect_url  : halaman pembayaran Midtrans (mode redirect/fallback)
ALTER TABLE orders
    ADD COLUMN IF NOT EXISTS payment_provider   TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS payment_status     TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS snap_token         TEXT,
    ADD COLUMN IF NOT EXISTS snap_redirect_url  TEXT;

CREATE INDEX IF NOT EXISTS orders_payment_provider_idx ON orders (payment_provider);
