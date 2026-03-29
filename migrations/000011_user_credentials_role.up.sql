-- ADMIN or NON_ADMIN per login identity. Promote first operator via SQL:
-- UPDATE user_credentials SET role = 'ADMIN' WHERE user_name = '...';
ALTER TABLE user_credentials
ADD COLUMN role TEXT NOT NULL DEFAULT 'NON_ADMIN'
CHECK (role IN ('ADMIN', 'NON_ADMIN'));
