type WorkspaceOnboardingModeInput = {
  create?: boolean;
  forceFullPage?: boolean;
  isLoading: boolean;
  workspaceCount: number;
  hasStartedOnboarding: boolean;
};

export const workspaceOnboardingSteps = [
  'details',
  'use-cases',
  'learning',
  'context',
  'teams',
  'invite',
] as const;

export type WorkspaceOnboardingStep = (typeof workspaceOnboardingSteps)[number];

export function parseWorkspaceOnboardingStep(value: unknown): WorkspaceOnboardingStep | undefined {
  return typeof value === 'string' && workspaceOnboardingSteps.includes(value as WorkspaceOnboardingStep)
    ? value as WorkspaceOnboardingStep
    : undefined;
}

export function shouldUseFullPageWorkspaceOnboarding({
  create,
  forceFullPage,
  isLoading,
  workspaceCount,
  hasStartedOnboarding,
}: WorkspaceOnboardingModeInput) {
  if (isLoading) {
    return false;
  }
  if (forceFullPage) {
    return true;
  }
  if (!create) {
    return false;
  }
  return workspaceCount === 0 || hasStartedOnboarding;
}

export function shouldShowWorkspaceOnboardingOrganizationSelector({
  isFullPageOnboarding,
  organizationCount,
}: {
  isFullPageOnboarding: boolean;
  organizationCount: number;
}) {
  return !isFullPageOnboarding && organizationCount > 0;
}

export function shouldContinueToInviteStepAfterWorkspaceCreate({
  isFullPageOnboarding,
}: {
  isFullPageOnboarding: boolean;
}) {
  return isFullPageOnboarding;
}

export function shouldClearWorkspaceCreateSearchAfterDialogOpen({
  create,
  isFullPageOnboarding,
}: {
  create?: boolean;
  isFullPageOnboarding: boolean;
}) {
  return Boolean(create) && !isFullPageOnboarding;
}

/**
 * Where the owner lands after creating a workspace. Whenever the API serves the
 * Setup guide, every new workspace opens it (dialog or full-page onboarding);
 * otherwise the owner lands on My Work.
 */
export function workspaceCreatedDestination({
  slug,
  setupGuideEnabled,
}: {
  slug: string;
  setupGuideEnabled: boolean;
}) {
  return setupGuideEnabled ? `/w/${slug}/setup` : `/w/${slug}/pm/my-work`;
}
