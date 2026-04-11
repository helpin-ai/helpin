import type { ReactNode } from 'react'
import { render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { SearchResultItem } from '@/components/search/SearchResultItem'
import { DocsProvider } from '@/contexts/DocsContext'
import type { HelpCenterConfig, Space } from '@/lib/types'

vi.mock('@tanstack/react-router', () => ({
  useNavigate: () => vi.fn(),
}))

const mockConfig: HelpCenterConfig = {
  id: 'cfg-1',
  workspace_id: 'ws-1',
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
  enabled_locales: ['en', 'fr'],
  show_language_switcher: true,
  fallback_to_default_locale: true,
  is_published: true,
  seo_title: null,
  seo_description: null,
  support_email: null,
  header_links: [],
  footer_config: { links: [], copyright_text: '' },
  homepage_config: { featured_cards: [] },
}

const mockSpaces: Space[] = [
  {
    id: 'space-1',
    name: 'Demarrage',
    slug: 'demarrage',
    icon: null,
    description: null,
  },
]

function renderWithDocsContext(ui: ReactNode) {
  return render(
    <DocsProvider
      basepath=""
      subdomain="replug"
      locale="fr"
      defaultLocale="en"
      enabledLocales={['en', 'fr']}
      multilingualEnabled
      config={mockConfig}
      spaces={mockSpaces}
    >
      {ui}
    </DocsProvider>,
  )
}

describe('SearchResultItem', () => {
  it('uses locale-aware article paths with collection slugs', () => {
    renderWithDocsContext(
      <SearchResultItem
        locale="fr"
        result={{
          id: 'article-1',
          title: 'Bonjour',
          slug: 'bonjour',
          locale: 'fr',
          excerpt: 'Salut',
          collection_name: 'Bases',
          collection_slug: 'bases',
          space_slug: 'demarrage',
          space_name: 'Demarrage',
        }}
        multilingualEnabled
      />,
    )

    const link = screen.getByText('Bonjour').closest('a')
    expect(link).not.toBeNull()
    expect(link?.getAttribute('href')).toBe('/fr/bases/bonjour')
  })
})
