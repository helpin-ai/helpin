import type { DocsHelpcenterConfig } from './docsTypes'
import { resolveHelpcenterPreviewBase, type HelpcenterPreviewBase } from './helpcenterPreview'

type ArticleUrlConfig = Pick<DocsHelpcenterConfig, 'subdomain' | 'public_url_mode' | 'custom_domain' | 'reverse_proxy_host' | 'reverse_proxy_base_path' | 'enabled_locales'>

export function buildHelpcenterArticleUrl(
  config: ArticleUrlConfig | undefined,
  liveSlug: string | undefined,
  publicId: string | undefined,
  locale: string,
  base: HelpcenterPreviewBase = resolveHelpcenterPreviewBase({
    explicit: import.meta.env.VITE_HELPCENTER_URL,
    appBase: import.meta.env.VITE_APP_BASE_URL,
    fallbackOrigin: typeof window !== 'undefined' ? window.location.origin : '',
  }),
): string | null {
  if (!config || !liveSlug || !publicId || !config.subdomain) return null
  const multilingual = new Set(config.enabled_locales.map(value => value.trim().toLowerCase()).filter(Boolean)).size > 1
  const path = `${multilingual ? `/${encodeURIComponent(locale)}` : ''}/articles/${encodeURIComponent(liveSlug)}-${encodeURIComponent(publicId)}`
  // Stage and local environments retain their own Help Center host.
  if (base.hostedSubdomain && base.hostRoot === 'helpin.center') {
    if (config.public_url_mode === 'reverse_proxy' && config.reverse_proxy_host) {
      const mount = config.reverse_proxy_base_path?.replace(/^\/+|\/+$/g, '')
      return `https://${config.reverse_proxy_host}${mount ? `/${mount}` : ''}${path}`
    }
    if (config.public_url_mode === 'custom_domain' && config.custom_domain) {
      return `https://${config.custom_domain}${path}`
    }
  }
  if (base.hostedSubdomain) return `https://${config.subdomain}.${base.hostRoot}${path}`
  const url = new URL(path, base.hostRoot)
  url.searchParams.set('subdomain', config.subdomain)
  return url.toString()
}
