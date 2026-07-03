import type { ReactNode } from 'react'
import { render, screen, within } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { DocsProvider } from '@/contexts/DocsContext'
import { Footer } from '@/components/layout/Footer'
import type { HelpCenterConfig, Space } from '@/lib/types'

const baseConfig: HelpCenterConfig = {
  id: 'cfg-1',
  workspace_id: 'ws-12345678',
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
  support_email: 'support@example.com',
  header_links: [],
  footer_config: {
    show_copyright: false,
    copyright_text: '© Replug',
    links: [
      { label: 'Privacy', url: 'https://example.com/privacy' },
      { label: 'Status', url: 'https://status.example.com' },
    ],
    social_links: [
      { platform: 'linkedin', url: 'https://linkedin.com/company/replug' },
      { platform: 'github', url: 'https://github.com/replug' },
    ],
  },
  homepage_config: { featured_cards: [] },
}

const spaces: Space[] = []

function renderWithDocsContext(ui: ReactNode, config: HelpCenterConfig = baseConfig) {
  return render(
    <DocsProvider
      basepath=""
      subdomain="replug"
      locale="en"
      defaultLocale="en"
      enabledLocales={['en']}
      multilingualEnabled={false}
      config={config}
      spaces={spaces}
    >
      {ui}
    </DocsProvider>,
  )
}

describe('Footer', () => {
  it('renders footer links, social links, and attribution without hidden copyright or automatic contact support', () => {
    renderWithDocsContext(<Footer />)

    const textLinks = screen.getByTestId('footer-text-links')
    expect(within(textLinks).queryByText('© Replug')).toBeNull()
    expect(within(textLinks).getByRole('link', { name: 'Privacy' }).getAttribute('href')).toBe(
      'https://example.com/privacy',
    )
    expect(within(textLinks).getByRole('link', { name: 'Status' }).getAttribute('href')).toBe(
      'https://status.example.com',
    )
    expect(within(textLinks).queryByRole('link', { name: 'Contact support' })).toBeNull()

    const socialLinks = screen.getByTestId('footer-social-links')
    expect(within(socialLinks).getByRole('link', { name: 'LinkedIn' }).getAttribute('href')).toBe(
      'https://linkedin.com/company/replug',
    )
    expect(within(socialLinks).getByRole('link', { name: 'GitHub' }).getAttribute('href')).toBe(
      'https://github.com/replug',
    )
    const githubIcon = within(socialLinks)
      .getByRole('link', { name: 'GitHub' })
      .querySelector('[data-social-brand-icon="github"]') as HTMLElement | null
    expect(githubIcon?.style.maskImage).toContain('/brands/github.svg')
    expect(within(socialLinks).getByRole('link', { name: 'GitHub' }).querySelector('svg')).toBeNull()

    expect(screen.getByRole('link', { name: 'Powered by Helpin' }).getAttribute('href')).toBe(
      'https://helpin.ai/?utm_campaign=poweredBy&utm_medium=referral&utm_source=replug-ws-12345',
    )
  })

  it('shows the default copyright when the visibility flag is omitted', () => {
    renderWithDocsContext(
      <Footer />,
      {
        ...baseConfig,
        footer_config: {
          links: [],
        },
      },
    )

    expect(screen.getByText(`© ${new Date().getFullYear()} Replug`)).not.toBeNull()
  })

  it('renders contact support when it is added as a footer link', () => {
    renderWithDocsContext(
      <Footer />,
      {
        ...baseConfig,
        footer_config: {
          ...baseConfig.footer_config,
          links: [
            ...(baseConfig.footer_config?.links ?? []),
            { label: 'Contact support', url: 'mailto:support@example.com' },
          ],
        },
      },
    )

    const textLinks = screen.getByTestId('footer-text-links')
    expect(within(textLinks).getByRole('link', { name: 'Contact support' }).getAttribute('href')).toBe(
      'mailto:support@example.com',
    )
  })
})
