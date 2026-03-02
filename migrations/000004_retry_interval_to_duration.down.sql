-- Revert retry_interval from TEXT to INT (seconds). Sub-second values become 1.
ALTER TABLE booking_presets ADD COLUMN retry_interval_old INT;
UPDATE booking_presets SET retry_interval_old = CASE
  WHEN retry_interval LIKE '%ms' THEN greatest(1, (regexp_replace(retry_interval, 'ms$', ''))::int / 1000)
  ELSE greatest(1, (regexp_replace(retry_interval, 's$', ''))::numeric::int)
END;
ALTER TABLE booking_presets DROP COLUMN retry_interval;
ALTER TABLE booking_presets RENAME COLUMN retry_interval_old TO retry_interval;
ALTER TABLE booking_presets ALTER COLUMN retry_interval SET NOT NULL;
ALTER TABLE booking_presets ALTER COLUMN retry_interval SET DEFAULT 1;
