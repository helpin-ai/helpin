-- Organization billing aggregate + first-class saved payment cards.
-- Idempotent: safe to run multiple times.

CREATE TABLE IF NOT EXISTS organization_billing (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id uuid NOT NULL UNIQUE REFERENCES organizations(id) ON DELETE CASCADE,
  stripe_customer_id text,
  default_payment_method_id uuid,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_organization_billing_org ON organization_billing(organization_id);

CREATE TABLE IF NOT EXISTS billing_payment_method (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  stripe_payment_method_id text NOT NULL,
  brand text NOT NULL DEFAULT '',
  last4 text NOT NULL DEFAULT '',
  exp_month integer NOT NULL DEFAULT 0,
  exp_year integer NOT NULL DEFAULT 0,
  cardholder text NOT NULL DEFAULT '',
  is_org_default boolean NOT NULL DEFAULT false,
  billing_details jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_billing_payment_method_org ON billing_payment_method(organization_id);

ALTER TABLE workspace_billing ADD COLUMN IF NOT EXISTS payment_method_id uuid;
ALTER TABLE workspace_billing ADD COLUMN IF NOT EXISTS billing_owner_user_id uuid;

CREATE INDEX IF NOT EXISTS idx_workspace_billing_payment_method ON workspace_billing(payment_method_id);
