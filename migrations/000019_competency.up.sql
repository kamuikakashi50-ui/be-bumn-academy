-- 000019_competency
-- Pemetaan skill & kompetensi (Fase 3 langkah 1, masukan #2): kamus kompetensi,
-- deskriptor level, standar jabatan, penilaian per peserta (multi-sumber), dan
-- pemetaan kategori bank soal → kompetensi.
--
-- Catatan desain (Fase 3 §2.4, §2.5, §4.3):
--   * Level dari `tryout` adalah **proksi latihan**, bukan asesmen formal. Karena
--     itu `user_competencies.source` wajib diisi dan selalu ditampilkan di UI,
--     sehingga HR tidak salah menafsirkan proksi sebagai asesmen formal.
--   * Tiga sumber penilaian **tidak dicampur**: satu baris per (user, kompetensi,
--     sumber). Gap/level tertinggi dihitung di service dari gabungan sumber.
--   * `bank_competency_map` memetakan kategori bank soal Fase 0 (mis. `numerik`)
--     ke kode kompetensi, sehingga data latihan yang sudah ada langsung
--     menghasilkan level tanpa menunggu instrumen berlisensi.
--   * Kompetensi tidak dihapus fisik bila sudah dirujuk penilaian; cukup
--     `active = false` (pola soft delete yang sama dengan soal/produk).

CREATE TABLE IF NOT EXISTS competencies (
    code       TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    cluster    TEXT NOT NULL DEFAULT 'teknis',
    definition TEXT NOT NULL DEFAULT '',
    active     BOOLEAN NOT NULL DEFAULT TRUE
);

CREATE TABLE IF NOT EXISTS competency_levels (
    competency_code TEXT NOT NULL REFERENCES competencies (code) ON DELETE CASCADE,
    level           INTEGER NOT NULL,
    descriptor      TEXT NOT NULL,
    PRIMARY KEY (competency_code, level),
    CONSTRAINT competency_levels_range_check CHECK (level BETWEEN 1 AND 5)
);

CREATE TABLE IF NOT EXISTS position_requirements (
    position_family TEXT NOT NULL,
    sector_key      TEXT NOT NULL DEFAULT 'umum',
    competency_code TEXT NOT NULL REFERENCES competencies (code) ON DELETE CASCADE,
    min_level       INTEGER NOT NULL,
    weight          NUMERIC(5,2) NOT NULL DEFAULT 1,
    PRIMARY KEY (position_family, sector_key, competency_code),
    CONSTRAINT position_requirements_level_check CHECK (min_level BETWEEN 1 AND 5)
);

CREATE TABLE IF NOT EXISTS user_competencies (
    user_id         UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    competency_code TEXT NOT NULL REFERENCES competencies (code) ON DELETE CASCADE,
    level           INTEGER NOT NULL,
    source          TEXT NOT NULL,
    evidence        TEXT NOT NULL DEFAULT '',
    assessed_by     UUID REFERENCES users (id) ON DELETE SET NULL,
    assessed_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, competency_code, source),
    CONSTRAINT user_competencies_level_check CHECK (level BETWEEN 1 AND 5),
    CONSTRAINT user_competencies_source_check
        CHECK (source IN ('tryout','self','asesor','manager','psikotes'))
);

CREATE TABLE IF NOT EXISTS bank_competency_map (
    bank_key        TEXT PRIMARY KEY,
    competency_code TEXT NOT NULL REFERENCES competencies (code) ON DELETE CASCADE,
    weight          NUMERIC(5,2) NOT NULL DEFAULT 1
);

CREATE INDEX IF NOT EXISTS user_competencies_user_idx ON user_competencies (user_id);
CREATE INDEX IF NOT EXISTS user_competencies_code_idx ON user_competencies (competency_code);
CREATE INDEX IF NOT EXISTS position_requirements_family_idx
    ON position_requirements (position_family, sector_key);
