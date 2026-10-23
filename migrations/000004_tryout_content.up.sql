-- 000004_tryout_content
-- Bank soal + definisi paket tryout. Struktur ini mengikuti model front-end:
-- paket tryout berisi beberapa subtes, dan setiap subtes mengambil soal dari
-- satu atau lebih "bank" (kategori soal).

CREATE TABLE IF NOT EXISTS questions (
    id           TEXT PRIMARY KEY,
    bank_key     TEXT        NOT NULL,
    text         TEXT        NOT NULL,
    options      JSONB       NOT NULL DEFAULT '[]'::jsonb,
    answer_index INTEGER     NOT NULL,
    explanation  TEXT        NOT NULL DEFAULT '',
    difficulty   TEXT        NOT NULL DEFAULT 'sedang',
    -- position menjaga urutan soal di dalam satu bank (tidak bergantung id).
    position     INTEGER     NOT NULL DEFAULT 0,
    active       BOOLEAN     NOT NULL DEFAULT true,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT questions_answer_index_check CHECK (answer_index >= 0),
    CONSTRAINT questions_difficulty_check CHECK (difficulty IN ('mudah', 'sedang', 'sulit'))
);

-- Penyusunan sesi tryout mengambil soal per bank secara terurut → index kunci.
CREATE INDEX IF NOT EXISTS questions_bank_position_idx ON questions (bank_key, position, id) WHERE active;
CREATE INDEX IF NOT EXISTS questions_updated_idx ON questions (updated_at DESC);

CREATE TABLE IF NOT EXISTS tryout_packages (
    id               TEXT PRIMARY KEY,
    title            TEXT        NOT NULL,
    description      TEXT        NOT NULL DEFAULT '',
    tier             TEXT        NOT NULL DEFAULT 'Gratis',
    required_product TEXT,
    attempts         INTEGER     NOT NULL DEFAULT 1,
    access_until     TEXT        NOT NULL DEFAULT 'Tanpa batas',
    active           BOOLEAN     NOT NULL DEFAULT true,
    sort_order       INTEGER     NOT NULL DEFAULT 0,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT tryout_packages_tier_check CHECK (tier IN ('Gratis', 'Premium')),
    CONSTRAINT tryout_packages_attempts_check CHECK (attempts > 0)
);

CREATE INDEX IF NOT EXISTS tryout_packages_active_sort_idx ON tryout_packages (active, sort_order);

CREATE TABLE IF NOT EXISTS subtests (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tryout_id  TEXT        NOT NULL REFERENCES tryout_packages (id) ON DELETE CASCADE,
    name       TEXT        NOT NULL,
    minutes    INTEGER     NOT NULL DEFAULT 5,
    banks      JSONB       NOT NULL DEFAULT '[]'::jsonb,
    sort_order INTEGER     NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT subtests_minutes_check CHECK (minutes > 0)
);

CREATE INDEX IF NOT EXISTS subtests_tryout_sort_idx ON subtests (tryout_id, sort_order);
