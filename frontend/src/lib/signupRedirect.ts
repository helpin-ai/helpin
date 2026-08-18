export type SignupRedirectTarget =
  | { to: string }
  | { to: '/onboarding'; search: { step: 'details' } };

export function signupSuccessRedirect(redirect: string | null | undefined): SignupRedirectTarget {
  if (redirect) {
    return { to: redirect };
  }
  return { to: '/onboarding', search: { step: 'details' } };
}
