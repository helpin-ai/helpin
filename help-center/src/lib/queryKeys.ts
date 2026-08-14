export const queryKeys = {
  helpCenter: {
    config: (subdomain: string) => ['helpCenter', 'config', subdomain] as const,
    bootstrap: (subdomain: string, localeScope: string) =>
      ['helpCenter', 'bootstrap', subdomain, localeScope] as const,
    spaces: (subdomain: string, locale: string) =>
      ['helpCenter', 'spaces', subdomain, locale] as const,
  },
  spaces: {
    navigation: (subdomain: string, locale: string, spaceSlug: string) =>
      ['spaces', subdomain, locale, spaceSlug, 'navigation'] as const,
    apiReferences: (subdomain: string, locale: string, spaceSlug: string) =>
      ['spaces', subdomain, locale, spaceSlug, 'apiReferences'] as const,
    apiReference: (
      subdomain: string,
      locale: string,
      spaceSlug: string,
      referenceSlug: string,
    ) =>
      ['spaces', subdomain, locale, spaceSlug, 'apiReferences', referenceSlug] as const,
  },
  collections: {
    bySlug: (subdomain: string, locale: string, collectionSlug: string) =>
      ['collections', subdomain, locale, collectionSlug] as const,
  },
  articles: {
    byKey: (
      subdomain: string,
      locale: string,
      articleKey: string,
    ) => ['articles', subdomain, locale, articleKey] as const,
    search: (
      subdomain: string,
      locale: string,
      query: string,
      spaceSlug?: string,
    ) => ['articles', 'search', subdomain, locale, query, spaceSlug] as const,
  },
}
