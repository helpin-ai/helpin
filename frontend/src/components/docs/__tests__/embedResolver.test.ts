import { describe, expect, it } from 'vitest'
import { resolveEmbedUrl } from '../embedResolver'

describe('resolveEmbedUrl', () => {
  it('recognizes known providers', () => {
    expect(resolveEmbedUrl('https://github.com/helpin-ai/helpin')?.provider).toBe('GitHub')
    expect(resolveEmbedUrl('https://www.figma.com/file/abc/Product')?.provider).toBe('Figma')
  })

  it('rejects non-http urls', () => {
    expect(resolveEmbedUrl('javascript:alert(1)')).toBeNull()
  })
})
