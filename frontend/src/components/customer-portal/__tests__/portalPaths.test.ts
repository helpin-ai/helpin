import { describe, expect, it } from 'vitest'
import { portalAppPath, portalMountedPath, portalPublicRedirect } from '../portalPaths'

const at = (href: string) => {
  const url = new URL(href)
  return { origin: url.origin, pathname: url.pathname, search: url.search, hash: url.hash }
}

describe('portal paths', () => {
  it('maps app pages to the short paths under /requests and back', () => {
    const pairs: [string, string][] = [
      ['/portal/acme', ''],
      ['/portal/acme/new', '/new'],
      ['/portal/acme/sign-in', '/sign-in'],
      ['/portal/acme/callback', '/callback'],
      ['/portal/acme/requests/req_1', '/req_1'],
    ]
    for (const [app, mounted] of pairs) {
      expect(portalMountedPath(app, 'acme')).toBe(mounted)
      expect(portalAppPath(mounted, 'acme')).toBe(app)
    }
    expect(portalMountedPath('/portal/acme/', 'acme')).toBe('')
    expect(portalMountedPath('/portal/other/new', 'acme')).toBeNull()
    expect(portalMountedPath('/workspaces', 'acme')).toBeNull()
    expect(portalAppPath('/a/b', 'acme')).toBeNull()
  })

  it('sends app visitors to the portal on the help center, keeping the page', () => {
    expect(portalPublicRedirect('https://acme.helpin.center/requests', at('https://app.helpin.ai/portal/acme/requests/req_1?x=1#m2'), 'acme'))
      .toBe('https://acme.helpin.center/requests/req_1?x=1#m2')
    expect(portalPublicRedirect('https://acme.com/docs/requests', at('https://app.helpin.ai/portal/acme/callback?token=t'), 'acme'))
      .toBe('https://acme.com/docs/requests/callback?token=t')
    expect(portalPublicRedirect('https://acme.helpin.center/requests', at('https://app.helpin.ai/portal/acme'), 'acme'))
      .toBe('https://acme.helpin.center/requests')
  })

  it('stays put when the portal lives in the app or the page is already public', () => {
    expect(portalPublicRedirect('https://app.helpin.ai/portal/acme', at('https://app.helpin.ai/portal/acme'), 'acme')).toBeNull()
    expect(portalPublicRedirect(undefined, at('https://app.helpin.ai/portal/acme'), 'acme')).toBeNull()
    expect(portalPublicRedirect('https://acme.helpin.center/requests', at('https://acme.helpin.center/requests/req_1'), 'acme')).toBeNull()
  })
})
