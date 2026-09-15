import { describe, expect, it } from 'vitest';
import type { WorkspaceBillingCard } from '@/ee/lib/billingTypes';
import { getWorkspaceBillingCardPresentation } from '../workspaceBillingCardPresentation';

const baseCard: WorkspaceBillingCard = {
  workspace_id: 'ws-1',
  workspace_name: 'Acme',
  workspace_slug: 'acme',
  plan: 'growth',
  status: 'trialing',
  locked: false,
  trialing: true,
  trial_ends_at: '2026-07-22T00:00:00Z',
  current_period_end: '2026-07-22T00:00:00Z',
  included_credits: 25000,
  credits_used: 1200,
  on_demand_enabled: false,
  on_demand_available: false,
  price_cents: 0,
  billing_interval: 'monthly',
  payment_method: null,
  billing_owner: null,
  can_manage: true,
};

describe('getWorkspaceBillingCardPresentation', () => {
  it('routes app-managed trial workspaces to Upgrade instead of card management', () => {
    const view = getWorkspaceBillingCardPresentation(baseCard);

    expect(view.primaryCtaLabel).toBe('Upgrade');
    expect(view.primaryCtaAction).toBe('upgrade');
    expect(view.billedToLabel).toBe('Upgrade to add billing');
    expect(view.showBilledToChange).toBe(false);
    expect(view.extraUsageLabel).toBe('Available after upgrade');
  });

  it('routes expired trials to Upgrade instead of Reactivate portal handling', () => {
    const view = getWorkspaceBillingCardPresentation({
      ...baseCard,
      status: 'trial_expired',
      locked: true,
      trialing: false,
    });

    expect(view.primaryCtaLabel).toBe('Upgrade');
    expect(view.primaryCtaAction).toBe('upgrade');
    expect(view.billedToLabel).toBe('Upgrade to add billing');
    expect(view.showBilledToChange).toBe(false);
  });

  it('keeps Stripe portal management for active paid workspaces', () => {
    const view = getWorkspaceBillingCardPresentation({
      ...baseCard,
      status: 'active',
      trialing: false,
      locked: false,
      on_demand_available: true,
      payment_method: { id: 'pm-1', brand: 'visa', last4: '4242' },
    });

    expect(view.primaryCtaLabel).toBe('Manage');
    expect(view.primaryCtaAction).toBe('manage');
    expect(view.billedToLabel).toBe('visa ···· 4242');
    expect(view.showBilledToChange).toBe(true);
    expect(view.extraUsageLabel).toBe('Add usage past your plan limit');
  });
});
