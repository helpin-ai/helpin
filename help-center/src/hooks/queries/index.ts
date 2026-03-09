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

export function useNavigation(subdomain: string) {
  return useQuery({
    queryKey: queryKeys.helpCenter.navigation(subdomain),
    queryFn: async () =>
      unwrap(await helpCenterService.getNavigation(subdomain)),
    enabled: !!subdomain,
  })
}

export function useArticle(subdomain: string, slug: string) {
  return useQuery({
    queryKey: queryKeys.articles.bySlug(subdomain, slug),
    queryFn: async () =>
      unwrap(await helpCenterService.getArticle(subdomain, slug)),
    enabled: !!subdomain && !!slug,
  })
}

export function useSearchArticles(subdomain: string, query: string) {
  return useQuery({
    queryKey: queryKeys.articles.search(subdomain, query),
    queryFn: async () =>
      unwrap(await helpCenterService.search(subdomain, query)),
    enabled: !!subdomain && query.length >= 2,
  })
}

export function useCollection(subdomain: string, slug: string) {
  return useQuery({
    queryKey: queryKeys.collections.bySlug(subdomain, slug),
    queryFn: async () =>
      unwrap(await helpCenterService.getCollection(subdomain, slug)),
    enabled: !!subdomain && !!slug,
  })
}
