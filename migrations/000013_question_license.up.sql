-- 000013_question_license
-- Izin/hak pakai soal dan daftar instrumen tes psikologi.
--
-- Permintaan klien: "harus dipastikan izinnya" — untuk soal maupun tes
-- psikologi. Karena itu setiap soal punya baris lisensi (asal hak pakai, nomor
-- perjanjian, berkasnya, masa berlaku, dan batas pemakaian), dan instrumen
-- psikologi punya status izin tersendiri.
--
-- Backfill: seluruh soal yang sudah ada dianggap milik sendiri (`internal`)
-- sehingga pengerjaan tryout tidak terhenti saat penegakan izin dinyalakan.
-- Soal baru hanya bisa dipakai bila lisensinya aktif — itulah bentuk
-- "fail-closed" yang diminta.

CREATE TABLE IF NOT EXISTS question_licenses (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    question_id     TEXT        NOT NULL REFERENCES questions (id) ON DELETE CASCADE,
    scope           TEXT        NOT NULL DEFAULT 'internal',
    owner           TEXT        NOT NULL DEFAULT '',
    agreement_no    TEXT        NOT NULL DEFAULT '',
    agreement_url   TEXT,
    commercial_use  BOOLEAN     NOT NULL DEFAULT TRUE,
    selection_use   BOOLEAN     NOT NULL DEFAULT FALSE,
    allowed_sectors JSONB       NOT NULL DEFAULT '[]'::jsonb,
    valid_from      TIMESTAMPTZ NOT NULL DEFAULT now(),
    valid_until     TIMESTAMPTZ,
    status          TEXT        NOT NULL DEFAULT 'aktif',
    notes           TEXT        NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT question_licenses_scope_check
        CHECK (scope IN ('internal', 'vendor', 'public_domain', 'client_owned')),
    CONSTRAINT question_licenses_status_check
        CHECK (status IN ('aktif', 'kedaluwarsa', 'dicabut')),
    CONSTRAINT question_licenses_period_check
        CHECK (valid_until IS NULL OR valid_until >= valid_from)
);

CREATE INDEX IF NOT EXISTS question_licenses_question_idx
    ON question_licenses (question_id);

-- Penegakan izin membaca (status, valid_until) untuk seluruh soal aktif.
CREATE INDEX IF NOT EXISTS question_licenses_status_idx
    ON question_licenses (status, valid_until);

INSERT INTO question_licenses (question_id, scope, owner, notes)
SELECT q.id, 'internal', 'BUMN Academy',
       'Backfill migrasi 000013: soal yang sudah ada sebelum kebijakan izin.'
  FROM questions q
 WHERE NOT EXISTS (
       SELECT 1 FROM question_licenses l WHERE l.question_id = q.id
 );

CREATE TABLE IF NOT EXISTS psychological_instruments (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    code            TEXT        NOT NULL UNIQUE,
    name            TEXT        NOT NULL,
    kind            TEXT        NOT NULL DEFAULT 'latihan',
    vendor          TEXT        NOT NULL DEFAULT '',
    license_no      TEXT        NOT NULL DEFAULT '',
    license_url     TEXT,
    license_from    TIMESTAMPTZ,
    license_until   TIMESTAMPTZ,
    status          TEXT        NOT NULL DEFAULT 'aktif',
    administered_by UUID        REFERENCES users (id) ON DELETE SET NULL,
    scoring_notes   TEXT        NOT NULL DEFAULT '',
    active          BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT psychological_instruments_kind_check
        CHECK (kind IN ('latihan', 'berlisensi')),
    CONSTRAINT psychological_instruments_status_check
        CHECK (status IN ('aktif', 'kedaluwarsa', 'dicabut'))
);

CREATE INDEX IF NOT EXISTS psychological_instruments_status_idx
    ON psychological_instruments (status, active);

-- Instrumen latihan yang boleh dipakai peserta.
INSERT INTO psychological_instruments (code, name, kind, vendor, status, scoring_notes, active)
VALUES
    ('deret-angka', 'Deret Angka & Huruf', 'latihan', 'internal', 'aktif',
     'Skor = jumlah jawaban benar; norma latihan internal.', TRUE),
    ('kraepelin', 'Kraepelin / Pauli', 'latihan', 'internal', 'aktif',
     'Skor = ketelitian & ketahanan; norma latihan internal.', TRUE),
    ('papi-kostick', 'Papi Kostick', 'berlisensi', '', 'aktif',
     'Wajib dijalankan psikolog berlisensi; norma dari pemilik alat tes.', FALSE)
ON CONFLICT (code) DO NOTHING;
