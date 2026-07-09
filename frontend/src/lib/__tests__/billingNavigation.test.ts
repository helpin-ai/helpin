import { describe, expect, it } from 'vitest';

import {
  BILLING_CHOOSE_PLAN_SEARCH,
  BILLING_OVERVIEW_SEARCH,
  shouldOpenBillingPlanChooser,
} from '../billingNavigation';

describe('billingNavigation', () => {
  it('uses choose_plan=true for upgrade links to the billing plan chooser', () => {
    expect(BILLING_CHOOSE_PLAN_SEARCH).toEqual({ choose_plan: true });
  });

  it('uses choose_plan undefined for ordinary billing overview links', () => {
    expect(BILLING_OVERVIEW_SEARCH).toEqual({ choose_plan: undefined });
  });

  it('opens the plan chooser only when the billing search flag is true', () => {
    expect(shouldOpenBillingPlanChooser({ choose_plan: true })).toBe(true);
    expect(shouldOpenBillingPlanChooser({ choose_plan: 'true' })).toBe(true);
    expect(shouldOpenBillingPlanChooser({ choose_plan: false })).toBe(false);
    expect(shouldOpenBillingPlanChooser({ choose_plan: 'false' })).toBe(false);
    expect(shouldOpenBillingPlanChooser({})).toBe(false);
  });
});
