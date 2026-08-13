import type { UsageResponse } from '@/lib/billingTypes';
import type { WorkspaceBillingSummary } from '@/lib/types';

const periodStart = '2026-08-12T00:00:00Z';
const periodEnd = '2026-09-12T00:00:00Z';

export const growthBillingPreview: WorkspaceBillingSummary = {
  workspace_id: 'billing-preview-growth',
  plan: 'growth',
  status: 'active',
  locked: false,
  billing_interval: 'monthly',
  trialing: false,
  current_period_start: periodStart,
  current_period_end: periodEnd,
  included_credits: 0,
  credits_used: 0,
  credits_remaining: 0,
  next_charge_cents: 29_900,
  on_demand_enabled: true,
  on_demand_available: true,
  cancel_at_period_end: false,
  manage_billing_enabled: false,
  on_demand_blocks_invoiced: 0,
  seat_limit: 0,
  seat_usage: 14,
  seat_over_limit: false,
  ai_usage_allowance_microusd: 299_000_000,
  ai_usage_used_microusd: 109_135_000,
  ai_usage_remaining_microusd: 177_905_000,
  ai_usage_reserved_microusd: 11_960_000,
  ai_usage_overage_microusd: 0,
  ai_usage_period_start: periodStart,
  ai_usage_period_end: periodEnd,
  ai_usage_unlimited: false,
  extra_ai_usage_enabled: true,
  extra_ai_usage_available: true,
  pricing_version: '2026-08-13',
};

export const founderBillingPreview: WorkspaceBillingSummary = {
  ...growthBillingPreview,
  workspace_id: 'billing-preview-founder',
  plan: 'founder',
  next_charge_cents: 0,
  ai_usage_allowance_microusd: 150_000_000,
  ai_usage_used_microusd: 175_500_000,
  ai_usage_remaining_microusd: 0,
  ai_usage_reserved_microusd: 4_500_000,
  ai_usage_overage_microusd: 0,
  ai_usage_unlimited: true,
  extra_ai_usage_enabled: false,
  extra_ai_usage_available: false,
};

export const growthUsagePreview: UsageResponse = {
  period: '2026-08-12..2026-09-12',
  period_start: periodStart,
  period_end: periodEnd,
  mode: 'daily',
  ai_usage_allowance_microusd: 299_000_000,
  ai_usage_used_microusd: 109_135_000,
  ai_usage_reserved_microusd: 11_960_000,
  ai_usage_overage_microusd: 0,
  features: [
    { feature_key: 'support_reply', label: 'Support replies', pct: 41, model_tiers: ['small', 'medium'], action_count: 1842, charged_microusd: 44_745_000, input_tokens: 4_820_000, cache_read_tokens: 1_140_000, cache_write_tokens: 210_000, output_tokens: 2_760_000, reasoning_tokens: 180_000, estimated_count: 0 },
    { feature_key: 'planning', label: 'Planning runs', pct: 27, model_tiers: ['large'], action_count: 38, charged_microusd: 29_466_000, input_tokens: 1_320_000, cache_read_tokens: 410_000, cache_write_tokens: 95_000, output_tokens: 620_000, reasoning_tokens: 390_000, estimated_count: 0 },
    { feature_key: 'coding', label: 'Coding and review', pct: 24, model_tiers: ['large', 'flagship'], action_count: 21, charged_microusd: 26_192_000, input_tokens: 980_000, cache_read_tokens: 320_000, cache_write_tokens: 82_000, output_tokens: 510_000, reasoning_tokens: 440_000, estimated_count: 1 },
    { feature_key: 'docs', label: 'Documentation', pct: 8, model_tiers: ['small'], action_count: 116, charged_microusd: 8_732_000, input_tokens: 760_000, cache_read_tokens: 190_000, cache_write_tokens: 35_000, output_tokens: 430_000, reasoning_tokens: 24_000, estimated_count: 0 },
  ],
  series: [
    { date: '2026-08-12', features: { support_reply: 2_400_000, planning: 0, coding: 0, docs: 620_000 } },
    { date: '2026-08-13', features: { support_reply: 3_100_000, planning: 4_800_000, coding: 0, docs: 540_000 } },
    { date: '2026-08-14', features: { support_reply: 2_850_000, planning: 0, coding: 6_200_000, docs: 410_000 } },
    { date: '2026-08-15', features: { support_reply: 3_450_000, planning: 5_600_000, coding: 3_900_000, docs: 780_000 } },
    { date: '2026-08-16', features: { support_reply: 2_200_000, planning: 3_100_000, coding: 0, docs: 490_000 } },
    { date: '2026-08-17', features: { support_reply: 3_900_000, planning: 0, coding: 5_700_000, docs: 680_000 } },
    { date: '2026-08-18', features: { support_reply: 3_300_000, planning: 4_900_000, coding: 4_100_000, docs: 590_000 } },
  ],
};

export const founderUsagePreview: UsageResponse = {
  ...growthUsagePreview,
  ai_usage_allowance_microusd: 150_000_000,
  ai_usage_used_microusd: 175_500_000,
  ai_usage_reserved_microusd: 4_500_000,
};
