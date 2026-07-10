ALTER TABLE workspace_billing ADD COLUMN IF NOT EXISTS pending_plan text;
ALTER TABLE workspace_billing ADD COLUMN IF NOT EXISTS pending_billing_interval text;
ALTER TABLE workspace_billing ADD COLUMN IF NOT EXISTS pending_change_at timestamptz;
ALTER TABLE workspace_billing ADD COLUMN IF NOT EXISTS cancel_at_period_end boolean NOT NULL DEFAULT false;
