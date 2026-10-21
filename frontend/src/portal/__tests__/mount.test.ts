import { describe, expect, it } from 'vitest'
import { portalMountRewrite, readPortalMount } from '../mount'

const rewrite = (basepath: string) => portalMountRewrite({ slug: 'acme', basepath })
const run = (fn: ((args: { url: URL }) => undefined | string | URL) | undefined, path: string) => {
  const url = new URL(`https://help.acme.com${path}`)
  const result = fn!({ url })
  return result ? new URL(String(result)).pathname : url.pathname
}

describe('portal mount', () => {
  it('reads the injected portal and ignores unsafe values', () => {
    const location = { search: '' } as Location
    expect(readPortalMount({ __HELPIN_PORTAL__: { slug: 'acme', basepath: '/docs/' }, location }, false)).toEqual({ slug: 'acme', basepath: '/docs' })
    expect(readPortalMount({ __HELPIN_PORTAL__: { slug: 'acme', basepath: '/../x' }, location }, false)).toEqual({ slug: 'acme', basepath: '' })
    expect(readPortalMount({ __HELPIN_PORTAL__: { slug: '../acme' }, location }, false)).toBeNull()
    expect(readPortalMount({ location }, false)).toBeNull()
    expect(readPortalMount({ location: { search: '?slug=acme' } as Location }, true)).toEqual({ slug: 'acme', basepath: '' })
  })

  it('serves the portal routes at /requests on the help center', () => {
    const { input, output } = rewrite('')
    expect(run(input, '/requests')).toBe('/portal/acme')
    expect(run(input, '/requests/req_1')).toBe('/portal/acme/requests/req_1')
    expect(run(input, '/requests/new')).toBe('/portal/acme/new')
    expect(run(output, '/portal/acme/requests/req_1')).toBe('/requests/req_1')
    expect(run(output, '/portal/acme')).toBe('/requests')
    expect(run(output, '/portal/acme/callback')).toBe('/requests/callback')
  })

  it('keeps a subpath help center’s base path', () => {
    const { input, output } = rewrite('/docs')
    expect(run(input, '/docs/requests/req_1')).toBe('/portal/acme/requests/req_1')
    expect(run(output, '/portal/acme/new')).toBe('/docs/requests/new')
    expect(run(input, '/requests/req_1')).toBe('/requests/req_1')
  })
})
