import fs from 'node:fs'
import http from 'node:http'
import net from 'node:net'
import os from 'node:os'
import path from 'node:path'
import { afterEach, describe, expect, it } from 'vitest'
import {
  createPortalMount,
  injectPortalContext,
  isPortalPagePath,
  portalApiSlug,
  rewriteCookiePath,
} from './portalMount.mjs'

const servers = []
const sockets = []
afterEach(async () => {
  // Upgraded sockets keep a server open until they are destroyed.
  sockets.splice(0).forEach((socket) => socket.destroy())
  await Promise.all(servers.splice(0).map((server) => new Promise((resolve) => server.close(resolve))))
})

function portalBuild() {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'portal-'))
  fs.mkdirSync(path.join(dir, 'assets'))
  fs.writeFileSync(path.join(dir, 'portal.html'), '<html><head><title>x</title><!-- helpin-portal-context --></head><body></body></html>')
  fs.writeFileSync(path.join(dir, 'assets', 'portal.js'), 'console.log(1)')
  fs.writeFileSync(path.join(dir, 'secret.txt'), 'nope')
  return dir
}

function lookupFetch(slugs, calls = []) {
  return async (url) => {
    calls.push(String(url))
    const identifier = decodeURIComponent(String(url).split('/hc/')[1].split('/')[0])
    if (identifier === 'broken') return new Response('down', { status: 503 })
    return slugs[identifier]
      ? Response.json({ slug: slugs[identifier] })
      : Response.json({ error: 'portal not found' }, { status: 404 })
  }
}

function fakeResponse() {
  const headers = {}
  return {
    statusCode: 0,
    headers,
    body: '',
    setHeader(name, value) { headers[name.toLowerCase()] = value },
    getHeader(name) { return headers[name.toLowerCase()] },
    end(body) { if (body) this.body += String(body) },
  }
}

const helpers = {
  serveStaticFile: async (_request, response, filePath) => { response.statusCode = 200; response.end(`file:${path.basename(filePath)}`) },
  writeBody: async (_request, response, body) => response.end(body),
}

describe('portal mount paths', () => {
  it('claims /requests pages and portal API paths only', () => {
    expect(isPortalPagePath('/requests')).toBe(true)
    expect(isPortalPagePath('/requests/req_1')).toBe(true)
    expect(isPortalPagePath('/requestsx')).toBe(false)
    expect(isPortalPagePath('/en/requests')).toBe(false)
    expect(portalApiSlug('/api/public/portal/acme/requests')).toBe('acme')
    expect(portalApiSlug('/api/public/portal/acme')).toBe('acme')
    expect(portalApiSlug('/api/hc/acme/config')).toBeNull()
  })

  it('injects the workspace and a base for relative assets, safely', () => {
    const html = injectPortalContext('<head><!-- helpin-portal-context --></head>', { slug: 'acme</script>', basepath: '/docs' })
    expect(html).toContain('<base href="/docs/requests/" />')
    expect(html).toContain('window.__HELPIN_PORTAL__={"slug":"acme\\u003c/script\\u003e","basepath":"/docs"}')
    expect(html).not.toContain('acme</script>')
  })

  it('scopes the session cookie to a subpath help center', () => {
    const cookie = 'helpin_portal_session=abc; Path=/api/public/portal/acme; HttpOnly; Secure; SameSite=Lax'
    expect(rewriteCookiePath(cookie, '')).toBe(cookie)
    expect(rewriteCookiePath(cookie, '/docs')).toBe('helpin_portal_session=abc; Path=/docs/api/public/portal/acme; HttpOnly; Secure; SameSite=Lax')
  })
})

