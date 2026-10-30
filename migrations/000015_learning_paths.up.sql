-- 000015_learning_paths
-- Jalur belajar, kuis per pelajaran, dan sertifikat penyelesaian.
--
-- Latar belakang (masukan klien): "buat e-learning" — ruang kelas yang sudah ada
-- baru berisi modul & materi, belum ada urutan belajar yang disusun per kategori
-- BUMN/jenis tes, latihan di akhir materi, maupun bukti penyelesaian.
--
-- Tiga hal yang dibangun di sini:
--   1. learning_paths  → urutan modul untuk satu kategori/kelas sasaran.
--   2. lesson_quizzes  → kuis per pelajaran dengan snapshot soal dari bank soal
--                        (Fase 0), bukan pengacakan saat peserta mengerjakan.
--   3. certificates    → bukti penyelesaian bernomor unik yang bisa diverifikasi publik.
--
-- Catatan desain:
--   * Modul tetap milik produk (`course_modules.product_slug`); jalur belajar hanya
--     menyusun ulang modul yang sudah ada, sehingga perbaikan materi berlaku untuk
--     semua jalur sekaligus.
--   * `lesson_quiz_questions.question_id` ON DELETE SET NULL: soal yang dihapus
--     membuat kuis "perlu ditinjau" di panel admin, bukan membuat peserta gagal.
--   * Nomor sertifikat diambil dari `certificate_counters` dengan penguncian baris
--     (SELECT ... FOR UPDATE) agar dua peserta yang lulus bersamaan tidak mendapat
--     nomor kembar.
--   * `certificates.cohort_id` sengaja tanpa foreign key: tabel `cohorts` baru
--     dibuat pada migrasi 000016 (bagian kelas khusus).

/* ------------------------------ jalur belajar ------------------------------ */

CREATE TABLE IF NOT EXISTS learning_paths (
    id           TEXT        PRIMARY KEY,
    slug         TEXT        NOT NULL UNIQUE,
    title        TEXT        NOT NULL,
    description  TEXT        NOT NULL DEFAULT '',
    -- Dimensi taksonomi Fase 0 agar jalur bisa disaring per kategori/klaster BUMN.
    sector_key   TEXT        NOT NULL DEFAULT 'umum',
    test_type    TEXT        NOT NULL DEFAULT 'tkd',
    product_slug TEXT        NOT NULL,
    -- Terbitkan sertifikat saat seluruh langkah wajib selesai?
    certificate  BOOLEAN     NOT NULL DEFAULT TRUE,
    sort_order   INTEGER     NOT NULL DEFAULT 0,
    active       BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS learning_paths_taxonomy_idx
    ON learning_paths (sector_key, test_type, active, sort_order);

CREATE TABLE IF NOT EXISTS learning_path_steps (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    path_id    TEXT        NOT NULL REFERENCES learning_paths (id) ON DELETE CASCADE,
    module_id  TEXT        NOT NULL REFERENCES course_modules (id) ON DELETE CASCADE,
    required   BOOLEAN     NOT NULL DEFAULT TRUE,
    sort_order INTEGER     NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- Satu modul hanya boleh muncul sekali dalam satu jalur.
    CONSTRAINT learning_path_steps_unique UNIQUE (path_id, module_id)
);

CREATE INDEX IF NOT EXISTS learning_path_steps_order_idx
    ON learning_path_steps (path_id, sort_order);

/* --------------------------- kuis per pelajaran ---------------------------- */

CREATE TABLE IF NOT EXISTS lesson_quizzes (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    lesson_id    TEXT        NOT NULL REFERENCES lessons (id) ON DELETE CASCADE,
    title        TEXT        NOT NULL,
    pass_score   INTEGER     NOT NULL DEFAULT 70,
    -- 0 = tanpa batas percobaan.
    max_attempts INTEGER     NOT NULL DEFAULT 1,
    -- Menit; 0 = tanpa batas waktu.
    time_limit   INTEGER     NOT NULL DEFAULT 0,
    active       BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT lesson_quizzes_pass_score_check CHECK (pass_score BETWEEN 0 AND 100),
    CONSTRAINT lesson_quizzes_max_attempts_check CHECK (max_attempts >= 0),
    CONSTRAINT lesson_quizzes_time_limit_check CHECK (time_limit >= 0)
);

CREATE INDEX IF NOT EXISTS lesson_quizzes_lesson_idx
    ON lesson_quizzes (lesson_id, active);

CREATE TABLE IF NOT EXISTS lesson_quiz_questions (
    quiz_id     UUID    NOT NULL REFERENCES lesson_quizzes (id) ON DELETE CASCADE,
    question_id TEXT    REFERENCES questions (id) ON DELETE SET NULL,
    position    INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (quiz_id, position)
);

CREATE TABLE IF NOT EXISTS quiz_attempts (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    quiz_id     UUID        NOT NULL REFERENCES lesson_quizzes (id) ON DELETE CASCADE,
    user_id     UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    score       INTEGER     NOT NULL DEFAULT 0,
    correct     INTEGER     NOT NULL DEFAULT 0,
    total       INTEGER     NOT NULL DEFAULT 0,
    passed      BOOLEAN     NOT NULL DEFAULT FALSE,
    -- [{questionId, selectedIndex}] — jawaban peserta untuk audit & rekap.
    answers     JSONB       NOT NULL DEFAULT '[]'::jsonb,
    started_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at TIMESTAMPTZ,
    CONSTRAINT quiz_attempts_score_check CHECK (score BETWEEN 0 AND 100),
    CONSTRAINT quiz_attempts_counts_check CHECK (correct >= 0 AND total >= 0)
);

-- Menampilkan riwayat percobaan peserta per kuis.
CREATE INDEX IF NOT EXISTS quiz_attempts_user_idx
    ON quiz_attempts (quiz_id, user_id, started_at DESC);

-- Menghitung percobaan yang sudah dikumpulkan (dipakai batas max_attempts).
CREATE INDEX IF NOT EXISTS quiz_attempts_finished_idx
    ON quiz_attempts (quiz_id, user_id)
 WHERE finished_at IS NOT NULL;

/* ------------------------------- sertifikat -------------------------------- */

CREATE TABLE IF NOT EXISTS certificates (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    number     TEXT        NOT NULL UNIQUE,
    user_id    UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    path_id    TEXT        REFERENCES learning_paths (id) ON DELETE SET NULL,
    cohort_id  UUID,
    issued_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at TIMESTAMPTZ,
    -- nama peserta, instansi, periode (dipakai PDF & laporan kelas).
    metadata   JSONB       NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX IF NOT EXISTS certificates_user_idx
    ON certificates (user_id, issued_at DESC);

-- Penomoran sertifikat per tahun; baris dikunci saat nomor berikutnya diambil.
CREATE TABLE IF NOT EXISTS certificate_counters (
    year        INTEGER     PRIMARY KEY,
    last_number INTEGER     NOT NULL DEFAULT 0,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT certificate_counters_last_number_check CHECK (last_number >= 0)
);

/* ---------------------- dimensi baru pada tabel lama ----------------------- */

ALTER TABLE course_modules
    ADD COLUMN IF NOT EXISTS sector_key TEXT NOT NULL DEFAULT 'umum',
    ADD COLUMN IF NOT EXISTS test_type  TEXT NOT NULL DEFAULT 'tkd';

-- Materi contoh yang boleh ditonton tanpa akses (katalog).
ALTER TABLE lessons
    ADD COLUMN IF NOT EXISTS is_preview BOOLEAN NOT NULL DEFAULT FALSE;

