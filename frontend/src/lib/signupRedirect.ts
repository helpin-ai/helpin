export type SignupRedirectTarget =
  | { to: string }
  | { to: '/workspaces'; search: { create: true } };

export function signupSuccessRedirect(redirect: string | null | undefined): SignupRedirectTarget {
  if (redirect) {
    return { to: redirect };
  }
  return { to: '/workspaces', search: { create: true } };
}
