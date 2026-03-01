ALTER TABLE booking_presets
    DROP COLUMN IF EXISTS last_run_status,
    DROP COLUMN IF EXISTS last_run_message,
    DROP COLUMN IF EXISTS last_run_at;
