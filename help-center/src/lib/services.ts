import { api } from './api'
import type {
  HelpCenterConfig,
  Space,
  ArticleDetail,
  CollectionPage,
  PreviewArticleDetail,
  SearchResult,
  NavItem,
} from './types'

export const helpCenterService = {
  getConfig: (subdomain: string) =>
    api.get<HelpCenterConfig>(`/hc/${subdomain}/config`),

  getSpaces: (subdomain: string, locale: string) =>
    api.get<Space[]>(`/hc/${subdomain}/${locale}/spaces`),

  getSpaceNavigation: (subdomain: string, locale: string, spaceSlug: string) =>
    api.get<NavItem[]>(`/hc/${subdomain}/${locale}/spaces/${spaceSlug}/navigation`),

  getCollection: (
    subdomain: string,
    locale: string,
    collectionSlug: string,
  ) =>
    api.get<CollectionPage>(
      `/hc/${subdomain}/${locale}/collections/${collectionSlug}`,
    ),

  getArticle: (
    subdomain: string,
    locale: string,
    collectionSlug: string,
    articleSlug: string,
  ) =>
    api.get<ArticleDetail>(
      `/hc/${subdomain}/${locale}/collections/${collectionSlug}/articles/${articleSlug}`,
    ),

  search: (subdomain: string, locale: string, query: string, spaceSlug?: string) =>
    api.get<SearchResult[]>(
      `/hc/${subdomain}/${locale}/search?q=${encodeURIComponent(query)}${spaceSlug ? `&space=${encodeURIComponent(spaceSlug)}` : ''}`,
    ),

  submitFeedback: (
    subdomain: string,
    locale: string,
    collectionSlug: string,
    articleSlug: string,
    payload: { is_helpful: boolean; comment?: string },
  ) =>
    api.post(
      `/hc/${subdomain}/${locale}/collections/${collectionSlug}/articles/${articleSlug}/feedback`,
      payload,
    ),

  getPreview: (subdomain: string, docId: string, token: string) =>
    api.get<PreviewArticleDetail>(
      `/hc/${subdomain}/preview/${docId}?token=${encodeURIComponent(token)}`,
    ),
}
