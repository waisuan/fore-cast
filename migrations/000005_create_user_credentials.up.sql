CREATE TABLE IF NOT EXISTS user_credentials (
    user_name    TEXT PRIMARY KEY,
    password_enc TEXT NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
