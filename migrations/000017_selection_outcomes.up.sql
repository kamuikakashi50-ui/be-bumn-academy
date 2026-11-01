-- 000017_selection_outcomes
-- KPI kelulusan BUMN: tracer study hasil seleksi + verifikasi bukti + target.
--
-- Latar belakang (masukan klien): "KPI terkait yang lolos tes BUMN sebagai
-- indikator keberhasilan" (Pak Miwan). Sebelum ini seluruh metrik internal
-- (mis. `passingRate`, "tingkat kesiapan") hanya mengukur performa **latihan**,
-- sehingga klaim keberhasilan tidak bisa diukur sama sekali.
--
-- Yang dibangun di sini:
--   1. selection_stages          → referensi tahapan seleksi BUMN (data, bukan kode).
--   2. selection_outcomes        → hasil seleksi per peserta per tahap.
--   3. selection_outcome_audits  → jejak audit verifikasi/tolak laporan.
--   4. kpi_targets               → target KPI instansi/kelas per periode.
--
-- Catatan desain:
--   * Tidak ada jalur resmi menarik data dari sistem seleksi BUMN, sehingga
--     sumbernya adalah **laporan peserta** yang diverifikasi admin. Karena itu
--     setiap baris menyimpan `source` (self_reported/admin_verified/document_verified)
--     dan seluruh angka KPI selalu dikirim terpisah: `verified` vs `reported`
--     (§2.3 — tidak ada angka gabungan yang menyembunyikan asal data).
--   * Tahapan seleksi disimpan sebagai **tabel referensi**, bukan teks bebas,
--     supaya "lolos tahap 1" punya arti yang sama untuk semua peserta (§2.2).
--   * `company_sector` & `position_family` memakai kunci taksonomi Fase 0
--     (`internal/pkg/bank`) agar KPI bisa disilangkan dengan bank soal.
--   * `cohort_id` diisi dari keanggotaan kelas saat laporan dibuat (Fase 1) dan
--     **tidak** dihapus saat peserta keluar kelas — riwayat kelas tetap utuh.
--   * `aggregate_consent` menyimpan persetujuan peserta untuk pemakaian agregat
--     (§4.7); bawaannya aktif karena angka yang dipublikasikan tidak memuat
--     identitas, dan peserta bisa mematikannya.
--   * Laporan yang ditolak admin **tidak** dihapus: cukup dikembalikan ke
--     `self_reported` + `note` alasan, sehingga peserta bisa memperbaiki.

/* --------------------------- referensi tahapan ------------------------------ */

CREATE TABLE IF NOT EXISTS selection_stages (
    key        TEXT    PRIMARY KEY,
    label      TEXT    NOT NULL,
    -- phase: 1 = administrasi, 2 = tes tahap 1, 3 = tes tahap 2, dst.
    phase      INTEGER NOT NULL,
    -- is_final menandai tahap penentu kelulusan akhir (dipakai `finalPassRate`).
    is_final   BOOLEAN NOT NULL DEFAULT FALSE,
    sort_order INTEGER NOT NULL DEFAULT 0,
    active     BOOLEAN NOT NULL DEFAULT TRUE
);

-- Seed tahapan mengikuti Rekrutmen Bersama BUMN. Idempoten: menjalankan migrasi
-- ulang tidak menggandakan baris, dan label boleh diperbarui admin lewat panel.
INSERT INTO selection_stages (key, label, phase, is_final, sort_order, active) VALUES
    ('administrasi',            'Seleksi Administrasi',           1, FALSE, 10, TRUE),
    ('tahap1_tkd',              'Tes Kemampuan Dasar (TKD)',      2, FALSE, 20, TRUE),
    ('tahap1_akhlaq',           'Tes AKHLAK',                     2, FALSE, 30, TRUE),
    ('tahap1_twk',              'Tes Wawasan Kebangsaan (TWK)',   2, FALSE, 40, TRUE),
    ('tahap2_bing',             'Tes Bahasa Inggris',             3, FALSE, 50, TRUE),
    ('tahap2_learning_agility', 'Tes Learning Agility',           3, FALSE, 60, TRUE),
    ('tkb',                     'Tes Kompetensi Bidang (TKB)',    4, FALSE, 70, TRUE),
    ('wawancara_user',          'Wawancara User',                 5, FALSE, 80, TRUE),
    ('wawancara_hc',            'Wawancara HC',                   6, FALSE, 90, TRUE),
    ('mcu',                     'Medical Check-Up (MCU)',         7, FALSE, 100, TRUE),
    ('offering',                'Offering / Penetapan',           8, TRUE,  110, TRUE)
ON CONFLICT (key) DO NOTHING;

/* ---------------------------- hasil seleksi peserta ------------------------ */

