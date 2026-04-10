import http from 'node:http'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { Readable } from 'node:stream'
import { AsyncLocalStorage } from 'node:async_hooks'
import serverEntry from './dist/server/server.js'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const CLIENT_DIR = path.join(__dirname, 'dist', 'client')

const MIME_TYPES = {
  '.js': 'application/javascript; charset=utf-8',
  '.mjs': 'application/javascript; charset=utf-8',
  '.css': 'text/css; charset=utf-8',
  '.html': 'text/html; charset=utf-8',
  '.json': 'application/json; charset=utf-8',
  '.png': 'image/png',
  '.jpg': 'image/jpeg',
  '.jpeg': 'image/jpeg',
  '.gif': 'image/gif',
  '.svg': 'image/svg+xml',
  '.ico': 'image/x-icon',
  '.woff': 'font/woff',
  '.woff2': 'font/woff2',
  '.ttf': 'font/ttf',
  '.map': 'application/json',
}

function serveStaticFile(response, filePath) {
  const ext = path.extname(filePath)
  const contentType = MIME_TYPES[ext] || 'application/octet-stream'
  const stat = fs.statSync(filePath)
  response.statusCode = 200
  response.setHeader('Content-Type', contentType)
  response.setHeader('Content-Length', stat.size)
  response.setHeader('Cache-Control', 'public, max-age=31536000, immutable')
  fs.createReadStream(filePath).pipe(response)
}

function tryServeStatic(url, response) {
  if (!url.pathname.startsWith('/assets/')) return false
  const safePath = path.normalize(url.pathname).replace(/^(\.\.[/\\])+/, '')
  const filePath = path.join(CLIENT_DIR, safePath)
  if (!filePath.startsWith(CLIENT_DIR)) return false
  if (!fs.existsSync(filePath)) return false
  serveStaticFile(response, filePath)
  return true
}

const PORT = Number.parseInt(process.env.PORT || '3000', 10)
const HTML_CACHE_TTL_MS = Number.parseInt(
  process.env.HTML_CACHE_TTL_MS || '120000',
  10,
)
const HTML_CACHE_MAX_ENTRIES = Number.parseInt(
  process.env.HTML_CACHE_MAX_ENTRIES || '500',
  10,
)

const htmlCache = new Map()

function normalizeHeaderValue(value) {
  return Array.isArray(value) ? value.join(', ') : value ?? ''
}

function shouldReadBody(method) {
  return !['GET', 'HEAD'].includes(method.toUpperCase())
}

function isHtmlRequest(request, url) {
  if (request.method !== 'GET') {
    return false
  }

  if (
    url.pathname === '/api' ||
    url.pathname.startsWith('/api/') ||
    url.pathname.startsWith('/preview/') ||
    url.pathname.startsWith('/assets/') ||
    url.pathname === '/robots.txt' ||
    url.pathname === '/sitemap.xml' ||
    url.pathname === '/healthz'
  ) {
    return false
  }

  const accept = normalizeHeaderValue(request.headers.accept)
  return accept.includes('text/html') || accept.includes('*/*') || accept === ''
}

function getCacheKey(url, request) {
  const host =
    normalizeHeaderValue(request.headers['x-forwarded-host']) ||
    normalizeHeaderValue(request.headers.host) ||
    url.host

  return `${request.method}:${host}${url.pathname}${url.search}`
}

function getCachedResponse(key) {
  const cached = htmlCache.get(key)
  if (!cached) {
    return null
  }

  if (cached.expiresAt <= Date.now()) {
    htmlCache.delete(key)
    return null
  }

  htmlCache.delete(key)
  htmlCache.set(key, cached)
  return cached
}

function setCachedResponse(key, response) {
  htmlCache.set(key, response)

  while (htmlCache.size > HTML_CACHE_MAX_ENTRIES) {
    const oldestKey = htmlCache.keys().next().value
    if (!oldestKey) {
      break
    }
    htmlCache.delete(oldestKey)
  }
}

function writeCachedResponse(nodeResponse, cached) {
  nodeResponse.statusCode = cached.status

  for (const [name, value] of cached.headers) {
    nodeResponse.setHeader(name, value)
  }

  nodeResponse.end(cached.body)
}

