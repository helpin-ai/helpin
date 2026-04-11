import { describe, expect, it } from 'vitest'
import { resolveHelpCenterContext } from '@/lib/utils'

describe('resolveHelpCenterContext', () => {
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
})
