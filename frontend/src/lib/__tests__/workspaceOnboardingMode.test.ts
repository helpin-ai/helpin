import { describe, expect, it } from 'vitest';

import {
  parseWorkspaceOnboardingSlug,
  parseWorkspaceOnboardingStep,
  shouldShowWorkspaceOnboardingOrganizationSelector,
  workspaceCreatedDestination,
} from '../workspaceOnboardingMode';

describe('workspaceOnboardingMode', () => {
  it('parses the onboarding steps for URL state', () => {
    for (const step of ['workspace', 'ai', 'context', 'teams', 'invite', 'finish']) {
      expect(parseWorkspaceOnboardingStep(step)).toBe(step);
    }
    expect(parseWorkspaceOnboardingStep('not-real')).toBeUndefined();
    expect(parseWorkspaceOnboardingStep(undefined)).toBeUndefined();
  });

  it('maps step names from earlier links onto the current flow', () => {
    expect(parseWorkspaceOnboardingStep('details')).toBe('workspace');
    expect(parseWorkspaceOnboardingStep('use-cases')).toBe('workspace');
    expect(parseWorkspaceOnboardingStep('learning')).toBe('context');
  });

  it('reads the workspace slug only when it is a non-empty string', () => {
    expect(parseWorkspaceOnboardingSlug('acme')).toBe('acme');
    expect(parseWorkspaceOnboardingSlug(' ')).toBeUndefined();
    expect(parseWorkspaceOnboardingSlug(42)).toBeUndefined();
  });

  it('asks for an organization only when there is more than one', () => {
    expect(shouldShowWorkspaceOnboardingOrganizationSelector({ organizationCount: 0 })).toBe(false);
    expect(shouldShowWorkspaceOnboardingOrganizationSelector({ organizationCount: 1 })).toBe(false);
    expect(shouldShowWorkspaceOnboardingOrganizationSelector({ organizationCount: 2 })).toBe(true);
  });

  it('opens the Setup guide after creating any workspace when the API serves it', () => {
    expect(workspaceCreatedDestination({ slug: 'acme', setupGuideEnabled: true })).toBe('/w/acme/setup');
  });

  it('lands on My Work after workspace creation when the Setup guide is off', () => {
    expect(workspaceCreatedDestination({ slug: 'acme', setupGuideEnabled: false })).toBe('/w/acme/pm/my-work');
  });
});
