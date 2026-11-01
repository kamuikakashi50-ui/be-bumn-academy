-- 000018_talent_mapping
-- Instrumen minat–bakat, skoring & norma, bobot lini bisnis, dan skor kecocokan
-- (Fase 3 langkah 3–4, masukan #1 "talent mapping minat–bakat untuk lini bisnis").
--
-- Catatan desain (Fase 3 §2.1–§2.3):
--   * Dimensi minat memakai kerangka **RIASEC/Holland**; dimensi bakat memakai
--     aptitude yang relevan dengan seleksi BUMN (numerik, verbal, figural, learning
--     agility). Tiga dimensi bakat pertama sudah terukur lewat bank soal Fase 0.
--   * **Norma disimpan berversi.** Setiap skor menyimpan `norm_version`, sehingga
--     hasil lama tetap dapat dijelaskan dengan norma saat itu (`talent_norms`).
--   * **Bobot lini bisnis adalah data**, bukan hardcode: `business_line_profiles.weights`
--     (JSONB) bisa disesuaikan klien; menghitung ulang menghasilkan `talent_fit_scores`
--     baru dengan `computed_at`, hasil lama tidak dihapus.
--   * `consent_records` dibuat di migrasi ini karena dirujuk sebagai foreign key oleh
--     `talent_assessments` (dan nanti `consultation_bookings` di migrasi 000020).

-- Persetujuan (consent) pemrosesan data asesmen/psikologis (UU PDP).
CREATE TABLE IF NOT EXISTS consent_records (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    purpose      TEXT NOT NULL,        -- assessment|consultation|hr_sharing|aggregate_publish
    text_version TEXT NOT NULL,
    granted_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at   TIMESTAMPTZ,
    ip_address   TEXT NOT NULL DEFAULT '',
    user_agent   TEXT NOT NULL DEFAULT ''
);

-- Instrumen minat–bakat (milik kami atau bertaut ke registri berlisensi Fase 0).
CREATE TABLE IF NOT EXISTS talent_instruments (
    id            TEXT PRIMARY KEY,
    code          TEXT NOT NULL UNIQUE,
    name          TEXT NOT NULL,
    kind          TEXT NOT NULL DEFAULT 'minat',   -- minat|bakat
    version       TEXT NOT NULL DEFAULT 'v1',
    -- Untuk instrumen berlisensi (mis. IST, Papi Kostick) tautkan ke registri Fase 0.
    instrument_id UUID REFERENCES psychological_instruments (id) ON DELETE SET NULL,
    norm_version  TEXT NOT NULL DEFAULT 'norm-2026',
    description   TEXT NOT NULL DEFAULT '',
    active        BOOLEAN NOT NULL DEFAULT TRUE,
    CONSTRAINT talent_instruments_kind_check CHECK (kind IN ('minat','bakat'))
);

CREATE TABLE IF NOT EXISTS talent_items (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    instrument_id TEXT NOT NULL REFERENCES talent_instruments (id) ON DELETE CASCADE,
    text          TEXT NOT NULL,
    dimension     TEXT NOT NULL,
    weight        NUMERIC(5,2) NOT NULL DEFAULT 1,
    reverse       BOOLEAN NOT NULL DEFAULT FALSE,  -- item terbalik (skor dibalik)
    sort_order    INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS talent_items_instrument_idx
    ON talent_items (instrument_id, sort_order);

-- Norma per dimensi per versi: dipakai mengubah skor mentah menjadi persentil.
CREATE TABLE IF NOT EXISTS talent_norms (
    norm_version TEXT NOT NULL,
    dimension    TEXT NOT NULL,
    mean         NUMERIC(6,2) NOT NULL,
    sd           NUMERIC(6,2) NOT NULL,
    PRIMARY KEY (norm_version, dimension),
    CONSTRAINT talent_norms_sd_check CHECK (sd > 0)
);

CREATE TABLE IF NOT EXISTS talent_assessments (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    instrument_id   TEXT NOT NULL REFERENCES talent_instruments (id) ON DELETE CASCADE,
    cohort_id       UUID REFERENCES cohorts (id) ON DELETE SET NULL,
    status          TEXT NOT NULL DEFAULT 'berjalan',  -- berjalan|selesai|dibatalkan
    administered_by UUID REFERENCES users (id) ON DELETE SET NULL,  -- psikolog bila berlisensi
    started_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at     TIMESTAMPTZ,
    consent_id      UUID REFERENCES consent_records (id) ON DELETE SET NULL,
    CONSTRAINT talent_assessments_status_check
        CHECK (status IN ('berjalan','selesai','dibatalkan'))
);

CREATE INDEX IF NOT EXISTS talent_assessments_user_idx
    ON talent_assessments (user_id, started_at DESC);

CREATE TABLE IF NOT EXISTS talent_answers (
    assessment_id UUID NOT NULL REFERENCES talent_assessments (id) ON DELETE CASCADE,
    item_id       UUID NOT NULL REFERENCES talent_items (id) ON DELETE CASCADE,
    value         INTEGER NOT NULL,
    PRIMARY KEY (assessment_id, item_id),
    CONSTRAINT talent_answers_value_check CHECK (value BETWEEN 0 AND 5)
);

CREATE TABLE IF NOT EXISTS talent_scores (
    user_id       UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    dimension     TEXT NOT NULL,
    raw           NUMERIC(6,2) NOT NULL,
    percentile    NUMERIC(5,2) NOT NULL,
    norm_version  TEXT NOT NULL,
    assessment_id UUID REFERENCES talent_assessments (id) ON DELETE SET NULL,
    computed_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, dimension, norm_version),
    CONSTRAINT talent_scores_percentile_check CHECK (percentile BETWEEN 0 AND 100)
);

-- Profil lini bisnis + bobot dimensi (per klaster BUMN; bisa spesifik instansi).
CREATE TABLE IF NOT EXISTS business_line_profiles (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sector_key  TEXT NOT NULL,
    name        TEXT NOT NULL,
    weights     JSONB NOT NULL DEFAULT '{}'::jsonb,
    description TEXT NOT NULL DEFAULT '',
    org_id      UUID REFERENCES organizations (id) ON DELETE CASCADE, -- NULL = default
    active      BOOLEAN NOT NULL DEFAULT TRUE
);

CREATE UNIQUE INDEX IF NOT EXISTS business_line_profiles_sector_idx
    ON business_line_profiles (sector_key, COALESCE(org_id, '00000000-0000-0000-0000-000000000000'::uuid));

CREATE TABLE IF NOT EXISTS talent_fit_scores (
    user_id          UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    business_line_id UUID NOT NULL REFERENCES business_line_profiles (id) ON DELETE CASCADE,
    score            NUMERIC(5,2) NOT NULL,
    computed_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, business_line_id),
    CONSTRAINT talent_fit_scores_score_check CHECK (score BETWEEN 0 AND 100)
);

