const DEFAULT_FAVICON_SIZE = 128;

function stripWWW(value: string): string {
  return value.replace(/^www\./i, '');
}

export function normalizeFaviconHost(value?: string | null): string {
  if (!value) {
    return '';
  }

  const trimmed = value.trim();
  if (!trimmed) {
    return '';
  }

  const candidate = trimmed.includes('://') ? trimmed : `https://${trimmed}`;

  try {
    const parsed = new URL(candidate);
    return stripWWW(parsed.hostname.toLowerCase());
  } catch {
    return '';
  }
}

export function getGoogleFaviconUrl(value?: string | null, size: number = DEFAULT_FAVICON_SIZE): string {
  const host = normalizeFaviconHost(value);
  if (!host) {
    return '';
  }
  return `https://www.google.com/s2/favicons?domain=${encodeURIComponent(host)}&sz=${size}`;
}
