export const queryKeys = {
  helpCenter: {
    config: (subdomain: string) => ['helpCenter', 'config', subdomain] as const,
    spaces: (subdomain: string) => ['helpCenter', 'spaces', subdomain] as const,
  },
  spaces: {
    navigation: (subdomain: string, spaceSlug: string) =>
      ['spaces', subdomain, spaceSlug, 'navigation'] as const,
  },
  articles: {
    bySlug: (subdomain: string, spaceSlug: string, articleSlug: string) =>
      ['articles', subdomain, spaceSlug, articleSlug] as const,
    search: (subdomain: string, query: string, spaceSlug?: string) =>
      ['articles', 'search', subdomain, query, spaceSlug] as const,
  },
}
