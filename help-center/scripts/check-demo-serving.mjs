import http from 'node:http'
import { spawn } from 'node:child_process'
import assert from 'node:assert/strict'
import { fileURLToPath } from 'node:url'

const api = http.createServer((request, response) => {
  const url = new URL(request.url, 'http://localhost')
  const demo = !url.pathname.includes('/normal/')
  const config = {
    subdomain: demo ? 'demo' : 'normal',
    brand_name: 'Example', brand_color: '#000000', is_published: true,
    default_locale: 'en', enabled_locales: ['en'], header_links: [],
    footer_config: {}, homepage_config: {}, space_nav_config: {},
    demo_mode: demo, search_indexing_enabled: !demo,
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
  for (const tenant of ['demo', 'normal']) {
    const headers = { 'x-forwarded-host': `${tenant}.helpin.center` }
    const head = await fetch(`http://127.0.0.1:${port}/`, { headers, method: 'HEAD' })
    assert.equal(head.status, 200)
    assert.equal(head.headers.get('x-robots-tag'), tenant === 'demo' ? 'noindex' : null)
    for (const pass of ['cold', 'warm']) {
      const response = await fetch(`http://127.0.0.1:${port}/`, { headers })
      const body = await response.text()
      assert.equal(response.status, 200, body.slice(0, 300))
      assert.equal(response.headers.get('x-robots-tag'), tenant === 'demo' ? 'noindex' : null)
      assert.equal(body.includes('Demo built by Helpin'), tenant === 'demo')
      if (tenant === 'demo') assert(body.includes('https://docs.example.com'))
      console.log(tenant, pass, 'HTML/header/banner PASS')
    }
    for (const path of ['/robots.txt', '/sitemap.xml', '/llms.txt']) {
      const response = await fetch(`http://127.0.0.1:${port}${path}`, { headers })
      const body = await response.text()
      if (tenant === 'demo') {
        assert.equal(response.status, path === '/robots.txt' ? 200 : 404)
        assert(!body.includes('Sitemap:'))
        if (path !== '/robots.txt') assert.equal(response.headers.get('x-robots-tag'), 'noindex')
      } else {
        assert.equal(response.status, 200)
      }
      console.log(tenant, path, response.status, 'PASS')
    }
  }
} finally {
  child.kill('SIGTERM')
  api.close()
}
