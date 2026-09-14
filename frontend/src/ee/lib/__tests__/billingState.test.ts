import { describe, expect, it } from 'vitest';

import { billingLockedCopy, isBillingLocked } from '../billingState';

describe('billingState', () => {
  it('treats expired trials and canceled subscriptions as locked', () => {
    expect(isBillingLocked({ status: 'trial_expired' })).toBe(true);
    expect(isBillingLocked({ status: 'unpaid' })).toBe(true);
    expect(isBillingLocked({ status: 'canceled' })).toBe(true);
    expect(isBillingLocked({ status: 'active' })).toBe(false);
    expect(isBillingLocked({ status: 'trialing' })).toBe(false);
  });

  it('explains locked workspaces without mentioning a Free plan', () => {
    expect(billingLockedCopy({ status: 'trial_expired' }).title).toBe('Trial ended');
    expect(billingLockedCopy({ status: 'unpaid' }).title).toBe('Payment overdue');
    expect(billingLockedCopy({ status: 'canceled' }).description).toContain('subscription ended');
    expect(billingLockedCopy({ status: 'trial_expired' }).description).not.toContain('Free');
  });
});