describe('portal pages', () => {
  it('serves the portal for the host’s workspace and caches the lookup', async () => {
    const calls = []
    const mount = createPortalMount({ apiBase: 'http://api.internal/api', portalDir: portalBuild(), fetchImpl: lookupFetch({ 'acme-help': 'acme' }, calls) })
    const response = fakeResponse()
    await mount.handlePage({}, response, '/requests/req_1', { subdomain: 'acme-help', basepath: '' }, helpers)
    expect(response.statusCode).toBe(200)
    expect(response.body).toContain('window.__HELPIN_PORTAL__={"slug":"acme","basepath":""}')
    expect(response.headers['cache-control']).toBe('no-store')
    await mount.handlePage({}, fakeResponse(), '/requests', { subdomain: 'acme-help', basepath: '' }, helpers)
    expect(calls).toEqual(['http://api.internal/api/hc/acme-help/portal'])
  })

  it('answers 404 where no portal is served', async () => {
    const mount = createPortalMount({ apiBase: 'http://api.internal/api', portalDir: portalBuild(), fetchImpl: lookupFetch({}) })
    const response = fakeResponse()
    await mount.handlePage({}, response, '/requests', { subdomain: 'nobody', basepath: '' }, helpers)
    expect(response.statusCode).toBe(404)
    await expect(mount.slugFor('broken')).rejects.toThrow('portal lookup failed')
  })

  it('serves build assets without escaping the assets folder', async () => {
    const mount = createPortalMount({ apiBase: 'http://api.internal/api', portalDir: portalBuild(), fetchImpl: lookupFetch({}) })
    const asset = fakeResponse()
    await mount.handlePage({}, asset, '/requests/assets/portal.js', { subdomain: 'x', basepath: '' }, helpers)
    expect(asset.body).toBe('file:portal.js')
    const escape = fakeResponse()
    await mount.handlePage({}, escape, '/requests/assets/../secret.txt', { subdomain: 'x', basepath: '' }, helpers)
    expect(escape.statusCode).toBe(404)
  })
})

describe('portal API proxy', () => {
  it('proxies only the host’s own portal and scopes its cookie', async () => {
    const forwarded = []
    const fetchImpl = async (url, init) => {
      if (String(url).includes('/hc/')) return lookupFetch({ 'acme-help': 'acme' })(url)
      forwarded.push({ url: String(url), method: init.method })
      const headers = new Headers({ 'content-type': 'application/json' })
      headers.append('set-cookie', 'helpin_portal_session=abc; Path=/api/public/portal/acme; HttpOnly')
      return new Response('{"ok":true}', { status: 200, headers })
    }
    const mount = createPortalMount({ apiBase: 'http://api.internal/api', portalDir: portalBuild(), fetchImpl })
    const context = { subdomain: 'acme-help', basepath: '/docs' }
    expect(await mount.allowedApiPath('/api/public/portal/acme/session', context)).toBe(true)
    expect(await mount.allowedApiPath('/api/public/portal/globex/session', context)).toBe(false)

    const response = fakeResponse()
    await mount.proxyApi({ method: 'POST' }, response, new URL('http://help.acme.com/api/public/portal/acme/auth/exchange?x=1'), context, {
      buildHeaders: () => new Headers(),
    })
    expect(forwarded).toEqual([{ url: 'http://api.internal/api/public/portal/acme/auth/exchange?x=1', method: 'POST' }])
    expect(response.statusCode).toBe(200)
    expect(response.body).toBe('{"ok":true}')
    expect(response.headers['set-cookie']).toEqual(['helpin_portal_session=abc; Path=/docs/api/public/portal/acme; HttpOnly'])
  })

  it('tunnels the live-update socket for the host’s own portal only', async () => {
    const api = http.createServer((_request, response) => response.end())
    let upstreamHost = ''
    api.on('upgrade', (request, socket) => {
      sockets.push(socket)
      upstreamHost = request.headers.host
      socket.write('HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n\r\n')
      socket.on('data', (data) => socket.write(`echo:${data}`))
    })
    await new Promise((resolve) => api.listen(0, '127.0.0.1', resolve))
    servers.push(api)
    const apiBase = `http://127.0.0.1:${api.address().port}/api`
    const mount = createPortalMount({ apiBase, portalDir: portalBuild(), fetchImpl: lookupFetch({ 'acme-help': 'acme' }) })
    const front = http.createServer()
    front.on('upgrade', (request, socket, head) => {
      sockets.push(socket)
      const internalPath = new URL(request.url, 'http://x').pathname
      void mount.proxyUpgrade(request, socket, head, internalPath, { subdomain: 'acme-help', basepath: '' })
    })
    await new Promise((resolve) => front.listen(0, '127.0.0.1', resolve))
    servers.push(front)

    const open = (pathname) => new Promise((resolve) => {
      const client = net.connect(front.address().port, '127.0.0.1', () => {
        client.write(`GET ${pathname} HTTP/1.1\r\nHost: help.acme.com\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n\r\n`)
      })
      let received = ''
      client.on('data', (data) => {
        received += data
        if (received.includes('101') && !received.includes('echo')) client.write('ping')
        if (received.includes('echo:ping') || received.includes('404')) { client.destroy(); resolve(received) }
      })
      client.on('close', () => resolve(received))
    })
    const own = await open('/api/public/portal/acme/ws')
    expect(own).toContain('101 Switching Protocols')
    expect(own).toContain('echo:ping')
    expect(upstreamHost).toBe('help.acme.com')
    expect(await open('/api/public/portal/globex/ws')).toContain('404 Not Found')
  })
})
