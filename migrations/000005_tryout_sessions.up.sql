-- 000005_tryout_sessions
-- Mesin tryout. Bagian ini adalah inti optimasi konkurensi:
--
--  * `tryout_sessions` menyimpan snapshot susunan soal + tenggat waktu yang
--    dihitung di server (server-authoritative). Klien tidak bisa memalsukan
--    waktu; sisa waktu selalu dihitung dari `subtest_deadlines`.
--  * Partial unique index `tryout_sessions_active_uidx` menjamin satu peserta
--    hanya punya SATU sesi aktif per tryout, sehingga permintaan "mulai sesi"
--    yang datang bersamaan (double click / retry) tidak menghasilkan sesi ganda.
--  * `tryout_answers` punya UNIQUE (session_id, question_id) sehingga autosave
--    bisa memakai INSERT ... ON CONFLICT DO UPDATE (satu statement, aman untuk
--    ratusan penulisan per detik saat peserta mengerjakan serentak).

CREATE TABLE IF NOT EXISTS tryout_sessions (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id           UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    tryout_id         TEXT        NOT NULL REFERENCES tryout_packages (id) ON DELETE CASCADE,
    status            TEXT        NOT NULL DEFAULT 'active',
    current_subtest   INTEGER     NOT NULL DEFAULT 0,
    -- [[question_id, ...] per subtes] — snapshot agar bank soal boleh berubah
    -- tanpa mengacaukan sesi yang sedang berjalan.
    question_order    JSONB       NOT NULL DEFAULT '[]'::jsonb,
    -- ["2026-10-23T10:00:00Z", ...] tenggat per subtes.
    subtest_deadlines JSONB       NOT NULL DEFAULT '[]'::jsonb,
    started_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at        TIMESTAMPTZ NOT NULL,
    submitted_at      TIMESTAMPTZ,
    attempt_id        UUID,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT tryout_sessions_status_check CHECK (status IN ('active', 'submitted', 'expired'))
);

CREATE UNIQUE INDEX IF NOT EXISTS tryout_sessions_active_uidx
    ON tryout_sessions (user_id, tryout_id) WHERE status = 'active';
CREATE INDEX IF NOT EXISTS tryout_sessions_user_status_idx
    ON tryout_sessions (user_id, status, created_at DESC);
-- Sesi yang kedaluwarsa perlu disapu berkala.
CREATE INDEX IF NOT EXISTS tryout_sessions_expires_idx ON tryout_sessions (expires_at) WHERE status = 'active';

CREATE TABLE IF NOT EXISTS tryout_answers (
    session_id     UUID        NOT NULL REFERENCES tryout_sessions (id) ON DELETE CASCADE,
    question_id    TEXT        NOT NULL,
    subtest_index  INTEGER     NOT NULL DEFAULT 0,
    selected_index INTEGER,
    flagged        BOOLEAN     NOT NULL DEFAULT false,
    answered_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (session_id, question_id)
);

-- Membaca seluruh jawaban satu sesi saat submit adalah operasi terpanas.
CREATE INDEX IF NOT EXISTS tryout_answers_session_subtest_idx ON tryout_answers (session_id, subtest_index);

CREATE TABLE IF NOT EXISTS attempts (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id          UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    tryout_id        TEXT        NOT NULL REFERENCES tryout_packages (id) ON DELETE CASCADE,
    title            TEXT        NOT NULL DEFAULT '',
    score            INTEGER     NOT NULL DEFAULT 0,
    correct          INTEGER     NOT NULL DEFAULT 0,
    total            INTEGER     NOT NULL DEFAULT 0,
    status           TEXT        NOT NULL DEFAULT 'perlu perbaikan',
    duration_seconds INTEGER     NOT NULL DEFAULT 0,
    taken_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT attempts_score_check CHECK (score BETWEEN 0 AND 100)
);

CREATE INDEX IF NOT EXISTS attempts_user_taken_idx ON attempts (user_id, taken_at DESC);
-- Leaderboard: ambil skor tertinggi per tryout, tie-break waktu tercepat.
CREATE INDEX IF NOT EXISTS attempts_leaderboard_idx ON attempts (tryout_id, score DESC, taken_at ASC);
CREATE INDEX IF NOT EXISTS attempts_tryout_user_idx ON attempts (tryout_id, user_id);

CREATE TABLE IF NOT EXISTS attempt_categories (
    id          BIGSERIAL PRIMARY KEY,
    attempt_id  UUID        NOT NULL REFERENCES attempts (id) ON DELETE CASCADE,
    name        TEXT        NOT NULL,
    score       INTEGER     NOT NULL DEFAULT 0,
    correct     INTEGER     NOT NULL DEFAULT 0,
    total       INTEGER     NOT NULL DEFAULT 0,
    sort_order  INTEGER     NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS attempt_categories_attempt_idx ON attempt_categories (attempt_id, sort_order);
