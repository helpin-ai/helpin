ALTER TABLE workspace_billing ADD COLUMN IF NOT EXISTS canceled_at timestamptz;
