import { describe, expect, it } from 'vitest'
import {
  buildCollectionKey,
  normalizeCollectionPublicId,
  parseCollectionKey,
} from '@/lib/collectionKey'

describe('collectionKey helpers', () => {
  it('builds stable slug-publicID collection keys', () => {
    expect(buildCollectionKey('getting-started', 'ABC123EF')).toBe(
      'getting-started-abc123ef',
    )
    expect(buildCollectionKey('/getting-started/', 'abc123ef')).toBe(
      'getting-started-abc123ef',
    )
    expect(buildCollectionKey('getting-started', '')).toBe('getting-started')
  })

  it('parses collection keys by the last dash', () => {
    expect(parseCollectionKey('how-to-get-started-abc123ef')).toEqual({
      slug: 'how-to-get-started',
      publicId: 'abc123ef',
    })
    expect(parseCollectionKey('/getting-started-ABC123EF/')).toEqual({
      slug: 'getting-started',
      publicId: 'abc123ef',
    })
  })

  it('returns null for legacy slug-only or malformed keys', () => {
    expect(parseCollectionKey('getting-started')).toBeNull()
    expect(parseCollectionKey('abc123ef')).toBeNull()
    expect(parseCollectionKey('getting-started-zzzzzzzz')).toBeNull()
    expect(parseCollectionKey('getting-started-abc123')).toBeNull()
  })

  it('normalizes only valid 8-char hex public IDs', () => {
    expect(normalizeCollectionPublicId('ABC123EF')).toBe('abc123ef')
    expect(normalizeCollectionPublicId('abc123')).toBe('abc123')
    expect(normalizeCollectionPublicId('zzzzzzzz')).toBe('')
  })
})
