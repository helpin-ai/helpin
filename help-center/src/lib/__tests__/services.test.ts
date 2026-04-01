import { afterEach, describe, expect, it, vi } from 'vitest'
import { helpCenterService } from '@/lib/services'

describe('helpCenterService', () => {
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
      /\/api\/hc\/contentpen\/fr\/collections\/bases$/,
    )
    expect(fetchMock.mock.calls[1]?.[0]).toMatch(
      /\/api\/hc\/contentpen\/fr\/search\?q=bonjour&space=demarrage$/,
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
    await helpCenterService.getArticle('docs.contentpen.ai', 'en', 'basics', 'start-here', false)
    await helpCenterService.search('docs.contentpen.ai', 'en', 'publish', false, 'help-center')
    await helpCenterService.submitFeedback(
      'docs.contentpen.ai',
      'en',
      'help-center',
      'basics',
      'start-here',
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
      /\/api\/hc\/docs\.contentpen\.ai\/c\/basics\/start-here$/,
    )
    expect(fetchMock.mock.calls[2]?.[0]).toMatch(
      /\/api\/hc\/docs\.contentpen\.ai\/search\?q=publish&space=help-center$/,
    )
    expect(fetchMock.mock.calls[3]?.[0]).toMatch(
      /\/api\/hc\/docs\.contentpen\.ai\/spaces\/help-center\/articles\/start-here\/feedback$/,
    )
  })
})
