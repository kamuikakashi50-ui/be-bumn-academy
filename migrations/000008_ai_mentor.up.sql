-- 000008_ai_mentor
-- Mentor AI ("MinSmart") untuk ulasan hasil tryout.
--
-- Tiga hal yang disimpan:
--   1. attempt_analyses   — analisis otomatis satu hasil tryout (di-cache agar
--                           tidak dihitung ulang setiap halaman dibuka),
--   2. ai_conversations   — percakapan peserta dengan mentor untuk satu hasil,
--      ai_messages        — pesan-pesannya (user/assistant),
--   3. ai_usage_counters  — kuota harian per peserta (kontrol biaya).

CREATE TABLE IF NOT EXISTS attempt_analyses (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    attempt_id       UUID        NOT NULL REFERENCES attempts (id) ON DELETE CASCADE,
    user_id          UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    provider         TEXT        NOT NULL DEFAULT 'stub',
    model            TEXT        NOT NULL DEFAULT '',
    headline         TEXT        NOT NULL DEFAULT '',
    summary          TEXT        NOT NULL DEFAULT '',
    readiness        TEXT        NOT NULL DEFAULT 'perlu perbaikan',
    readiness_score  INTEGER     NOT NULL DEFAULT 0,
    strengths        JSONB       NOT NULL DEFAULT '[]'::jsonb,
    weaknesses       JSONB       NOT NULL DEFAULT '[]'::jsonb,
    focus_areas      JSONB       NOT NULL DEFAULT '[]'::jsonb,
    study_plan       JSONB       NOT NULL DEFAULT '[]'::jsonb,
    question_notes   JSONB       NOT NULL DEFAULT '[]'::jsonb,
    generated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT attempt_analyses_readiness_check CHECK (readiness IN ('siap', 'hampir siap', 'perlu perbaikan')),
    CONSTRAINT attempt_analyses_score_check CHECK (readiness_score BETWEEN 0 AND 100)
);

-- Satu analisis per hasil tryout.
CREATE UNIQUE INDEX IF NOT EXISTS attempt_analyses_attempt_uidx ON attempt_analyses (attempt_id);
CREATE INDEX IF NOT EXISTS attempt_analyses_user_idx ON attempt_analyses (user_id, generated_at DESC);

CREATE TABLE IF NOT EXISTS ai_conversations (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    attempt_id      UUID        NOT NULL REFERENCES attempts (id) ON DELETE CASCADE,
    user_id         UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    title           TEXT        NOT NULL DEFAULT 'Ulasan Mentor AI',
    message_count   INTEGER     NOT NULL DEFAULT 0,
    last_message_at TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS ai_conversations_attempt_uidx ON ai_conversations (attempt_id);
CREATE INDEX IF NOT EXISTS ai_conversations_user_idx ON ai_conversations (user_id, updated_at DESC);

CREATE TABLE IF NOT EXISTS ai_messages (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id   UUID        NOT NULL REFERENCES ai_conversations (id) ON DELETE CASCADE,
    user_id           UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    role              TEXT        NOT NULL,
    content           TEXT        NOT NULL,
    scope             TEXT        NOT NULL DEFAULT 'attempt',
    question_id       TEXT,
    question_number   INTEGER,
    subtest_name      TEXT,
    provider          TEXT        NOT NULL DEFAULT '',
    model             TEXT        NOT NULL DEFAULT '',
    prompt_tokens     INTEGER     NOT NULL DEFAULT 0,
    completion_tokens INTEGER     NOT NULL DEFAULT 0,
    latency_ms        INTEGER     NOT NULL DEFAULT 0,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT ai_messages_role_check CHECK (role IN ('user', 'assistant')),
    CONSTRAINT ai_messages_scope_check CHECK (scope IN ('attempt', 'question'))
);

-- Membaca riwayat percakapan selalu terurut waktu per percakapan.
CREATE INDEX IF NOT EXISTS ai_messages_conversation_idx ON ai_messages (conversation_id, created_at);
CREATE INDEX IF NOT EXISTS ai_messages_user_idx ON ai_messages (user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS ai_usage_counters (
    user_id           UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    usage_date        DATE        NOT NULL,
    requests          INTEGER     NOT NULL DEFAULT 0,
    prompt_tokens     INTEGER     NOT NULL DEFAULT 0,
    completion_tokens INTEGER     NOT NULL DEFAULT 0,
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, usage_date)
);
