import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { LocalizedHomePage } from '@/components/home/LocalizedHomePage'
import { DocsProvider } from '@/contexts/DocsContext'
import type { HelpCenterConfig } from '@/lib/types'

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

function renderHomePage() {
  return render(
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { queries: { retry: false } } })
      }
    >
      <DocsProvider
        basepath=""
        subdomain="docs"
        locale="en"
        defaultLocale="en"
        enabledLocales={['en']}
        multilingualEnabled={false}
        config={config}
        spaces={[]}
      >
        <LocalizedHomePage />
      </DocsProvider>
    </QueryClientProvider>,
  )
}

describe('LocalizedHomePage', () => {
  it('shows an empty state instead of nothing when no cards or spaces exist', () => {
    renderHomePage()

    expect(screen.getByText('No help spaces configured yet')).toBeTruthy()
    expect(
      screen.getByText(
        'Content will appear here once spaces are published to this Help Center.',
      ),
    ).toBeTruthy()
  })
})
