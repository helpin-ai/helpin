import { createFileRoute } from '@tanstack/react-router'
import { buildCanonicalArticlePath, buildCanonicalCollectionPath, buildCanonicalHomePath, isMultilingualEnabled } from '@/lib/locale'
import { resolveSubdomain } from '@/lib/utils'
import type { HelpCenterConfig, NavItem, Space } from '@/lib/types'

const SITEMAP_CACHE_TTL_MS = 60 * 60 * 1000
const sitemapCache = new Map<string, { body: string; expiresAt: number }>()

function getInternalApiUrl() {
  return process.env.INTERNAL_API_URL || 'http://127.0.0.1:8080/api'
}

async function fetchApi<T>(path: string) {
  const response = await fetch(`${getInternalApiUrl()}${path}`)
  if (!response.ok) {
    throw new Error(`Sitemap request failed (${response.status}) for ${path}`)
  }

  return response.json() as Promise<T>
}

function xmlEscape(value: string) {
  return value
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&apos;')
}

function getCachedSitemap(cacheKey: string) {
  const cached = sitemapCache.get(cacheKey)
  if (!cached) {
    return null
  }

  if (cached.expiresAt <= Date.now()) {
    sitemapCache.delete(cacheKey)
    return null
  }

  return cached.body
}

function setCachedSitemap(cacheKey: string, body: string) {
  sitemapCache.set(cacheKey, {
    body,
    expiresAt: Date.now() + SITEMAP_CACHE_TTL_MS,
  })
}

function formatLastMod(value?: string | null) {
  if (!value) {
    return null
  }

  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) {
    return null
  }

  return parsed.toISOString()
}

export const Route = createFileRoute('/sitemap.xml')({
  server: {
    handlers: {
      GET: async ({ request }) => {
        try {
          const url = new URL(request.url)
          const host =
            request.headers.get('x-forwarded-host') ||
            request.headers.get('host') ||
            url.host
          const protocol =
            request.headers.get('x-forwarded-proto') ||
            url.protocol.replace(/:$/, '')
          const origin = `${protocol}://${host}`
          const cacheKey = `${origin}/sitemap.xml`
          const cachedBody = getCachedSitemap(cacheKey)

          if (cachedBody) {
            return new Response(cachedBody, {
              headers: {
                'cache-control': 'public, max-age=3600',
                'content-type': 'application/xml; charset=utf-8',
              },
            })
          }

          const subdomain = resolveSubdomain(host)
          const config = await fetchApi<HelpCenterConfig>(`/hc/${subdomain}/config`)
          const multilingualEnabled = isMultilingualEnabled(config.enabled_locales)
          const locales = multilingualEnabled
            ? config.enabled_locales
            : [config.default_locale]
          const urls = new Map<string, string | null>()

          urls.set(
            `${origin}${buildCanonicalHomePath(multilingualEnabled, config.default_locale)}`,
            null,
          )

          for (const locale of locales) {
            const spaces = await fetchApi<Space[]>(
              multilingualEnabled
                ? `/hc/${subdomain}/${locale}/spaces`
                : `/hc/${subdomain}/spaces`,
            )

            for (const space of spaces) {
              const navigation = await fetchApi<NavItem[]>(
                multilingualEnabled
                  ? `/hc/${subdomain}/${locale}/spaces/${space.slug}/navigation`
                  : `/hc/${subdomain}/spaces/${space.slug}/navigation`,
              )

              for (const collection of navigation) {
                urls.set(
                  `${origin}${buildCanonicalCollectionPath(
                    multilingualEnabled,
                    locale,
                    collection.slug,
                  )}`,
                  null,
                )

                for (const article of collection.articles) {
                  urls.set(
                    `${origin}${buildCanonicalArticlePath(
                      multilingualEnabled,
                      locale,
                      collection.slug,
                      article.slug,
                    )}`,
                    formatLastMod(article.published_at),
                  )
                }
              }
            }
          }

          const body = `<?xml version="1.0" encoding="UTF-8"?>\n<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">\n${Array.from(
            urls.entries(),
          )
            .map(([entry, lastmod]) =>
              lastmod
                ? `  <url><loc>${xmlEscape(entry)}</loc><lastmod>${xmlEscape(lastmod)}</lastmod></url>`
                : `  <url><loc>${xmlEscape(entry)}</loc></url>`,
            )
            .join('\n')}\n</urlset>`

          setCachedSitemap(cacheKey, body)

          return new Response(body, {
            headers: {
              'cache-control': 'public, max-age=3600',
              'content-type': 'application/xml; charset=utf-8',
            },
          })
        } catch (error) {
          console.error('failed to generate sitemap', error)

          return new Response('Sitemap unavailable', {
            status: 503,
            headers: {
              'cache-control': 'no-store',
              'content-type': 'text/plain; charset=utf-8',
            },
          })
        }
      },
    },
  },
})
