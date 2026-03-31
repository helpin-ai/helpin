import http from 'node:http'
import { Readable } from 'node:stream'
import serverEntry from './dist/server/server.js'

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

async function handleRequest(request, response) {
  if (request.url === '/healthz') {
    response.statusCode = 200
    response.setHeader('Cache-Control', 'no-store')
    response.setHeader('Content-Type', 'text/plain; charset=utf-8')
    response.end('ok')
    return
  }

  const forwardedHost = normalizeHeaderValue(request.headers['x-forwarded-host'])
  const host = forwardedHost || normalizeHeaderValue(request.headers.host) || 'localhost'
  const forwardedProto = normalizeHeaderValue(request.headers['x-forwarded-proto'])
  const protocol = forwardedProto || 'http'
  const url = new URL(request.url || '/', `${protocol}://${host}`)
  const cacheKey = getCacheKey(url, request)

  if (isApiRequest(url)) {
    await proxyApiRequest(request, response, url)
    return
  }

  if (isHtmlRequest(request, url)) {
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

  const fetchRequest = new Request(url, {
    method: request.method,
    headers,
    body: shouldReadBody(request.method || 'GET') ? Readable.toWeb(request) : undefined,
    duplex: 'half',
  })

  const fetchResponse = await serverEntry.fetch(fetchRequest)
  const responseType = fetchResponse.headers.get('content-type') || ''

  if (isHtmlRequest(request, url) && responseType.includes('text/html') && fetchResponse.ok) {
    const body = await fetchResponse.text()
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

  await writeFetchResponse(response, fetchResponse, url)
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
