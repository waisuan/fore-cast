CREATE TABLE IF NOT EXISTS booking_attempts (
    id          SERIAL PRIMARY KEY,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    user_name   TEXT NOT NULL,
    course_id   TEXT NOT NULL,
    txn_date    TEXT NOT NULL,
    tee_time    TEXT,
    tee_box     TEXT,
    booking_id  TEXT,
    status      TEXT NOT NULL,
    message     TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_attempts_user_created ON booking_attempts (user_name, created_at DESC);
