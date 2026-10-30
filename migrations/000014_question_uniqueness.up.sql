-- 000014_question_uniqueness
-- Sidik jari soal dan jejak audit akses bank soal.
--
-- Latar belakang (masukan klien): "susah kalau mencari soal yang sama karena
-- sudah ada pakta-nya" — soal terikat perjanjian kerahasiaan, sehingga soal yang
-- sudah dipakai tidak boleh muncul lagi secara identik. Mencari soal kembar
-- secara manual di bank soal berisi ribuan baris tidak realistis, jadi sidik
-- jari teksnya disimpan terpisah dari tabel soal.
--
-- Sidik jari TIDAK di-backfill di sini: normalisasi teks hanya diimplementasikan
-- sekali di Go (internal/pkg/fingerprint) supaya hash yang tersimpan selalu
-- identik dengan yang dihitung aplikasi. Baris yang belum ada sidik jarinya
-- diisi saat soal dibuat/diubah/diimpor, atau saat laporan duplikat dibuka.
--
-- Grup varian (`questions.variant_group`, migrasi 000012) melengkapi ini: soal
-- satu keluarga dengan angka berbeda sah berdampingan, dan sesi tryout hanya
-- memakai satu soal per grup.

CREATE TABLE IF NOT EXISTS question_fingerprints (
    question_id   TEXT        PRIMARY KEY REFERENCES questions (id) ON DELETE CASCADE,
    fingerprint   TEXT        NOT NULL,
    template_hash TEXT        NOT NULL,
    tokens        JSONB       NOT NULL DEFAULT '[]'::jsonb,
    computed_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Pencarian kembar persis & satu keluarga.
CREATE INDEX IF NOT EXISTS question_fingerprints_hash_idx
    ON question_fingerprints (fingerprint);
CREATE INDEX IF NOT EXISTS question_fingerprints_template_idx
    ON question_fingerprints (template_hash);

CREATE TABLE IF NOT EXISTS question_access_logs (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    question_id TEXT        NOT NULL,
    actor_id    UUID        REFERENCES users (id) ON DELETE SET NULL,
    action      TEXT        NOT NULL,
    detail      TEXT        NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Jejak audit dibaca per soal dan per pelaku.
CREATE INDEX IF NOT EXISTS question_access_logs_question_idx
    ON question_access_logs (question_id, created_at DESC);
CREATE INDEX IF NOT EXISTS question_access_logs_actor_idx
    ON question_access_logs (actor_id, created_at DESC);
