import { queryOptions, useQuery } from '@tanstack/react-query'
import { queryKeys } from '@/lib/queryKeys'
import { helpCenterService } from '@/lib/services'

function unwrap<T>(res: { data: T | null; error: string | null }): T {
  if (res.error) throw new Error(res.error)
  return res.data as T
}

export function helpCenterConfigQueryOptions(subdomain: string) {
  return queryOptions({
    queryKey: queryKeys.helpCenter.config(subdomain),
    queryFn: async () => unwrap(await helpCenterService.getConfig(subdomain)),
  })
}

export function useHelpCenterConfig(subdomain: string) {
  return useQuery({
    ...helpCenterConfigQueryOptions(subdomain),
    enabled: !!subdomain,
  })
}

export function spacesQueryOptions(
  subdomain: string,
  locale: string,
  multilingualEnabled: boolean,
) {
  return queryOptions({
    queryKey: queryKeys.helpCenter.spaces(subdomain, locale),
    queryFn: async () =>
      unwrap(await helpCenterService.getSpaces(subdomain, locale, multilingualEnabled)),
  })
}

export function useSpaces(
  subdomain: string,
  locale: string,
  multilingualEnabled: boolean,
  enabled = true,
) {
  return useQuery({
    ...spacesQueryOptions(subdomain, locale, multilingualEnabled),
    enabled: enabled && !!subdomain && !!locale,
  })
}

export function spaceNavigationQueryOptions(
  subdomain: string,
  locale: string,
  spaceSlug: string,
  multilingualEnabled: boolean,
) {
  return queryOptions({
    queryKey: queryKeys.spaces.navigation(subdomain, locale, spaceSlug),
    queryFn: async () =>
      unwrap(
        await helpCenterService.getSpaceNavigation(
          subdomain,
          locale,
          spaceSlug,
          multilingualEnabled,
        ),
      ),
  })
}

export function useSpaceNavigation(
  subdomain: string,
  locale: string,
  spaceSlug: string,
  multilingualEnabled: boolean,
) {
  return useQuery({
    ...spaceNavigationQueryOptions(
      subdomain,
      locale,
      spaceSlug,
      multilingualEnabled,
    ),
    enabled: !!subdomain && !!locale && !!spaceSlug,
  })
}

export function collectionQueryOptions(
  subdomain: string,
  locale: string,
  collectionSlug: string,
  multilingualEnabled: boolean,
) {
  return queryOptions({
    queryKey: queryKeys.collections.bySlug(subdomain, locale, collectionSlug),
    queryFn: async () =>
      unwrap(
        await helpCenterService.getCollection(
          subdomain,
          locale,
          collectionSlug,
          multilingualEnabled,
        ),
      ),
  })
}

export function useCollection(
  subdomain: string,
  locale: string,
  collectionSlug: string,
  multilingualEnabled: boolean,
) {
  return useQuery({
    ...collectionQueryOptions(
      subdomain,
      locale,
      collectionSlug,
      multilingualEnabled,
    ),
    enabled: !!subdomain && !!locale && !!collectionSlug,
  })
}

export function articleQueryOptions(
  subdomain: string,
  locale: string,
  articleKey: string,
  multilingualEnabled: boolean,
) {
  return queryOptions({
    queryKey: queryKeys.articles.byKey(subdomain, locale, articleKey),
    queryFn: async () =>
      unwrap(
        await helpCenterService.getArticle(
          subdomain,
          locale,
          articleKey,
          multilingualEnabled,
        ),
      ),
  })
}

export function useArticle(
  subdomain: string,
  locale: string,
  articleKey: string,
  multilingualEnabled: boolean,
) {
  return useQuery({
    ...articleQueryOptions(
      subdomain,
      locale,
      articleKey,
      multilingualEnabled,
    ),
    enabled: !!subdomain && !!locale && !!articleKey,
  })
}

export function previewArticleQueryOptions(subdomain: string, docId: string, token: string) {
  return queryOptions({
    queryKey: ['preview', subdomain, docId],
    queryFn: async () => unwrap(await helpCenterService.getPreview(subdomain, docId, token)),
    staleTime: 0,
    retry: false,
  })
}

export function usePreviewArticle(subdomain: string, docId: string, token: string) {
  return useQuery({
    ...previewArticleQueryOptions(subdomain, docId, token),
    enabled: !!subdomain && !!docId && !!token,
  })
}

export function searchArticlesQueryOptions(
  subdomain: string,
  locale: string,
  query: string,
  multilingualEnabled: boolean,
  spaceSlug?: string,
) {
  return queryOptions({
    queryKey: queryKeys.articles.search(subdomain, locale, query, spaceSlug),
    queryFn: async () =>
      unwrap(
        await helpCenterService.search(
          subdomain,
          locale,
          query,
          multilingualEnabled,
          spaceSlug,
        ),
      ),
  })
}

export function useSearchArticles(
  subdomain: string,
  locale: string,
  query: string,
  multilingualEnabled: boolean,
  spaceSlug?: string,
) {
  return useQuery({
    ...searchArticlesQueryOptions(
      subdomain,
      locale,
      query,
      multilingualEnabled,
      spaceSlug,
    ),
    enabled: !!subdomain && !!locale && query.length >= 2,
  })
}
