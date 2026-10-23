-- 000001_init_core
-- Pengguna, sesi refresh token, dan ekstensi dasar.
-- Catatan: seluruh tabel memakai UUID (gen_random_uuid(), bawaan PostgreSQL 13+)
-- agar aman dibuat di beberapa replika API sekaligus tanpa urutan global.

CREATE TABLE IF NOT EXISTS users (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            TEXT        NOT NULL,
    email           TEXT        NOT NULL,
    password_hash   TEXT        NOT NULL,
    role            TEXT        NOT NULL DEFAULT 'peserta',
    program         TEXT        NOT NULL DEFAULT 'Peserta',
    referral_code   TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT users_role_check CHECK (role IN ('peserta', 'admin'))
);

-- Email selalu dicari saat login → index unik pada lower(email).
CREATE UNIQUE INDEX IF NOT EXISTS users_email_lower_uidx ON users (lower(email));
CREATE UNIQUE INDEX IF NOT EXISTS users_referral_code_uidx ON users (referral_code) WHERE referral_code IS NOT NULL;

-- Refresh token disimpan sebagai hash (bukan token mentah) agar bocoran DB
-- tidak langsung bisa dipakai. Rotasi dilakukan saat refresh.
CREATE TABLE IF NOT EXISTS refresh_tokens (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash  TEXT        NOT NULL,
    user_agent  TEXT,
    ip_address  TEXT,
    expires_at  TIMESTAMPTZ NOT NULL,
    revoked_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS refresh_tokens_hash_uidx ON refresh_tokens (token_hash);
-- Pembersihan token kedaluwarsa memakai index ini.
CREATE INDEX IF NOT EXISTS refresh_tokens_user_expires_idx ON refresh_tokens (user_id, expires_at DESC);
