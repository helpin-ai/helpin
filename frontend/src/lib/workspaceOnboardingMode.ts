/**
 * URL-addressable steps of the workspace onboarding at `/onboarding?step=`.
 * Steps after `workspace` also carry `workspace=<slug>` so a reload resumes
 * against the workspace that was already created.
 */
export const workspaceOnboardingSteps = [
  'workspace',
  'method',
  'assistant',
  'ai',
  'github',
  'context',
  'teams',
  'invite',
  'finish',
] as const;

export type WorkspaceOnboardingStep = (typeof workspaceOnboardingSteps)[number];

/** Step names used by earlier versions of the flow, kept so old links still open it. */
const legacyStepAliases: Record<string, WorkspaceOnboardingStep> = {
  details: 'workspace',
  'use-cases': 'workspace',
  learning: 'context',
};

export function parseWorkspaceOnboardingStep(value: unknown): WorkspaceOnboardingStep | undefined {
  if (typeof value !== 'string') return undefined;
  if (workspaceOnboardingSteps.includes(value as WorkspaceOnboardingStep)) {
    return value as WorkspaceOnboardingStep;
  }
  return legacyStepAliases[value];
}

export function parseWorkspaceOnboardingSlug(value: unknown): string | undefined {
  return typeof value === 'string' && value.trim() ? value.trim() : undefined;
}

/** Choosing an organization only matters when the person belongs to more than one. */
export function shouldShowWorkspaceOnboardingOrganizationSelector({
  organizationCount,
}: {
  organizationCount: number;
}) {
  return organizationCount > 1;
}

/**
 * Where the owner lands after creating a workspace. Whenever the API serves the
 * Setup guide, every new workspace opens it; otherwise the owner lands on My Work.
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
