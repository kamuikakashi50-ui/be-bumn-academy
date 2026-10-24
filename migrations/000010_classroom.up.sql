-- Ruang kelas digital: modul, pelajaran (video/e-book), dan progres menonton.

CREATE TABLE IF NOT EXISTS course_modules (
    id           TEXT        PRIMARY KEY,
    product_slug TEXT        NOT NULL,
    title        TEXT        NOT NULL,
    description  TEXT        NOT NULL DEFAULT '',
    sort_order   INTEGER     NOT NULL DEFAULT 0,
    active       BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS course_modules_product_idx
    ON course_modules (product_slug, sort_order);

CREATE TABLE IF NOT EXISTS lessons (
    id               TEXT        PRIMARY KEY,
    module_id        TEXT        NOT NULL REFERENCES course_modules (id) ON DELETE CASCADE,
    product_slug     TEXT        NOT NULL,
    title            TEXT        NOT NULL,
    summary          TEXT        NOT NULL DEFAULT '',
    kind             TEXT        NOT NULL DEFAULT 'video' CHECK (kind IN ('video', 'ebook')),
    video_url        TEXT,
    ebook_url        TEXT,
    poster_url       TEXT,
    content          TEXT        NOT NULL DEFAULT '',
    duration_seconds INTEGER     NOT NULL DEFAULT 0,
    sort_order       INTEGER     NOT NULL DEFAULT 0,
    active           BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS lessons_module_idx ON lessons (module_id, sort_order);
CREATE INDEX IF NOT EXISTS lessons_product_idx ON lessons (product_slug);

-- Progres per peserta per pelajaran (satu baris per pasangan).
CREATE TABLE IF NOT EXISTS lesson_progress (
    user_id          UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    lesson_id        TEXT        NOT NULL REFERENCES lessons (id) ON DELETE CASCADE,
    position_seconds INTEGER     NOT NULL DEFAULT 0,
    progress_percent INTEGER     NOT NULL DEFAULT 0,
    completed        BOOLEAN     NOT NULL DEFAULT FALSE,
    last_viewed_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, lesson_id)
);

CREATE INDEX IF NOT EXISTS lesson_progress_user_idx
    ON lesson_progress (user_id, last_viewed_at DESC);
