CREATE TABLE IF NOT EXISTS workspace_billing (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id uuid NOT NULL UNIQUE REFERENCES workspaces(id) ON DELETE CASCADE,
  plan text NOT NULL DEFAULT 'free',
  status text NOT NULL DEFAULT 'active',
  stripe_customer_id text,
  stripe_subscription_id text,
  stripe_price_id text,
  billing_interval text NOT NULL DEFAULT 'monthly',
  included_credits integer NOT NULL DEFAULT 1000,
  credits_used integer NOT NULL DEFAULT 0,
  on_demand_enabled boolean NOT NULL DEFAULT false,
  on_demand_blocks_invoiced integer NOT NULL DEFAULT 0,
  current_period_start timestamptz NOT NULL DEFAULT now(),
  current_period_end timestamptz NOT NULL DEFAULT (now() + interval '1 month'),
  trial_ends_at timestamptz,
  last_stripe_event_id text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_workspace_billing_plan ON workspace_billing(plan);
CREATE INDEX IF NOT EXISTS idx_workspace_billing_status ON workspace_billing(status);
CREATE INDEX IF NOT EXISTS idx_workspace_billing_stripe_customer ON workspace_billing(stripe_customer_id);
CREATE INDEX IF NOT EXISTS idx_workspace_billing_stripe_subscription ON workspace_billing(stripe_subscription_id);

CREATE TABLE IF NOT EXISTS billing_credit_ledger (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  kind text NOT NULL,
  feature_key text NOT NULL DEFAULT '',
  credits integer NOT NULL,
  idempotency_key text NOT NULL UNIQUE,
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_billing_credit_ledger_workspace ON billing_credit_ledger(workspace_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_billing_credit_ledger_kind ON billing_credit_ledger(kind);
CREATE INDEX IF NOT EXISTS idx_billing_credit_ledger_feature ON billing_credit_ledger(feature_key);

CREATE TABLE IF NOT EXISTS stripe_webhook_events (
  id text PRIMARY KEY,
  type text NOT NULL,
  processed boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

INSERT INTO workspace_billing (
  workspace_id,
  plan,
  status,
  billing_interval,
  included_credits,
  credits_used,
  on_demand_enabled,
  current_period_start,
  current_period_end
)
SELECT
  id,
  'free',
  'active',
  'monthly',
  1000,
  0,
  false,
  now(),
  now() + interval '1 month'
FROM workspaces
ON CONFLICT (workspace_id) DO NOTHING;
