-- Human-readable run log for each attempt (JSON array of {at, text}),
-- shown in History so users can see exactly what happened.
ALTER TABLE booking_attempts
    ADD COLUMN IF NOT EXISTS steps JSONB NOT NULL DEFAULT '[]';
