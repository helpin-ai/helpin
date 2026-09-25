type WorkspaceOnboardingDetailsInput = {
  hasOrganization: boolean;
  name: string;
  slug: string;
  goalCount: number;
};

/** Checks the workspace step. The website is optional and asked for later. */
export function validateWorkspaceOnboardingDetails({
  hasOrganization,
  name,
  slug,
  goalCount,
}: WorkspaceOnboardingDetailsInput) {
  if (!hasOrganization) {
    return 'Enter an organization name to continue.';
  }
  if (!name.trim() || !slug.trim()) {
    return 'Enter a workspace name.';
  }
  if (goalCount < 1) {
    return 'Select at least one thing to set up.';
  }
  return null;
}
