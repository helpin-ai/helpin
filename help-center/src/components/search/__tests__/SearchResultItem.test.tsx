import type { ReactNode } from 'react'
import { render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { SearchResultItem } from '@/components/search/SearchResultItem'

vi.mock('@tanstack/react-router', () => ({
  Link: ({
    to,
    params,
    children,
    ...props
  }: {
    to: string
    params: Record<string, string>
    children: ReactNode
  }) => (
    <a data-to={to} data-params={JSON.stringify(params)} {...props}>
      {children}
    </a>
  ),
}))

describe('SearchResultItem', () => {
  it('uses locale-aware article paths with collection slugs', () => {
    render(
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
      />,
    )

    const link = screen.getByText('Bonjour').closest('a')
    expect(link).not.toBeNull()
    expect(link?.getAttribute('data-to')).toBe(
      '/$locale/$spaceSlug/$collectionSlug/$articleSlug',
    )
    expect(link?.getAttribute('data-params')).toBe(
      JSON.stringify({
        locale: 'fr',
        spaceSlug: 'demarrage',
        collectionSlug: 'bases',
        articleSlug: 'bonjour',
      }),
    )
  })
})
