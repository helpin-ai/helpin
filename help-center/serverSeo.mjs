const PUBLIC_ID_LENGTH = 8

export function xmlEscape(value) {
  return String(value)
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&apos;')
}

function normalizeHost(value) {
  return String(value || '')
    .trim()
    .replace(/^https?:\/\//i, '')
    .replace(/\/.*$/, '')
    .toLowerCase()
}

function normalizeBasepath(value) {
  const raw = String(value || '').trim()
  if (!raw || raw === '/') return ''
  const withSlash = raw.startsWith('/') ? raw : `/${raw}`
  const trimmed = withSlash.replace(/\/+$/, '')
  if (!trimmed || trimmed === '/') return ''
  if (trimmed.includes('//') || trimmed.includes('\\')) return ''
  const segments = trimmed.split('/').filter(Boolean)
  if (segments.some((segment) => segment === '.' || segment === '..')) return ''
  return trimmed
}

function normalizePublicId(value) {
  const normalized = String(value || '').trim().toLowerCase()
  if (normalized.length !== PUBLIC_ID_LENGTH) return normalized
  return /^[0-9a-f]+$/.test(normalized) ? normalized : ''
}

function buildPublicKey(slug, publicId) {
  const trimmedSlug = String(slug || '').trim().replace(/^\/+|\/+$/g, '')
  const trimmedPublicId = normalizePublicId(publicId)
  if (!trimmedSlug) return trimmedPublicId
  if (!trimmedPublicId) return trimmedSlug
  return `${trimmedSlug}-${trimmedPublicId}`
}

function requestLooksLikeHostedOrigin(hcContext, config) {
  const host = normalizeHost(hcContext.host)
  const subdomain = normalizeHost(config?.subdomain)
  return Boolean(subdomain && (host === `${subdomain}.helpin.center` || host === `${subdomain}.stage.helpin.center`))
}

export function resolvePublicUrlParts(hcContext, config) {
  const mode = config?.public_url_mode || (config?.custom_domain ? 'custom_domain' : 'hosted_subdomain')

  if (mode === 'reverse_proxy') {
    const host = normalizeHost(config?.reverse_proxy_host)
    const basepath = normalizeBasepath(config?.reverse_proxy_base_path)
    if (host && basepath) {
      return { origin: `https://${host}`, basepath }
    }
  }

  if (mode === 'custom_domain') {
    const host = normalizeHost(config?.custom_domain)
    if (host) {
      return { origin: `https://${host}`, basepath: '' }
    }
  }

  if (requestLooksLikeHostedOrigin(hcContext, config)) {
    return { origin: hcContext.origin, basepath: '' }
  }

  const subdomain = normalizeHost(config?.subdomain || hcContext.subdomain)
  return {
    origin: subdomain ? `https://${subdomain}.helpin.center` : hcContext.origin,
    basepath: '',
  }
}

export function joinPublicUrl(publicUrl, path) {
  const normalizedPath = path.startsWith('/') ? path : `/${path}`
  const normalizedBasepath = publicUrl.basepath || ''
  const prefix = normalizedBasepath ? `${publicUrl.origin}${normalizedBasepath}` : publicUrl.origin
  if (normalizedPath === '/') return `${prefix}/`
  return `${prefix}${normalizedPath}`
}

export function buildSitemapPath({ multilingual, locale, kind, slug, publicId }) {
  const localePrefix = multilingual ? `/${locale}` : ''
  if (kind === 'home') {
    return multilingual ? localePrefix : '/'
  }
  if (kind === 'collection') {
    return `${localePrefix}/c/${buildPublicKey(slug, publicId)}`
  }
  if (kind === 'article') {
    return `${localePrefix}/articles/${buildPublicKey(slug, publicId)}`
  }
  return '/'
}

export function collectSitemapEntries({ publicUrl, config, navigationByLocale }) {
  const multilingual = (config?.enabled_locales || []).length > 1
  const locales = multilingual ? config.enabled_locales : [config?.default_locale || 'en']
  const entries = new Map()

  for (const locale of locales) {
    entries.set(
      joinPublicUrl(publicUrl, buildSitemapPath({ multilingual, locale, kind: 'home' })),
      null,
    )

    for (const collection of navigationByLocale.get(locale) || []) {
      entries.set(
        joinPublicUrl(
          publicUrl,
          buildSitemapPath({
            multilingual,
            locale,
            kind: 'collection',
            slug: collection.slug,
            publicId: collection.public_id,
          }),
        ),
        null,
      )

      for (const article of collection.articles || []) {
        entries.set(
          joinPublicUrl(
            publicUrl,
            buildSitemapPath({
              multilingual,
              locale,
              kind: 'article',
              slug: article.slug,
              publicId: article.public_id,
            }),
          ),
          article.published_at || null,
        )
      }
    }
  }

  return Array.from(entries.entries())
}

export function renderSitemapXml(entries) {
  const body = entries
    .map(([loc, lastmod]) => {
      const lastmodTag = lastmod ? `<lastmod>${xmlEscape(new Date(lastmod).toISOString())}</lastmod>` : ''
      return `  <url><loc>${xmlEscape(loc)}</loc>${lastmodTag}</url>`
    })
    .join('\n')
  return `<?xml version="1.0" encoding="UTF-8"?>\n<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">\n${body}\n</urlset>`
}

export function renderRobotsTxt(publicUrl) {
  return [
    'User-agent: *',
    `Disallow: ${publicUrl.basepath || ''}/preview/`,
    'Crawl-delay: 1',
    `Sitemap: ${joinPublicUrl(publicUrl, '/sitemap.xml')}`,
  ].join('\n')
}
