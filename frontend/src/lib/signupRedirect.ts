export type SignupRedirectTarget =
  | { to: string }
  | { to: '/onboarding'; search: { step: 'workspace' } };

export function signupSuccessRedirect(redirect: string | null | undefined): SignupRedirectTarget {
  if (redirect) {
    return { to: redirect };
  }
  return { to: '/onboarding', search: { step: 'workspace' } };
}

/**
 * Where a signed-in person goes when no redirect was requested: their default
 * workspace, else their first one, else onboarding to create one.
 */
export function loginDestination({
  workspaces,
  defaultWorkspaceId,
}: {
  workspaces: readonly { id: string; slug: string }[] | null | undefined;
  defaultWorkspaceId?: string | null;
}):
  | { to: '/w/$slug/pm/my-work'; params: { slug: string } }
  | { to: '/onboarding'; search: { step: 'workspace' } } {
  if (!workspaces || workspaces.length === 0) {
    return { to: '/onboarding', search: { step: 'workspace' } };
  }
  const preferred = defaultWorkspaceId ? workspaces.find((workspace) => workspace.id === defaultWorkspaceId) : undefined;
  return { to: '/w/$slug/pm/my-work', params: { slug: (preferred ?? workspaces[0]).slug } };
}
