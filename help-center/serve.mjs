import http from 'node:http'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { Readable } from 'node:stream'
import { AsyncLocalStorage } from 'node:async_hooks'
import serverEntry from './dist/server/server.js'
import {
  resolvePublicRedirect,
  shouldAttemptRedirectResolution,
} from './serverRedirects.mjs'
import {
  collectSitemapEntries,
  renderRobotsTxt,
  renderSitemapXml,
  resolvePublicUrlParts,
} from './serverSeo.mjs'
import { prefixAssetUrls } from './assetUrls.mjs'
import {
  appendVary,
  compressBody,
  compressedAssetPath,
  negotiateEncoding,
} from './serverCompression.mjs'

import { createSharedRenderCache, helpcenterIdentifierTag } from './serverRenderCache.mjs'
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

async function serveStaticFile(request, response, filePath, basepath = '') {
  const ext = path.extname(filePath)
  const contentType = MIME_TYPES[ext] || 'application/octet-stream'
  response.statusCode = 200
  response.setHeader('Content-Type', contentType)
  response.setHeader('Cache-Control', 'public, max-age=31536000, immutable')
  response.setHeader('Vary', appendVary(response.getHeader('Vary'), 'Accept-Encoding'))

  if (basepath && (ext === '.js' || ext === '.mjs' || ext === '.css')) {
    const body = prefixAssetUrls(fs.readFileSync(filePath, 'utf8'), basepath)
    await writeCompressedBody(request, response, body)
    return
  }

  const requestedEncoding = negotiateEncoding(request.headers['accept-encoding'])
  const selected = compressedAssetPath(filePath, requestedEncoding, fs.existsSync)
  const stat = fs.statSync(selected.filePath)
  if (selected.encoding) response.setHeader('Content-Encoding', selected.encoding)
  response.setHeader('Content-Length', stat.size)
  fs.createReadStream(selected.filePath).pipe(response)
}

