import { describe, expect, it } from 'vitest'
import {
  buildArticleKey,
  normalizeArticlePublicId,
  parseArticleKey,
} from '@/lib/articleKey'

describe('articleKey helpers', () => {
  it('builds stable slug-publicID article keys', () => {
    expect(buildArticleKey('getting-started', 'ABC123EF')).toBe(
      'getting-started-abc123ef',
    )
    expect(buildArticleKey('/getting-started/', 'abc123ef')).toBe(
      'getting-started-abc123ef',
    )
    expect(buildArticleKey('getting-started', '')).toBe('getting-started')
    expect(buildArticleKey('', 'abc123ef')).toBe('abc123ef')
  })

  it('parses article keys by the last dash', () => {
    expect(parseArticleKey('how-to-get-started-abc123ef')).toEqual({
      slug: 'how-to-get-started',
      publicId: 'abc123ef',
    })
    expect(parseArticleKey('/getting-started-ABC123EF/')).toEqual({
      slug: 'getting-started',
      publicId: 'abc123ef',
    })
  })

  it('parses bare public ID', () => {
    expect(parseArticleKey('abc123ef')).toEqual({
      slug: '',
      publicId: 'abc123ef',
    })
    expect(parseArticleKey('ABC123EF')).toEqual({
      slug: '',
      publicId: 'abc123ef',
    })
  })

  it('returns null for legacy slug-only or malformed keys', () => {
    expect(parseArticleKey('getting-started')).toBeNull()
    expect(parseArticleKey('getting-started-zzzzzzzz')).toBeNull()
    expect(parseArticleKey('getting-started-abc123')).toBeNull()
    expect(parseArticleKey('zzzzzzzz')).toBeNull()
    expect(parseArticleKey('')).toBeNull()
  })

  it('normalizes only valid 8-char hex public IDs', () => {
    expect(normalizeArticlePublicId('ABC123EF')).toBe('abc123ef')
    expect(normalizeArticlePublicId('abc123')).toBe('abc123')
    expect(normalizeArticlePublicId('zzzzzzzz')).toBe('')
  })
})
