import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import { describe, expect, it } from 'vitest'
import {
  buildFingerprint,
  helpcenterIdentifierTag,
  isSharedCacheable,
  renderCacheKey,
} from './serverRenderCache.mjs'

describe('shared render cache keys', () => {
  it('hashes request cache keys without leaking paths', () => {
    const key = renderCacheKey('GET:docs.example.com:/articles/private-looking-slug')
    expect(key).toMatch(/^render:[a-f0-9]{64}$/)
    expect(key).not.toContain('private-looking-slug')
  })

  it('normalizes public identifiers into existing cache tags', () => {
    expect(helpcenterIdentifierTag(' Docs.Example.COM ')).toBe(
      'hc:host:docs.example.com',
    )
  })
})

describe('build fingerprint', () => {
  function assetsDir(names) {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'hc-assets-'))
    for (const name of names) fs.writeFileSync(path.join(dir, name), '')
    return dir
  }

  it('is stable for the same build', () => {
    const a = assetsDir(['main-abc.js', 'app-def.css'])
    const b = assetsDir(['app-def.css', 'main-abc.js'])
    expect(buildFingerprint(a)).toBe(buildFingerprint(b))
    expect(buildFingerprint(a)).toMatch(/^[a-f0-9]{12}$/)
  })

  it('changes when a hashed asset changes', () => {
    const before = assetsDir(['main-abc.js', 'app-def.css'])
    const after = assetsDir(['main-xyz.js', 'app-def.css'])
    expect(buildFingerprint(before)).not.toBe(buildFingerprint(after))
  })

  it('falls back when the assets folder is missing', () => {
    expect(buildFingerprint('/nonexistent/hc-assets')).toBe('unknown')
  })
})

describe('shared cache eligibility', () => {
  it('shares canonical URLs only', () => {
    expect(isSharedCacheable(new URL('https://docs.example.com/articles/a-1'))).toBe(true)
    expect(isSharedCacheable(new URL('https://docs.example.com/articles/a-1?utm_source=x'))).toBe(false)
    expect(isSharedCacheable(new URL('https://docs.example.com/search?q=billing'))).toBe(false)
  })
})
