import { afterEach, describe, expect, it, vi } from 'vitest'

const { getMock, postMock } = vi.hoisted(() => ({
  getMock: vi.fn(),
  postMock: vi.fn(),
}))

vi.mock('@/lib/api', () => ({
  api: {
    post: postMock,
    get: getMock,
    put: vi.fn(),
    patch: vi.fn(),
    del: vi.fn(),
  },
  API_BASE: 'http://localhost:8080/api',
}))

import { docsService } from '@/lib/services/docsService'

describe('docsService.listDocuments', () => {
  afterEach(() => {
    getMock.mockReset()
  })

  it('sends owner and archived filters for the My Documents view', async () => {
    getMock.mockResolvedValueOnce({ data: [], error: null, status: 200 })

    await docsService.listDocuments('ws-1', {
      owner_id: 'member-1',
      include_archived: 'true',
    })

    expect(getMock).toHaveBeenCalledWith(
      '/docs/documents?workspace_id=ws-1&owner_id=member-1&include_archived=true',
    )
  })
})

describe('docsService.getCollectionDeleteImpact', () => {
  afterEach(() => {
    getMock.mockReset()
  })

  it('calls the collection delete impact endpoint', async () => {
    getMock.mockResolvedValueOnce({ data: { collection_id: 'collection-1' }, error: null, status: 200 })

    const result = await docsService.getCollectionDeleteImpact('ws-1', 'collection-1')

    expect(getMock).toHaveBeenCalledWith(
      '/docs/collections/collection-1/delete-impact?workspace_id=ws-1',
    )
    expect(result.error).toBeNull()
  })
})

describe('docsService API references', () => {
  afterEach(() => {
    getMock.mockReset()
    postMock.mockReset()
  })

  it('lists references inside the selected space', async () => {
    getMock.mockResolvedValueOnce({ data: [], error: null, status: 200 })

    await docsService.listAPIReferences('ws-1', 'space-1')

    expect(getMock).toHaveBeenCalledWith(
      '/docs/spaces/space-1/api-references?workspace_id=ws-1',
    )
  })

  it('imports a managed draft and publishes it explicitly', async () => {
    postMock.mockResolvedValue({ data: { id: 'reference-1' }, error: null, status: 200 })
    const payload = {
      name: 'Product API',
      source_type: 'url' as const,
      source_url: 'https://api.example.com/openapi.json',
    }

    await docsService.createAPIReference('ws-1', 'space-1', payload)
    await docsService.publishAPIReference('ws-1', 'reference-1')

    expect(postMock).toHaveBeenNthCalledWith(
      1,
      '/docs/spaces/space-1/api-references?workspace_id=ws-1',
      payload,
    )
    expect(postMock).toHaveBeenNthCalledWith(
      2,
      '/docs/api-references/reference-1/publish?workspace_id=ws-1',
    )
  })
})

describe('docsService.updateArticleSlug', () => {
  afterEach(() => {
    postMock.mockReset()
  })

  it('keeps source slug edits on the draft update-slug endpoint', async () => {
    postMock.mockResolvedValueOnce({ data: null, error: 'Not Found', status: 404 })

    const result = await docsService.updateArticleSlug('ws-1', 'doc-1', 'new-slug')

    expect(postMock).toHaveBeenNthCalledWith(
      1,
      '/docs/documents/doc-1/update-slug?workspace_id=ws-1',
      { slug: 'new-slug' },
    )
    expect(postMock).toHaveBeenCalledTimes(1)
    expect(result.error).toBe('Not Found')
  })

  it('returns the direct update-slug response when the backend route exists', async () => {
    postMock.mockResolvedValueOnce({ data: { message: 'slug updated' }, error: null, status: 200 })

    const result = await docsService.updateArticleSlug('ws-1', 'doc-1', 'new-slug')

    expect(postMock).toHaveBeenCalledTimes(1)
    expect(postMock).toHaveBeenCalledWith(
      '/docs/documents/doc-1/update-slug?workspace_id=ws-1',
      { slug: 'new-slug' },
    )
    expect(result.error).toBeNull()
  })
})

describe('docsService.publishArticleTranslation', () => {
  afterEach(() => {
    postMock.mockReset()
  })

  it('sends an explicit slug on first locale publish when provided', async () => {
    postMock.mockResolvedValueOnce({ data: { locale: 'fr', slug: 'premiers-pas' }, error: null, status: 200 })

    const result = await docsService.publishArticleTranslation('ws-1', 'doc-1', {
      locale: 'fr',
      slug: 'premiers-pas',
    })

    expect(postMock).toHaveBeenCalledWith(
      '/docs/documents/doc-1/helpcenter/translations/fr/publish?workspace_id=ws-1',
      { slug: 'premiers-pas' },
    )
    expect(result.error).toBeNull()
  })
})

describe('docsService.publishSpaceTranslation', () => {
  afterEach(() => {
    postMock.mockReset()
  })

  it('sends an explicit slug on first locale publish when provided', async () => {
    postMock.mockResolvedValueOnce({ data: { locale: 'fr', slug: 'centre-support' }, error: null, status: 200 })

    const result = await docsService.publishSpaceTranslation('ws-1', 'space-1', 'fr', 'centre-support')

    expect(postMock).toHaveBeenCalledWith(
      '/docs/spaces/space-1/helpcenter/translations/fr/publish?workspace_id=ws-1',
      { slug: 'centre-support' },
    )
    expect(result.error).toBeNull()
  })
})

describe('docsService.publishCollectionTranslation', () => {
  afterEach(() => {
    postMock.mockReset()
  })

  it('sends an explicit slug on first locale publish when provided', async () => {
    postMock.mockResolvedValueOnce({ data: { locale: 'fr', slug: 'commencer-maintenant' }, error: null, status: 200 })

    const result = await docsService.publishCollectionTranslation('ws-1', 'collection-1', 'fr', 'commencer-maintenant')

    expect(postMock).toHaveBeenCalledWith(
      '/docs/collections/collection-1/helpcenter/translations/fr/publish?workspace_id=ws-1',
      { slug: 'commencer-maintenant' },
    )
    expect(result.error).toBeNull()
  })
})

describe('docsService.updateArticleTranslationSlug', () => {
  afterEach(() => {
    postMock.mockReset()
  })

  it('posts locale-aware slug updates for published translations', async () => {
    postMock.mockResolvedValueOnce({ data: { message: 'translation slug updated' }, error: null, status: 200 })

    const result = await docsService.updateArticleTranslationSlug('ws-1', 'doc-1', 'fr', 'nouveau-slug')

    expect(postMock).toHaveBeenCalledWith(
      '/docs/documents/doc-1/helpcenter/translations/fr/update-slug?workspace_id=ws-1',
      { slug: 'nouveau-slug' },
    )
    expect(result.error).toBeNull()
  })
})
