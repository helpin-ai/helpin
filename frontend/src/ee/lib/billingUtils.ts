import dayjs from 'dayjs';
import type { BillingInterval, BillingPlan, BillingStatus } from './billingTypes';

/** Format integer cents → "$X" (no decimals for whole dollars, else 2dp). */
export function formatCents(cents: number): string {
  const dollars = (cents || 0) / 100;
  const whole = Number.isInteger(dollars);
  return `$${dollars.toLocaleString('en-US', {
    minimumFractionDigits: whole ? 0 : 2,
    maximumFractionDigits: 2,
  })}`;
}

export function formatNumber(n: number): string {
  return (n || 0).toLocaleString('en-US');
}

/** Days remaining until an ISO date (rounded up, min 0). */
export function daysUntil(iso?: string | null): number {
  if (!iso) return 0;
  const diff = dayjs(iso).diff(dayjs(), 'day', true);
  return Math.max(0, Math.ceil(diff));
}

export function formatDate(iso?: string | null): string {
  if (!iso) return '—';
  return dayjs(iso).format('MMM D, YYYY');
}

export const PLAN_LABEL: Record<BillingPlan, string> = {
  starter: 'Starter',
  growth: 'Growth',
  founder: 'Founder',
};

export const INTERVAL_LABEL: Record<BillingInterval, string> = {
  monthly: 'Monthly',
  annual: 'Annual',
};

export interface PlanOption {
  plan: BillingPlan;
  label: string;
  monthlyCents: number;
  annualCents: number; // total per year
}

// Reference pricing from docs/pricing-strategy.md (see design spec §4).
export const PLAN_OPTIONS: PlanOption[] = [
  { plan: 'starter', label: 'Starter', monthlyCents: 9900, annualCents: 94800 },
  { plan: 'growth', label: 'Growth', monthlyCents: 29900, annualCents: 286800 },
];

export function planPriceCents(opt: PlanOption, interval: BillingInterval): number {
  return interval === 'annual' ? Math.round(opt.annualCents / 12) : opt.monthlyCents;
}

/** Effective annual discount percentage vs paying monthly. */
export function annualDiscountPct(opt: PlanOption): number {
  const fullYear = opt.monthlyCents * 12;
  if (fullYear === 0) return 0;
  return Math.round(((fullYear - opt.annualCents) / fullYear) * 100);
}

export function statusBadgeVariant(
  status: BillingStatus,
  trialing: boolean,
): 'default' | 'secondary' | 'destructive' | 'outline' {
  if (status === 'past_due' || status === 'unpaid' || status === 'canceled' || status === 'trial_expired') return 'destructive';
  if (trialing) return 'secondary';
  return 'default';
}

export function planBadgeLabel(plan: BillingPlan, trialing: boolean): string {
  if (trialing) return `${PLAN_LABEL[plan]} trial`;
  return PLAN_LABEL[plan];
}

// Stable color palette for feature segments in the usage chart.
export const FEATURE_COLORS = [
  '#6366f1', // indigo
  '#06b6d4', // cyan
  '#22c55e', // green
  '#f59e0b', // amber
  '#ef4444', // red
  '#a855f7', // purple
  '#ec4899', // pink
  '#14b8a6', // teal
];

export function featureColor(index: number): string {
  return FEATURE_COLORS[index % FEATURE_COLORS.length];
}
