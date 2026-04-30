import type { ReactNode } from 'react'
import { act, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { SearchDialog } from '@/components/search/SearchDialog'
import { DocsProvider } from '@/contexts/DocsContext'
import type { HelpCenterConfig } from '@/lib/types'

const hooks = vi.hoisted(() => ({
  useSearchArticles: vi.fn(),
}))

vi.mock('@/hooks/queries', () => ({
  useSearchArticles: hooks.useSearchArticles,
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
  enabled_locales: ['en'],
  show_language_switcher: false,
  fallback_to_default_locale: true,
  is_published: true,
  seo_title: null,
  seo_description: null,
  support_email: null,
  header_links: [],
  footer_config: { links: [], copyright_text: '' },
  homepage_config: { featured_cards: [] },
}

function renderWithDocsContext(ui: ReactNode) {
  return render(
    <DocsProvider
      basepath=""
      subdomain="replug"
      locale="en"
      defaultLocale="en"
      enabledLocales={['en']}
      multilingualEnabled={false}
      config={mockConfig}
      spaces={[]}
    >
      {ui}
    </DocsProvider>,
  )
}

describe('SearchDialog', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    hooks.useSearchArticles.mockReturnValue({ data: [], isLoading: false })
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.restoreAllMocks()
  })

  it('debounces article search input before querying', () => {
    renderWithDocsContext(<SearchDialog open onClose={vi.fn()} />)

    const input = screen.getByPlaceholderText('Search documentation...')
    fireEvent.change(input, { target: { value: 'start' } })

    expect(hooks.useSearchArticles.mock.calls.map((call) => call[2])).not.toContain('start')

    act(() => {
      vi.advanceTimersByTime(299)
    })

    expect(hooks.useSearchArticles.mock.calls.map((call) => call[2])).not.toContain('start')

    act(() => {
      vi.advanceTimersByTime(1)
    })

    expect(hooks.useSearchArticles.mock.calls.map((call) => call[2])).toContain('start')
  })
})
