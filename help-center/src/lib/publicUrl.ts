import type { RootRouteData } from '@/lib/rootLoader'
import type { HelpCenterConfig } from '@/lib/types'
import { prefixBasepath } from '@/lib/pathUtils'

function normalizeHost(value?: string | null) {
  return (value || '')
    .trim()
    .replace(/^https?:\/\//i, '')
    .replace(/\/.*$/, '')
    .toLowerCase()
}

function normalizeBasepath(value?: string | null) {
  const raw = (value || '').trim()
  if (!raw || raw === '/') return ''
  const withSlash = raw.startsWith('/') ? raw : `/${raw}`
  const trimmed = withSlash.replace(/\/+$/, '')
  if (!trimmed || trimmed === '/') return ''
  if (trimmed.includes('//') || trimmed.includes('\\')) return ''
  const segments = trimmed.split('/').filter(Boolean)
  if (segments.some((segment) => segment === '.' || segment === '..')) return ''
  return trimmed
}

function hostedHelpCenterOrigin(config: HelpCenterConfig) {
  const subdomain = normalizeHost(config.subdomain)
  return subdomain ? `https://${subdomain}.helpin.center` : ''
}

function requestLooksLikeHostedOrigin(rootData: RootRouteData) {
  const host = normalizeHost(rootData.host)
  const subdomain = normalizeHost(rootData.config.subdomain)
  return Boolean(subdomain && (host === `${subdomain}.helpin.center` || host === `${subdomain}.stage.helpin.center`))
}

export function resolvePublicUrlParts(rootData: RootRouteData) {
  const mode = rootData.config.public_url_mode || 'hosted_subdomain'

  if (mode === 'reverse_proxy') {
    const host = normalizeHost(rootData.config.reverse_proxy_host)
    const basepath = normalizeBasepath(rootData.config.reverse_proxy_base_path)
    if (host && basepath) {
      return {
        origin: `https://${host}`,
        basepath,
      }
    }
  }

  if (mode === 'custom_domain') {
    const host = normalizeHost(rootData.config.custom_domain)
    if (host) {
      return {
        origin: `https://${host}`,
        basepath: '',
      }
    }
  }

  const hostedOrigin = hostedHelpCenterOrigin(rootData.config)
  return {
    origin: requestLooksLikeHostedOrigin(rootData) ? rootData.origin : hostedOrigin || rootData.origin,
    basepath: '',
  }
}

export function absolutePublicUrl(rootData: RootRouteData, path: string) {
  const publicUrl = resolvePublicUrlParts(rootData)
  return new URL(prefixBasepath(publicUrl.basepath, path), publicUrl.origin).toString()
}
