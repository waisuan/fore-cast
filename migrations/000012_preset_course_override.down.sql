ALTER TABLE booking_presets
    DROP COLUMN IF EXISTS override_course,
    DROP COLUMN IF EXISTS override_until;
