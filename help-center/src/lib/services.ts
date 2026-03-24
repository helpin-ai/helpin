import { api } from './api'
import type {
  HelpCenterConfig,
  Space,
  ArticleDetail,
  PreviewArticleDetail,
  SearchResult,
  NavItem,
} from './types'

export const helpCenterService = {
  getConfig: (subdomain: string) =>
    api.get<HelpCenterConfig>(`/hc/${subdomain}/config`),

  getSpaces: (subdomain: string) =>
    api.get<Space[]>(`/hc/${subdomain}/spaces`),

  getSpaceNavigation: (subdomain: string, spaceSlug: string) =>
    api.get<NavItem[]>(`/hc/${subdomain}/spaces/${spaceSlug}/navigation`),

  getArticle: (subdomain: string, spaceSlug: string, articleSlug: string) =>
    api.get<ArticleDetail>(
      `/hc/${subdomain}/spaces/${spaceSlug}/articles/${articleSlug}`,
    ),

  search: (subdomain: string, query: string, spaceSlug?: string) =>
    api.get<SearchResult[]>(
      `/hc/${subdomain}/search?q=${encodeURIComponent(query)}${spaceSlug ? `&space=${encodeURIComponent(spaceSlug)}` : ''}`,
    ),

  submitFeedback: (
    subdomain: string,
    spaceSlug: string,
    articleSlug: string,
    payload: { is_helpful: boolean; comment?: string },
  ) =>
    api.post(
      `/hc/${subdomain}/spaces/${spaceSlug}/articles/${articleSlug}/feedback`,
      payload,
    ),

  getPreview: (subdomain: string, docId: string, token: string) =>
    api.get<PreviewArticleDetail>(
      `/hc/${subdomain}/preview/${docId}?token=${encodeURIComponent(token)}`,
    ),
}
