import { useQuery } from '@tanstack/react-query'
import { queryKeys } from '@/lib/queryKeys'
import { helpCenterService } from '@/lib/services'

function unwrap<T>(res: { data: T | null; error: string | null }): T {
  if (res.error) throw new Error(res.error)
  return res.data as T
}

export function useHelpCenterConfig(subdomain: string) {
  return useQuery({
    queryKey: queryKeys.helpCenter.config(subdomain),
    queryFn: async () => unwrap(await helpCenterService.getConfig(subdomain)),
    enabled: !!subdomain,
  })
}

export function useSpaces(subdomain: string, locale: string) {
  return useQuery({
    queryKey: queryKeys.helpCenter.spaces(subdomain, locale),
    queryFn: async () => unwrap(await helpCenterService.getSpaces(subdomain, locale)),
    enabled: !!subdomain && !!locale,
  })
}

export function useSpaceNavigation(
  subdomain: string,
  locale: string,
  spaceSlug: string,
) {
  return useQuery({
    queryKey: queryKeys.spaces.navigation(subdomain, locale, spaceSlug),
    queryFn: async () =>
      unwrap(
        await helpCenterService.getSpaceNavigation(subdomain, locale, spaceSlug),
      ),
    enabled: !!subdomain && !!locale && !!spaceSlug,
  })
}

export function useCollection(
  subdomain: string,
  locale: string,
  collectionSlug: string,
) {
  return useQuery({
    queryKey: queryKeys.collections.bySlug(subdomain, locale, collectionSlug),
    queryFn: async () =>
      unwrap(
        await helpCenterService.getCollection(
          subdomain,
          locale,
          collectionSlug,
        ),
      ),
    enabled: !!subdomain && !!locale && !!collectionSlug,
  })
}

export function useArticle(
  subdomain: string,
  locale: string,
  collectionSlug: string,
  articleSlug: string,
) {
  return useQuery({
    queryKey: queryKeys.articles.bySlug(subdomain, locale, collectionSlug, articleSlug),
    queryFn: async () =>
      unwrap(
        await helpCenterService.getArticle(
          subdomain,
          locale,
          collectionSlug,
          articleSlug,
        ),
      ),
    enabled: !!subdomain && !!locale && !!collectionSlug && !!articleSlug,
  })
}

export function usePreviewArticle(subdomain: string, docId: string, token: string) {
  return useQuery({
    queryKey: ['preview', subdomain, docId],
    queryFn: async () => unwrap(await helpCenterService.getPreview(subdomain, docId, token)),
    enabled: !!subdomain && !!docId && !!token,
    staleTime: 0,
    retry: false,
  })
}

export function useSearchArticles(
  subdomain: string,
  locale: string,
  query: string,
  spaceSlug?: string,
) {
  return useQuery({
    queryKey: queryKeys.articles.search(subdomain, locale, query, spaceSlug),
    queryFn: async () =>
      unwrap(await helpCenterService.search(subdomain, locale, query, spaceSlug)),
    enabled: !!subdomain && !!locale && query.length >= 2,
  })
}
