import { describe, expect, it } from 'vitest'
import {
  buildHelpcenterPreviewUrl,
  normalizeExplicitHelpcenterPreviewBase,
  resolveHelpcenterPreviewBase,
} from '@/lib/helpcenterPreview'

describe('helpcenterPreview', () => {
  it('maps the legacy helpcenter host to the hosted production root', () => {
    expect(
      normalizeExplicitHelpcenterPreviewBase('https://helpcenter.helpin.ai'),
    ).toEqual({
      hostRoot: 'helpin.center',
      hostedSubdomain: true,
    })
  })

  it('uses the workspace custom domain for production hosted previews', () => {
    const url = buildHelpcenterPreviewUrl(
      { subdomain: 'replug', customDomain: 'docs.replug.com' },
      'doc-1',
      'token-123',
      { hostRoot: 'helpin.center', hostedSubdomain: true },
    )

    expect(url).toBe('https://docs.replug.com/preview/doc-1?token=token-123')
  })

  it('keeps stage previews on the hosted stage subdomain even when a custom domain exists', () => {
    const url = buildHelpcenterPreviewUrl(
      { subdomain: 'replug', customDomain: 'docs.replug.com' },
      'doc-1',
      'token-123',
      { hostRoot: 'stage.helpin.center', hostedSubdomain: true },
    )

    expect(url).toBe('https://replug.stage.helpin.center/preview/doc-1?token=token-123')
  })

  it('falls back to query-param routing for local preview runtimes', () => {
    const url = buildHelpcenterPreviewUrl(
      { subdomain: 'replug' },
      'doc-1',
      'token-123',
      resolveHelpcenterPreviewBase({ appBase: 'http://localhost:3000' }),
    )

    expect(url).toBe('http://localhost:5174/preview/doc-1?subdomain=replug&token=token-123')
  })
})
