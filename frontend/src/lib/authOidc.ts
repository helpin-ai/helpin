import { API_BASE } from '@/lib/api';

export function buildOidcAuthStartUrl(redirect?: string | null): string {
  const url = new URL(`${API_BASE}/auth/oidc/start`);
  if (redirect) {
    url.searchParams.set('redirect', redirect);
  }
  return url.toString();
}

const LOGIN_ERROR_MESSAGES: Record<string, string> = {
  invalid_state: 'The sign-in session expired or was interrupted. Try again.',
  invalid_nonce: 'The sign-in response could not be verified. Try again.',
  invalid_token: 'The sign-in response could not be verified. Try again.',
  missing_code: 'The sign-in provider did not return a sign-in code. Try again.',
  exchange_failed: 'Could not complete sign-in with the provider. Try again.',
  userinfo_failed: 'Could not read your profile from the sign-in provider.',
  provider_unavailable: 'The sign-in provider is not reachable right now. Try again later.',
  provider_error: 'The sign-in provider declined the request.',
  signup_restricted: 'Signup on this server is by invitation. Ask your admin for an invite.',
  email_in_use: 'An account with this email already exists and is not linked to this sign-in provider. Ask your admin to link it.',
  signin_failed: 'Sign-in failed. Make sure your provider account has a verified email address.',
};

/** Human-readable text for a `/login?error=<reason>` code, or null when there is none to show. */
export function loginErrorMessage(reason: string | null | undefined): string | null {
  if (!reason) return null;
  return LOGIN_ERROR_MESSAGES[reason] ?? 'Sign-in failed. Try again.';
}
