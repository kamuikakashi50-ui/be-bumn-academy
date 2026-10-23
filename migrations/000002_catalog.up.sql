-- 000002_catalog
-- Konten katalog publik: program, kategori materi, produk, metode bayar, voucher.

CREATE TABLE IF NOT EXISTS programs (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug        TEXT        NOT NULL,
    name        TEXT        NOT NULL,
    tagline     TEXT        NOT NULL DEFAULT '',
    audience    TEXT        NOT NULL DEFAULT '',
    format      TEXT        NOT NULL DEFAULT '',
    duration    TEXT        NOT NULL DEFAULT '',
    features    JSONB       NOT NULL DEFAULT '[]'::jsonb,
    highlighted BOOLEAN     NOT NULL DEFAULT false,
    sort_order  INTEGER     NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS programs_slug_uidx ON programs (slug);
CREATE INDEX IF NOT EXISTS programs_sort_idx ON programs (sort_order);

CREATE TABLE IF NOT EXISTS materi_categories (
    id          TEXT PRIMARY KEY,
    label       TEXT        NOT NULL,
    description TEXT        NOT NULL DEFAULT '',
    topics      JSONB       NOT NULL DEFAULT '[]'::jsonb,
    sort_order  INTEGER     NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS materi_categories_sort_idx ON materi_categories (sort_order);

CREATE TABLE IF NOT EXISTS products (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug        TEXT        NOT NULL,
    name        TEXT        NOT NULL,
    description TEXT        NOT NULL DEFAULT '',
    price       BIGINT      NOT NULL DEFAULT 0,
    type        TEXT        NOT NULL DEFAULT 'program',
    includes    JSONB       NOT NULL DEFAULT '[]'::jsonb,
    badge       TEXT,
    active      BOOLEAN     NOT NULL DEFAULT true,
    sort_order  INTEGER     NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT products_type_check CHECK (type IN ('program', 'tryout')),
    CONSTRAINT products_price_check CHECK (price >= 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS products_slug_uidx ON products (slug);
CREATE INDEX IF NOT EXISTS products_active_sort_idx ON products (active, sort_order);

CREATE TABLE IF NOT EXISTS payment_methods (
    id         TEXT PRIMARY KEY,
    label      TEXT        NOT NULL,
    channel    TEXT        NOT NULL DEFAULT '',
    fee        BIGINT      NOT NULL DEFAULT 0,
    active     BOOLEAN     NOT NULL DEFAULT true,
    sort_order INTEGER     NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS vouchers (
    code         TEXT PRIMARY KEY,
    label        TEXT        NOT NULL DEFAULT '',
    type         TEXT        NOT NULL,
    value        BIGINT      NOT NULL DEFAULT 0,
    min_purchase BIGINT      NOT NULL DEFAULT 0,
    max_uses     INTEGER,
    used_count   INTEGER     NOT NULL DEFAULT 0,
    starts_at    TIMESTAMPTZ,
    ends_at      TIMESTAMPTZ,
    active       BOOLEAN     NOT NULL DEFAULT true,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT vouchers_type_check CHECK (type IN ('percent', 'fixed')),
    CONSTRAINT vouchers_value_check CHECK (value >= 0)
);

CREATE INDEX IF NOT EXISTS vouchers_active_idx ON vouchers (active);
