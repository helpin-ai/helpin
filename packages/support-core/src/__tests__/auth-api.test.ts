// @vitest-environment node
import { afterEach, describe, expect, it, vi } from 'vitest'
import { createApiClient } from '../auth-api'

afterEach(() => vi.unstubAllGlobals())

describe('API error responses', () => {
  it.each([['404 page not found\n', 404], ['null', 503], ['{}', 500]] as const)(
    'rejects %s with status %i even without an HTTP status label',
    async (body, status) => {
      vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(body, { status })))
      const result = await createApiClient('https://example.test/api').get('/crm/outreach/enrollments')
      expect(result.data).toBeNull()
      expect(result.error).toBe(`Request failed (HTTP ${status})`)
      expect(result.status).toBe(status)
      expect(result.isNetworkError).not.toBe(true)
    },
  )
  it('marks only an unmatched route as eligible for compatibility fallback', async () => {
    const fetch = vi.fn()
      .mockResolvedValueOnce(new Response('404 page not found\n', { status: 404 }))
      .mockResolvedValueOnce(new Response('module is disabled for this installation\n', { status: 404 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ error: 'Conversation missing' }), { status: 404 }))
    vi.stubGlobal('fetch', fetch)
    const client = createApiClient('https://example.test/api')
    expect((await client.post('/translation/sends', {})).isMissingRoute).toBe(true)
    expect((await client.post('/translation/sends', {})).isMissingRoute).toBe(false)
    expect((await client.post('/translation/sends', {})).isMissingRoute).toBe(false)
  })
  it('preserves a useful server validation message', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify({error:'Add a sequence name'}), {status:400})))
    expect((await createApiClient('https://example.test/api').post('/crm/outreach/sequences', {})).error).toBe('Add a sequence name')
  })
  it('keeps an intentional no-content success valid', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(null, {status:204})))
    expect(await createApiClient('https://example.test/api').del('/item')).toEqual({data:null,error:null,status:204})
  })
})
