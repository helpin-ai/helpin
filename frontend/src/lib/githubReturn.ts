import type { GitHubReturnTo } from '@/lib/pmTypes';

/** Outcome of a GitHub App create or install flow, reported in the return URL. */
export type GitHubReturnStatus = 'connected' | 'created' | 'error';

export type GitHubReturnResult = { status: GitHubReturnStatus; message: string };

/**
 * Result flags the server adds to the return URL. `github` is current;
 * `github_app` (install) and `github_app_manifest` (create) come from older
 * redirects and are still honored.
 */
const RESULT_FLAGS = ['github', 'github_app', 'github_app_manifest'] as const;
const RESULT_PARAMS = [...RESULT_FLAGS, 'github_message', 'integration_id'];

const DEFAULT_MESSAGES: Record<GitHubReturnStatus, string> = {
  connected: 'GitHub connected.',
  created: 'GitHub App created. Install it to connect repositories.',
  error: 'GitHub could not be connected. Try again.',
};

/** GitHub organization login rule used by GitHub (1–39 characters, no leading hyphen). */
export const GITHUB_LOGIN_PATTERN = /^[A-Za-z0-9](?:[A-Za-z0-9-]{0,38})$/;

export function readGitHubReturnResult(search: string): GitHubReturnResult | null {
  const params = new URLSearchParams(search);
  for (const flag of RESULT_FLAGS) {
    const value = params.get(flag);
    if (!value) continue;
    const status: GitHubReturnStatus = value === 'connected' || value === 'created' ? value : 'error';
    return { status, message: params.get('github_message')?.trim() || DEFAULT_MESSAGES[status] };
  }
  return null;
}

/** Removes the result flags from the address bar without navigating. */
export function stripGitHubReturnParams() {
  const url = new URL(window.location.href);
  let changed = false;
  for (const key of RESULT_PARAMS) {
    if (url.searchParams.has(key)) {
      url.searchParams.delete(key);
      changed = true;
    }
  }
  if (changed) window.history.replaceState(window.history.state, '', `${url.pathname}${url.search}${url.hash}`);
}

/** Workspace page for a return destination, matching the server's redirects. */
export function gitHubReturnPath(slug: string, returnTo: GitHubReturnTo = 'settings') {
  if (returnTo === 'onboarding') {
    return `/onboarding?${new URLSearchParams({ step: 'github', workspace: slug }).toString()}`;
  }
  const base = `/w/${encodeURIComponent(slug)}`;
  if (returnTo === 'setup') return `${base}/setup`;
  if (returnTo === 'system_status') return `${base}/settings/system-status`;
  return `${base}/settings/git-connections`;
}

/** Query GitHub (or Helpin's callback) sends to /github/installed. */
export type GitHubInstalledQuery = {
  installation_id: string;
  setup_action: string;
  github: string;
  github_message: string;
};

export function parseGitHubInstalledQuery(search: Record<string, unknown>): GitHubInstalledQuery {
  const text = (value: unknown) => (typeof value === 'string' ? value : typeof value === 'number' ? String(value) : '');
  return {
    installation_id: text(search.installation_id),
    setup_action: text(search.setup_action),
    github: text(search.github),
    github_message: text(search.github_message),
  };
}

/** Full-page navigation (to GitHub, or to a page that reads its result from the URL). */
export function assignBrowserLocation(url: string) {
  window.location.assign(url);
}
