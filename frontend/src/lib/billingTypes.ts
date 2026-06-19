// Billing types — match backend DTOs for the organization-billing feature.
// See docs/superpowers/specs/2026-06-19-organization-billing-design.md

export type BillingPlan = 'free' | 'starter' | 'growth';
export type BillingStatus = 'trialing' | 'active' | 'past_due' | 'canceled';
export type BillingInterval = 'monthly' | 'annual';
export type UsageMode = 'daily' | 'cumulative';

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
  trialing: boolean;
  trial_ends_at?: string;
  current_period_end: string;
  included_credits: number;
  credits_used: number;
  on_demand_enabled: boolean;
  on_demand_available: boolean;
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
  credits_used: number;
  included_credits_total: number;
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

export interface SetupIntentResponse {
  client_secret: string;
  customer_id: string;
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
  cost: number;
  usage: number;
  credits: number;
  pct: number;
}

export interface UsageResponse {
  period: string;
  included_credits: number;
  credits_used: number;
  series: UsageSeriesPoint[];
  features: UsageFeatureRow[];
}

export interface CheckoutResponse {
  url: string;
}

export interface PortalResponse {
  url: string;
}

// Request payloads
export interface UpdateCardRequest {
  is_org_default?: boolean;
  cardholder?: string;
}

export interface LinkPaymentMethodRequest {
  payment_method_id: string | null;
}

export interface SetBillingOwnerRequest {
  user_id: string | null;
}

export interface CheckoutRequest {
  plan: Exclude<BillingPlan, 'free'>;
  interval: BillingInterval;
  return_url?: string;
}
