// @vitest-environment jsdom
import { describe, expect, it } from 'vitest'
import { sanitizeHtml } from '../htmlSanitizer'

describe('sanitizeHtml', () => {
  it('keeps safe Help Scout presentation styles and strips unsafe CSS', () => {
    const got = sanitizeHtml(
      '<div style="border:1px solid #e5e7eb; border-radius:10px; padding:16px; display:flex; align-items:flex-start; position:absolute">' +
        '<span style="background:#007BFF;color:#fff;width:24px;height:24px;line-height:24px;text-align:center;display:inline-block;border-radius:50%;font-weight:bold;flex-shrink:0;background-image:url(javascript:alert(1))">2</span>' +
        ' Click on <strong>Generate API Key</strong><script>alert(1)</script></div>',
    )

    expect(got).toContain('border: 1px solid #e5e7eb')
    expect(got).toContain('border-radius: 10px')
    expect(got).toContain('padding: 16px')
    expect(got).toContain('background: #007BFF')
    expect(got).toContain('display: flex')
    expect(got).toContain('align-items: flex-start')
    expect(got).toContain('display: inline-block')
    expect(got).toContain('flex-shrink: 0')
    expect(got).toContain('Generate API Key')
    expect(got).not.toContain('position')
    expect(got).not.toContain('background-image')
    expect(got).not.toContain('javascript')
    expect(got).not.toContain('<script')
  })
})
