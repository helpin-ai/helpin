// @vitest-environment jsdom

import { beforeEach, describe, expect, it } from 'vitest'
import {
  normalizeHelpCenterBasepath,
  resolveHelpCenterContext,
} from '@/lib/utils'

declare global {
  interface Window {
    __HELPIN_HC_CONTEXT__?: {
      subdomain?: string
      basepath?: string
    }
  }
}

describe('resolveHelpCenterContext', () => {
  beforeEach(() => {
    if (typeof window !== 'undefined') {
      delete window.__HELPIN_HC_CONTEXT__
    }
  })

  it('extracts the hosted subdomain from helpin.center hosts', () => {
    expect(resolveHelpCenterContext('Replug.Helpin.Center:443', '/')).toEqual({
      subdomain: 'replug',
      basepath: '',
    })
  })

  it('extracts the hosted subdomain from stage helpin.center hosts', () => {
    expect(resolveHelpCenterContext('replug.stage.helpin.center', '/')).toEqual({
      subdomain: 'replug',
      basepath: '',
    })
  })

  it('keeps custom domains intact for backend resolution', () => {
    expect(resolveHelpCenterContext('docs.contentpen.ai', '/brands')).toEqual({
      subdomain: 'docs.contentpen.ai',
      basepath: '',
    })
  })

  it('uses the query override on localhost and empty hosts', () => {
    expect(
      resolveHelpCenterContext('localhost:5174', '/', '?subdomain=replug'),
    ).toEqual({
      subdomain: 'replug',
      basepath: '',
    })

    expect(resolveHelpCenterContext('', '/', '?subdomain=contentpen')).toEqual({
      subdomain: 'contentpen',
      basepath: '',
    })
  })

  it('uses explicit reverse-proxy query context when provided', () => {
    expect(
      resolveHelpCenterContext(
        'usermaven.helpin.center',
        '/articles/installing',
        '?helpin_tenant=usermaven&helpin_basepath=/docs/',
      ),
    ).toEqual({
      subdomain: 'usermaven',
      basepath: '/docs',
    })
  })

  it('uses the SSR-injected browser context during hydration', () => {
    window.__HELPIN_HC_CONTEXT__ = {
      subdomain: 'usermaven',
      basepath: '/docs',
    }

    expect(resolveHelpCenterContext('usermaven.com', '/docs')).toEqual({
      subdomain: 'usermaven',
      basepath: '/docs',
    })
  })

  it('normalizes unsafe base paths to root', () => {
    expect(normalizeHelpCenterBasepath('/docs/')).toBe('/docs')
    expect(normalizeHelpCenterBasepath('docs')).toBe('/docs')
    expect(normalizeHelpCenterBasepath('/docs/../admin')).toBe('')
    expect(normalizeHelpCenterBasepath('/docs//admin')).toBe('')
    expect(normalizeHelpCenterBasepath('/docs%2f%2fadmin')).toBe('')
  })
})
