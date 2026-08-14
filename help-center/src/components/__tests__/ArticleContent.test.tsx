import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import type { ReactNode } from 'react'
import { render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ArticleContent } from '@/components/ArticleContent'
import { DocsProvider } from '@/contexts/DocsContext'
import type { HelpCenterConfig, Space } from '@/lib/types'

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

const mockSpaces: Space[] = [
  {
    id: 'space-1',
    name: 'Docs',
    slug: 'docs',
    icon: null,
    description: null,
  },
]

function renderWithDocsContext(ui: ReactNode, basepath = '') {
  return render(
    <DocsProvider
      basepath={basepath}
      subdomain="replug"
      locale="en"
      defaultLocale="en"
      enabledLocales={['en']}
      multilingualEnabled={false}
      config={mockConfig}
      spaces={mockSpaces}
    >
      {ui}
    </DocsProvider>,
  )
}

describe('ArticleContent', () => {
  const writeText = vi.fn().mockResolvedValue(undefined)

  beforeEach(() => {
    vi.stubGlobal('navigator', {
      clipboard: {
        writeText,
      },
    })
  })

  afterEach(() => {
    writeText.mockClear()
    vi.unstubAllGlobals()
  })

  it('renders article HTML and enhances headings and code blocks', async () => {
    renderWithDocsContext(
      <ArticleContent
        html={`
          <h2>Getting Started</h2>
          <h4>Step 1</h4>
          <div class="docs-callout docs-callout--yellow"><p>Important note</p></div>
          <pre><code>console.log("hi")</code></pre>
        `}
      />,
    )

    expect(screen.getByText('Step 1')).not.toBeNull()
    expect(screen.getByText('Important note')).not.toBeNull()

    await waitFor(() => {
      const heading = screen.getByText('Getting Started')
      expect(heading.getAttribute('id')).toBe('getting-started')
      expect(screen.getByRole('button', { name: 'Copy code' })).not.toBeNull()
    })
  })

  it('prefixes internal article and collection links with the help center basepath', () => {
    renderWithDocsContext(
      <ArticleContent
        html={`
          <p>
            <a href="/articles/start-here-abc123ef">Start here</a>
            <a href='/c/basics-def456gh'>Basics</a>
            <a href="/docs/articles/already-prefixed">Prefixed</a>
            <a href="https://example.com/articles/external">External</a>
          </p>
        `}
      />,
      '/docs',
    )

    expect(screen.getByText('Start here').closest('a')?.getAttribute('href')).toBe(
      '/docs/articles/start-here-abc123ef',
    )
    expect(screen.getByText('Basics').closest('a')?.getAttribute('href')).toBe(
      '/docs/c/basics-def456gh',
    )
    expect(screen.getByText('Prefixed').closest('a')?.getAttribute('href')).toBe(
      '/docs/articles/already-prefixed',
    )
    expect(screen.getByText('External').closest('a')?.getAttribute('href')).toBe(
      'https://example.com/articles/external',
    )
  })

  it('uses the shared block contract for published toggle sections', () => {
    const { container } = renderWithDocsContext(
      <ArticleContent
        html={`
          <details class="docs-toggle-section" data-toggle-section data-toggle-style="helpScoutCard">
            <summary>
              <span class="docs-toggle-icon">🔑</span>
              <span class="docs-toggle-title">Authentication &amp; Setup</span>
              <span class="docs-toggle-badge">3 topics</span>
              <span class="docs-toggle-chevron" aria-hidden="true"></span>
            </summary>
            <div class="docs-toggle-content" data-toggle-content>
              <p>How to Get Your ContentStudio API Key</p>
            </div>
          </details>
        `}
      />,
    )

    const toggle = container.querySelector('details.docs-toggle-section')
    expect(toggle?.getAttribute('data-toggle-style')).toBe('helpScoutCard')
    expect(toggle?.querySelector('.docs-toggle-icon')?.textContent).toBe('🔑')
    expect(toggle?.querySelector('.docs-toggle-badge')?.textContent).toBe('3 topics')
    expect(toggle?.querySelector('.docs-toggle-content')?.textContent).toContain(
      'How to Get Your ContentStudio API Key',
    )

    const appCss = readFileSync(resolve(process.cwd(), 'src/app.css'), 'utf8')
    expect(appCss).toContain('@import "../../packages/shared/src/docs-blocks.css";')

    const sharedCss = readFileSync(
      resolve(process.cwd(), '../packages/shared/src/docs-blocks.css'),
      'utf8',
    )
    expect(sharedCss).toContain('.docs-toggle-section[data-toggle-style="helpScoutCard"]')
    expect(sharedCss).toContain('.docs-toggle-chevron')
  })
})
