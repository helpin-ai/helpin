ALTER TABLE support_inbound_jobs
 ADD COLUMN IF NOT EXISTS email_log_id uuid,
 ADD COLUMN IF NOT EXISTS recovery_conversation_id uuid,
 ADD COLUMN IF NOT EXISTS recovery_requested_by uuid,
 ADD COLUMN IF NOT EXISTS recovery_requested_at timestamptz;

CREATE INDEX IF NOT EXISTS idx_support_inbound_recovery
 ON support_inbound_jobs (recovery_requested_at DESC)
 WHERE kind = 'email' AND recovery_requested_at IS NOT NULL;
