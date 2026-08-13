CREATE TABLE IF NOT EXISTS billing_ai_usage_periods (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), workspace_id uuid NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
 period_start timestamptz NOT NULL, period_end timestamptz NOT NULL, allowance_microusd bigint NOT NULL,
 used_microusd bigint NOT NULL DEFAULT 0, overage_microusd bigint NOT NULL DEFAULT 0, reserved_microusd bigint NOT NULL DEFAULT 0,
 enforcement_mode text NOT NULL, status text NOT NULL DEFAULT 'open', pricing_version text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_ai_usage_one_open_period ON billing_ai_usage_periods(workspace_id) WHERE status = 'open';
CREATE TABLE IF NOT EXISTS billing_ai_usage_ledger (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), workspace_id uuid NOT NULL REFERENCES workspaces(id), period_id uuid NOT NULL REFERENCES billing_ai_usage_periods(id),
 entry_kind text NOT NULL, feature_key text NOT NULL, category text NOT NULL, model_tier text NOT NULL, provider text NOT NULL,
 canonical_model text NOT NULL, route text NOT NULL, service_tier text NOT NULL, funding_mode text NOT NULL, pricing_version text NOT NULL,
 rate_snapshot jsonb NOT NULL, tool_usage jsonb NOT NULL DEFAULT '[]', input_tokens_total bigint NOT NULL DEFAULT 0,
 uncached_input_tokens bigint NOT NULL DEFAULT 0, cache_read_tokens bigint NOT NULL DEFAULT 0, cache_write_tokens bigint NOT NULL DEFAULT 0,
 output_tokens bigint NOT NULL DEFAULT 0, reasoning_tokens bigint NOT NULL DEFAULT 0, published_charge_microusd bigint NOT NULL DEFAULT 0,
 final_charged_microusd bigint NOT NULL DEFAULT 0, measurement_status text NOT NULL, estimation_method text NOT NULL DEFAULT '',
 source_sample_size integer NOT NULL DEFAULT 0, idempotency_key text NOT NULL UNIQUE, metadata jsonb NOT NULL DEFAULT '{}', created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS billing_ai_usage_reservations (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), workspace_id uuid NOT NULL REFERENCES workspaces(id), period_id uuid NOT NULL REFERENCES billing_ai_usage_periods(id),
 task_nature text NOT NULL, model_tier text NOT NULL, execution_id text NOT NULL, idempotency_key text NOT NULL UNIQUE,
 reserved_microusd bigint NOT NULL, consumed_microusd bigint NOT NULL DEFAULT 0, status text NOT NULL,
 expires_at timestamptz NOT NULL, heartbeat_at timestamptz NOT NULL, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS billing_ai_usage_settlements (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), workspace_id uuid NOT NULL REFERENCES workspaces(id), period_id uuid NOT NULL UNIQUE REFERENCES billing_ai_usage_periods(id),
 exact_overage_microusd bigint NOT NULL, rounded_invoice_cents bigint NOT NULL, rounding_adjustment_microusd bigint NOT NULL,
 stripe_customer_id text, stripe_subscription_id text, stripe_invoice_item_id text, stripe_invoice_id text,
 idempotency_key text NOT NULL UNIQUE, status text NOT NULL, attempt_count integer NOT NULL DEFAULT 0, last_error text,
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS billing_ai_usage_task_estimates (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), task_nature text NOT NULL, model_tier text NOT NULL, funding_mode text NOT NULL,
 pricing_version text NOT NULL, sample_size integer NOT NULL, average_tokens jsonb NOT NULL, average_charge_microusd bigint NOT NULL,
 p50_charge_microusd bigint NOT NULL, p90_charge_microusd bigint NOT NULL, calculated_at timestamptz NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(task_nature, model_tier, funding_mode, pricing_version)
);
CREATE TABLE IF NOT EXISTS billing_ai_usage_pricing_state (
 id boolean PRIMARY KEY DEFAULT true CHECK (id), pricing_version text NOT NULL,
 activated_at timestamptz NOT NULL, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
);
INSERT INTO billing_ai_usage_pricing_state (id, pricing_version, activated_at)
VALUES (true, '2026-08-13', now()) ON CONFLICT (id) DO UPDATE
SET pricing_version = EXCLUDED.pricing_version, activated_at = EXCLUDED.activated_at, updated_at = now();
INSERT INTO billing_ai_usage_periods (
 workspace_id, period_start, period_end, allowance_microusd, enforcement_mode, pricing_version
)
SELECT wb.workspace_id, now(),
 CASE WHEN wb.status = 'trialing' AND wb.trial_ends_at IS NOT NULL THEN wb.trial_ends_at ELSE wb.current_period_end END,
 CASE
  WHEN wb.plan = 'founder' THEN 150000000
  WHEN wb.status = 'trialing' THEN 140000000
  WHEN wb.plan = 'starter' AND wb.billing_interval = 'annual' THEN 79000000
  WHEN wb.plan = 'starter' THEN 99000000
  WHEN wb.plan = 'growth' AND wb.billing_interval = 'annual' THEN 239000000
  ELSE 299000000
 END,
 CASE WHEN wb.plan = 'founder' THEN 'soft'
      WHEN wb.on_demand_enabled THEN 'extra_allowed' ELSE 'enforced' END,
 '2026-08-13'
FROM workspace_billing wb
WHERE NOT EXISTS (SELECT 1 FROM billing_ai_usage_periods p WHERE p.workspace_id = wb.workspace_id AND p.status = 'open');
DO $$ BEGIN
 IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'workspace_billing' AND column_name = 'on_demand_enabled')
 AND NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'workspace_billing' AND column_name = 'extra_ai_usage_enabled') THEN
  ALTER TABLE workspace_billing RENAME COLUMN on_demand_enabled TO extra_ai_usage_enabled;
 END IF;
END $$;
DO $$ BEGIN
 IF to_regclass('billing_credit_ledger') IS NOT NULL AND to_regclass('billing_credit_ledger_legacy') IS NULL THEN
  ALTER TABLE billing_credit_ledger RENAME TO billing_credit_ledger_legacy;
 END IF;
END $$;
ALTER TABLE workspace_billing DROP COLUMN IF EXISTS included_credits;
ALTER TABLE workspace_billing DROP COLUMN IF EXISTS credits_used;
ALTER TABLE workspace_billing DROP COLUMN IF EXISTS on_demand_blocks_invoiced;
