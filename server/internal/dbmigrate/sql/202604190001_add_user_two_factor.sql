ALTER TABLE users
    ADD COLUMN IF NOT EXISTS totp_secret_encrypted TEXT,
    ADD COLUMN IF NOT EXISTS totp_verified BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS recovery_codes_encrypted TEXT;
