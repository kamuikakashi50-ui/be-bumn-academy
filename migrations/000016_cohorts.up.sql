-- 000016_cohorts
-- Kelas khusus (kohort) per instansi + isolasi bank soal antar klien.
--
-- Latar belakang (masukan klien): "usulan dibuat kelas khusus" (Rafi Nurul).
-- Sebelum ini akses materi hanya lewat pembelian produk per peserta
-- (`enrollments`), sehingga tidak ada cara menyelenggarakan satu kelas privat
-- untuk sebuah instansi dengan kurikulum, kode undangan, jadwal, dan laporan
-- tersendiri.
--
-- Yang dibangun di sini:
--   1. organizations     → instansi mitra (klaster BUMN dari taksonomi Fase 0).
--   2. cohorts           → satu batch/kelas untuk sebuah instansi.
--   3. cohort_members    → keanggotaan peserta/mentor pada kelas itu.
--   4. cohort_curriculum → modul ruang kelas yang diikutkan ke kelas.
--   5. cohort_invites    → kode undangan berkuota, berkedaluwarsa, bisa dicabut.
--   6. cohort_audit_logs → jejak audit (siapa mengundang/mengeluarkan siapa).
--
-- Catatan desain:
--   * Kohort menempel pada produk & modul yang sudah ada (keputusan §2.1), jadi
--     perbaikan materi berlaku untuk semua kelas sekaligus — bukan menggandakan
--     materi per klien.
--   * Isolasi bank soal memakai `questions.owner_org_id`: NULL = soal bersama
--     (mis. TKD umum, boleh dipakai semua kelas), terisi = hanya boleh dipakai
--     kelas instansi tersebut (§4.5).
--   * `attempts.cohort_id` diisi saat peserta memulai sesi tryout dari
--     keanggotaan kohort aktifnya, supaya laporan kelas tidak menebak dari
--     tanggal (cara yang rapuh dan tidak bisa diaudit).
--   * `certificates.cohort_id` dibuat di migrasi 000015 tanpa foreign key karena
--     tabel `cohorts` belum ada; di sini kolom itu ditautkan agar sertifikat
--     kelas tidak bisa menggantung ke kohort yang sudah dihapus.

/* ------------------------------- instansi ---------------------------------- */

CREATE TABLE IF NOT EXISTS organizations (
    id         UUID    PRIMARY KEY DEFAULT gen_random_uuid(),
    name       TEXT    NOT NULL,
    slug       TEXT    NOT NULL UNIQUE,
    -- Klaster BUMN (taksonomi Fase 0) agar laporan & bank soal bisa disilangkan.
    sector_key TEXT    NOT NULL DEFAULT 'umum',
    logo_url   TEXT,
    pic_name   TEXT    NOT NULL DEFAULT '',
    pic_email  TEXT    NOT NULL DEFAULT '',
    pic_phone  TEXT    NOT NULL DEFAULT '',
    active     BOOLEAN NOT NULL DEFAULT TRUE
);

CREATE INDEX IF NOT EXISTS organizations_sector_idx
    ON organizations (sector_key, active);

/* -------------------------------- kohort ----------------------------------- */

CREATE TABLE IF NOT EXISTS cohorts (
    id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    -- NULL = kelas internal (bukan milik instansi tertentu).
    org_id         UUID        REFERENCES organizations (id) ON DELETE SET NULL,
    name           TEXT        NOT NULL,
    -- Kode internal kohort, dipakai sebagai label laporan (bukan kode undangan).
    code           TEXT        NOT NULL UNIQUE,
    sector_key     TEXT        NOT NULL DEFAULT 'umum',
    status         TEXT        NOT NULL DEFAULT 'draft',
    starts_at      TIMESTAMPTZ,
    ends_at        TIMESTAMPTZ,
    -- 0 = tanpa batas kuota peserta.
    quota          INTEGER     NOT NULL DEFAULT 0,
    -- Akhir masa akses materi peserta; setelah lewat, akses ditutup tetapi
    -- riwayat & sertifikat tetap bisa dilihat (§4.1).
    access_until   TIMESTAMPTZ,
    mentor_user_id UUID        REFERENCES users (id) ON DELETE SET NULL,
    notes          TEXT        NOT NULL DEFAULT '',
    CONSTRAINT cohorts_status_check
        CHECK (status IN ('draft', 'aktif', 'selesai', 'arsip')),
    CONSTRAINT cohorts_quota_check CHECK (quota >= 0)
);

CREATE INDEX IF NOT EXISTS cohorts_org_idx
    ON cohorts (org_id, status);

