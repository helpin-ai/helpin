-- CRM email account lifecycle and reconnect support

ALTER TABLE crm_email_accounts
    ADD COLUMN IF NOT EXISTS normalized_email_address VARCHAR(255);

ALTER TABLE crm_email_accounts
    ADD COLUMN IF NOT EXISTS last_history_id VARCHAR(255);

ALTER TABLE crm_email_accounts
    ADD COLUMN IF NOT EXISTS status VARCHAR(32) NOT NULL DEFAULT 'connected';

ALTER TABLE crm_email_accounts
    ADD COLUMN IF NOT EXISTS disconnected_at TIMESTAMPTZ;

UPDATE crm_email_accounts
SET status = CASE
    WHEN oauth_state IS NOT NULL
        OR LOWER(TRIM(COALESCE(email_address, ''))) = 'pending@oauth.local'
        OR COALESCE(sync_state->>'status', '') = 'pending_oauth'
        THEN 'pending_oauth'
    WHEN is_active THEN 'connected'
    ELSE 'disconnected'
END
WHERE COALESCE(status, '') = '';

UPDATE crm_email_accounts
SET normalized_email_address = CASE
    WHEN status = 'pending_oauth'
        OR TRIM(COALESCE(email_address, '')) = ''
        OR LOWER(TRIM(COALESCE(email_address, ''))) = 'pending@oauth.local'
        THEN NULL
    ELSE LOWER(TRIM(email_address))
END
WHERE normalized_email_address IS NULL
    OR normalized_email_address = '';

UPDATE crm_email_accounts
SET last_history_id = NULLIF(COALESCE(last_history_id, sync_state->>'last_history_id'), '')
WHERE last_history_id IS NULL;

UPDATE crm_email_accounts
SET disconnected_at = COALESCE(disconnected_at, updated_at)
WHERE status = 'disconnected' AND disconnected_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_crm_email_accounts_workspace_provider_email
    ON crm_email_accounts(workspace_id, provider, normalized_email_address);

CREATE UNIQUE INDEX IF NOT EXISTS idx_crm_email_accounts_workspace_provider_normalized_email_unique
    ON crm_email_accounts(workspace_id, provider, normalized_email_address)
    WHERE normalized_email_address IS NOT NULL;
