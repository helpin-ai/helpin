import type { GitHubAppStatus } from '@/lib/pmTypes';

export const GITHUB_APP_OWNER_HINT =
  'Private Apps can only be installed on the account that owns them. Choose the organization that owns your repositories.';

/**
 * Sends the browser to GitHub with the App manifest. The manifest flow only
 * accepts a top-level form POST with a `manifest` field, so fetch cannot be used.
 */
export function submitGitHubAppManifest(postURL: string, manifest: Record<string, unknown>, doc: Document = document) {
  const form = doc.createElement('form');
  form.method = 'post';
  form.action = postURL;
  form.style.display = 'none';
  const input = doc.createElement('input');
  input.type = 'hidden';
  input.name = 'manifest';
  input.value = JSON.stringify(manifest);
  form.appendChild(input);
  doc.body.appendChild(form);
  form.submit();
}

/**
 * Explains who can install a private App (one created from Helpin). GitHub
 * only lets the owner account install it, and shows a 404 to everyone else.
 */
export function gitHubAppOwnerNote(status: GitHubAppStatus | undefined): string | null {
  const owner = status?.owner_login?.trim();
  if (!status?.configured || !status.private || !owner) return null;
  return `Private App owned by ${owner}. Install it on ${owner} to connect its repositories.`;
}
