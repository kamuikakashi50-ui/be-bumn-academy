-- 000020_psychologist
-- Mitra psikolog, konsultasi daring, dan jejak akses data asesmen
-- (Fase 3 langkah 6–7, masukan #1 & #3).
--
-- Catatan desain (Fase 3 §2.6–§2.9, §3.3, §4.5–§4.7):
--   * Hasil asesmen berlisensi **digerbangi psikolog**: mitra wajib
--     `verification_status = 'terverifikasi'` dan `valid_until` belum lewat
--     sebelum boleh ditugaskan atau membuka data peserta.
--   * Konsultasi daring memakai pola `live_sessions` (jadwal) + slot per psikolog
--     dan rekaman persetujuan (`consent_records`, migrasi 000018).
--   * Catatan sesi punya **tiga tingkat visibilitas** (`private`,
--     `shared_to_user`, `shared_to_hr`). HR hanya melihat yang diizinkan peserta;
--     catatan mentah tidak pernah dibuka ke admin (§4.6).
--   * `assessment_access_logs` mencatat siapa membuka data asesmen siapa —
--     wajib untuk UU PDP No. 27/2022 (akses minimal + dapat diaudit).
--   * Peran pengguna diperluas dengan `psikolog` dan `admin_instansi` (§2.8).
--   * `assessment_disputes` adalah **tambahan** di luar §3.3: hak peserta
--     mengajukan koreksi (§2.9, §4.7) butuh tempat menyimpan permohonannya.

-- ------------------------------- mitra psikolog -----------------------------

CREATE TABLE IF NOT EXISTS psychologist_partners (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id             UUID REFERENCES users (id) ON DELETE SET NULL,
    full_name           TEXT NOT NULL,
    email               TEXT NOT NULL DEFAULT '',
    phone               TEXT NOT NULL DEFAULT '',
    sipp_number         TEXT NOT NULL DEFAULT '',
    sippk_number        TEXT,
    str_number          TEXT,
    specializations     JSONB NOT NULL DEFAULT '[]'::jsonb,
    verification_status TEXT NOT NULL DEFAULT 'pending',
    valid_until         DATE,
    evidence_url        TEXT,
    notes               TEXT NOT NULL DEFAULT '',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT psychologist_status_check
        CHECK (verification_status IN ('pending', 'terverifikasi', 'nonaktif'))
);

-- Satu akun pengguna hanya boleh mewakili satu mitra (bila ditautkan).
CREATE UNIQUE INDEX IF NOT EXISTS psychologist_partners_user_id_key
    ON psychologist_partners (user_id) WHERE user_id IS NOT NULL;

-- ------------------------------ slot konsultasi -----------------------------

CREATE TABLE IF NOT EXISTS consultation_slots (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    psychologist_id UUID NOT NULL REFERENCES psychologist_partners (id) ON DELETE CASCADE,
    starts_at       TIMESTAMPTZ NOT NULL,
    ends_at         TIMESTAMPTZ NOT NULL,
    mode            TEXT NOT NULL DEFAULT 'online',
    quota           INTEGER NOT NULL DEFAULT 1,
    booked          INTEGER NOT NULL DEFAULT 0,
    channel_url     TEXT,
    active          BOOLEAN NOT NULL DEFAULT TRUE,
    CONSTRAINT consultation_slots_mode_check CHECK (mode IN ('online', 'offline')),
    CONSTRAINT consultation_slots_window_check CHECK (ends_at > starts_at),
    CONSTRAINT consultation_slots_quota_check CHECK (quota > 0),
    -- Kuota tidak boleh dilampaui; dijaga di database sebagai jaring terakhir
    -- bila dua pemesanan berbarengan.
    CONSTRAINT consultation_slots_booked_check CHECK (booked >= 0 AND booked <= quota)
);

CREATE INDEX IF NOT EXISTS consultation_slots_psychologist_idx
    ON consultation_slots (psychologist_id, starts_at);

-- ------------------------------ pemesanan ----------------------------------

CREATE TABLE IF NOT EXISTS consultation_bookings (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slot_id     UUID NOT NULL REFERENCES consultation_slots (id) ON DELETE CASCADE,
    user_id     UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    cohort_id   UUID REFERENCES cohorts (id) ON DELETE SET NULL,
    status      TEXT NOT NULL DEFAULT 'terpesan',
    agenda      TEXT NOT NULL DEFAULT '',
    consent_id  UUID REFERENCES consent_records (id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT consultation_bookings_status_check
        CHECK (status IN ('terpesan', 'selesai', 'batal', 'tidak_hadir'))
);

-- Peserta hanya boleh punya **satu pemesanan aktif** per slot. Pemesanan yang
-- sudah dibatalkan tidak menghalangi pemesanan ulang, sehingga constraint-nya
-- berupa unique index parsial — bukan unique biasa (§4.5).
CREATE UNIQUE INDEX IF NOT EXISTS consultation_bookings_active_unique
    ON consultation_bookings (slot_id, user_id) WHERE status <> 'batal';

-- ------------------------------ catatan sesi -------------------------------

CREATE TABLE IF NOT EXISTS consultation_notes (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    booking_id     UUID NOT NULL REFERENCES consultation_bookings (id) ON DELETE CASCADE,
    author_id      UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    summary        TEXT NOT NULL,
    recommendation TEXT NOT NULL DEFAULT '',
    visibility     TEXT NOT NULL DEFAULT 'private',
    retention_until DATE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT consultation_notes_visibility_check
        CHECK (visibility IN ('private', 'shared_to_user', 'shared_to_hr'))
);

CREATE INDEX IF NOT EXISTS consultation_notes_booking_idx
    ON consultation_notes (booking_id, created_at DESC);

-- --------------------------- jejak akses data ------------------------------

-- Siapa (actor) membuka data asesmen siapa (subject), apa, dan kapan.
CREATE TABLE IF NOT EXISTS assessment_access_logs (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_id    UUID REFERENCES users (id) ON DELETE SET NULL,
    subject_id  UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    resource    TEXT NOT NULL,
    resource_id TEXT NOT NULL DEFAULT '',
    action      TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT assessment_access_logs_resource_check
        CHECK (resource IN ('talent_profile', 'competency', 'consultation_note', 'item_answers')),
    CONSTRAINT assessment_access_logs_action_check
        CHECK (action IN ('view', 'export', 'update'))
);

CREATE INDEX IF NOT EXISTS assessment_access_logs_subject_idx
    ON assessment_access_logs (subject_id, created_at DESC);
CREATE INDEX IF NOT EXISTS assessment_access_logs_actor_idx
    ON assessment_access_logs (actor_id, created_at DESC);

-- -------------------------- koreksi hasil peserta ---------------------------

CREATE TABLE IF NOT EXISTS assessment_disputes (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    assessment_id UUID NOT NULL REFERENCES talent_assessments (id) ON DELETE CASCADE,
    user_id       UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    reason        TEXT NOT NULL,
    status        TEXT NOT NULL DEFAULT 'terbuka',
    resolution    TEXT NOT NULL DEFAULT '',
    resolved_by   UUID REFERENCES users (id) ON DELETE SET NULL,
    resolved_at   TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT assessment_disputes_status_check
        CHECK (status IN ('terbuka', 'ditinjau', 'selesai', 'ditolak'))
);

CREATE INDEX IF NOT EXISTS assessment_disputes_user_idx
    ON assessment_disputes (user_id, created_at DESC);

-- --------------------- interpretasi instrumen berlisensi --------------------

-- Hasil instrumen berlisensi hanya diinterpretasi akun berperan `psikolog`
-- (§2.6). Tabel ini menyimpan catatan interpretasinya.
--
-- **Tambahan** di luar §3.3 (seperti `assessment_disputes`): tanpa tabel ini
-- endpoint `POST /psikolog/instruments/{code}/interpretation` tidak punya tempat
-- menyimpan hasil kerjanya, dan interpretasi psikolog tidak dapat diaudit.
CREATE TABLE IF NOT EXISTS instrument_interpretations (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    instrument_code TEXT NOT NULL,
    psychologist_id UUID NOT NULL REFERENCES psychologist_partners (id) ON DELETE CASCADE,
    subject_id      UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    assessment_id   UUID REFERENCES talent_assessments (id) ON DELETE SET NULL,
    interpretation  TEXT NOT NULL,
    retention_until DATE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS instrument_interpretations_subject_idx
    ON instrument_interpretations (subject_id, created_at DESC);

-- -------------------------------- peran baru -------------------------------

ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_check;
ALTER TABLE users ADD CONSTRAINT users_role_check
    CHECK (role IN ('peserta', 'admin', 'psikolog', 'admin_instansi'));


CREATE INDEX IF NOT EXISTS consultation_bookings_user_idx
    ON consultation_bookings (user_id, created_at DESC);
