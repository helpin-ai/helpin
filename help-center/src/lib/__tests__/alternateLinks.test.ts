import { describe, expect, it } from 'vitest'
import { buildCollectionAlternateLinks } from '@/lib/alternateLinks'
import type { RootRouteData } from '@/lib/rootLoader'
import type { CollectionPage } from '@/lib/types'

describe('buildCollectionAlternateLinks', () => {
  it('preserves published locale alternates and x-default without navigation requests', () => {
    const rootData = {
      activeLocale: 'fr',
      basepath: '',
      config: {
        default_locale: 'en',
        enabled_locales: ['en', 'fr', 'de'],
      },
      host: 'docs.example.com',
      multilingualEnabled: true,
      origin: 'https://docs.example.com',
      spaces: [],
      subdomain: 'docs',
    } as unknown as RootRouteData
    const collection = {
      collection: {
        id: 'collection-1',
        name: 'Premiers pas',
        slug: 'premiers-pas',
        public_id: 'abc123ef',
        icon: null,
        parent_collection_id: null,
        depth: 0,
        position: 0,
        articles: [],
      },
      articles: [],
      alternate_paths: {
        en: '/en/c/getting-started-abc123ef',
        fr: '/fr/c/premiers-pas-abc123ef',
      },
    } satisfies CollectionPage

    expect(buildCollectionAlternateLinks(rootData, collection)).toEqual([
      {
        rel: 'alternate',
        hrefLang: 'en',
        href: 'https://docs.example.com/en/c/getting-started-abc123ef',
      },
      {
        rel: 'alternate',
        hrefLang: 'fr',
        href: 'https://docs.example.com/fr/c/premiers-pas-abc123ef',
      },
      {
        rel: 'alternate',
        hrefLang: 'x-default',
        href: 'https://docs.example.com/en/c/getting-started-abc123ef',
      },
    ])
  })
})
