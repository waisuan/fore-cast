-- Restore password column to preset
ALTER TABLE booking_presets ADD COLUMN password_enc TEXT;

-- Copy back from user_credentials
UPDATE booking_presets p SET password_enc = c.password_enc
FROM user_credentials c WHERE p.user_name = c.user_name;

ALTER TABLE booking_presets ALTER COLUMN password_enc SET NOT NULL;

-- Drop FK
ALTER TABLE booking_presets DROP CONSTRAINT IF EXISTS fk_preset_user;
