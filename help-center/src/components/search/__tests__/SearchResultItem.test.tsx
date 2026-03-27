import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { SearchResultItem } from '@/components/search/SearchResultItem'

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
        multilingualEnabled
      />,
    )

    const link = screen.getByText('Bonjour').closest('a')
    expect(link).not.toBeNull()
    expect(link?.getAttribute('href')).toBe('/fr/bases/bonjour')
  })
})
