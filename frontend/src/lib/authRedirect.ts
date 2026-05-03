export const REDIRECT_AFTER_LOGIN_KEY = 'helpin_redirect_after_login';

function isInternalRedirect(path: string): boolean {
  if (!path.startsWith('/') || path.startsWith('//')) return false;
  return path !== '/login' && path !== '/logout' && !path.startsWith('/login?');
}

export function currentRedirectPath(): string {
  return `${window.location.pathname}${window.location.search}${window.location.hash}`;
}

export function storeRedirectAfterLogin(path = currentRedirectPath()): void {
  if (!isInternalRedirect(path)) return;
  try {
    sessionStorage.setItem(REDIRECT_AFTER_LOGIN_KEY, path);
  } catch {
    // Ignore storage access failures; login can still fall back normally.
  }
}

export function consumeRedirectAfterLogin(): string | null {
  try {
    const path = sessionStorage.getItem(REDIRECT_AFTER_LOGIN_KEY);
    if (path) {
      sessionStorage.removeItem(REDIRECT_AFTER_LOGIN_KEY);
    }
    return path && isInternalRedirect(path) ? path : null;
  } catch {
    return null;
  }
}
