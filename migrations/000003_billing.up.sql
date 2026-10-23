-- 000003_billing
-- Pesanan, kepemilikan produk (enrollment), dan log notifikasi transaksi.

CREATE TABLE IF NOT EXISTS orders (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    invoice_no        TEXT        NOT NULL,
    user_id           UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    product_slug      TEXT        NOT NULL,
    product_name      TEXT        NOT NULL DEFAULT '',
    price             BIGINT      NOT NULL DEFAULT 0,
    discount          BIGINT      NOT NULL DEFAULT 0,
    total             BIGINT      NOT NULL DEFAULT 0,
    voucher_code      TEXT,
    referral_code     TEXT,
    method            TEXT        NOT NULL DEFAULT '',
    status            TEXT        NOT NULL DEFAULT 'pending',
    payment_reference TEXT,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    paid_at           TIMESTAMPTZ,
    expires_at        TIMESTAMPTZ,
    CONSTRAINT orders_status_check CHECK (status IN ('pending', 'paid', 'expired')),
    CONSTRAINT orders_total_check CHECK (total >= 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS orders_invoice_no_uidx ON orders (invoice_no);
-- Halaman "Riwayat Pesanan" selalu query per user terbaru → index komposit.
CREATE INDEX IF NOT EXISTS orders_user_created_idx ON orders (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS orders_status_idx ON orders (status);
-- Idempotensi callback pembayaran: satu referensi pembayaran hanya sekali.
CREATE UNIQUE INDEX IF NOT EXISTS orders_payment_reference_uidx
    ON orders (payment_reference) WHERE payment_reference IS NOT NULL;

-- Kepemilikan produk. Dibuat idempotent lewat unique index (user_id, product_slug)
-- sehingga callback pembayaran boleh diulang tanpa menghasilkan data ganda.
CREATE TABLE IF NOT EXISTS enrollments (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    product_slug TEXT        NOT NULL,
    order_id     UUID        REFERENCES orders (id) ON DELETE SET NULL,
    source       TEXT        NOT NULL DEFAULT 'purchase',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS enrollments_user_product_uidx ON enrollments (user_id, product_slug);
CREATE INDEX IF NOT EXISTS enrollments_user_idx ON enrollments (user_id);

-- Log notifikasi transaksi (WhatsApp/email) supaya pengiriman tidak dobel.
CREATE TABLE IF NOT EXISTS notification_logs (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID        REFERENCES users (id) ON DELETE SET NULL,
    order_id   UUID        REFERENCES orders (id) ON DELETE SET NULL,
    channel    TEXT        NOT NULL DEFAULT 'whatsapp',
    event      TEXT        NOT NULL,
    status     TEXT        NOT NULL DEFAULT 'queued',
    message    TEXT        NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    sent_at    TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS notification_logs_order_idx ON notification_logs (order_id);
