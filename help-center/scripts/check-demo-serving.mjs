import http from 'node:http'
import { spawn } from 'node:child_process'
import assert from 'node:assert/strict'
import { fileURLToPath } from 'node:url'

const failures = new Map()
const api = http.createServer((request, response) => {
  const url = new URL(request.url, 'http://localhost')
  const tenant = url.pathname.split('/')[3]
  const failure = failures.get(tenant)
  if (failure && /\/(config|bootstrap)$/.test(url.pathname)) {
    if (failure === 'network') return request.socket.destroy()
    if (failure === 'timeout') return
    response.statusCode = typeof failure === 'number' ? failure : 200
    response.end(failure === 'invalid' ? '{' : 'null')
    return
  }
  const demo = tenant === 'demo'
  const config = {
    subdomain: tenant,
    brand_name: 'Example', brand_color: '#000000', is_published: true,
    default_locale: 'en', enabled_locales: ['en'], header_links: [],
    footer_config: {}, homepage_config: {}, space_nav_config: {},
    demo_mode: demo, search_indexing_enabled: tenant !== 'private',
    official_docs_url: 'https://docs.example.com', chat_widget_enabled: false,
  }
  response.setHeader('Content-Type', 'application/json')
  if (url.pathname.endsWith('/bootstrap')) {
    response.end(JSON.stringify({ config, locale: 'en', spaces: [] }))
  } else if (url.pathname.endsWith('/config')) {
    response.end(JSON.stringify(config))
  } else if (url.pathname.includes('/redirect')) {
    response.statusCode = 404
    response.end('{}')
  } else {
    response.end('[]')
  }
})
await new Promise((resolve, reject) => {
  api.once('error', reject)
  api.listen(0, '127.0.0.1', resolve)
})
const port = Number(process.env.DEMO_TEST_PORT || 3198)
const child = spawn(process.execPath, ['serve.mjs'], {
  cwd: fileURLToPath(new URL('..', import.meta.url)),
  env: {
    ...process.env,
    PORT: String(port),
    INTERNAL_API_URL: `http://127.0.0.1:${api.address().port}/api`,
  },
  stdio: ['ignore', 'pipe', 'pipe'],
})
child.stderr.on('data', data => process.stderr.write(data))
try {
  await new Promise((resolve, reject) => {
    const timer = setTimeout(() => reject(Error('server startup timed out')), 15000)
    timer.unref()
    child.stdout.on('data', data => {
      if (String(data).includes('listening')) {
        clearTimeout(timer)
        resolve()
      }
    })
    child.once('error', reject)
    child.once('exit', code => reject(Error(`server exited ${code}`)))
  })
  for (const tenant of ['demo', 'normal', 'private']) {
    const noindex = tenant !== 'normal'
    const headers = { 'x-forwarded-host': `${tenant}.helpin.center` }
    const head = await fetch(`http://127.0.0.1:${port}/`, { headers, method: 'HEAD' })
    assert.equal(head.status, 200)
    assert.equal(head.headers.get('x-robots-tag'), noindex ? 'noindex' : null)
    for (const pass of ['cold', 'warm']) {
      const response = await fetch(`http://127.0.0.1:${port}/`, { headers })
      const body = await response.text()
      assert.equal(response.status, 200, body.slice(0, 300))
      assert.equal(response.headers.get('x-robots-tag'), noindex ? 'noindex' : null)
      assert.equal(body.includes('Demo built by Helpin'), tenant === 'demo')
      if (tenant === 'demo') assert(body.includes('https://docs.example.com'))
      console.log(tenant, pass, 'HTML/header/banner PASS')
    }
    for (const path of ['/robots.txt', '/sitemap.xml', '/llms.txt']) {
      const response = await fetch(`http://127.0.0.1:${port}${path}`, { headers })
      const body = await response.text()
      if (noindex) {
        assert.equal(response.status, path === '/robots.txt' ? 200 : 404)
        assert(!body.includes('Sitemap:'))
        if (path !== '/robots.txt') assert.equal(response.headers.get('x-robots-tag'), 'noindex')
      } else {
        assert.equal(response.status, 200)
      }
      console.log(tenant, path, response.status, 'PASS')
    }
  }
  for (const [tenant, failure, status] of [
    ['unavailable', 503, 200], ['network', 'network', 200], ['timeout', 'timeout', 200],
    ['invalid', 'invalid', 200], ['missing', 404, 404], ['empty', 'empty', 404],
  ]) {
    failures.set(tenant, failure)
    const headers = { 'x-forwarded-host': `${tenant}.helpin.center` }
    for (const method of ['GET', 'HEAD']) {
      const response = await fetch(`http://127.0.0.1:${port}/robots.txt`, { headers, method })
      const body = await response.text()
      assert.equal(response.status, status, `${tenant} robots status`)
      assert.equal(response.headers.get('cache-control'), 'no-store')
      assert.match(response.headers.get('content-type'), /text\/plain/)
      assert(!body.includes('Sitemap:'))
      if (status === 200 && method === 'GET') {
        assert(body.includes('User-agent: *'))
        assert(body.includes('Disallow: /preview/'))
        assert(!/^Disallow: \/\s*$/m.test(body))
      }
    }
    if (status === 404) {
      for (const path of ['/sitemap.xml', '/llms.txt']) {
        assert.equal((await fetch(`http://127.0.0.1:${port}${path}`, { headers })).status, 404)
      }
    }
    failures.delete(tenant)
    const recovered = await fetch(`http://127.0.0.1:${port}/robots.txt`, { headers })
    assert.equal(recovered.status, 200)
    assert((await recovered.text()).includes('Sitemap:'))
    console.log(tenant, 'robots failure/recovery PASS')
  }
  const headers = { 'x-forwarded-host': 'recovery.helpin.center' }
  failures.set('recovery', 503)
  const unavailable = await fetch(`http://127.0.0.1:${port}/`, { headers })
  assert.equal(unavailable.status, 500)
  assert.equal(unavailable.headers.get('x-robots-tag'), null)
  assert(!/<meta[^>]*name="robots"[^>]*content="noindex"/.test(await unavailable.text()))
  failures.delete('recovery')
  for (const pass of ['cold', 'warm']) {
    const response = await fetch(`http://127.0.0.1:${port}/`, { headers })
    assert.equal(response.status, 200)
    assert.equal(response.headers.get('x-helpin-cache'), pass === 'cold' ? 'MISS' : 'L1')
    assert.equal(response.headers.get('x-robots-tag'), null)
    assert(!/<meta[^>]*name="robots"[^>]*content="noindex"/.test(await response.text()))
    console.log(pass, 'HTML recovery without noindex PASS')
  }
} finally {
  child.kill('SIGTERM')
  api.close()
}
