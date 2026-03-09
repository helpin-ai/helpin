import { api } from './api'
import type {
  HelpCenterConfig,
  ArticleDetail,
  SearchResult,
  NavItem,
  Collection,
  Article,
} from './types'

export const helpCenterService = {
  getConfig: (subdomain: string) =>
    api.get<HelpCenterConfig>(`/hc/${subdomain}/config`),

  getNavigation: (subdomain: string) =>
    api.get<NavItem[]>(`/hc/${subdomain}/navigation`),

  getArticle: (subdomain: string, slug: string) =>
    api.get<ArticleDetail>(`/hc/${subdomain}/articles/${slug}`),

  search: (subdomain: string, query: string) =>
    api.get<SearchResult[]>(
      `/hc/${subdomain}/search?q=${encodeURIComponent(query)}`,
    ),

  getCollection: (subdomain: string, slug: string) =>
    api.get<{ collection: Collection; articles: Article[] }>(
      `/hc/${subdomain}/collections/${slug}`,
    ),
}
