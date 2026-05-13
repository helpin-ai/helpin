import { describe, expect, it } from 'vitest'

import {
  buildLoginPathForRedirect,
  currentPathForLoginRedirect,
  normalizeSafeAppRedirect,
} from '../authRedirect'

describe('authRedirect', () => {
  it('preserves a safe app path with query and hash for login', () => {
    const target = '/w/test-docs/docs/documents/doc-1?rail=comments#block-1'
    expect(buildLoginPathForRedirect(target)).toBe(
      `/login?redirect=${encodeURIComponent(target)}`,
    )
  })

  it('normalizes same-origin absolute redirects and rejects external redirects', () => {
    expect(normalizeSafeAppRedirect('https://app.helpin.ai/w/acme/docs')).toBe('/w/acme/docs')
    expect(normalizeSafeAppRedirect('https://evil.example/w/acme/docs')).toBeNull()
    expect(normalizeSafeAppRedirect('//evil.example/w/acme/docs')).toBeNull()
  })

  it('does not redirect back to login or logout pages', () => {
    expect(normalizeSafeAppRedirect('/login?redirect=/w/acme/docs')).toBeNull()
    expect(normalizeSafeAppRedirect('/logout')).toBeNull()
  })

  it('builds the current path from router location parts', () => {
    expect(currentPathForLoginRedirect({
      pathname: '/w/acme/support/conv-1',
      searchStr: '?tab=activity',
      hash: '#reply',
    })).toBe('/w/acme/support/conv-1?tab=activity#reply')
  })
})
