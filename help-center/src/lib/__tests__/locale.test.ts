import { describe, expect, it } from 'vitest'
import {
  buildCanonicalArticlePath,
  buildCanonicalAPIReferencePath,
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
    expect(buildCanonicalCollectionPath(false, 'en', 'bases', 'abc123ef')).toBe(
      '/c/bases-abc123ef',
    )
    expect(buildCanonicalArticlePath(false, 'en', 'bonjour-fr', 'abc123ef')).toBe(
      '/articles/bonjour-fr-abc123ef',
    )
    expect(buildCanonicalSearchPath(false, 'en', 'billing', 'facturation')).toBe(
      '/search?q=billing&space=facturation',
    )
  })

  it('builds canonical paths for multilingual mode with locale prefixes', () => {
    expect(buildCanonicalHomePath(true, 'en')).toBe('/en')
    expect(buildCanonicalCollectionPath(true, 'en', 'bases', 'abc123ef')).toBe(
      '/en/c/bases-abc123ef',
    )
    expect(buildCanonicalArticlePath(true, 'en', 'bonjour-fr', 'abc123ef')).toBe(
      '/en/articles/bonjour-fr-abc123ef',
    )
    expect(buildCanonicalSearchPath(true, 'en', 'billing', 'facturation')).toBe(
      '/en/search?q=billing&space=facturation',
    )
  })

  it('builds canonical API-reference paths and preserves them across locales', () => {
    expect(
      buildCanonicalAPIReferencePath(false, 'en', 'developers', 'product-api'),
    ).toBe('/developers/api/product-api')
    expect(
      buildCanonicalAPIReferencePath(true, 'fr', 'developpeurs', 'product-api'),
    ).toBe('/fr/developpeurs/api/product-api')

    expect(
      resolveLocaleSwitchPath({
        multilingualEnabled: true,
        targetLocale: 'fr',
        defaultLocale: 'en',
        current: {
          kind: 'api_reference',
          spaceId: 'space-1',
          apiReferenceSlug: 'product-api',
        },
        targetSpaces: [
          {
            id: 'space-1',
            name: 'Développeurs',
            slug: 'developpeurs',
            icon: null,
            description: null,
          },
        ],
      }),
    ).toBe('/fr/developpeurs/api/product-api')
  })

  it('builds locale-aware article and collection paths', () => {
    expect(buildLocaleCollectionPath('fr', 'bases', 'abc123ef')).toBe(
      '/fr/c/bases-abc123ef',
    )
    expect(buildLocaleArticlePath('fr', 'bonjour-fr', 'abc123ef')).toBe(
      '/fr/articles/bonjour-fr-abc123ef',
    )
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
          public_id: '11111111',
          icon: null,
          parent_collection_id: null,
          depth: 0,
          position: 0,
          articles: [
            {
              id: 'article-1',
              title: 'Start Here',
              slug: 'start-here',
              public_id: 'abc123ef',
              position: 0,
            },
          ],
        },
      ],
    })

    expect(path).toBe('/en/articles/start-here-abc123ef')
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
          public_id: '22222222',
          icon: null,
          parent_collection_id: null,
          depth: 0,
          position: 0,
          articles: [
            {
              id: 'article-1',
              title: 'Bonjour',
              slug: 'bonjour-fr',
              public_id: 'abc123ef',
              position: 0,
            },
          ],
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
          public_id: '11111111',
          icon: null,
          parent_collection_id: null,
          depth: 0,
          position: 0,
          articles: [
            {
              id: 'article-1',
              title: 'Start Here',
              slug: 'start-here',
              public_id: 'def456ab',
              position: 0,
            },
          ],
        },
      ],
    })

    expect(path).toBe('/articles/bonjour-fr-abc123ef')
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
          public_id: '22222222',
          icon: null,
          parent_collection_id: null,
          depth: 0,
          position: 0,
          articles: [
            {
              id: 'article-1',
              title: 'Bonjour',
              slug: 'bonjour-fr',
              public_id: 'abc123ef',
              position: 0,
            },
          ],
        },
      ],
    })

    expect(path).toBe('/fr/articles/bonjour-fr-abc123ef')
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
