import { describe, expect, it, vi } from 'vitest';

import {
  ONBOARDING_USE_CASE_OPTIONS,
  mapOnboardingUseCasesToSetupGoals,
  trackWorkspaceOnboardingUseCases,
} from '../workspaceOnboardingUseCases';

describe('workspaceOnboardingUseCases', () => {
  it('defines the product use cases shown during workspace onboarding', () => {
    expect(ONBOARDING_USE_CASE_OPTIONS.map((option) => option.value)).toEqual([
      'product_engineering',
      'team_project_management',
      'customer_support',
      'help_center_docs',
      'internal_docs',
      'sales_crm',
    ]);
  });

  it('shows replacement tool examples for every use case', () => {
    for (const option of ONBOARDING_USE_CASE_OPTIONS) {
      expect(option.replaces).toMatch(/^Replaces /);
    }
  });

  it('preserves selected intent in setup goals without duplicates', () => {
    expect(mapOnboardingUseCasesToSetupGoals([
      'product_engineering',
      'team_project_management',
      'customer_support',
      'sales_crm',
    ])).toEqual(['product_delivery', 'customer_support', 'sales_crm']);
  });

  it('sends selected use cases to Usermaven when the tracker is available', () => {
    const track = vi.fn();
    const target = { location: { hostname: 'app.helpin.ai' }, usermaven: { track } };

    trackWorkspaceOnboardingUseCases(['customer_support', 'sales_crm'], target);

    expect(track).toHaveBeenCalledWith('workspace_onboarding_use_cases_selected', {
      use_cases: ['customer_support', 'sales_crm'],
      use_case_count: 2,
    });
  });

  it('supports callable Usermaven snippets', () => {
    const usermaven = vi.fn();
    const target = { location: { hostname: 'app.helpin.ai' }, usermaven };

    trackWorkspaceOnboardingUseCases(['internal_docs'], target);

    expect(usermaven).toHaveBeenCalledWith('track', 'workspace_onboarding_use_cases_selected', {
      use_cases: ['internal_docs'],
      use_case_count: 1,
    });
  });

  it('does nothing when Usermaven tracking is unavailable', () => {
    expect(() => trackWorkspaceOnboardingUseCases(['customer_support'], {})).not.toThrow();
  });

  it('does not send selected use cases outside app.helpin.ai', () => {
    const track = vi.fn();

    trackWorkspaceOnboardingUseCases(['customer_support'], {
      location: { hostname: 'stage.helpin.ai' },
      usermaven: { track },
    });

    expect(track).not.toHaveBeenCalled();
  });
});
