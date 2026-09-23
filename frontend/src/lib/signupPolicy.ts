/** The server's own wording for self-signup on an invite-only server. */
export const INVITE_ONLY_MESSAGE = 'Signup on this server is by invitation. Ask your admin for an invite.';

/** Parses "acme.com, @beta.io" into domains; the server validates them. */
export function parseDomains(value: string): string[] {
  return value.split(/[\s,]+/).map((domain) => domain.trim().replace(/^@/, '').toLowerCase()).filter(Boolean);
}

/** Joins domains for a sentence: "a.com", "a.com or b.com", "a.com, b.com or c.com". */
export function formatDomainList(domains: string[]): string {
  if (domains.length <= 1) return domains[0] ?? '';
  return `${domains.slice(0, -1).join(', ')} or ${domains[domains.length - 1]}`;
}
