export const queryKeys = {
  helpCenter: {
    config: (subdomain: string) => ['helpCenter', 'config', subdomain] as const,
    spaces: (subdomain: string, locale: string) =>
      ['helpCenter', 'spaces', subdomain, locale] as const,
  },
  spaces: {
    navigation: (subdomain: string, locale: string, spaceSlug: string) =>
      ['spaces', subdomain, locale, spaceSlug, 'navigation'] as const,
  },
  collections: {
    bySlug: (subdomain: string, locale: string, collectionSlug: string) =>
      ['collections', subdomain, locale, collectionSlug] as const,
  },
  articles: {
    bySlug: (
      subdomain: string,
      locale: string,
      collectionSlug: string,
      articleSlug: string,
    ) =>
      ['articles', subdomain, locale, collectionSlug, articleSlug] as const,
    search: (
      subdomain: string,
      locale: string,
      query: string,
      spaceSlug?: string,
    ) => ['articles', 'search', subdomain, locale, query, spaceSlug] as const,
  },
}
