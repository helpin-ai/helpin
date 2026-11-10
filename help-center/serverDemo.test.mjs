import { describe, it, expect } from 'vitest'
import { isSearchIndexingDisabled, renderRobotsTxt, collectSitemapEntries, renderLlmsTxt } from './serverSeo.mjs'

describe('demo discovery policy', () => {
  for (const config of [{demo_mode:true,search_indexing_enabled:true}, {search_indexing_enabled:false}]) {
    it(`suppresses discovery ${JSON.stringify(config)}`, () => {
      const publicUrl = {origin:'https://docs.example.com',basepath:'/help'}
      expect(isSearchIndexingDisabled(config)).toBe(true)
      expect(renderRobotsTxt(publicUrl,config)).not.toContain('Sitemap:')
      expect(renderRobotsTxt(publicUrl,config)).not.toMatch(/Disallow: \/\s*$/m)
      expect(collectSitemapEntries({publicUrl,config,navigationByLocale:new Map()})).toEqual([])
      expect(renderLlmsTxt({publicUrl,config,spaces:[]})).toBe('')
    })
  }
  it('preserves existing public indexing', () => {
    expect(isSearchIndexingDisabled({})).toBe(false)
    expect(renderRobotsTxt({origin:'https://docs.example.com',basepath:'/help'},{})).toContain('Sitemap: https://docs.example.com/help/sitemap.xml')
  })
})
