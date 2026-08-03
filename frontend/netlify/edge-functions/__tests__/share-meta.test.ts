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

describe('shared document edge metadata', () => {
  it('uses public APIs only for known app hosts', () => {
    expect(resolvePublicApiBase('app.helpin.ai')).toBe('https://api.helpin.ai/api')
    expect(resolvePublicApiBase('app.stage.helpin.ai')).toBe('https://api.stage.helpin.ai/api')
    expect(resolvePublicApiBase('localhost')).toBe('http://localhost:8080/api')
    expect(resolvePublicApiBase('deploy-preview-42--helpin.netlify.app')).toBeNull()
  })

  it('replaces the marked block with escaped, privacy-safe share metadata', () => {
    const html = injectShareMetadata({
      html: defaultHtml,
      requestUrl: 'https://app.helpin.ai/share/token?campaign=private',
      documentTitle: '<script>alert("x")</script> & Roadmap',
    })

    expect(html).toContain('<title>&lt;script&gt;alert(&quot;x&quot;)&lt;/script&gt; &amp; Roadmap — Shared via Helpin</title>')
    expect(html).toContain('property="og:url" content="https://app.helpin.ai/share/token"')
    expect(html).toContain('property="og:image" content="https://app.helpin.ai/og/helpin-shared-document.png"')
    expect(html).toContain('property="og:image:width" content="1200"')
    expect(html).toContain('property="og:image:height" content="630"')
    expect(html).toContain('name="twitter:card" content="summary_large_image"')
    expect(html).toContain('name="robots" content="noindex, nofollow, noarchive, nosnippet"')
    expect(html).toContain('name="googlebot" content="noindex, nofollow, noarchive, nosnippet"')
    expect(html).toContain('A document securely shared via Helpin.')
    expect(html).not.toContain('<script>alert')
    expect(html).not.toContain('campaign=private')
    expect(html.match(/<!-- helpin-meta:start -->/g)).toHaveLength(1)
    expect(html.match(/<!-- helpin-meta:end -->/g)).toHaveLength(1)
  })

  it('fetches only the encoded public share token and never emits document content', async () => {
    const next = vi.fn(async () => new Response(defaultHtml, {
      status: 200,
      headers: { 'content-type': 'text/html; charset=utf-8', 'x-origin': 'spa' },
    }))
    const fetcher = vi.fn(async () => new Response(JSON.stringify({
      document: { title: 'Launch & learn' },
      content: { text: 'secret body must never enter metadata' },
    }), { status: 200, headers: { 'content-type': 'application/json' } }))

    const response = await handleShareMetadata(
      new Request('https://app.helpin.ai/share/token%20with%20spaces'),
      { next },
      fetcher,
    )
    const html = await response.text()

    expect(next).toHaveBeenCalledOnce()
    expect(fetcher).toHaveBeenCalledOnce()
    expect(fetcher.mock.calls[0]?.[0]).toBe('https://api.helpin.ai/api/docs/shared/token%20with%20spaces')
    expect(html).toContain('Launch &amp; learn — Shared via Helpin')
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
