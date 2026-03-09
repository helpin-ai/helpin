export const queryKeys = {
  helpCenter: {
    config: (subdomain: string) => ['helpCenter', 'config', subdomain] as const,
    navigation: (subdomain: string) =>
      ['helpCenter', 'navigation', subdomain] as const,
    bootstrap: (subdomain: string) =>
      ['helpCenter', 'bootstrap', subdomain] as const,
  },
  articles: {
    bySlug: (subdomain: string, slug: string) =>
      ['articles', subdomain, slug] as const,
    search: (subdomain: string, query: string) =>
      ['articles', 'search', subdomain, query] as const,
  },
  collections: {
    bySlug: (subdomain: string, slug: string) =>
      ['collections', subdomain, slug] as const,
  },
}
