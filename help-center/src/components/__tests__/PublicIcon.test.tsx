import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { PublicIcon } from '@/components/PublicIcon'
import { DocsProvider } from '@/contexts/DocsContext'
import type { HelpCenterConfig } from '@/lib/types'

const config = {
  id: 'config-1',
  workspace_id: 'workspace-1',
  subdomain: 'docs',
  custom_domain: null,
  brand_name: 'Docs',
  brand_logo_url: null,
  brand_logo_dark_url: null,
  brand_color: '#000000',
  favicon_url: null,
  theme_mode: 'system',
  search_placeholder: 'Search',
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
} satisfies HelpCenterConfig

function renderIcon(name: string) {
  return render(
    <DocsProvider
      basepath="/help"
      subdomain="docs"
      locale="en"
      defaultLocale="en"
      enabledLocales={['en']}
      multilingualEnabled={false}
      config={config}
      spaces={[]}
    >
      <PublicIcon name={name} size={24} />
    </DocsProvider>,
  )
}

describe('PublicIcon', () => {
  it('renders canonical IDs as basepath-aware immutable SVG masks', () => {
    const { container } = renderIcon('rocket01')
    const icon = container.firstElementChild as HTMLElement

    expect(icon.style.maskImage).toContain(
      '/help/assets/helpin-icons/hugeicons/4.1.1/rocket01.svg',
    )
    expect(icon.style.width).toBe('24px')
    expect(icon.getAttribute('aria-hidden')).toBe('true')
    expect(icon.textContent).toBe('')
  })

  it('preserves intentional display text without requesting an asset', () => {
    renderIcon('🚀')
    const icon = screen.getByText('🚀')

    expect(icon.style.maskImage).toBe('')
    expect(icon.getAttribute('aria-hidden')).toBe('true')
  })

  it('never exposes an unresolved raw export name during a mixed-version rollout', () => {
    const { container } = renderIcon('DefinitelyNotAnIcon')
    expect(container.textContent).toBe('')
    expect((container.firstElementChild as HTMLElement).style.width).toBe('24px')
  })
})
