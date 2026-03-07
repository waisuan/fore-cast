-- Migrate existing passwords from preset to user_credentials
INSERT INTO user_credentials (user_name, password_enc)
SELECT user_name, password_enc FROM booking_presets
ON CONFLICT (user_name) DO NOTHING;

-- Add FK before dropping column (preset.user_name references user_credentials)
ALTER TABLE booking_presets ADD CONSTRAINT fk_preset_user
    FOREIGN KEY (user_name) REFERENCES user_credentials(user_name);

-- Drop password from preset
ALTER TABLE booking_presets DROP COLUMN password_enc;
