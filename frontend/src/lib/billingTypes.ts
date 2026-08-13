// Billing types — match backend DTOs for the organization-billing feature.
// See docs/superpowers/specs/2026-06-19-organization-billing-design.md

export type BillingPlan = 'starter' | 'growth' | 'founder';
export type BillingStatus = 'trialing' | 'active' | 'trial_expired' | 'past_due' | 'unpaid' | 'canceled';
export type BillingInterval = 'monthly' | 'annual';
export type UsageMode = 'daily' | 'cumulative';
export type BillingTestScenarioID =
  | 'reset_starter'
  | 'trial_cap'
  | 'past_due_grace'
  | 'unpaid_locked'
  | 'starter_ai_cap'
  | 'starter_on_demand'
  | 'starter_docs_limit'
  | 'starter_contacts_over_limit';

export interface BillingPaymentMethodRef {
  id: string;
  brand: string;
  last4: string;
}

export interface BillingOwnerRef {
  user_id: string;
  name: string;
  email: string;
}

export interface WorkspaceBillingCard {
  workspace_id: string;
  workspace_name: string;
  workspace_slug: string;
  plan: BillingPlan;
  status: BillingStatus;
  locked: boolean;
  trialing: boolean;
  trial_ends_at?: string;
  current_period_end: string;
  ai_usage_allowance_microusd?: number;
  ai_usage_used_microusd?: number;
  ai_usage_reserved_microusd?: number;
  ai_usage_percent?: number;
  extra_ai_usage_enabled?: boolean;
  extra_ai_usage_available?: boolean;
  price_cents: number;
  billing_interval: BillingInterval;
  payment_method: BillingPaymentMethodRef | null;
  billing_owner: BillingOwnerRef | null;
  can_manage: boolean;
}

export interface OrgBillingSummary {
  total_monthly_spend_cents: number;
  paid_count: number;
  trialing_count: number;
  ai_usage_used_microusd?: number;
  ai_usage_allowance_microusd?: number;
  ai_usage_percent?: number;
  setup_complete: boolean;
}

export interface OrgBillingResponse extends OrgBillingSummary {
  organization_id: string;
  workspaces: WorkspaceBillingCard[];
}

export interface LinkedWorkspaceRef {
  id: string;
  name: string;
}

export interface PaymentMethod {
  id: string;
  brand: string;
  last4: string;
  exp_month: number;
  exp_year: number;
  cardholder: string;
  is_org_default: boolean;
  linked_workspace_count: number;
  linked_workspaces: LinkedWorkspaceRef[];
}

export interface Invoice {
  id: string;
  number: string;
  workspace_id: string;
  workspace_name: string;
  amount_cents: number;
  currency: string;
  status: string;
  created: string;
  hosted_url: string;
  pdf_url: string;
}

export interface UsageSeriesPoint {
  date: string;
  features: Record<string, number>;
}

export interface UsageFeatureRow {
  feature_key: string;
  label: string;
  pct: number;
  model_tier?: 'small' | 'medium' | 'large' | 'flagship';
  action_count?: number;
  charged_microusd?: number;
  input_tokens?: number;
  cache_read_tokens?: number;
  cache_write_tokens?: number;
  output_tokens?: number;
  reasoning_tokens?: number;
  estimated_count?: number;
}

export interface UsageResponse {
  period: string;
  period_start: string;
  period_end: string;
  mode: UsageMode;
  ai_usage_allowance_microusd: number;
  ai_usage_used_microusd: number;
  ai_usage_reserved_microusd: number;
  ai_usage_overage_microusd: number;
  series: UsageSeriesPoint[];
  features: UsageFeatureRow[];
}

export interface CheckoutResponse {
  url: string;
}

export interface ConfirmCheckoutRequest {
  session_id: string;
}

export interface PortalResponse {
  url: string;
}

export interface PlanChangePreviewLine {
  description: string;
  amount_cents: number;
  proration: boolean;
}

export interface PlanChangePreview {
  workspace_id: string;
  current_plan: BillingPlan;
  current_interval: BillingInterval;
  target_plan: BillingPlan;
  target_interval: BillingInterval;
  effective: 'immediate' | string;
  proration_date: number;
  amount_due_cents: number;
  subtotal_cents: number;
  total_cents: number;
  currency: string;
  next_payment_attempt?: string;
  current_period_end: string;
  current_included_credits: number;
  target_included_credits: number;
  credits_used: number;
  credits_remaining_after: number;
  lines: PlanChangePreviewLine[];
}

// Request payloads
export interface UpdateCardRequest {
  is_org_default?: boolean;
  cardholder?: string;
}

export interface LinkPaymentMethodRequest {
  payment_method_id: string | null;
}

export interface CheckoutRequest {
  plan: BillingPlan;
  interval: BillingInterval;
  return_url?: string;
}

export interface PlanChangeRequest {
  plan: BillingPlan;
  interval: BillingInterval;
  proration_date?: number;
}
