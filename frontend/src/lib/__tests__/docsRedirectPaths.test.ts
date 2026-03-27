import { describe, expect, it } from 'vitest'

import { formatRedirectTargetPath, normalizeRedirectSourcePathForDisplay } from '@/lib/docsRedirectPaths'

describe('docsRedirectPaths', () => {
  it('formats collectionless article targets without a double slash', () => {
    expect(formatRedirectTargetPath('', 'updated-article')).toBe('/updated-article')
  })

  it('normalizes malformed source paths for display', () => {
    expect(normalizeRedirectSourcePathForDisplay('//legacy-article')).toBe('/legacy-article')
  })

  it('collapses duplicate slashes anywhere in the source path', () => {
    expect(normalizeRedirectSourcePathForDisplay('//legacy//article')).toBe('/legacy/article')
  })
})
