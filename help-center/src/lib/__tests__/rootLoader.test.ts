// @vitest-environment node

import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest'
import { createServer, type IncomingMessage, type ServerResponse } from 'node:http'
import type { AddressInfo } from 'node:net'
import { QueryClient } from '@tanstack/react-query'
import type { HelpCenterConfig, Space } from '@/lib/types'

const { getHelpCenterRequestContext } = vi.hoisted(() => ({
  getHelpCenterRequestContext: vi.fn(),
}))

vi.mock('@/lib/requestContext', () => ({
  getHelpCenterRequestContext,
}))

import { loadRootRouteData } from '@/lib/rootLoader'

const hostedConfig: HelpCenterConfig = {
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
  search_placeholder: 'Search docs...',
  default_locale: 'en',
  enabled_locales: ['en', 'fr'],
  show_language_switcher: true,
  fallback_to_default_locale: true,
  is_published: true,
  seo_title: 'Replug Help Center',
  seo_description: 'Hosted help center',
  support_email: null,
  header_links: [],
  footer_config: { links: [], copyright_text: '© Replug' },
  homepage_config: { featured_cards: [] },
}

const hostedFrenchSpaces: Space[] = [
  {
    id: 'space-help-center-fr',
    name: 'Centre d’aide',
    slug: 'centre-daide',
    icon: null,
    description: null,
  },
]

const customDomainConfig: HelpCenterConfig = {
  ...hostedConfig,
  id: 'cfg-contentpen',
  workspace_id: 'ws-contentpen',
  subdomain: 'docs.contentpen.ai',
  custom_domain: 'docs.contentpen.ai',
  brand_name: 'ContentPen',
  enabled_locales: ['en'],
  default_locale: 'en',
  show_language_switcher: false,
  seo_title: 'ContentPen Docs',
}

const customDomainSpaces: Space[] = [
  {
    id: 'space-help-center-en',
    name: 'Help Center',
    slug: 'help-center',
    icon: null,
    description: null,
  },
]

const proxyConfig: HelpCenterConfig = {
  ...customDomainConfig,
  id: 'cfg-usermaven',
  workspace_id: 'ws-usermaven',
  subdomain: 'usermaven',
  custom_domain: null,
  brand_name: 'Usermaven',
  seo_title: 'Usermaven Docs',
}

const proxySpaces: Space[] = [
  {
    id: 'space-usermaven',
    name: 'Docs',
    slug: 'docs',
    icon: null,
    description: null,
  },
]

function writeJSON(res: ServerResponse, body: unknown, statusCode = 200) {
  res.statusCode = statusCode
  res.setHeader('Content-Type', 'application/json')
  res.end(JSON.stringify(body))
}

