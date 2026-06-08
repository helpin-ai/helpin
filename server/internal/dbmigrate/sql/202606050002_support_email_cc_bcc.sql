ALTER TABLE support_email_logs
ADD COLUMN IF NOT EXISTS cc_emails text[] NOT NULL DEFAULT '{}';

ALTER TABLE support_email_logs
ADD COLUMN IF NOT EXISTS bcc_emails text[] NOT NULL DEFAULT '{}';
