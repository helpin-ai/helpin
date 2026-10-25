import { describe, expect, it } from 'vitest'
import { buildHelpcenterArticleUrl } from '../helpcenterArticleUrl'

const base = { hostRoot: 'helpin.center', hostedSubdomain: true }
const config = { subdomain: 'acme', public_url_mode: 'hosted_subdomain' as const, enabled_locales: ['en'] }
describe('live help center article URL', () => {
  it('uses the public article key on the hosted domain', () => {
    expect(buildHelpcenterArticleUrl(config, 'live-slug', 'abcd1234', 'en', base)).toBe('https://acme.helpin.center/articles/live-slug-abcd1234')
  })
  it('includes the locale and reverse proxy mount path', () => {
    expect(buildHelpcenterArticleUrl({ ...config, public_url_mode: 'reverse_proxy', reverse_proxy_host: 'example.com', reverse_proxy_base_path: '/help/', enabled_locales: ['en', 'fr'] }, 'bonjour', 'abcd1234', 'fr', base)).toBe('https://example.com/help/fr/articles/bonjour-abcd1234')
  })
  it('uses a selected custom domain', () => {
    expect(buildHelpcenterArticleUrl({ ...config, public_url_mode: 'custom_domain', custom_domain: 'docs.example.com' }, 'live', 'abcd1234', 'en', base)).toBe('https://docs.example.com/articles/live-abcd1234')
  })
  it('preserves stage routing', () => {
    expect(buildHelpcenterArticleUrl(config, 'live', 'abcd1234', 'en', { ...base, hostRoot: 'stage.helpin.center' })).toBe('https://acme.stage.helpin.center/articles/live-abcd1234')
  })
  it('does not produce an incomplete URL', () => {
    expect(buildHelpcenterArticleUrl(config, '', 'abcd1234', 'en', base)).toBeNull()
    expect(buildHelpcenterArticleUrl(config, 'live', undefined, 'en', base)).toBeNull()
  })
})
