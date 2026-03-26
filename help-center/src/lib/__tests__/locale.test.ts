import { describe, expect, it } from 'vitest'
import {
  buildLocaleArticlePath,
  buildLocaleCollectionPath,
  resolveLocaleSwitchPath,
} from '@/lib/locale'

describe('locale helpers', () => {
  it('builds locale-aware article and collection paths', () => {
    expect(buildLocaleCollectionPath('fr', 'demarrage', 'bases')).toBe(
      '/fr/demarrage/bases',
    )
    expect(
      buildLocaleArticlePath('fr', 'demarrage', 'bases', 'bonjour-fr'),
    ).toBe('/fr/demarrage/bases/bonjour-fr')
  })

  it('falls back to default locale path when translation is missing', () => {
    const path = resolveLocaleSwitchPath({
      targetLocale: 'de',
      defaultLocale: 'en',
      current: {
        kind: 'article',
        spaceId: 'space-1',
        collectionId: 'collection-1',
        articleId: 'article-1',
      },
      targetSpaces: [],
      targetNavigation: [],
      fallbackSpaces: [{ id: 'space-1', name: 'Getting Started', slug: 'getting-started', icon: null, description: null }],
      fallbackNavigation: [
        {
          id: 'collection-1',
          name: 'Basics',
          slug: 'basics',
          icon: null,
          articles: [{ id: 'article-1', title: 'Start Here', slug: 'start-here' }],
        },
      ],
    })

    expect(path).toBe('/en/getting-started/basics/start-here')
  })

  it('preserves the scoped search space when switching locales', () => {
    const path = resolveLocaleSwitchPath({
      targetLocale: 'fr',
      defaultLocale: 'en',
      current: {
        kind: 'search',
        spaceId: 'space-1',
        searchQuery: 'billing',
      },
      targetSpaces: [
        {
          id: 'space-1',
          name: 'Facturation',
          slug: 'facturation',
          icon: null,
          description: null,
        },
      ],
      fallbackSpaces: [
        {
          id: 'space-1',
          name: 'Billing',
          slug: 'billing',
          icon: null,
          description: null,
        },
      ],
    })

    expect(path).toBe('/fr/search?q=billing&space=facturation')
  })
})
