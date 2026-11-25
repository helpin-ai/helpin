/**
 * URL-addressable steps of the support onboarding at `/onboarding?step=`.
 * Steps after `website` also carry `workspace=<slug>` so a reload resumes
 * against the workspace that was already created.
 */
export const workspaceOnboardingSteps = ['website', 'ai', 'setup'] as const;

export type WorkspaceOnboardingStep = (typeof workspaceOnboardingSteps)[number];

/** Step names used by earlier versions of the flow, kept so old links still open it. */
const legacyStepAliases: Record<string, WorkspaceOnboardingStep> = {
  details: 'website',
  'use-cases': 'website',
  workspace: 'website',
  method: 'setup',
  assistant: 'setup',
  github: 'setup',
  context: 'setup',
  learning: 'setup',
  teams: 'setup',
  invite: 'setup',
  finish: 'setup',
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

/** Where onboarding ends: the support inbox, where conversations arrive. */
export function workspaceCreatedDestination(slug: string) {
  return `/w/${slug}/support`;
}
