-- 000006_learning
-- Kelas live, rekaman, pengingat peserta, dan log broadcast admin.

CREATE TABLE IF NOT EXISTS live_sessions (
    id               TEXT PRIMARY KEY,
    title            TEXT        NOT NULL,
    topic            TEXT        NOT NULL DEFAULT '',
    mentor           TEXT        NOT NULL DEFAULT '',
    mentor_role      TEXT        NOT NULL DEFAULT '',
    starts_at        TIMESTAMPTZ NOT NULL,
    duration_minutes INTEGER     NOT NULL DEFAULT 60,
    status           TEXT        NOT NULL DEFAULT 'upcoming',
    platform         TEXT        NOT NULL DEFAULT 'Ruang live BUMN Academy',
    note             TEXT,
    meeting_url      TEXT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT live_sessions_status_check CHECK (status IN ('live', 'upcoming', 'ended'))
);

CREATE INDEX IF NOT EXISTS live_sessions_starts_idx ON live_sessions (starts_at DESC);

CREATE TABLE IF NOT EXISTS recordings (
    id               TEXT PRIMARY KEY,
    title            TEXT        NOT NULL,
    description      TEXT        NOT NULL DEFAULT '',
    topic            TEXT        NOT NULL DEFAULT '',
    category         TEXT        NOT NULL DEFAULT 'TKD',
    mentor           TEXT        NOT NULL DEFAULT '',
    duration_minutes INTEGER     NOT NULL DEFAULT 0,
    published_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    accent           TEXT        NOT NULL DEFAULT 'from-brand/80 to-navy',
    video_url        TEXT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS recordings_published_idx ON recordings (published_at DESC);
CREATE INDEX IF NOT EXISTS recordings_category_idx ON recordings (category, published_at DESC);

CREATE TABLE IF NOT EXISTS session_reminders (
    user_id         UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    live_session_id TEXT        NOT NULL REFERENCES live_sessions (id) ON DELETE CASCADE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, live_session_id)
);

CREATE TABLE IF NOT EXISTS broadcast_logs (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_by   UUID        REFERENCES users (id) ON DELETE SET NULL,
    audience     TEXT        NOT NULL DEFAULT 'all',
    message      TEXT        NOT NULL,
    recipients   INTEGER     NOT NULL DEFAULT 0,
    status       TEXT        NOT NULL DEFAULT 'queued',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    sent_at      TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS broadcast_logs_created_idx ON broadcast_logs (created_at DESC);
