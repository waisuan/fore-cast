-- Run-log steps of the current (or last) scheduler run, appended while the run
-- is in progress so Home can show them live. JSON array of {at, text, course}.
ALTER TABLE booking_presets
    ADD COLUMN IF NOT EXISTS live_steps JSONB NOT NULL DEFAULT '[]';
