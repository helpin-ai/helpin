export const HELPIN_ATTRIBUTION_BASE_URL = 'https://helpin.ai/'

function trackingSlug(value: string | null | undefined, fallback: string): string {
  const slug = (value ?? '')
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')

  return slug || fallback
}

function trackingContent(value: string): string {
  return value
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '_')
    .replace(/^_+|_+$/g, '') || 'unknown'
}

export function buildAttributionSource(name: string | null | undefined, workspaceId: string | null | undefined): string {
  const workspaceSlug = trackingSlug(name, 'workspace')
  const idPrefix = trackingSlug((workspaceId ?? '').trim().slice(0, 8), '')
  return idPrefix ? `${workspaceSlug}-${idPrefix}` : workspaceSlug
}

export function buildHelpinAttributionUrl(source: string, content: string): string {
  const url = new URL(HELPIN_ATTRIBUTION_BASE_URL)
  url.searchParams.set('utm_source', trackingSlug(source, 'workspace'))
  url.searchParams.set('utm_medium', 'referral')
  url.searchParams.set('utm_campaign', 'powered_by_helpin')
  url.searchParams.set('utm_content', trackingContent(content))
  return url.toString()
}
