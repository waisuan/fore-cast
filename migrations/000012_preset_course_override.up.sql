ALTER TABLE booking_presets
    ADD COLUMN IF NOT EXISTS override_course TEXT,
    ADD COLUMN IF NOT EXISTS override_until  TIMESTAMPTZ;