function isApiRequest(url) {
  return url.pathname === '/api' || url.pathname.startsWith('/api/')
}

function buildProxyHeaders(request) {
  const headers = new Headers()
  for (const [name, value] of Object.entries(request.headers)) {
    if (value == null) {
      continue
    }
    headers.set(name, normalizeHeaderValue(value))
  }
  headers.delete('host')
  headers.delete('connection')
  headers.delete('content-length')
  return headers
}

function buildApiProxyUrl(url) {
  const apiBase = process.env.INTERNAL_API_URL
  if (!apiBase) {
    return null
  }

  const target = new URL(apiBase)
  const apiBasePath = target.pathname.replace(/\/$/, '')
  const requestPath = url.pathname.replace(/^\/api/, '')
  target.pathname = `${apiBasePath}${requestPath || ''}`
  target.search = url.search
  return target
}

async function proxyApiRequest(request, nodeResponse, url) {
  const targetUrl = buildApiProxyUrl(url)
  if (!targetUrl) {
    nodeResponse.statusCode = 502
    nodeResponse.setHeader('Cache-Control', 'no-store')
    nodeResponse.setHeader('Content-Type', 'text/plain; charset=utf-8')
    nodeResponse.end('API proxy is not configured')
    return
  }

  const proxyRequest = new Request(targetUrl, {
    method: request.method,
    headers: buildProxyHeaders(request),
    body: shouldReadBody(request.method || 'GET') ? Readable.toWeb(request) : undefined,
    duplex: 'half',
  })

  const proxyResponse = await fetch(proxyRequest)
  await writeFetchResponse(nodeResponse, proxyResponse, targetUrl)
}

async function writeFetchResponse(nodeResponse, response, url) {
  nodeResponse.statusCode = response.status

  if (
    url.pathname.startsWith('/assets/') &&
    !response.headers.has('cache-control')
  ) {
    response.headers.set('Cache-Control', 'public, max-age=31536000, immutable')
  }

  for (const [name, value] of response.headers.entries()) {
    nodeResponse.setHeader(name, value)
  }

  if (!response.body) {
    nodeResponse.end()
    return
  }

  await new Promise((resolve, reject) => {
    const stream = Readable.fromWeb(response.body)
    stream.on('error', reject)
    nodeResponse.on('close', resolve)
    nodeResponse.on('finish', resolve)
    stream.pipe(nodeResponse)
  })
}

function resolveHostInfo(request) {
  const forwardedHost = normalizeHeaderValue(request.headers['x-forwarded-host'])
  const host = forwardedHost || normalizeHeaderValue(request.headers.host) || 'localhost'
  const forwardedProto = normalizeHeaderValue(request.headers['x-forwarded-proto'])
  const protocol = forwardedProto || 'http'
  return { host, protocol, origin: `${protocol}://${host}` }
}

const PATH_HOST_TENANT_ROOTS = new Set(['helpin.center', 'stage.helpin.center'])

function normalizeHostname(host) {
  return host.split(':')[0].trim().toLowerCase()
}

function isPathHostTenantRoot(host) {
  return PATH_HOST_TENANT_ROOTS.has(normalizeHostname(host))
}

function isRootStaticAssetPath(pathname) {
  return pathname === '/assets' || pathname.startsWith('/assets/')
}

function extractFirstPathSegment(pathname) {
  if (!pathname) return ''
  const trimmed = pathname.replace(/^\/+/, '')
  const slash = trimmed.indexOf('/')
  return slash === -1 ? trimmed : trimmed.slice(0, slash)
}

/**
 * Mirrors `resolveHelpCenterContext` in src/lib/utils.ts but adapted for the
 * Node entrypoint. Keep these in sync.
 */
function resolveHelpCenterContext(host, pathname) {
  const hostname = normalizeHostname(host)

  if (isPathHostTenantRoot(hostname)) {
    if (isRootStaticAssetPath(pathname)) {
      return { subdomain: '', basepath: '' }
    }
    const slug = extractFirstPathSegment(pathname)
    if (!slug) return { subdomain: '', basepath: '' }
    return { subdomain: slug, basepath: `/${slug}` }
  }

  if (
    hostname === 'localhost' ||
    hostname === '127.0.0.1' ||
    /^\d{1,3}(\.\d{1,3}){3}$/.test(hostname)
  ) {
    return { subdomain: process.env.VITE_HC_SUBDOMAIN || 'demo', basepath: '' }
  }

  // Custom domain — pass hostname through; backend resolves it.
  return { subdomain: hostname, basepath: '' }
}

