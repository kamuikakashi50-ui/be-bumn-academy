-- 000012_question_taxonomy
-- Dimensi taksonomi soal: sektor/klaster BUMN, jenis tes, keluarga jabatan,
-- grup varian, dan batas masa simpan.
--
-- Permintaan klien: "Bank soal harus sesuai kategori BUMN-nya masing-masing,
-- termasuk tes psikologi di dalamnya". Sebelum ini soal hanya punya `bank_key`,
-- sehingga soal Tes Kemampuan Bidang (TKB) untuk sektor keuangan dan manufaktur
-- tidak bisa dibedakan, dan panel bank soal tidak bisa menyaringnya.
--
-- Semua kolom baru diberi nilai bawaan supaya baris lama tetap valid tanpa
-- diisi ulang, lalu `test_type` di-backfill dari kategori bank soal mengikuti
-- `bank.TestTypeForBank` (akhlaq → akhlaq, wawasan → twk, reading/structure →
-- bing, kategori bidang → tkb, sisanya tetap tkd).

ALTER TABLE questions
    ADD COLUMN IF NOT EXISTS sector_key      TEXT NOT NULL DEFAULT 'umum',
    ADD COLUMN IF NOT EXISTS company_key     TEXT,
    ADD COLUMN IF NOT EXISTS test_type       TEXT NOT NULL DEFAULT 'tkd',
    ADD COLUMN IF NOT EXISTS position_family TEXT NOT NULL DEFAULT 'umum',
    ADD COLUMN IF NOT EXISTS variant_group   TEXT,
    ADD COLUMN IF NOT EXISTS retention_until TIMESTAMPTZ;

UPDATE questions SET test_type = 'akhlaq' WHERE bank_key = 'akhlaq';
UPDATE questions SET test_type = 'twk' WHERE bank_key = 'wawasan';
UPDATE questions SET test_type = 'bing' WHERE bank_key IN ('reading', 'structure');
UPDATE questions SET test_type = 'psikotes' WHERE bank_key = 'psikotes';
UPDATE questions
   SET test_type = 'tkb'
 WHERE bank_key IN (
       'akuntansi', 'risiko_kepatuhan', 'teknis_produksi', 'k3',
       'teknis_operasi', 'hse', 'it_dasar', 'layanan', 'manajemen_proyek'
   );

-- Panel bank soal menyaring per sektor/jenis tes/keluarga jabatan → index kunci.
CREATE INDEX IF NOT EXISTS questions_taxonomy_idx
    ON questions (sector_key, test_type, position_family, bank_key);

-- Penyusunan sesi memakai satu soal per grup varian.
CREATE INDEX IF NOT EXISTS questions_variant_group_idx
    ON questions (variant_group)
 WHERE variant_group IS NOT NULL;

-- Pemeriksaan masa simpan soal (perjanjian kerahasiaan).
CREATE INDEX IF NOT EXISTS questions_retention_idx
    ON questions (retention_until)
 WHERE retention_until IS NOT NULL;
