export interface HelpcenterPreviewBase {
  hostRoot: string
  hostedSubdomain: boolean
}

export interface HelpcenterPreviewTarget {
  subdomain: string
  customDomain?: string | null
}

function normalizePreviewHost(value: string | null | undefined): string | null {
  const trimmed = value?.trim()
  if (!trimmed) return null

  try {
    const url = new URL(trimmed.includes('://') ? trimmed : `https://${trimmed}`)
    return url.host.toLowerCase()
  } catch {
    return null
  }
}

export function normalizeExplicitHelpcenterPreviewBase(
  explicit: string,
): HelpcenterPreviewBase | null {
  try {
    const url = new URL(explicit)

    if (url.hostname === 'localhost' || url.hostname === '127.0.0.1') {
      return { hostRoot: url.origin, hostedSubdomain: false }
    }

    if (url.hostname === 'helpcenter.helpin.ai') {
      return { hostRoot: 'helpin.center', hostedSubdomain: true }
    }

    if (url.hostname === 'helpcenter-stage.helpin.ai') {
      return { hostRoot: 'stage.helpin.center', hostedSubdomain: true }
    }

    if (
      url.hostname === 'helpin.center'
      || url.hostname === 'stage.helpin.center'
    ) {
      return { hostRoot: url.hostname, hostedSubdomain: true }
    }

    return { hostRoot: url.origin, hostedSubdomain: false }
  } catch {
    return null
  }
}

export function resolveHelpcenterPreviewBase({
  explicit,
  appBase,
  fallbackOrigin,
}: {
  explicit?: string | null
  appBase?: string | null
  fallbackOrigin?: string | null
} = {}): HelpcenterPreviewBase {
  const normalizedExplicit = explicit?.trim()
  if (normalizedExplicit) {
    const normalized = normalizeExplicitHelpcenterPreviewBase(normalizedExplicit)
    if (normalized) {
      return normalized
    }
  }

  const candidate = appBase?.trim() || fallbackOrigin?.trim() || ''
  if (candidate) {
    try {
      const url = new URL(candidate)
      if (url.hostname === 'app.helpin.ai') {
        return { hostRoot: 'helpin.center', hostedSubdomain: true }
      }
      if (
        url.hostname === 'client.stage.helpin.ai'
        || url.hostname === 'stage.helpin.ai'
      ) {
        return { hostRoot: 'stage.helpin.center', hostedSubdomain: true }
      }
      if (url.hostname === 'localhost' || url.hostname === '127.0.0.1') {
        return { hostRoot: 'http://localhost:5174', hostedSubdomain: false }
      }
    } catch {
      // Fall through to the local dev default below.
    }
  }

  return { hostRoot: 'http://localhost:5174', hostedSubdomain: false }
}

export function buildHelpcenterPreviewUrl(
  target: HelpcenterPreviewTarget,
  docId: string,
  token: string,
  base: HelpcenterPreviewBase,
): string {
  const previewPath = `/preview/${encodeURIComponent(docId)}`

  // Production previews can use the workspace custom domain when present so
  // the rendered chrome, absolute links, and copied URL match the live host.
  if (base.hostedSubdomain && base.hostRoot === 'helpin.center') {
    const customDomainHost = normalizePreviewHost(target.customDomain)
    if (customDomainHost) {
      const customDomainURL = new URL(`https://${customDomainHost}${previewPath}`)
      customDomainURL.searchParams.set('token', token)
      return customDomainURL.toString()
    }
  }

  if (base.hostedSubdomain) {
    const hostedURL = new URL(`https://${target.subdomain}.${base.hostRoot}${previewPath}`)
    hostedURL.searchParams.set('token', token)
    return hostedURL.toString()
  }

  const fallbackURL = new URL(previewPath, base.hostRoot)
  fallbackURL.searchParams.set('subdomain', target.subdomain)
  fallbackURL.searchParams.set('token', token)
  return fallbackURL.toString()
}

export function buildHelpcenterPreviewUrlFromEnv(
  target: HelpcenterPreviewTarget,
  docId: string,
  token: string,
): string {
  return buildHelpcenterPreviewUrl(
    target,
    docId,
    token,
    resolveHelpcenterPreviewBase({
      explicit: import.meta.env.VITE_HELPCENTER_URL,
      appBase: import.meta.env.VITE_APP_BASE_URL,
      fallbackOrigin:
        typeof window !== 'undefined' ? window.location.origin : '',
    }),
  )
}