const requestContextStorage = new AsyncLocalStorage()

// Exposed to the bundled SSR entry (router.tsx) so it can read per-request
// context without importing any server-only modules.
globalThis.__hcGetRequestContext__ = () => requestContextStorage.getStore() ?? null


function xmlEscape(value) {
  return value
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&apos;')
}

async function handleRobotsTxt(_request, response, hcContext) {
  const { origin, basepath } = hcContext
  const body = [
    'User-agent: *',
    'Disallow: /preview/',
    'Crawl-delay: 1',
    `Sitemap: ${origin}${basepath || ''}/sitemap.xml`,
  ].join('\n')
  response.statusCode = 200
  response.setHeader('Cache-Control', 'public, max-age=3600')
  response.setHeader('Content-Type', 'text/plain; charset=utf-8')
  response.end(body)
}

async function handleSitemapXml(_request, response, hcContext) {
  try {
    const { origin, basepath, subdomain } = hcContext
    const apiBase = process.env.INTERNAL_API_URL
    if (!apiBase) {
      response.statusCode = 503
      response.setHeader('Content-Type', 'text/plain')
      response.end('Sitemap unavailable')
      return
    }
    if (!subdomain) {
      response.statusCode = 404
      response.setHeader('Content-Type', 'text/plain')
      response.end('Sitemap unavailable')
      return
    }
    const configRes = await fetch(`${apiBase}/hc/${subdomain}/config`)
    if (!configRes.ok) throw new Error(`config fetch failed: ${configRes.status}`)
    const config = await configRes.json()
    const locales = config.enabled_locales?.length > 1
      ? config.enabled_locales
      : [config.default_locale || 'en']
    const multilingual = config.enabled_locales?.length > 1
    const baseUrl = `${origin}${basepath || ''}`
    const urls = new Map()
    urls.set(`${baseUrl}${multilingual ? `/${config.default_locale}` : '/'}`, null)

    for (const locale of locales) {
      const spacesPath = multilingual
        ? `/hc/${subdomain}/${locale}/spaces`
        : `/hc/${subdomain}/spaces`
      const spacesRes = await fetch(`${apiBase}${spacesPath}`)
      if (!spacesRes.ok) continue
      const spaces = await spacesRes.json()

      for (const space of spaces) {
        const navPath = multilingual
          ? `/hc/${subdomain}/${locale}/spaces/${space.slug}/navigation`
          : `/hc/${subdomain}/spaces/${space.slug}/navigation`
        const navRes = await fetch(`${apiBase}${navPath}`)
        if (!navRes.ok) continue
        const navigation = await navRes.json()

        for (const coll of navigation) {
          const collPath = multilingual
            ? `/${locale}/${coll.slug}`
            : `/${coll.slug}`
          urls.set(`${baseUrl}${collPath}`, null)

          for (const article of coll.articles || []) {
            const artPath = multilingual
              ? `/${locale}/${coll.slug}/${article.slug}`
              : `/${coll.slug}/${article.slug}`
            urls.set(`${baseUrl}${artPath}`, article.published_at || null)
          }
        }
      }
    }

    const entries = Array.from(urls.entries())
      .map(([loc, lastmod]) => {
        const lastmodTag = lastmod ? `<lastmod>${xmlEscape(new Date(lastmod).toISOString())}</lastmod>` : ''
        return `  <url><loc>${xmlEscape(loc)}</loc>${lastmodTag}</url>`
      })
      .join('\n')
    const body = `<?xml version="1.0" encoding="UTF-8"?>\n<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">\n${entries}\n</urlset>`

    response.statusCode = 200
    response.setHeader('Cache-Control', 'public, max-age=3600')
    response.setHeader('Content-Type', 'application/xml; charset=utf-8')
    response.end(body)
  } catch (error) {
    console.error('failed to generate sitemap', error)
    response.statusCode = 503
    response.setHeader('Cache-Control', 'no-store')
    response.setHeader('Content-Type', 'text/plain; charset=utf-8')
    response.end('Sitemap unavailable')
  }
}

