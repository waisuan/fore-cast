-- Per-preset Malaysia-time clock when the scheduler may start club API
-- calls (login / slots / book). Default matches the previous process-wide
-- wait of 21:59. Cron should still fire a few minutes earlier.
ALTER TABLE booking_presets
    ADD COLUMN IF NOT EXISTS booking_open TEXT NOT NULL DEFAULT '21:59';
