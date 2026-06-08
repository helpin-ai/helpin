// @vitest-environment jsdom

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { helpCenterService } from '@/lib/services'

declare global {
  interface Window {
    __HELPIN_HC_CONTEXT__?: {
      subdomain?: string
      basepath?: string
    }
  }
}

describe('helpCenterService', () => {
  beforeEach(() => {
    delete window.__HELPIN_HC_CONTEXT__
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('uses locale-prefixed collection and search endpoints for multilingual help centers', async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValue(
        new Response(JSON.stringify([]), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        }),
      )

    vi.stubGlobal('fetch', fetchMock)

    await helpCenterService.getCollection('contentpen', 'fr', 'bases', true)
    await helpCenterService.search('contentpen', 'fr', 'bonjour', true, 'demarrage')

    expect(fetchMock).toHaveBeenNthCalledWith(1, expect.any(String), expect.any(Object))
    expect(fetchMock).toHaveBeenNthCalledWith(2, expect.any(String), expect.any(Object))
    expect(fetchMock.mock.calls[0]?.[0]).toMatch(
      /\/api\/hc\/contentpen\/fr\/c\/bases$/,
    )
    expect(fetchMock.mock.calls[1]?.[0]).toMatch(
      /\/api\/hc\/contentpen\/fr\/search\?q=bonjour&limit=12&space=demarrage$/,
    )
  })

  it('uses canonical single-locale endpoints when multilingual mode is disabled', async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValue(
        new Response(JSON.stringify({ status: 'ok' }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        }),
      )

    vi.stubGlobal('fetch', fetchMock)

    await helpCenterService.getCollection('docs.contentpen.ai', 'en', 'basics', false)
    await helpCenterService.getArticle(
      'docs.contentpen.ai',
      'en',
      'start-here-abc123ef',
      false,
    )
    await helpCenterService.search('docs.contentpen.ai', 'en', 'publish', false, 'help-center')
    await helpCenterService.submitFeedback(
      'docs.contentpen.ai',
      'en',
      'start-here-abc123ef',
      false,
      { is_helpful: true },
    )

    expect(fetchMock).toHaveBeenNthCalledWith(1, expect.any(String), expect.any(Object))
    expect(fetchMock).toHaveBeenNthCalledWith(2, expect.any(String), expect.any(Object))
    expect(fetchMock).toHaveBeenNthCalledWith(3, expect.any(String), expect.any(Object))
    expect(fetchMock).toHaveBeenNthCalledWith(4, expect.any(String), expect.any(Object))
    expect(fetchMock.mock.calls[0]?.[0]).toMatch(
      /\/api\/hc\/docs\.contentpen\.ai\/c\/basics$/,
    )
    expect(fetchMock.mock.calls[1]?.[0]).toMatch(
      /\/api\/hc\/docs\.contentpen\.ai\/articles\/start-here-abc123ef$/,
    )
    expect(fetchMock.mock.calls[2]?.[0]).toMatch(
      /\/api\/hc\/docs\.contentpen\.ai\/search\?q=publish&limit=12&space=help-center$/,
    )
    expect(fetchMock.mock.calls[3]?.[0]).toMatch(
      /\/api\/hc\/docs\.contentpen\.ai\/articles\/start-here-abc123ef\/feedback$/,
    )
  })

  it('prefixes browser API calls with the reverse-proxy base path', async () => {
    window.__HELPIN_HC_CONTEXT__ = {
      subdomain: 'usermaven',
      basepath: '/docs',
    }

    const fetchMock = vi
      .fn()
      .mockResolvedValue(
        new Response(JSON.stringify({ status: 'ok' }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        }),
      )

    vi.stubGlobal('fetch', fetchMock)

    await helpCenterService.getConfig('usermaven')

    expect(fetchMock.mock.calls[0]?.[0]).toMatch(
      /\/docs\/api\/hc\/usermaven\/config$/,
    )
  })
})
