export const INVALID_ORIGIN = 'Enter a website like example.com, a pattern like *.example.com, or tauri://localhost.';
const LOCAL_HOST = /^(localhost|127(?:\.\d{1,3}){3}|\[::1\])(?::\d+)?(?:[/?#]|$)/i;
const IP_HOST = /^(\d{1,3}(?:\.\d{1,3}){3}|\[.*\])$/;

/**
 * Turns one typed website into a stored allowlist entry. Paths are dropped, a
 * missing scheme becomes https (http for local test hosts), and *.example.com
 * admits every subdomain over https. The server repeats this check and also
 * rejects patterns on shared domains such as *.co.uk.
 */
export function normalizeWidgetOrigin(raw: string): string {
  const value = raw.trim();
  if (value === '*') throw new Error('To allow every website, select "Allow any website or app" instead.');
  // URL.origin serializes custom schemes as "null". Preserve only the exact
  // supported Tauri origin instead of saving that opaque value.
  if (/^tauri:\/\/localhost\/*$/i.test(value)) return 'tauri://localhost';
  let candidate = value.includes('://') ? value : `${LOCAL_HOST.test(value) ? 'http' : 'https'}://${value}`;
  const pattern = /^([a-z][a-z\d+.-]*):\/\/\*\.(.+)$/i.exec(candidate);
  if (pattern) {
    if (pattern[1].toLowerCase() !== 'https') throw new Error('Subdomain patterns must use https, like *.example.com.');
    candidate = `https://${pattern[2]}`;
  }
  let url: URL;
  try {
    url = new URL(candidate);
  } catch {
    throw new Error(INVALID_ORIGIN);
  }
  const { hostname } = url;
  if (!['http:', 'https:'].includes(url.protocol) || url.username || url.password || !hostname || hostname.includes('*') ||
    (!hostname.includes('.') && hostname !== 'localhost' && !IP_HOST.test(hostname))) {
    throw new Error(INVALID_ORIGIN);
  }
  if (!pattern) return url.origin;
  if (!hostname.includes('.') || IP_HOST.test(hostname)) throw new Error('Subdomain patterns need a domain, like *.example.com.');
  return `https://*.${url.host}`;
}
