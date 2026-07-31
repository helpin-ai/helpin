import { describe, expect, it } from 'vitest';

import {
  parseWorkspaceOnboardingStep,
  shouldClearWorkspaceCreateSearchAfterDialogOpen,
  shouldContinueToInviteStepAfterWorkspaceCreate,
  shouldShowWorkspaceOnboardingOrganizationSelector,
  shouldUseFullPageWorkspaceOnboarding,
} from '../workspaceOnboardingMode';

describe('workspaceOnboardingMode', () => {
  it('uses the full-page flow for first-workspace signup creation', () => {
    expect(shouldUseFullPageWorkspaceOnboarding({
      create: true,
      isLoading: false,
      workspaceCount: 0,
      hasStartedOnboarding: false,
    })).toBe(true);
  });

  it('keeps the full-page flow active after the workspace is created during onboarding', () => {
    expect(shouldUseFullPageWorkspaceOnboarding({
      create: true,
      isLoading: false,
      workspaceCount: 1,
      hasStartedOnboarding: true,
    })).toBe(true);
  });

  it('does not use the full-page flow for later workspace creation', () => {
    expect(shouldUseFullPageWorkspaceOnboarding({
      create: true,
      isLoading: false,
      workspaceCount: 2,
      hasStartedOnboarding: false,
    })).toBe(false);
  });

  it('uses the full-page flow for the dedicated onboarding route', () => {
    expect(shouldUseFullPageWorkspaceOnboarding({
      forceFullPage: true,
      isLoading: false,
      workspaceCount: 2,
      hasStartedOnboarding: false,
    })).toBe(true);
  });

  it('waits for workspace loading before choosing the mode', () => {
    expect(shouldUseFullPageWorkspaceOnboarding({
      create: true,
      isLoading: true,
      workspaceCount: 0,
      hasStartedOnboarding: false,
    })).toBe(false);
  });

  it('hides the organization selector in full-page onboarding', () => {
    expect(shouldShowWorkspaceOnboardingOrganizationSelector({
      isFullPageOnboarding: true,
      organizationCount: 2,
    })).toBe(false);
  });

  it('shows the organization selector in the workspace creation dialog', () => {
    expect(shouldShowWorkspaceOnboardingOrganizationSelector({
      isFullPageOnboarding: false,
      organizationCount: 2,
    })).toBe(true);
  });

  it('does not show the organization selector when there are no organizations', () => {
    expect(shouldShowWorkspaceOnboardingOrganizationSelector({
      isFullPageOnboarding: false,
      organizationCount: 0,
    })).toBe(false);
  });

  it('parses known onboarding steps for URL state', () => {
    expect(parseWorkspaceOnboardingStep('use-cases')).toBe('use-cases');
    expect(parseWorkspaceOnboardingStep('teams')).toBe('teams');
    expect(parseWorkspaceOnboardingStep('more-context')).toBeUndefined();
    expect(parseWorkspaceOnboardingStep('not-real')).toBeUndefined();
    expect(parseWorkspaceOnboardingStep(undefined)).toBeUndefined();
  });

  it('continues to invites only during first-workspace full-page onboarding', () => {
    expect(shouldContinueToInviteStepAfterWorkspaceCreate({ isFullPageOnboarding: true })).toBe(true);
    expect(shouldContinueToInviteStepAfterWorkspaceCreate({ isFullPageOnboarding: false })).toBe(false);
  });

  it('clears the create query after it opens the in-app workspace dialog', () => {
    expect(shouldClearWorkspaceCreateSearchAfterDialogOpen({
      create: true,
      isFullPageOnboarding: false,
    })).toBe(true);
    expect(shouldClearWorkspaceCreateSearchAfterDialogOpen({
      create: true,
      isFullPageOnboarding: true,
    })).toBe(false);
    expect(shouldClearWorkspaceCreateSearchAfterDialogOpen({
      create: false,
      isFullPageOnboarding: false,
    })).toBe(false);
  });
});
