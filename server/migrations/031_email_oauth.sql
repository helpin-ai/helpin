ALTER TABLE crm_email_accounts ADD COLUMN IF NOT EXISTS oauth_state TEXT;
ALTER TABLE crm_email_accounts ADD COLUMN IF NOT EXISTS token_expires_at TIMESTAMPTZ;
