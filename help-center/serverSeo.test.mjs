import { describe, expect, it } from 'vitest'
import {
  buildSitemapPath,
  collectSitemapEntries,
  renderLlmsTxt,
  renderRobotsTxt,
  renderSitemapXml,
  resolvePublicUrlParts,
} from './serverSeo.mjs'

const hcContext = {
  host: 'acme.helpin.center',
  origin: 'https://acme.helpin.center',
  subdomain: 'acme',
  basepath: '',
}

const config = {
  subdomain: 'acme',
  custom_domain: null,
  public_url_mode: 'reverse_proxy',
  reverse_proxy_host: 'acme.com',
  reverse_proxy_base_path: '/docs',
  default_locale: 'en',
  enabled_locales: ['en'],
}

describe('server SEO helpers', () => {
  it('uses saved reverse proxy URL parts', () => {
    expect(resolvePublicUrlParts(hcContext, config)).toEqual({
      origin: 'https://acme.com',
      basepath: '/docs',
    })
  })

  it('builds current canonical sitemap route paths', () => {
    expect(buildSitemapPath({
      multilingual: false,
      locale: 'en',
      kind: 'collection',
      slug: 'getting-started',
      publicId: 'abc123ef',
    })).toBe('/c/getting-started-abc123ef')
    expect(buildSitemapPath({
      multilingual: false,
      locale: 'en',
      kind: 'article',
      slug: 'install',
      publicId: 'def456ab',
    })).toBe('/articles/install-def456ab')
  })

  it('renders reverse-proxy sitemap and robots URLs', () => {
    const publicUrl = resolvePublicUrlParts(hcContext, config)
    const entries = collectSitemapEntries({
      publicUrl,
      config,
      navigationByLocale: new Map([
        ['en', [
          {
            slug: 'getting-started',
            public_id: 'abc123ef',
            articles: [
              {
                slug: 'install',
                public_id: 'def456ab',
                published_at: '2026-04-24T12:00:00Z',
              },
            ],
          },
        ]],
      ]),
    })
    const xml = renderSitemapXml(entries)

    expect(xml).toContain('<loc>https://acme.com/docs/</loc>')
    expect(xml).toContain('<loc>https://acme.com/docs/c/getting-started-abc123ef</loc>')
    expect(xml).toContain('<loc>https://acme.com/docs/articles/install-def456ab</loc>')
    expect(renderRobotsTxt(publicUrl)).toContain('Sitemap: https://acme.com/docs/sitemap.xml')
    expect(renderRobotsTxt(publicUrl)).toContain('Disallow: /docs/preview/')
  })
})

describe('renderLlmsTxt', () => {
  it('lists public articles by space with absolute links', () => {
    const body = renderLlmsTxt({
      publicUrl: resolvePublicUrlParts(hcContext, { ...config, public_url_mode: 'hosted_subdomain' }),
      config: { ...config, brand_name: 'Acme', seo_description: 'Guides for Acme.' },
      spaces: [
        { name: 'Guides', navigation: [{ name: 'Billing', articles: [{ title: 'Export [CSV] reports', slug: 'export', public_id: 'abc' }] }] },
        { name: 'Empty', navigation: [] },
      ],
    })
    expect(body).toContain('# Acme help center\n\n> Guides for Acme.')
    expect(body).toContain('## Guides')
    expect(body).toMatch(/- \[Export \\\[CSV\\\] reports\]\(https:\/\/acme\.helpin\.center\/articles\/export-abc\): Billing/)
    expect(body).not.toContain('## Empty')
  })
})
