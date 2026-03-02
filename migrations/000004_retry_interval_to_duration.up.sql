-- Migrate retry_interval from INT (seconds) to TEXT (duration string e.g. "1s", "500ms")
ALTER TABLE booking_presets ADD COLUMN retry_interval_new TEXT;
UPDATE booking_presets SET retry_interval_new = retry_interval::text || 's';
ALTER TABLE booking_presets DROP COLUMN retry_interval;
ALTER TABLE booking_presets RENAME COLUMN retry_interval_new TO retry_interval;
ALTER TABLE booking_presets ALTER COLUMN retry_interval SET NOT NULL;
ALTER TABLE booking_presets ALTER COLUMN retry_interval SET DEFAULT '1s';
