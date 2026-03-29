CREATE TABLE IF NOT EXISTS user_sessions (
    id          TEXT PRIMARY KEY,
    user_name   TEXT NOT NULL,
    expires_at  TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_user_sessions_expires_at ON user_sessions (expires_at);
