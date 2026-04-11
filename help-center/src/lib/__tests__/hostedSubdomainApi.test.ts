// @vitest-environment node

import { afterAll, afterEach, beforeAll, describe, expect, it } from 'vitest'
import { createServer, type IncomingMessage, type ServerResponse } from 'node:http'
import type { AddressInfo } from 'node:net'
import { QueryClient } from '@tanstack/react-query'
import {
  helpCenterConfigQueryOptions,
  spacesQueryOptions,
} from '@/hooks/queries'
import { prefetchCollectionRouteData, prefetchHomeRouteData } from '@/lib/routeData'
import { queryKeys } from '@/lib/queryKeys'
import type { RootRouteData } from '@/lib/rootLoader'
import type { CollectionPage, HelpCenterConfig, NavItem, Space } from '@/lib/types'

const mockConfig: HelpCenterConfig = {
  id: 'cfg-replug',
  workspace_id: 'ws-replug',
  subdomain: 'replug',
  custom_domain: null,
  brand_name: 'Replug',
  brand_logo_url: null,
  brand_logo_dark_url: null,
  brand_color: '#2b70fb',
  favicon_url: null,
  theme_mode: 'system',
  search_placeholder: 'Search for articles...',
  default_locale: 'en',
  enabled_locales: ['en'],
  show_language_switcher: false,
  fallback_to_default_locale: true,
  is_published: true,
  seo_title: 'Replug Help Center',
  seo_description: 'Hosted help center docs for Replug.',
  support_email: null,
  header_links: [],
  footer_config: { links: [], copyright_text: '© Replug' },
  homepage_config: {
    featured_cards: [
      {
        title: 'Brands',
        description: '',
        icon: '',
        link_type: 'collection',
        link_value: 'brands',
        space_slug: 'help-center',
      },
    ],
  },
}

const mockSpaces: Space[] = [
  {
    id: 'space-help-center',
    name: 'Help Center',
    slug: 'help-center',
    icon: null,
    description: null,
  },
  {
    id: 'space-api',
    name: 'Developer / API Docs',
    slug: 'developer-api-docs',
    icon: null,
    description: null,
  },
]

const brandNavigationItem: NavItem = {
  id: 'collection-brands',
  name: 'Brands',
  slug: 'brands',
  icon: null,
  articles: [
    {
      id: 'article-brand-settings',
      title: 'Brand settings',
      slug: 'brand-settings',
      published_at: '2026-04-10T18:30:00Z',
    },
    {
      id: 'article-brand-domains',
      title: 'Custom brand domains',
      slug: 'custom-brand-domains',
      published_at: '2026-04-10T18:35:00Z',
    },
  ],
}

const mockNavigation: NavItem[] = [brandNavigationItem]

const mockCollection: CollectionPage = {
  collection: brandNavigationItem,
  articles: brandNavigationItem.articles,
  space_slug: 'help-center',
}

function writeJSON(res: ServerResponse, body: unknown, statusCode = 200) {
  res.statusCode = statusCode
  res.setHeader('Content-Type', 'application/json')
  res.end(JSON.stringify(body))
}

describe('hosted help-center mock server integration', () => {
  let server: ReturnType<typeof createServer>
  let baseUrl = ''
  let originalInternalApiUrl: string | undefined
  const requests: string[] = []

  beforeAll(async () => {
    originalInternalApiUrl = process.env.INTERNAL_API_URL

    server = createServer((req: IncomingMessage, res: ServerResponse) => {
      const path = req.url || '/'
      requests.push(path)

      switch (path) {
        case '/api/hc/replug/config':
          writeJSON(res, mockConfig)
          return
        case '/api/hc/replug/spaces':
          writeJSON(res, mockSpaces)
          return
        case '/api/hc/replug/spaces/help-center/navigation':
          writeJSON(res, mockNavigation)
          return
        case '/api/hc/replug/c/brands':
          writeJSON(res, mockCollection)
          return
        default:
          writeJSON(res, { error: `Unhandled path: ${path}` }, 404)
      }
    })

    await new Promise<void>((resolve) => {
      server.listen(0, '127.0.0.1', () => resolve())
    })

    const address = server.address() as AddressInfo
    baseUrl = `http://${address.address}:${address.port}/api`
    process.env.INTERNAL_API_URL = baseUrl
  })

  afterEach(() => {
    requests.length = 0
  })

  afterAll(async () => {
    process.env.INTERNAL_API_URL = originalInternalApiUrl
    await new Promise<void>((resolve, reject) => {
      server.close((err) => (err ? reject(err) : resolve()))
    })
  })

  it('loads hosted subdomain spaces, sidebar navigation, and collection articles through the public API', async () => {
    const queryClient = new QueryClient({
      defaultOptions: {
        queries: {
          retry: false,
        },
      },
    })

    const config = await queryClient.ensureQueryData(
      helpCenterConfigQueryOptions('replug'),
    )
    const spaces = await queryClient.ensureQueryData(
      spacesQueryOptions('replug', 'en', false),
    )

    const rootData: RootRouteData = {
      activeLocale: 'en',
      basepath: '',
      config,
      host: 'replug.helpin.center',
      multilingualEnabled: false,
      origin: 'https://replug.helpin.center',
      spaces,
      subdomain: 'replug',
    }

    await prefetchHomeRouteData(queryClient, rootData)
    const collection = await prefetchCollectionRouteData(
      queryClient,
      rootData,
      'brands',
    )

    const cachedNavigation = queryClient.getQueryData<NavItem[]>(
      queryKeys.spaces.navigation('replug', 'en', 'help-center'),
    )

    expect(config.brand_name).toBe('Replug')
    expect(spaces.map((space) => space.slug)).toEqual([
      'help-center',
      'developer-api-docs',
    ])
    expect(cachedNavigation?.[0]?.slug).toBe('brands')
    expect(cachedNavigation?.[0]?.articles).toHaveLength(2)
    expect(collection?.space_slug).toBe('help-center')
    expect(collection?.articles).toHaveLength(2)
    expect(collection?.articles.map((article) => article.slug)).toEqual([
      'brand-settings',
      'custom-brand-domains',
    ])

    expect(requests).toEqual([
      '/api/hc/replug/config',
      '/api/hc/replug/spaces',
      '/api/hc/replug/spaces/help-center/navigation',
      '/api/hc/replug/c/brands',
      '/api/hc/replug/spaces/help-center/navigation',
    ])
  })
})
