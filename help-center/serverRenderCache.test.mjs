import { describe, expect, it } from 'vitest'
import {
  helpcenterIdentifierTag,
  renderCacheKey,
} from './serverRenderCache.mjs'

describe('shared render cache keys', () => {
  it('hashes request cache keys without leaking paths', () => {
    const key = renderCacheKey('GET:docs.example.com:/articles/private-looking-slug')
    expect(key).toMatch(/^render:[a-f0-9]{64}$/)
    expect(key).not.toContain('private-looking-slug')
  })

  it('normalizes public identifiers into existing cache tags', () => {
    expect(helpcenterIdentifierTag(' Docs.Example.COM ')).toBe(
      'hc:host:docs.example.com',
    )
  })
})
