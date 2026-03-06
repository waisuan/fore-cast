ALTER TABLE booking_presets
    ADD COLUMN IF NOT EXISTS last_run_status  TEXT NOT NULL DEFAULT 'idle',
    ADD COLUMN IF NOT EXISTS last_run_message TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS last_run_at      TIMESTAMPTZ;