/**
 * Strip the resolved basepath from a pathname so we can match
 * top-level routes like /sitemap.xml or /robots.txt under multi-tenant hosts
 * (e.g. helpin.center/{slug}/sitemap.xml).
 */
function stripBasepath(pathname, basepath) {
  if (!basepath) return pathname
  if (pathname === basepath) return '/'
  if (pathname.startsWith(`${basepath}/`)) {
    return pathname.slice(basepath.length) || '/'
  }
  return pathname
}

function prefixAssetUrls(html, basepath) {
  if (!basepath) return html
  return html.replaceAll(/([("'=])\/assets\//g, `$1${basepath}/assets/`)
}

async function handleRequest(request, response) {
  if (request.url === '/healthz') {
    response.statusCode = 200
    response.setHeader('Cache-Control', 'no-store')
    response.setHeader('Content-Type', 'text/plain; charset=utf-8')
    response.end('ok')
    return
  }

  const { host, protocol, origin } = resolveHostInfo(request)
  const url = new URL(request.url || '/', `${protocol}://${host}`)
  const resolved = resolveHelpCenterContext(host, url.pathname)
  const hcContext = {
    host: normalizeHostname(host),
    protocol,
    origin,
    pathname: url.pathname,
    subdomain: resolved.subdomain,
    basepath: resolved.basepath,
  }
  const internalPath = stripBasepath(url.pathname, hcContext.basepath)
  const routeUrl = new URL(url)
  routeUrl.pathname = internalPath

  await requestContextStorage.run(hcContext, async () => {
    if (internalPath === '/robots.txt') {
      await handleRobotsTxt(request, response, hcContext)
      return
    }

    if (internalPath === '/sitemap.xml') {
      await handleSitemapXml(request, response, hcContext)
      return
    }

    if (tryServeStatic(routeUrl, response)) {
      return
    }

    if (isApiRequest(routeUrl)) {
      await proxyApiRequest(request, response, routeUrl)
      return
    }

    const cacheKey = getCacheKey(url, request)

    if (isHtmlRequest(request, routeUrl)) {
      const cached = getCachedResponse(cacheKey)
      if (cached) {
        writeCachedResponse(response, cached)
        return
      }
    }

    const headers = new Headers()
    for (const [name, value] of Object.entries(request.headers)) {
      if (value == null) {
        continue
      }
      headers.set(name, normalizeHeaderValue(value))
    }

    const fetchRequest = new Request(routeUrl, {
      method: request.method,
      headers,
      body: shouldReadBody(request.method || 'GET') ? Readable.toWeb(request) : undefined,
      duplex: 'half',
    })

    const fetchResponse = await serverEntry.fetch(fetchRequest)
    const responseType = fetchResponse.headers.get('content-type') || ''

    if (isHtmlRequest(request, routeUrl) && responseType.includes('text/html') && fetchResponse.ok) {
      const renderedBody = await fetchResponse.text()
      const body = prefixAssetUrls(renderedBody, hcContext.basepath)
      const headersToCache = Array.from(fetchResponse.headers.entries())
      setCachedResponse(cacheKey, {
        body,
        expiresAt: Date.now() + HTML_CACHE_TTL_MS,
        headers: headersToCache,
        status: fetchResponse.status,
      })

      response.statusCode = fetchResponse.status
      for (const [name, value] of headersToCache) {
        response.setHeader(name, value)
      }
      response.end(body)
      return
    }

    await writeFetchResponse(response, fetchResponse, routeUrl)
  })
}

const server = http.createServer((request, response) => {
  handleRequest(request, response).catch((error) => {
    console.error('help-center server error', error)
    response.statusCode = 500
    response.setHeader('Cache-Control', 'no-store')
    response.setHeader('Content-Type', 'text/plain; charset=utf-8')
    response.end('Internal Server Error')
  })
})

server.listen(PORT, '0.0.0.0', () => {
  console.log(`help-center listening on ${PORT}`)
})