async function tryServeStatic(request, url, response, basepath = '') {
  if (!url.pathname.startsWith('/assets/')) return false
  const safePath = path.normalize(url.pathname).replace(/^(\.\.[/\\])+/, '')
  const filePath = path.join(CLIENT_DIR, safePath)
  if (!filePath.startsWith(CLIENT_DIR)) return false
  if (!fs.existsSync(filePath)) return false
  await serveStaticFile(request, response, filePath, basepath)
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
const REDIRECT_CACHE_TTL_MS = Number.parseInt(
  process.env.REDIRECT_CACHE_TTL_MS || '300000',
  10,
)

const SHARED_HTML_CACHE_TTL_SECONDS = Number.parseInt(
  process.env.SHARED_HTML_CACHE_TTL_SECONDS || '300',
  10,
)
const htmlCache = new Map()
const redirectCache = new Map()


function invalidateLocalRenderedPages(tags) {
  const invalidatedTags = new Set(tags)
  for (const [key, cached] of htmlCache.entries()) {
    if (invalidatedTags.has(helpcenterIdentifierTag(cached.identifier))) {
      htmlCache.delete(key)
    }
  }
}

const sharedRenderCache = await createSharedRenderCache({
  redisURL: process.env.REDIS_URL,
  ttlSeconds: SHARED_HTML_CACHE_TTL_SECONDS,
  onInvalidate: invalidateLocalRenderedPages,
})

if (sharedRenderCache) {
  console.log('help-center shared render cache enabled')
}
function normalizeHeaderValue(value) {
  return Array.isArray(value) ? value.join(', ') : value ?? ''
}

function firstHeaderValue(value) {
  return normalizeHeaderValue(value).split(',')[0].trim()
}

async function writeCompressedBody(request, response, body) {
  const input = Buffer.isBuffer(body) ? body : Buffer.from(body)
  const encoding = input.byteLength >= 1024
    ? negotiateEncoding(request.headers['accept-encoding'])
    : ''
  const output = encoding ? await compressBody(input, encoding) : input
  response.setHeader('Vary', appendVary(response.getHeader('Vary'), 'Accept-Encoding'))
  response.removeHeader('Content-Length')
  if (encoding) {
    response.setHeader('Content-Encoding', encoding)
  } else {
    response.removeHeader('Content-Encoding')
  }
  response.setHeader('Content-Length', output.byteLength)
  response.end(output)
}

function normalizeIdentifier(value) {
  const normalized = firstHeaderValue(value).toLowerCase()
  if (!normalized || /^[a-z]+:\/\//i.test(normalized)) return ''
  if (normalized.includes('/') || normalized.includes('\\')) return ''
  return normalized
}

function normalizeBasepath(value) {
  const raw = firstHeaderValue(value)
  if (!raw || raw === '/') return ''

  let decoded = raw
  try {
    decoded = decodeURIComponent(raw)
  } catch {
    return ''
  }

  if (!decoded.startsWith('/')) {
    decoded = `/${decoded}`
  }

  decoded = decoded.replace(/\/+$/, '')
  if (!decoded || decoded === '/') return ''
  if (decoded.includes('//') || decoded.includes('\\')) return ''

  const segments = decoded.split('/').filter(Boolean)
  if (segments.some((segment) => segment === '.' || segment === '..')) {
    return ''
  }

  return decoded
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

function getCacheKey(url, request, hcContext) {
  const host =
    firstHeaderValue(request.headers['x-forwarded-host']) ||
    firstHeaderValue(request.headers.host) ||
    url.host

  return [
    request.method,
    host,
    hcContext?.subdomain || '',
    hcContext?.basepath || '',
    url.pathname,
    url.search,
  ].join(':')
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

function getRedirectCacheKey(hcContext, url) {
  return [
    hcContext.host,
    hcContext.subdomain,
    hcContext.basepath,
    url.pathname,
    url.search,
  ].join(':')
}

function getCachedRedirect(key) {
  const cached = redirectCache.get(key)
  if (!cached) {
    return { hit: false, redirect: null }
  }

  if (cached.expiresAt <= Date.now()) {
    redirectCache.delete(key)
    return { hit: false, redirect: null }
  }

  redirectCache.delete(key)
  redirectCache.set(key, cached)
  return { hit: true, redirect: cached.redirect }
}

function setCachedRedirect(key, redirect) {
  redirectCache.set(key, {
    redirect,
    expiresAt: Date.now() + REDIRECT_CACHE_TTL_MS,
  })

  while (redirectCache.size > HTML_CACHE_MAX_ENTRIES) {
    const oldestKey = redirectCache.keys().next().value
    if (!oldestKey) {
      break
    }
    redirectCache.delete(oldestKey)
  }
}

async function writeCachedResponse(request, nodeResponse, cached) {
  nodeResponse.statusCode = cached.status

  for (const [name, value] of cached.headers) {
    nodeResponse.setHeader(name, value)
  }

  await writeCompressedBody(request, nodeResponse, cached.body)
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
  const forwardedHost = firstHeaderValue(request.headers['x-forwarded-host'])
  const rawHost = firstHeaderValue(request.headers.host) || 'localhost'
  const host = forwardedHost || rawHost
  const forwardedProto = firstHeaderValue(request.headers['x-forwarded-proto'])
  const protocol = forwardedProto || 'http'
  return { host, rawHost, protocol, origin: `${protocol}://${host}` }
}

const HOSTED_HELP_CENTER_ROOTS = ['stage.helpin.center', 'helpin.center']

function normalizeHostname(host) {
  return firstHeaderValue(host).split(':')[0].trim().toLowerCase()
}

function resolveHostedSubdomain(host) {
  const hostname = normalizeHostname(host)

  for (const root of HOSTED_HELP_CENTER_ROOTS) {
    if (hostname === root) {
      return ''
    }
    const suffix = `.${root}`
    if (!hostname.endsWith(suffix)) {
      continue
    }
    const candidate = hostname.slice(0, -suffix.length)
    if (candidate && !candidate.includes('.')) {
      return candidate
    }
  }

  return ''
}

function isLocalHostname(hostname) {
  return (
    hostname === 'localhost' ||
    hostname === '127.0.0.1' ||
    /^\d{1,3}(\.\d{1,3}){3}$/.test(hostname)
  )
}

function canUseExplicitTenant(tenant, rawHostname) {
  if (!tenant) return false
  if (isLocalHostname(rawHostname)) return true

  const hostedSubdomain = resolveHostedSubdomain(rawHostname)
  return hostedSubdomain === tenant
}

/**
 * Mirrors `resolveHelpCenterContext` in src/lib/utils.ts but adapted for the
 * Node entrypoint. Keep these in sync.
 */
function resolveHelpCenterContext(host, pathname, search = '', proxy = {}) {
  const hostname = normalizeHostname(host)
  const rawHostname = normalizeHostname(proxy.rawHost || host)
  const searchParams = new URLSearchParams(search)
  const queryTenant = normalizeIdentifier(
    searchParams.get('helpin_tenant') || searchParams.get('subdomain'),
  )
  const headerTenant = normalizeIdentifier(proxy.tenant)
  const explicitTenant = canUseExplicitTenant(headerTenant, rawHostname)
    ? headerTenant
    : canUseExplicitTenant(queryTenant, rawHostname)
      ? queryTenant
      : ''
  const basepath = normalizeBasepath(
    proxy.basepath ||
      proxy.forwardedPrefix ||
      searchParams.get('helpin_basepath'),
  )

  if (explicitTenant) {
    return { subdomain: explicitTenant, basepath }
  }

  const hostedSubdomain = resolveHostedSubdomain(hostname)
  if (hostedSubdomain) {
    const overrideParam = queryTenant === hostedSubdomain ? queryTenant : ''
    return { subdomain: overrideParam || hostedSubdomain, basepath }
  }

  if (isLocalHostname(hostname)) {
    return {
      subdomain:
        queryTenant || normalizeIdentifier(process.env.VITE_HC_SUBDOMAIN) || 'demo',
      basepath,
    }
  }

  // Custom domain — pass hostname through; backend resolves it.
  return { subdomain: hostname, basepath }
}

const requestContextStorage = new AsyncLocalStorage()

// Exposed to the bundled SSR entry (router.tsx) so it can read per-request
// context without importing any server-only modules.
globalThis.__hcGetRequestContext__ = () => requestContextStorage.getStore() ?? null


async function fetchHelpCenterConfig(apiBase, subdomain) {
  if (!apiBase || !subdomain) return null
  const configRes = await fetch(`${apiBase}/hc/${subdomain}/config`)
  if (!configRes.ok) throw new Error(`config fetch failed: ${configRes.status}`)
  return configRes.json()
}

async function handleRobotsTxt(_request, response, hcContext) {
  const apiBase = process.env.INTERNAL_API_URL
  let publicUrl = { origin: hcContext.origin, basepath: hcContext.basepath }
  try {
    const config = await fetchHelpCenterConfig(apiBase, hcContext.subdomain)
    if (config) {
      publicUrl = resolvePublicUrlParts(hcContext, config)
    }
  } catch (error) {
    console.error('failed to resolve robots public URL', error)
  }
  const body = renderRobotsTxt(publicUrl)
  response.statusCode = 200
  response.setHeader('Cache-Control', 'public, max-age=3600')
  response.setHeader('Content-Type', 'text/plain; charset=utf-8')
  response.end(body)
}

async function handleSitemapXml(_request, response, hcContext) {
  try {
    const { subdomain } = hcContext
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
    const config = await fetchHelpCenterConfig(apiBase, subdomain)
    const locales = config.enabled_locales?.length > 1
      ? config.enabled_locales
      : [config.default_locale || 'en']
    const multilingual = config.enabled_locales?.length > 1
    const publicUrl = resolvePublicUrlParts(hcContext, config)
    const navigationByLocale = new Map()

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

        navigationByLocale.set(locale, [
          ...(navigationByLocale.get(locale) || []),
          ...navigation,
        ])
      }
    }

    const entries = collectSitemapEntries({ publicUrl, config, navigationByLocale })
    const body = renderSitemapXml(entries)

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
 * Strip the resolved basepath from a pathname so we can match top-level routes
 * like /sitemap.xml or /robots.txt. Hosted and custom-domain help centers use
 * an empty basepath, but the helper remains safe for any future prefixed mode.
 */
function stripBasepath(pathname, basepath) {
  if (!basepath) return pathname
  if (pathname === basepath) return '/'
  if (pathname.startsWith(`${basepath}/`)) {
    return pathname.slice(basepath.length) || '/'
  }
  return pathname
}

async function maybeResolvePublicRedirect(request, routeUrl, hcContext) {
  if (!shouldAttemptRedirectResolution(request.method, routeUrl.pathname)) {
    return null
  }

  const apiBase = process.env.INTERNAL_API_URL
  if (!apiBase || !hcContext.subdomain) {
    return null
  }

  const cacheKey = getRedirectCacheKey(hcContext, routeUrl)
  const cached = getCachedRedirect(cacheKey)
  if (cached.hit) {
    return cached.redirect
  }

  try {
    const redirect = await resolvePublicRedirect({
      apiBase,
      subdomain: hcContext.subdomain,
      pathname: routeUrl.pathname,
      basepath: hcContext.basepath,
      search: routeUrl.search,
    })
    setCachedRedirect(cacheKey, redirect)
    return redirect
  } catch (error) {
    console.error('help-center redirect resolution failed', {
      path: routeUrl.pathname,
      host: hcContext.host,
      error,
    })
    return null
  }
}

function toAbsoluteLocation(origin, location) {
  if (/^[a-z]+:/i.test(location) || location.startsWith('//')) {
    return location
  }
  return `${origin}${location}`
}

async function handleRequest(request, response) {
  const requestStartedAt = performance.now()
  if (request.url === '/healthz') {
    response.statusCode = 200
    response.setHeader('Cache-Control', 'no-store')
    response.setHeader('Content-Type', 'text/plain; charset=utf-8')
    response.end('ok')
    return
  }

  const { host, rawHost, protocol, origin } = resolveHostInfo(request)
  const url = new URL(request.url || '/', `${protocol}://${host}`)
  const resolved = resolveHelpCenterContext(host, url.pathname, url.search, {
    rawHost,
    tenant: request.headers['x-helpin-hc-tenant'],
    basepath: request.headers['x-helpin-hc-basepath'],
    forwardedPrefix: request.headers['x-forwarded-prefix'],
  })
  const hcContext = {
    host: normalizeHostname(host),
    rawHost: normalizeHostname(rawHost),
    protocol,
    origin,
    pathname: url.pathname,
    search: url.search,
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

    if (await tryServeStatic(request, routeUrl, response, hcContext.basepath)) {
      return
    }

    if (isApiRequest(routeUrl)) {
      // The public help-center is not a proxy for staff or internal APIs.
      let decodedPath
      try { decodedPath = decodeURIComponent(routeUrl.pathname) } catch { decodedPath = '' }
      if (!decodedPath.startsWith('/api/hc/') || decodedPath.includes('..') || decodedPath.includes('\\')) {
        response.statusCode = 404
        response.end('Not found')
        return
      }
      await proxyApiRequest(request, response, routeUrl)
      return
    }

    const redirect = await maybeResolvePublicRedirect(request, routeUrl, hcContext)
    if (redirect) {
      response.statusCode = redirect.status
      response.setHeader('Cache-Control', 'public, max-age=300')
      response.setHeader('Location', toAbsoluteLocation(hcContext.origin, redirect.location))
      response.end()
      return
    }

    const cacheKey = getCacheKey(url, request, hcContext)

    if (isHtmlRequest(request, routeUrl)) {
      const cached = getCachedResponse(cacheKey)
      if (cached) {
        response.setHeader('X-Helpin-Cache', 'L1')
        response.setHeader(
          'Server-Timing',
          `cache;desc="L1";dur=${(performance.now() - requestStartedAt).toFixed(1)}`,
        )
        await writeCachedResponse(request, response, cached)
        return
      }
      if (sharedRenderCache) {
        const shared = await sharedRenderCache.get(cacheKey)
        if (shared) {
          shared.expiresAt = Date.now() + HTML_CACHE_TTL_MS
          setCachedResponse(cacheKey, shared)
          response.setHeader('X-Helpin-Cache', 'L2')
          response.setHeader(
            'Server-Timing',
            `cache;desc="L2";dur=${(performance.now() - requestStartedAt).toFixed(1)}`,
          )
          await writeCachedResponse(request, response, shared)
          return
        }
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

    if (responseType.includes('text/html') && fetchResponse.ok) {
      const renderedBody = await fetchResponse.text()
      const body = prefixAssetUrls(renderedBody, hcContext.basepath)
      const headersToCache = Array.from(fetchResponse.headers.entries())
        .filter(([name]) => !['cache-control', 'content-encoding', 'content-length'].includes(name))
      headersToCache.push(['cache-control', 'public, max-age=0, must-revalidate'])
      if (isHtmlRequest(request, routeUrl)) {
        const cacheEntry = {
          body,
          expiresAt: Date.now() + HTML_CACHE_TTL_MS,
          headers: headersToCache,
          identifier: hcContext.subdomain,
          status: fetchResponse.status,
        }
        setCachedResponse(cacheKey, cacheEntry)
        if (sharedRenderCache) {
          await sharedRenderCache.set(cacheKey, cacheEntry, hcContext.subdomain)
        }
      }

      response.setHeader('X-Helpin-Cache', 'MISS')
      response.statusCode = fetchResponse.status
      response.setHeader(
        'Server-Timing',
        `ssr;desc="MISS";dur=${(performance.now() - requestStartedAt).toFixed(1)}`,
      )
      for (const [name, value] of headersToCache) {
        response.setHeader(name, value)
      }
      await writeCompressedBody(request, response, body)
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

// Node runs as PID 1 in the container, where signals without a handler are
// ignored; without this, `docker stop` waits for the timeout and kills us.
function shutdown(signal) {
  console.log(`help-center received ${signal}, shutting down`)
  server.close(() => process.exit(0))
  server.closeIdleConnections?.()
  setTimeout(() => process.exit(0), 5000).unref()
}
process.once('SIGTERM', () => shutdown('SIGTERM'))
process.once('SIGINT', () => shutdown('SIGINT'))
