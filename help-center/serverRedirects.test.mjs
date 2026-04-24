import { describe, expect, it, vi } from 'vitest'
import {
  buildRedirectLocation,
  buildRedirectResolverURL,
  resolvePublicRedirect,
  shouldAttemptRedirectResolution,
} from './serverRedirects.mjs'

describe('serverRedirects', () => {
  it('skips non-page requests', () => {
    expect(shouldAttemptRedirectResolution('GET', '/brands')).toBe(true)
    expect(shouldAttemptRedirectResolution('HEAD', '/brands')).toBe(true)
    expect(shouldAttemptRedirectResolution('POST', '/brands')).toBe(false)
    expect(shouldAttemptRedirectResolution('GET', '/api/hc/replug/config')).toBe(false)
    expect(shouldAttemptRedirectResolution('GET', '/assets/main.js')).toBe(false)
    expect(shouldAttemptRedirectResolution('GET', '/preview/123')).toBe(false)
  })

  it('skips canonical public-id routes but still checks legacy slugs', () => {
    expect(shouldAttemptRedirectResolution('GET', '/articles/start-here-a1b2c3d4')).toBe(false)
    expect(shouldAttemptRedirectResolution('GET', '/fr/articles/start-here-a1b2c3d4')).toBe(false)
    expect(shouldAttemptRedirectResolution('GET', '/c/getting-started-a1b2c3d4')).toBe(false)
    expect(shouldAttemptRedirectResolution('GET', '/articles/start-here')).toBe(true)
    expect(shouldAttemptRedirectResolution('GET', '/c/getting-started')).toBe(true)
  })

  it('builds redirect locations with basepaths and query preservation', () => {
    expect(
      buildRedirectLocation('/brands/replug-links', {
        basepath: '',
        search: '?utm_source=legacy',
      }),
    ).toBe('/brands/replug-links?utm_source=legacy')

    expect(
      buildRedirectLocation('/brands/replug-links', {
        basepath: '/replug',
        search: '?utm_source=legacy',
      }),
    ).toBe('/replug/brands/replug-links?utm_source=legacy')
  })

  it('builds resolver URLs without double slashes', () => {
    expect(
      buildRedirectResolverURL(
        'http://127.0.0.1:8080/api/',
        'docs.contentpen.ai',
        '/article/185-how-to-create-a-campaign',
      ),
    ).toBe(
      'http://127.0.0.1:8080/api/hc/docs.contentpen.ai/resolve/article/185-how-to-create-a-campaign',
    )
  })

  it('resolves redirects from the backend and preserves search params', async () => {
    const fetchImpl = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({
        redirect: true,
        status: 301,
        target: '/brands/replug-links',
      }),
    })

    await expect(
      resolvePublicRedirect({
        apiBase: 'http://127.0.0.1:8080/api',
        subdomain: 'replug',
        pathname: '/article/185-how-to-create-a-campaign',
        search: '?utm_source=legacy',
        fetchImpl,
      }),
    ).resolves.toEqual({
      status: 301,
      location: '/brands/replug-links?utm_source=legacy',
    })
  })

  it('returns null when no redirect exists', async () => {
    const fetchImpl = vi.fn().mockResolvedValue({
      ok: false,
      status: 404,
    })

    await expect(
      resolvePublicRedirect({
        apiBase: 'http://127.0.0.1:8080/api',
        subdomain: 'replug',
        pathname: '/missing',
        fetchImpl,
      }),
    ).resolves.toBeNull()
  })
})
