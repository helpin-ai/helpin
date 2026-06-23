ALTER TABLE organization_billing
  ADD COLUMN IF NOT EXISTS founder_plan_enabled boolean NOT NULL DEFAULT false;

INSERT INTO organization_billing (
  id,
  organization_id,
  founder_plan_enabled,
  created_at,
  updated_at
)
SELECT
  gen_random_uuid(),
  organizations.id,
  true,
  NOW(),
  NOW()
FROM organizations
ON CONFLICT (organization_id) DO UPDATE
SET
  founder_plan_enabled = true,
  updated_at = NOW();

UPDATE workspace_billing AS wb
SET
  plan = 'founder',
  status = 'active',
  billing_interval = 'monthly',
  included_credits = 100000,
  credits_used = LEAST(wb.credits_used, 100000),
  on_demand_enabled = false,
  on_demand_blocks_invoiced = 0,
  current_period_start = CASE
    WHEN wb.current_period_start IS NULL OR wb.current_period_start > NOW() THEN NOW()
    ELSE wb.current_period_start
  END,
  current_period_end = CASE
    WHEN wb.current_period_end IS NULL OR wb.current_period_end <= NOW() THEN NOW() + INTERVAL '1 month'
    ELSE wb.current_period_end
  END,
  trial_ends_at = NULL,
  pending_plan = NULL,
  pending_billing_interval = NULL,
  pending_change_at = NULL,
  cancel_at_period_end = false,
  canceled_at = NULL,
  billing_notice_type = NULL,
  billing_notice_message = NULL,
  billing_notice_at = NULL,
  payment_failed_at = NULL,
  trial_will_end_at = NULL,
  stripe_subscription_id = NULL,
  stripe_price_id = NULL,
  updated_at = NOW()
FROM workspaces AS w
JOIN organization_billing AS ob ON ob.organization_id = w.organization_id
WHERE
  wb.workspace_id = w.id
  AND ob.founder_plan_enabled = true;

INSERT INTO workspace_billing (
  id,
  workspace_id,
  plan,
  status,
  billing_interval,
  included_credits,
  credits_used,
  on_demand_enabled,
  on_demand_blocks_invoiced,
  current_period_start,
  current_period_end,
  cancel_at_period_end,
  created_at,
  updated_at
)
SELECT
  gen_random_uuid(),
  w.id,
  'founder',
  'active',
  'monthly',
  100000,
  0,
  false,
  0,
  NOW(),
  NOW() + INTERVAL '1 month',
  false,
  NOW(),
  NOW()
FROM workspaces AS w
JOIN organization_billing AS ob ON ob.organization_id = w.organization_id
LEFT JOIN workspace_billing AS wb ON wb.workspace_id = w.id
WHERE
  ob.founder_plan_enabled = true
  AND wb.id IS NULL;
