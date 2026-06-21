ALTER TABLE workspace_billing ADD COLUMN IF NOT EXISTS billing_notice_type text;
ALTER TABLE workspace_billing ADD COLUMN IF NOT EXISTS billing_notice_message text;
ALTER TABLE workspace_billing ADD COLUMN IF NOT EXISTS billing_notice_at timestamptz;
ALTER TABLE workspace_billing ADD COLUMN IF NOT EXISTS payment_failed_at timestamptz;
ALTER TABLE workspace_billing ADD COLUMN IF NOT EXISTS trial_will_end_at timestamptz;

