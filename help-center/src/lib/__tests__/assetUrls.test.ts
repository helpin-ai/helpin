import { describe, expect, it } from 'vitest'
import { prefixAssetUrls } from '../../../assetUrls.mjs'

describe('prefixAssetUrls', () => {
  it('prefixes absolute asset references for a mounted help center', () => {
    const source = `const css = "/assets/app.css"; const chunk = './chunk.js'; url(/assets/font.woff2)`

    expect(prefixAssetUrls(source, '/docs')).toBe(
      `const css = "/docs/assets/app.css"; const chunk = './chunk.js'; url(/docs/assets/font.woff2)`,
    )
  })

  it('leaves asset references unchanged when served at the root', () => {
    expect(prefixAssetUrls('url(/assets/app.css)', '')).toBe('url(/assets/app.css)')
  })
})
