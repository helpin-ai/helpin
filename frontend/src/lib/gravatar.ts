/** Resolve a public avatar without sending the raw email in the image URL. */
export async function getGravatarUrl(email?: string | null): Promise<string | undefined> {
  const normalized = email?.trim().toLowerCase();
  if (!normalized || !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(normalized)) return undefined;

  // Web Crypto can be unavailable in insecure previews or restricted browsers.
  // In that case the caller keeps its usual initials fallback.
  if (!globalThis.crypto?.subtle) return undefined;
  try {
    const digest = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(normalized));
    const hash = Array.from(new Uint8Array(digest), (byte) => byte.toString(16).padStart(2, '0')).join('');
    return `https://www.gravatar.com/avatar/${hash}?s=128&d=404&r=g`;
  } catch {
    return undefined;
  }
}
