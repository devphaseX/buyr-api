ALTER TABLE admin_users
ADD COLUMN IF NOT EXISTS admin_level VARCHAR(50) NOT NULL;
