import { describe, expect, it } from 'vitest';
import type { WorkspaceBillingSummary } from '@/lib/types';
import { getBillingNoticePresentation } from '../billingNoticePresentation';

const baseBilling: WorkspaceBillingSummary = {
  workspace_id: 'ws-1',
  plan: 'growth',
  status: 'trialing',
  locked: false,
  billing_interval: 'monthly',
  trialing: true,
  trial_ends_at: '2026-07-22T00:00:00Z',
  current_period_start: '2026-07-08T00:00:00Z',
  current_period_end: '2026-07-22T00:00:00Z',
  included_credits: 25000,
  credits_used: 100,
  credits_remaining: 24900,
  next_charge_cents: 0,
  on_demand_enabled: false,
  on_demand_available: false,
  cancel_at_period_end: false,
  manage_billing_enabled: false,
  on_demand_blocks_invoiced: 0,
};

describe('getBillingNoticePresentation', () => {
  it('uses Upgrade for app-managed trial ending notices', () => {
    const notice = getBillingNoticePresentation({
      ...baseBilling,
      billing_notice_type: 'trial_will_end',
      trial_will_end_at: '2026-07-20T00:00:00Z',
    });

    expect(notice?.kind).toBe('trial');
    expect(notice?.actionLabel).toBe('Upgrade');
    expect(notice?.action).toBe('upgrade');
  });

  it('uses Portal payment handling for paid payment failures', () => {
    const notice = getBillingNoticePresentation({
      ...baseBilling,
      status: 'past_due',
      trialing: false,
      manage_billing_enabled: true,
      billing_notice_type: 'payment_failed',
    });

    expect(notice?.kind).toBe('payment');
    expect(notice?.actionLabel).toBe('Update payment');
    expect(notice?.action).toBe('portal');
  });
});
