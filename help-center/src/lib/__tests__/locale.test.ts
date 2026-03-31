import { describe, expect, it } from 'vitest'
import {
  buildCanonicalArticlePath,
  buildCanonicalCollectionPath,
  buildCanonicalHomePath,
  buildCanonicalSearchPath,
  buildLocaleArticlePath,
  buildLocaleCollectionPath,
  isMultilingualEnabled,
  resolveActiveLocale,
  resolveExactLocalePath,
  resolveLocaleSwitchPath,
} from '@/lib/locale'

describe('locale helpers', () => {
  it('detects multilingual mode only when more than one locale is enabled', () => {
    expect(isMultilingualEnabled(['en'])).toBe(false)
    expect(isMultilingualEnabled(['en', 'fr'])).toBe(true)
    expect(isMultilingualEnabled(['en', 'en'])).toBe(false)
  })

  it('builds canonical paths for non-multilingual mode without locale prefixes', () => {
    expect(buildCanonicalHomePath(false, 'en')).toBe('/')
    expect(buildCanonicalCollectionPath(false, 'en', 'bases')).toBe('/bases')
    expect(buildCanonicalArticlePath(false, 'en', 'bases', 'bonjour-fr')).toBe(
      '/bases/bonjour-fr',
    )
    expect(buildCanonicalSearchPath(false, 'en', 'billing', 'facturation')).toBe(
      '/search?q=billing&space=facturation',
    )
  })

  it('builds canonical paths for multilingual mode with locale prefixes', () => {
    expect(buildCanonicalHomePath(true, 'en')).toBe('/en')
    expect(buildCanonicalCollectionPath(true, 'en', 'bases')).toBe('/en/bases')
    expect(buildCanonicalArticlePath(true, 'en', 'bases', 'bonjour-fr')).toBe(
      '/en/bases/bonjour-fr',
    )
    expect(buildCanonicalSearchPath(true, 'en', 'billing', 'facturation')).toBe(
      '/en/search?q=billing&space=facturation',
    )
  })

  it('builds locale-aware article and collection paths', () => {
    expect(buildLocaleCollectionPath('fr', 'bases')).toBe(
      '/fr/bases',
    )
    expect(
      buildLocaleArticlePath('fr', 'bases', 'bonjour-fr'),
    ).toBe('/fr/bases/bonjour-fr')
  })

  it('falls back to default locale path when translation is missing', () => {
    const path = resolveLocaleSwitchPath({
      multilingualEnabled: true,
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

    expect(path).toBe('/en/basics/start-here')
  })

  it('preserves the scoped search space when switching locales', () => {
    const path = resolveLocaleSwitchPath({
      multilingualEnabled: true,
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

  it('returns non-locale canonical paths when multilingual is disabled', () => {
    const path = resolveLocaleSwitchPath({
      multilingualEnabled: false,
      targetLocale: 'fr',
      defaultLocale: 'en',
      current: {
        kind: 'article',
        spaceId: 'space-1',
        collectionId: 'collection-1',
        articleId: 'article-1',
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
      targetNavigation: [
        {
          id: 'collection-1',
          name: 'Bases',
          slug: 'bases',
          icon: null,
          articles: [{ id: 'article-1', title: 'Bonjour', slug: 'bonjour-fr' }],
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

    expect(path).toBe('/bases/bonjour-fr')
  })

  it('returns an exact locale path only when the translated entity exists', () => {
    const path = resolveExactLocalePath({
      multilingualEnabled: true,
      targetLocale: 'fr',
      current: {
        kind: 'article',
        spaceId: 'space-1',
        collectionId: 'collection-1',
        articleId: 'article-1',
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
      targetNavigation: [
        {
          id: 'collection-1',
          name: 'Bases',
          slug: 'bases',
          icon: null,
          articles: [{ id: 'article-1', title: 'Bonjour', slug: 'bonjour-fr' }],
        },
      ],
    })

    expect(path).toBe('/fr/bases/bonjour-fr')
  })

  it('returns null for exact locale paths when the translation does not exist', () => {
    const path = resolveExactLocalePath({
      multilingualEnabled: true,
      targetLocale: 'fr',
      current: {
        kind: 'collection',
        spaceId: 'space-1',
        collectionId: 'collection-1',
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
      targetNavigation: [],
    })

    expect(path).toBeNull()
  })

  it('treats an enabled locale slug as the active locale on legacy root paths', () => {
    expect(
      resolveActiveLocale({
        paramsSpaceSlug: 'fr',
        enabledLocales: ['en', 'fr'],
        defaultLocale: 'en',
      }),
    ).toBe('fr')
  })

  it('falls back to the default locale when the locale param is not enabled', () => {
    expect(
      resolveActiveLocale({
        paramsLocale: 'getting-started',
        enabledLocales: ['en', 'fr'],
        defaultLocale: 'en',
      }),
    ).toBe('en')
  })

  it('falls back to the default locale when the root slug is not an enabled locale', () => {
    expect(
      resolveActiveLocale({
        paramsSpaceSlug: 'help-center',
        enabledLocales: ['en', 'fr'],
        defaultLocale: 'en',
      }),
    ).toBe('en')
  })
})
