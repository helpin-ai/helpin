import { describe, expect, it } from 'vitest';

import {
  parseWorkspaceOnboardingSlug,
  parseWorkspaceOnboardingStep,
  shouldShowWorkspaceOnboardingOrganizationSelector,
  workspaceCreatedDestination,
} from '../workspaceOnboardingMode';

describe('workspaceOnboardingMode', () => {
  it('parses the onboarding steps for URL state', () => {
    for (const step of ['website', 'ai', 'setup']) {
      expect(parseWorkspaceOnboardingStep(step)).toBe(step);
    }
    expect(parseWorkspaceOnboardingStep('not-real')).toBeUndefined();
    expect(parseWorkspaceOnboardingStep(undefined)).toBeUndefined();
  });

  it('maps step names from earlier links onto the current flow', () => {
    expect(parseWorkspaceOnboardingStep('workspace')).toBe('website');
    expect(parseWorkspaceOnboardingStep('details')).toBe('website');
    expect(parseWorkspaceOnboardingStep('teams')).toBe('setup');
    expect(parseWorkspaceOnboardingStep('finish')).toBe('setup');
  });

  it('reads the workspace slug only when it is a non-empty string', () => {
    expect(parseWorkspaceOnboardingSlug('acme')).toBe('acme');
    expect(parseWorkspaceOnboardingSlug(' ')).toBeUndefined();
    expect(parseWorkspaceOnboardingSlug(42)).toBeUndefined();
  });

  it('asks for an organization only when there is more than one', () => {
    expect(shouldShowWorkspaceOnboardingOrganizationSelector({ organizationCount: 1 })).toBe(false);
    expect(shouldShowWorkspaceOnboardingOrganizationSelector({ organizationCount: 2 })).toBe(true);
  });

  it('ends onboarding in the support inbox', () => {
    expect(workspaceCreatedDestination('acme')).toBe('/w/acme/support');
  });
});