CREATE TABLE IF NOT EXISTS selection_outcomes (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    -- Kelas khusus peserta saat laporan dibuat (Fase 1); NULL = bukan anggota kelas.
    cohort_id       UUID        REFERENCES cohorts (id) ON DELETE SET NULL,
    stage_key       TEXT        NOT NULL REFERENCES selection_stages (key) ON DELETE RESTRICT,
    company_name    TEXT        NOT NULL DEFAULT '',      -- BUMN tujuan
    company_sector  TEXT        NOT NULL DEFAULT 'umum',  -- sector_key (Fase 0)
    position        TEXT        NOT NULL DEFAULT '',
    position_family TEXT        NOT NULL DEFAULT 'umum',  -- keluarga jabatan (Fase 0)
    selection_year  INTEGER     NOT NULL,
    status          TEXT        NOT NULL,                 -- lolos|gagal|menunggu|mengundurkan_diri
    announced_at    DATE,
    source          TEXT        NOT NULL DEFAULT 'self_reported',
    evidence_url    TEXT,                                 -- unggahan bukti pengumuman
    note            TEXT        NOT NULL DEFAULT '',
    -- Persetujuan pemakaian agregat (§4.7); bawaan aktif karena angkanya anonim.
    aggregate_consent BOOLEAN   NOT NULL DEFAULT TRUE,
    verified_by     UUID        REFERENCES users (id) ON DELETE SET NULL,
    verified_at     TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT selection_outcomes_status_check
        CHECK (status IN ('lolos','gagal','menunggu','mengundurkan_diri')),
    CONSTRAINT selection_outcomes_source_check
        CHECK (source IN ('self_reported','admin_verified','document_verified')),
    CONSTRAINT selection_outcomes_year_check
        CHECK (selection_year BETWEEN 2020 AND 2100)
);

-- Satu peserta boleh punya beberapa outcome (beda instansi/tahun/tahap), tetapi
-- tidak boleh ganda pada kombinasi yang sama (§4.1).
CREATE UNIQUE INDEX IF NOT EXISTS selection_outcomes_unique_idx
    ON selection_outcomes (user_id, selection_year, stage_key, company_name);

CREATE INDEX IF NOT EXISTS selection_outcomes_stage_idx
    ON selection_outcomes (stage_key, status);

CREATE INDEX IF NOT EXISTS selection_outcomes_sector_idx
    ON selection_outcomes (company_sector, selection_year);

CREATE INDEX IF NOT EXISTS selection_outcomes_cohort_idx
    ON selection_outcomes (cohort_id);

-- Daftar tinjauan admin (§4.2) selalu menyaring laporan yang belum terverifikasi,
-- jadi indeks parsial ini yang dipakai.
CREATE INDEX IF NOT EXISTS selection_outcomes_pending_idx
    ON selection_outcomes (source, created_at)
    WHERE source = 'self_reported';

CREATE INDEX IF NOT EXISTS selection_outcomes_user_year_idx
    ON selection_outcomes (user_id, selection_year);

/* ------------------------------ jejak audit KPI ---------------------------- */

-- Perubahan angka KPI harus bisa ditelusuri (§4.2 butir 5): siapa memverifikasi
-- atau menolak laporan mana, dan kapan. Tabel ini hanya mencatat, tidak diubah.
CREATE TABLE IF NOT EXISTS selection_outcome_audits (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    outcome_id UUID        NOT NULL REFERENCES selection_outcomes (id) ON DELETE CASCADE,
    actor_id   UUID        REFERENCES users (id) ON DELETE SET NULL,
    action     TEXT        NOT NULL,   -- outcome_reported|outcome_verified|outcome_rejected|...
    note       TEXT        NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS selection_outcome_audits_outcome_idx
    ON selection_outcome_audits (outcome_id, created_at DESC);

/* ------------------------------- target KPI -------------------------------- */

CREATE TABLE IF NOT EXISTS kpi_targets (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    -- Target bisa menempel ke instansi, ke kelas khusus, atau ke keduanya;
    -- NULL pada keduanya berarti target lintas program (semua peserta).
    org_id       UUID        REFERENCES organizations (id) ON DELETE CASCADE,
    cohort_id    UUID        REFERENCES cohorts (id) ON DELETE CASCADE,
    period_year  INTEGER     NOT NULL,
    metric       TEXT        NOT NULL,  -- final_pass_rate|stage_pass_rate|participants|curriculum_completion_rate
    stage_key    TEXT        REFERENCES selection_stages (key) ON DELETE CASCADE,
    target_value NUMERIC(6,2) NOT NULL,
    note         TEXT        NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT kpi_targets_metric_check
        CHECK (metric IN ('final_pass_rate','stage_pass_rate','participants','curriculum_completion_rate')),
    CONSTRAINT kpi_targets_period_check
        CHECK (period_year BETWEEN 2020 AND 2100),
    -- Tahap hanya bermakna untuk metric per tahap.
    CONSTRAINT kpi_targets_stage_check
        CHECK ((metric = 'stage_pass_rate') OR (stage_key IS NULL))
);

-- Satu target per kombinasi kelompok + periode + metric (+ tahap).
CREATE UNIQUE INDEX IF NOT EXISTS kpi_targets_unique_idx
    ON kpi_targets (
        COALESCE(org_id, '00000000-0000-0000-0000-000000000000'::uuid),
        COALESCE(cohort_id, '00000000-0000-0000-0000-000000000000'::uuid),
        period_year,
        metric,
        COALESCE(stage_key, '')
    );

CREATE INDEX IF NOT EXISTS kpi_targets_period_idx
    ON kpi_targets (period_year, metric);
