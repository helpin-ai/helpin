import { trackAnalyticsEvent, type AnalyticsOptions } from '@/lib/analytics';
import type { WorkspaceBillingSummary } from '@/lib/types';

function daysUntil(isoDate?: string) {
  if (!isoDate) return undefined;
  const time = new Date(isoDate).getTime();
  if (!Number.isFinite(time)) return undefined;
  return Math.max(0, Math.ceil((time - Date.now()) / 86_400_000));
}

export function buildAnalyticsBillingEventProperties(
  eventName: string,
  workspaceId: string,
  billing?: WorkspaceBillingSummary | null,
  extra?: Record<string, unknown>,
) {
  return {
    event_name: eventName,
    workspace_id: workspaceId,
    plan: billing?.plan,
    billing_status: billing?.status,
    billing_interval: billing?.billing_interval,
    trialing: !!billing?.trialing,
    trial_ends_at: billing?.trial_ends_at,
    trial_days_left: daysUntil(billing?.trial_ends_at),
    locked: !!billing?.locked,
    ai_usage_allowance_microusd: billing?.ai_usage_allowance_microusd,
    ai_usage_used_microusd: billing?.ai_usage_used_microusd,
    ai_usage_remaining_microusd: billing?.ai_usage_remaining_microusd,
    ai_usage_reserved_microusd: billing?.ai_usage_reserved_microusd,
    ai_usage_overage_microusd: billing?.ai_usage_overage_microusd,
    seat_limit: billing?.seat_limit,
    seat_usage: billing?.seat_usage,
    seat_over_limit: billing?.seat_over_limit,
    extra_ai_usage_enabled: billing?.extra_ai_usage_enabled,
    extra_ai_usage_available: billing?.extra_ai_usage_available,
    pricing_version: billing?.pricing_version,
    manage_billing_enabled: billing?.manage_billing_enabled,
    cancel_at_period_end: billing?.cancel_at_period_end,
    ...extra,
  };
}

export function trackWorkspaceBillingEvent(
  eventName: string,
  workspaceId: string | undefined,
  billing?: WorkspaceBillingSummary | null,
  extra?: Record<string, unknown>,
  options?: AnalyticsOptions,
) {
  if (!workspaceId) return;
  trackAnalyticsEvent(eventName, buildAnalyticsBillingEventProperties(eventName, workspaceId, billing, extra), options);
}
