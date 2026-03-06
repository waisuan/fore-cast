CREATE TABLE IF NOT EXISTS booking_presets (
    id              SERIAL PRIMARY KEY,
    user_name       TEXT NOT NULL UNIQUE,
    password_enc    TEXT NOT NULL,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    course          TEXT,
    cutoff          TEXT NOT NULL DEFAULT '8:15',
    retry_interval  INT NOT NULL DEFAULT 1,
    timeout         TEXT NOT NULL DEFAULT '10m',
    ntfy_topic      TEXT,
    enabled         BOOLEAN NOT NULL DEFAULT FALSE
);