describe('loadRootRouteData', () => {
  let server: ReturnType<typeof createServer>
  let originalInternalApiUrl: string | undefined
  const requests: string[] = []

  beforeAll(async () => {
    originalInternalApiUrl = process.env.INTERNAL_API_URL

    server = createServer((req: IncomingMessage, res: ServerResponse) => {
      const path = req.url || '/'
      requests.push(path)

      switch (path) {
        case '/api/hc/replug/bootstrap?path=%2Ffr%2Fbrands':
          writeJSON(res, { config: hostedConfig, locale: 'fr', spaces: hostedFrenchSpaces })
          return
        case '/api/hc/docs.contentpen.ai/bootstrap?path=%2Fbrands':
          writeJSON(res, { config: customDomainConfig, locale: 'en', spaces: customDomainSpaces })
          return
        case '/api/hc/usermaven/bootstrap?path=%2Fbrands':
          writeJSON(res, { config: proxyConfig, locale: 'en', spaces: proxySpaces })
          return
        case '/api/hc/replug/config':
          writeJSON(res, hostedConfig)
          return
        case '/api/hc/replug/fr/spaces':
          writeJSON(res, hostedFrenchSpaces)
          return
        case '/api/hc/docs.contentpen.ai/config':
          writeJSON(res, customDomainConfig)
          return
        case '/api/hc/docs.contentpen.ai/spaces':
          writeJSON(res, customDomainSpaces)
          return
        case '/api/hc/usermaven/config':
          writeJSON(res, proxyConfig)
          return
        case '/api/hc/usermaven/spaces':
          writeJSON(res, proxySpaces)
          return
        default:
          writeJSON(res, { error: `Unhandled path: ${path}` }, 404)
      }
    })

    await new Promise<void>((resolve) => {
      server.listen(0, '127.0.0.1', () => resolve())
    })

    const address = server.address() as AddressInfo
    process.env.INTERNAL_API_URL = `http://${address.address}:${address.port}/api`
  })

  beforeEach(() => {
    requests.length = 0
    getHelpCenterRequestContext.mockReset()
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

  it('loads hosted subdomain root data and derives the active locale from the pathname', async () => {
    getHelpCenterRequestContext.mockResolvedValue({
      host: 'replug.helpin.center',
      protocol: 'https',
      subdomain: 'replug',
      basepath: '',
    })

    const queryClient = new QueryClient({
      defaultOptions: {
        queries: {
          retry: false,
        },
      },
    })

    const rootData = await loadRootRouteData(queryClient, '/fr/brands')

    expect(rootData.subdomain).toBe('replug')
    expect(rootData.host).toBe('replug.helpin.center')
    expect(rootData.origin).toBe('https://replug.helpin.center')
    expect(rootData.activeLocale).toBe('fr')
    expect(rootData.multilingualEnabled).toBe(true)
    expect(rootData.spaces.map((space) => space.slug)).toEqual(['centre-daide'])
    expect(requests).toEqual(['/api/hc/replug/bootstrap?path=%2Ffr%2Fbrands'])
  })

  it('loads custom-domain root data without locale prefixes in single-locale mode', async () => {
    getHelpCenterRequestContext.mockResolvedValue({
      host: 'docs.contentpen.ai',
      protocol: 'https',
      subdomain: 'docs.contentpen.ai',
      basepath: '',
    })

    const queryClient = new QueryClient({
      defaultOptions: {
        queries: {
          retry: false,
        },
      },
    })

    const rootData = await loadRootRouteData(queryClient, '/brands')

    expect(rootData.subdomain).toBe('docs.contentpen.ai')
    expect(rootData.host).toBe('docs.contentpen.ai')
    expect(rootData.origin).toBe('https://docs.contentpen.ai')
    expect(rootData.activeLocale).toBe('en')
    expect(rootData.multilingualEnabled).toBe(false)
    expect(rootData.spaces.map((space) => space.slug)).toEqual(['help-center'])
    expect(requests).toEqual(['/api/hc/docs.contentpen.ai/bootstrap?path=%2Fbrands'])
  })

  it('loads reverse-proxied root data with public origin and stripped base path', async () => {
    getHelpCenterRequestContext.mockResolvedValue({
      host: 'usermaven.com',
      protocol: 'https',
      subdomain: 'usermaven',
      basepath: '/docs',
    })

    const queryClient = new QueryClient({
      defaultOptions: {
        queries: {
          retry: false,
        },
      },
    })

    const rootData = await loadRootRouteData(queryClient, '/docs/brands')

    expect(rootData.subdomain).toBe('usermaven')
    expect(rootData.host).toBe('usermaven.com')
    expect(rootData.origin).toBe('https://usermaven.com')
    expect(rootData.basepath).toBe('/docs')
    expect(rootData.activeLocale).toBe('en')
    expect(rootData.multilingualEnabled).toBe(false)
    expect(rootData.spaces.map((space) => space.slug)).toEqual(['docs'])
    expect(requests).toEqual(['/api/hc/usermaven/bootstrap?path=%2Fbrands'])
  })
})
