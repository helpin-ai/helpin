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

    expect(fetchMock).toHaveBeenNthCalledWith(
      1,
      '/api/hc/contentpen/fr/collections/bases',
      expect.any(Object),
    )
    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      '/api/hc/contentpen/fr/search?q=bonjour&space=demarrage',
      expect.any(Object),
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

    expect(fetchMock).toHaveBeenNthCalledWith(
      1,
      '/api/hc/docs.contentpen.ai/c/basics',
      expect.any(Object),
    )
    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      '/api/hc/docs.contentpen.ai/c/basics/start-here',
      expect.any(Object),
    )
    expect(fetchMock).toHaveBeenNthCalledWith(
      3,
      '/api/hc/docs.contentpen.ai/search?q=publish&space=help-center',
      expect.any(Object),
    )
    expect(fetchMock).toHaveBeenNthCalledWith(
      4,
      '/api/hc/docs.contentpen.ai/spaces/help-center/articles/start-here/feedback',
      expect.any(Object),
    )
  })
})
