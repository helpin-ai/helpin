import { describe, expect, it } from 'vitest'
import { absolutePublicUrl, resolvePublicUrlParts } from '@/lib/publicUrl'
import type { RootRouteData } from '@/lib/rootLoader'

function rootData(overrides: Partial<RootRouteData> = {}): RootRouteData {
  return {
    activeLocale: 'en',
    basepath: '',
    config: {
      id: 'cfg',
      workspace_id: 'ws',
      subdomain: 'acme',
      custom_domain: null,
      brand_name: 'Acme',
      brand_logo_url: null,
      brand_logo_dark_url: null,
      brand_color: '#000000',
      favicon_url: null,
      theme_mode: 'system',
      search_placeholder: null,
      default_locale: 'en',
      enabled_locales: ['en'],
      show_language_switcher: false,
      fallback_to_default_locale: true,
      is_published: true,
      seo_title: null,
      seo_description: null,
      og_title: null,
      og_description: null,
      og_image_url: null,
      og_image_alt: null,
      support_email: null,
      header_links: [],
      footer_config: {},
      homepage_config: {},
    },
    host: 'acme.helpin.center',
    multilingualEnabled: false,
    origin: 'https://acme.helpin.center',
    spaces: [],
    subdomain: 'acme',
    ...overrides,
  }
}

describe('public URL resolution', () => {
  it('uses the hosted request origin for hosted subdomain mode', () => {
    const data = rootData({ host: 'acme.stage.helpin.center', origin: 'https://acme.stage.helpin.center' })

    expect(resolvePublicUrlParts(data)).toEqual({
      origin: 'https://acme.stage.helpin.center',
      basepath: '',
    })
  })

  it('uses custom domain mode when configured', () => {
    const data = rootData({
      config: {
        ...rootData().config,
        custom_domain: 'docs.acme.com',
        public_url_mode: 'custom_domain',
      },
    })

    expect(absolutePublicUrl(data, '/articles/start-abc123ef')).toBe(
      'https://docs.acme.com/articles/start-abc123ef',
    )
  })

  it('uses saved reverse proxy host and base path even on the raw origin', () => {
    const data = rootData({
      config: {
        ...rootData().config,
        public_url_mode: 'reverse_proxy',
        reverse_proxy_host: 'acme.com',
        reverse_proxy_base_path: '/docs',
      },
      host: 'acme.helpin.center',
      origin: 'https://acme.helpin.center',
    })

    expect(absolutePublicUrl(data, '/articles/start-abc123ef')).toBe(
      'https://acme.com/docs/articles/start-abc123ef',
    )
  })
})
