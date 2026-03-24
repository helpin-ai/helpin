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

export function useSpaces(subdomain: string) {
  return useQuery({
    queryKey: queryKeys.helpCenter.spaces(subdomain),
    queryFn: async () => unwrap(await helpCenterService.getSpaces(subdomain)),
    enabled: !!subdomain,
  })
}

export function useSpaceNavigation(subdomain: string, spaceSlug: string) {
  return useQuery({
    queryKey: queryKeys.spaces.navigation(subdomain, spaceSlug),
    queryFn: async () =>
      unwrap(
        await helpCenterService.getSpaceNavigation(subdomain, spaceSlug),
      ),
    enabled: !!subdomain && !!spaceSlug,
  })
}

export function useArticle(
  subdomain: string,
  spaceSlug: string,
  articleSlug: string,
) {
  return useQuery({
    queryKey: queryKeys.articles.bySlug(subdomain, spaceSlug, articleSlug),
    queryFn: async () =>
      unwrap(
        await helpCenterService.getArticle(subdomain, spaceSlug, articleSlug),
      ),
    enabled: !!subdomain && !!spaceSlug && !!articleSlug,
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
  query: string,
  spaceSlug?: string,
) {
  return useQuery({
    queryKey: queryKeys.articles.search(subdomain, query, spaceSlug),
    queryFn: async () =>
      unwrap(await helpCenterService.search(subdomain, query, spaceSlug)),
    enabled: !!subdomain && query.length >= 2,
  })
}
