type WorkspaceOnboardingDetailsInput = {
  hasOrganization: boolean;
  name: string;
  slug: string;
  websiteUrl: string;
};

export function validateWorkspaceOnboardingDetails({
  hasOrganization,
  name,
  slug,
  websiteUrl,
}: WorkspaceOnboardingDetailsInput) {
  if (!hasOrganization) {
    return 'Please select an organization first';
  }
  if (!name.trim() || !slug.trim()) {
    return 'Enter a workspace name and slug';
  }
  if (!websiteUrl.trim()) {
    return 'Enter your company or product website';
  }
  return null;
}
