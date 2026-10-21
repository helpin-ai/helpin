import fs from 'node:fs'
import http from 'node:http'
import https from 'node:https'
import path from 'node:path'

/**
 * The customer portal is mounted on each help center at /requests. This
 * module serves the portal-only build (built from frontend/ into ./portal),
 * tells it which workspace's portal the host serves, and proxies that
 * portal's API (including its live-update socket) on the help center origin
 * so the portal session cookie stays first-party.
 */

export const PORTAL_MOUNT_PATH = '/requests'
const PORTAL_API = /^\/api\/public\/portal\/([^/]+)(\/.*)?$/
const PORTAL_SOCKET = /^\/api\/public\/portal\/([^/]+)\/ws$/
const CONTEXT_MARKER = '<!-- helpin-portal-context -->'
const SLUG_CACHE_TTL_MS = 60_000
const SLUG_CACHE_LIMIT = 5_000

/** isPortalPagePath reports paths the portal app serves (below the help center base path). */
export function isPortalPagePath(internalPath) {
  return internalPath === PORTAL_MOUNT_PATH || internalPath.startsWith(`${PORTAL_MOUNT_PATH}/`)
}

/** portalApiSlug returns the slug of a portal API path, or null. */
export function portalApiSlug(internalPath) {
  const match = PORTAL_API.exec(internalPath)
  if (!match) return null
  try {
    return decodeURIComponent(match[1])
  } catch {
    return null
  }
}

function escapeForScript(value) {
  return JSON.stringify(value).replace(/</g, '\\u003c').replace(/>/g, '\\u003e').replace(/&/g, '\\u0026')
}

