import { describe, expect, it } from 'vitest'

import { buildSandboxedHtmlBlockSrcDoc } from '../HtmlBlockNodeView'

describe('buildSandboxedHtmlBlockSrcDoc', () => {
  it('adds the docs font fallback without removing source HTML styles', () => {
    const srcDoc = buildSandboxedHtmlBlockSrcDoc(
      '<div style="font-family: Georgia, serif">Keep source font</div><p>Fallback text</p>',
    )

    expect(srcDoc).toContain('font-family: ui-sans-serif')
    expect(srcDoc).toContain('font-family: Georgia, serif')
    expect(srcDoc).toContain('Keep source font')
    expect(srcDoc).toContain('Fallback text')
  })
})