CREATE INDEX IF NOT EXISTS cohorts_taxonomy_idx
    ON cohorts (sector_key, status);

CREATE TABLE IF NOT EXISTS cohort_members (
    cohort_id UUID        NOT NULL REFERENCES cohorts (id) ON DELETE CASCADE,
    user_id   UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    role      TEXT        NOT NULL DEFAULT 'peserta',
    status    TEXT        NOT NULL DEFAULT 'aktif',
    joined_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (cohort_id, user_id),
    CONSTRAINT cohort_members_role_check
        CHECK (role IN ('peserta', 'mentor', 'admin_instansi')),
    CONSTRAINT cohort_members_status_check
        CHECK (status IN ('aktif', 'keluar', 'lulus'))
);

-- Pencarian kohort aktif seorang peserta (dipakai pemeriksaan akses materi).
CREATE INDEX IF NOT EXISTS cohort_members_user_idx
    ON cohort_members (user_id, status);

-- Kurikulum kelas: modul yang diikutkan, dengan urutan sendiri.
CREATE TABLE IF NOT EXISTS cohort_curriculum (
    cohort_id  UUID    NOT NULL REFERENCES cohorts (id) ON DELETE CASCADE,
    module_id  TEXT    NOT NULL REFERENCES course_modules (id) ON DELETE CASCADE,
    required   BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (cohort_id, module_id)
);

CREATE INDEX IF NOT EXISTS cohort_curriculum_order_idx
    ON cohort_curriculum (cohort_id, sort_order);

/* ------------------------------ kode undangan ------------------------------ */

CREATE TABLE IF NOT EXISTS cohort_invites (
    -- 8 karakter huruf besar + angka, tanpa karakter ambigu (0/O, 1/I).
    code       TEXT        PRIMARY KEY,
    cohort_id  UUID        NOT NULL REFERENCES cohorts (id) ON DELETE CASCADE,
    created_by UUID        REFERENCES users (id) ON DELETE SET NULL,
    -- 0 = tanpa batas pemakaian.
    max_uses   INTEGER     NOT NULL DEFAULT 0,
    used_count INTEGER     NOT NULL DEFAULT 0,
    expires_at TIMESTAMPTZ,
    active     BOOLEAN     NOT NULL DEFAULT TRUE,
    CONSTRAINT cohort_invites_max_uses_check CHECK (max_uses >= 0),
    CONSTRAINT cohort_invites_used_count_check CHECK (used_count >= 0)
);

CREATE INDEX IF NOT EXISTS cohort_invites_cohort_idx
    ON cohort_invites (cohort_id, active);

/* ------------------------------- jejak audit ------------------------------- */

CREATE TABLE IF NOT EXISTS cohort_audit_logs (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    cohort_id  UUID        NOT NULL REFERENCES cohorts (id) ON DELETE CASCADE,
    actor_id   UUID        REFERENCES users (id) ON DELETE SET NULL,
    action     TEXT        NOT NULL,
    detail     TEXT        NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS cohort_audit_logs_cohort_idx
    ON cohort_audit_logs (cohort_id, created_at DESC);

/* ---------------------- dimensi baru pada tabel lama ----------------------- */

-- Soal milik instansi: NULL berarti soal bersama (boleh dipakai lintas kelas).
ALTER TABLE questions
    ADD COLUMN IF NOT EXISTS owner_org_id UUID REFERENCES organizations (id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS questions_owner_org_idx
    ON questions (owner_org_id, bank_key)
 WHERE owner_org_id IS NOT NULL;

-- Kelas live khusus kohort: NULL berarti kelas live umum.
ALTER TABLE live_sessions
    ADD COLUMN IF NOT EXISTS cohort_id UUID REFERENCES cohorts (id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS live_sessions_cohort_idx
    ON live_sessions (cohort_id, starts_at DESC)
 WHERE cohort_id IS NOT NULL;

-- Riwayat percobaan tryout dipakai laporan kelas (Fase 1 langkah 6).
ALTER TABLE attempts
    ADD COLUMN IF NOT EXISTS cohort_id UUID REFERENCES cohorts (id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS attempts_cohort_idx
    ON attempts (cohort_id, taken_at DESC)
 WHERE cohort_id IS NOT NULL;

-- Sertifikat kelas: kolomnya sudah ada sejak 000015 (tanpa FK), kini ditautkan.
ALTER TABLE certificates
    DROP CONSTRAINT IF EXISTS certificates_cohort_fk;

ALTER TABLE certificates
    ADD CONSTRAINT certificates_cohort_fk
    FOREIGN KEY (cohort_id) REFERENCES cohorts (id) ON DELETE SET NULL;
