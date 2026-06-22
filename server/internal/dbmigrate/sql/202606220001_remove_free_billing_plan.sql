-- Remove the customer-facing Free billing plan.
-- Existing Free workspaces become locked Growth trial-expired workspaces so
-- they can reactivate through checkout without losing data.

UPDATE workspace_billing
SET
  plan = 'growth',
  status = 'trial_expired',
  billing_interval = 'monthly',
  included_credits = 25000,
  on_demand_enabled = false,
  on_demand_blocks_invoiced = 0,
  updated_at = now()
WHERE plan = 'free';

UPDATE workspace_billing
SET
  pending_plan = NULL,
  pending_billing_interval = NULL,
  updated_at = now()
WHERE pending_plan = 'free';

ALTER TABLE workspace_billing ALTER COLUMN plan SET DEFAULT 'growth';
ALTER TABLE workspace_billing ALTER COLUMN status SET DEFAULT 'trialing';
ALTER TABLE workspace_billing ALTER COLUMN included_credits SET DEFAULT 25000;
