import { describe, expect, it } from 'vitest'
import { createBasepathRewrite } from '../pathUtils'

describe('createBasepathRewrite', () => {
  const rewrite = createBasepathRewrite('/docs')
  const input = (path: string) => rewrite.input({ url: new URL(path, 'https://helpin.ai') }).pathname
  const output = (path: string) => rewrite.output({ url: new URL(path, 'https://helpin.ai') }).pathname

  it('strips the mount path from incoming locations', () => {
    expect(input('/docs')).toBe('/')
    expect(input('/docs/c/getting-started')).toBe('/c/getting-started')
    // SSR receives paths already stripped by serve.mjs.
    expect(input('/c/getting-started')).toBe('/c/getting-started')
  })

  it('prefixes every generated href with the mount path', () => {
    expect(output('/')).toBe('/docs')
    expect(output('/c/getting-started')).toBe('/docs/c/getting-started')
    // A route that itself starts with the mount path still gets prefixed.
    expect(output('/docs')).toBe('/docs/docs')
  })
})
