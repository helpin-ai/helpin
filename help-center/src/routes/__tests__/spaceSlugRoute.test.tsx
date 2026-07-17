import { QueryClient } from '@tanstack/react-query'
import { createMemoryHistory, createRouter } from '@tanstack/react-router'
import { describe, expect, it, vi } from 'vitest'
import type { HelpCenterConfig, Space } from '@/lib/types'
import type { RootRouteData } from '@/lib/rootLoader'

const { loadRootRouteData } = vi.hoisted(() => ({
  loadRootRouteData: vi.fn(),
}))

vi.mock('@/lib/rootLoader', () => ({
  loadRootRouteData,
}))

import { routeTree } from '@/routeTree.gen'

const config: HelpCenterConfig = {
  id: 'cfg-1',
  workspace_id: 'ws-1',
  subdomain: 'docs',
  custom_domain: null,
  brand_name: 'Acme',
  brand_logo_url: null,
  brand_logo_dark_url: null,
  brand_color: '#2b70fb',
  favicon_url: null,
  theme_mode: 'system',
  search_placeholder: 'Search docs...',
  default_locale: 'en',
  enabled_locales: ['en'],
  show_language_switcher: false,
  fallback_to_default_locale: true,
  is_published: true,
  seo_title: null,
  seo_description: null,
  support_email: null,
  header_links: [],
  footer_config: { links: [] },
  homepage_config: { featured_cards: [] },
}

const spaces: Space[] = [
  {
    id: 'space-1',
    name: 'Acme Help Center',
    slug: 'acme-help-center',
    icon: null,
    description: null,
  },
]

const rootData: RootRouteData = {
  activeLocale: 'en',
  basepath: '',
  config,
  host: 'docs.acme.com',
  multilingualEnabled: false,
  origin: 'https://docs.acme.com',
  spaces,
  subdomain: 'docs',
}

function createTestRouter(initialPath: string) {
  return createRouter({
    routeTree,
    history: createMemoryHistory({ initialEntries: [initialPath] }),
    context: {
      queryClient: new QueryClient({
        defaultOptions: { queries: { retry: false } },
      }),
    },
  })
}

// Regression test: $spaceSlug/index.tsx once redirected to its own URL,
// which loops the router synchronously and freezes the page (seen twice:
// 48046d978 fixed it, 5d38dc975 reintroduced it). A space URL must settle
// on the index route in a single load pass.
describe('/$spaceSlug route', () => {
  it('resolves a non-multilingual space URL without redirecting to itself', async () => {
    loadRootRouteData.mockResolvedValue(rootData)
    const router = createTestRouter('/acme-help-center')
    await router.load()

    expect(router.state.location.pathname).toBe('/acme-help-center')
    const matchedRouteIds = router.state.matches.map((match) => match.routeId)
    expect(matchedRouteIds).toContain('/$spaceSlug')
    expect(matchedRouteIds).toContain('/$spaceSlug/')
  }, 10_000)

  it('resolves a trailing-slash space URL without looping', async () => {
    loadRootRouteData.mockResolvedValue(rootData)
    const router = createTestRouter('/acme-help-center/')
    await router.load()

    const matchedRouteIds = router.state.matches.map((match) => match.routeId)
    expect(matchedRouteIds).toContain('/$spaceSlug/')
  }, 10_000)
})
