import { describe, expect, it, vi } from 'vitest'
import {
  handleShareMetadata,
  injectShareMetadata,
  resolvePublicApiBase,
} from '../share-meta'

const defaultHtml = `<!doctype html><html><head>
<!-- helpin-meta:start -->
<title>Helpin default</title>
<meta property="og:title" content="Helpin default" />
<!-- helpin-meta:end -->
</head><body><div id="root"></div></body></html>`

describe('app link edge metadata', () => {
  it('uses public APIs only for known app hosts', () => {
    expect(resolvePublicApiBase('app.helpin.ai')).toBe('https://api.helpin.ai/api')
    expect(resolvePublicApiBase('app.stage.helpin.ai')).toBe('https://api.stage.helpin.ai/api')
    expect(resolvePublicApiBase('localhost')).toBe('http://localhost:8080/api')
    expect(resolvePublicApiBase('deploy-preview-42--helpin.netlify.app')).toBeNull()
  })

  it('inserts the escaped Go metadata block while preserving the SPA shell', () => {
    const head = `<title>Roadmap &amp; launch</title>
<meta name="description" content="Coordinate the upcoming release." />
<meta property="og:image" content="https://app.helpin.ai/api/link-previews/image?path=%2Fshare%2Ftoken" />`
    const html = injectShareMetadata({ html: defaultHtml, head })
    expect(html).toContain(head)
    expect(html).not.toContain('Helpin default')
    expect(html).toContain('<div id="root"></div>')
    expect(html.match(/<!-- helpin-meta:start -->/g)).toHaveLength(1)
    expect(html.match(/<!-- helpin-meta:end -->/g)).toHaveLength(1)
  })

  it('fetches the allowlisted preview response for a normal app URL', async () => {
    const next = vi.fn(async () => new Response(defaultHtml, {
      status: 200,
      headers: { 'content-type': 'text/html; charset=utf-8', 'x-origin': 'spa' },
    }))
    const fetcher = vi.fn(async () => new Response(JSON.stringify({
      head: '<title>Launch &amp; learn</title>',
      content: { text: 'secret body must never enter metadata' },
    }), { status: 200, headers: { 'content-type': 'application/json' } }))

    const response = await handleShareMetadata(
      new Request('https://app.helpin.ai/w/demo/pm/tasks/task-id?task=DEMO-12'),
      { next },
      fetcher,
    )
    const html = await response.text()

    expect(next).toHaveBeenCalledOnce()
    expect(fetcher).toHaveBeenCalledOnce()
    expect(fetcher.mock.calls[0]?.[0]).toBe('https://api.helpin.ai/api/link-previews/metadata?path=%2Fw%2Fdemo%2Fpm%2Ftasks%2Ftask-id&query=task%3DDEMO-12')
    expect(html).toContain('Launch &amp; learn')
    expect(html).not.toContain('secret body')
    expect(response.status).toBe(200)
    expect(response.headers.get('x-origin')).toBe('spa')
    expect(response.headers.get('cache-control')).toBe('private, no-store')
    expect(response.headers.get('x-robots-tag')).toBe('noindex, nofollow, noarchive, nosnippet')
  })

  it('returns the untouched SPA response when metadata lookup fails', async () => {
    const origin = new Response(defaultHtml, {
      status: 200,
      headers: { 'content-type': 'text/html; charset=utf-8' },
    })
    const fetcher = vi.fn(async () => {
      throw new Error('upstream unavailable')
    })

    const response = await handleShareMetadata(
      new Request('https://app.helpin.ai/share/token'),
      { next: async () => origin },
      fetcher,
    )

    expect(await response.text()).toBe(defaultHtml)
  })

  it('does not fetch metadata for non-HTML responses or unknown hosts', async () => {
    const fetcher = vi.fn()
    const asset = new Response('binary', { headers: { 'content-type': 'application/octet-stream' } })
    const assetResponse = await handleShareMetadata(
      new Request('https://app.helpin.ai/share/token'),
      { next: async () => asset },
      fetcher,
    )
    expect(await assetResponse.text()).toBe('binary')

    const previewResponse = await handleShareMetadata(
      new Request('https://deploy-preview--helpin.netlify.app/share/token'),
      { next: async () => new Response(defaultHtml, { headers: { 'content-type': 'text/html' } }) },
      fetcher,
    )
    expect(await previewResponse.text()).toBe(defaultHtml)
    expect(fetcher).not.toHaveBeenCalled()
  })
})