function escapeAttribute(value) {
  return value.replace(/&/g, '&amp;').replace(/"/g, '&quot;').replace(/</g, '&lt;')
}

/**
 * injectPortalContext adds the portal's workspace and a <base> for its
 * relative asset URLs to the portal page.
 */
export function injectPortalContext(html, { slug, basepath }) {
  const context = `<base href="${escapeAttribute(`${basepath}${PORTAL_MOUNT_PATH}/`)}" />` +
    `<script>window.__HELPIN_PORTAL__=${escapeForScript({ slug, basepath })}</script>`
  return html.includes(CONTEXT_MARKER) ? html.replace(CONTEXT_MARKER, context) : html.replace('</head>', `${context}</head>`)
}

/**
 * rewriteCookiePath scopes the portal session cookie to the help center's
 * base path when the help center is served under a subpath.
 */
export function rewriteCookiePath(cookie, basepath) {
  if (!basepath) return cookie
  return cookie.replace(/(;\s*Path=)(\/api\/public\/portal\/)/i, `$1${basepath}$2`)
}

/**
 * createPortalMount wires the portal to a help center server. apiBase is the
 * internal API root (INTERNAL_API_URL); portalDir holds the portal build.
 */
export function createPortalMount({ apiBase, portalDir, fetchImpl = fetch, now = Date.now }) {
  const slugs = new Map()
  let pageTemplate = null

  function readTemplate() {
    if (pageTemplate === null) {
      const file = path.join(portalDir, 'portal.html')
      pageTemplate = fs.existsSync(file) ? fs.readFileSync(file, 'utf8') : ''
    }
    return pageTemplate
  }

  /** slugFor returns the portal slug the help center identifier serves, or null. */
  async function slugFor(identifier) {
    if (!apiBase || !identifier) return null
    const cached = slugs.get(identifier)
    if (cached && cached.expiresAt > now()) return cached.slug
    const response = await fetchImpl(`${apiBase.replace(/\/+$/, '')}/hc/${encodeURIComponent(identifier)}/portal`)
    let slug = null
    if (response.ok) {
      const body = await response.json()
      slug = typeof body?.slug === 'string' && body.slug ? body.slug : null
    } else if (response.status !== 404) {
      throw new Error(`portal lookup failed: ${response.status}`)
    }
    if (slugs.size >= SLUG_CACHE_LIMIT) slugs.clear()
    slugs.set(identifier, { slug, expiresAt: now() + SLUG_CACHE_TTL_MS })
    return slug
  }

  /** serveAsset serves a file of the portal build, or returns false. */
  async function serveAsset(request, response, internalPath, serveStaticFile) {
    const relative = internalPath.slice(PORTAL_MOUNT_PATH.length)
    if (!relative.startsWith('/assets/')) return false
    const filePath = path.join(portalDir, path.normalize(relative))
    if (!filePath.startsWith(path.join(portalDir, 'assets') + path.sep) || !fs.existsSync(filePath)) {
      response.statusCode = 404
      response.end('Not found')
      return true
    }
    await serveStaticFile(request, response, filePath)
    return true
  }

  /** handlePage serves the portal app for the host's workspace. */
  async function handlePage(request, response, internalPath, hcContext, { serveStaticFile, writeBody }) {
    if (await serveAsset(request, response, internalPath, serveStaticFile)) return
    const template = readTemplate()
    const slug = template ? await slugFor(hcContext.subdomain) : null
    response.setHeader('Cache-Control', 'no-store')
    response.setHeader('Content-Type', 'text/html; charset=utf-8')
    response.setHeader('X-Robots-Tag', 'noindex, nofollow')
    if (!slug) {
      response.statusCode = 404
      response.end('<!doctype html><meta charset="utf-8"><title>Not found</title><p>This support portal isn’t available.</p>')
      return
    }
    response.statusCode = 200
    await writeBody(request, response, injectPortalContext(template, { slug, basepath: hcContext.basepath }))
  }

  /**
   * allowedApiPath reports whether a portal API path belongs to the portal
   * this host serves; one help center never proxies another workspace's portal.
   */
  async function allowedApiPath(internalPath, hcContext) {
    const slug = portalApiSlug(internalPath)
    return Boolean(slug) && slug === (await slugFor(hcContext.subdomain))
  }

  /** proxyApi forwards a portal API request to the API server. */
  async function proxyApi(request, response, routeUrl, hcContext, { buildHeaders, readBody }) {
    const target = new URL(apiBase)
    target.pathname = `${target.pathname.replace(/\/+$/, '')}${routeUrl.pathname.replace(/^\/api/, '')}`
    target.search = routeUrl.search
    const upstream = await fetchImpl(target, {
      method: request.method,
      headers: buildHeaders(request),
      body: readBody ? readBody(request) : undefined,
      duplex: 'half',
      redirect: 'manual',
    })
    response.statusCode = upstream.status
    for (const [name, value] of upstream.headers.entries()) {
      if (name === 'set-cookie' || name === 'content-encoding' || name === 'content-length' || name === 'transfer-encoding') continue
      response.setHeader(name, value)
    }
    const cookies = upstream.headers.getSetCookie?.() ?? []
    if (cookies.length) response.setHeader('Set-Cookie', cookies.map((cookie) => rewriteCookiePath(cookie, hcContext.basepath)))
    response.setHeader('Cache-Control', upstream.headers.get('cache-control') || 'no-store')
    const body = upstream.body ? Buffer.from(await upstream.arrayBuffer()) : null
    response.end(body)
  }

  /**
   * proxyUpgrade tunnels the portal's live-update socket to the API server.
   * The original Host is kept so the API's same-origin check passes.
   */
  async function proxyUpgrade(request, socket, head, internalPath, hcContext) {
    const match = PORTAL_SOCKET.exec(internalPath)
    if (!match || !apiBase || !(await allowedApiPath(internalPath, hcContext))) {
      socket.end('HTTP/1.1 404 Not Found\r\nConnection: close\r\nContent-Length: 0\r\n\r\n')
      return
    }
    const target = new URL(apiBase)
    const transport = target.protocol === 'https:' ? https : http
    const upstream = transport.request({
      hostname: target.hostname,
      port: target.port || (target.protocol === 'https:' ? 443 : 80),
      path: `${target.pathname.replace(/\/+$/, '')}${internalPath.replace(/^\/api/, '')}`,
      method: 'GET',
      headers: request.headers,
    })
    const writeHead = (response) => {
      let head = `HTTP/1.1 ${response.statusCode} ${response.statusMessage}\r\n`
      for (let i = 0; i < response.rawHeaders.length; i += 2) head += `${response.rawHeaders[i]}: ${response.rawHeaders[i + 1]}\r\n`
      socket.write(`${head}\r\n`)
    }
    upstream.on('upgrade', (response, upstreamSocket, upstreamHead) => {
      writeHead(response)
      if (upstreamHead?.length) socket.write(upstreamHead)
      if (head?.length) upstreamSocket.write(head)
      upstreamSocket.pipe(socket)
      socket.pipe(upstreamSocket)
      upstreamSocket.on('error', () => socket.destroy())
      socket.on('error', () => upstreamSocket.destroy())
    })
    upstream.on('response', (response) => {
      // A refused upgrade (such as 401) keeps its status, then closes.
      response.resume()
      socket.end(`HTTP/1.1 ${response.statusCode} ${response.statusMessage}\r\nConnection: close\r\nContent-Length: 0\r\n\r\n`)
    })
    upstream.on('error', () => socket.destroy())
    upstream.end()
  }

  return { slugFor, handlePage, allowedApiPath, proxyApi, proxyUpgrade }
}
