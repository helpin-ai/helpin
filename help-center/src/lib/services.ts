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

function buildHelpCenterPath(
  subdomain: string,
  locale: string,
  multilingualEnabled: boolean,
  localizedPath: string,
  canonicalPath: string,
) {
  if (multilingualEnabled) {
    return `/hc/${subdomain}/${locale}${localizedPath}`
  }
  return `/hc/${subdomain}${canonicalPath}`
}

export const helpCenterService = {
  getConfig: (subdomain: string) =>
    api.get<HelpCenterConfig>(`/hc/${subdomain}/config`),

  getSpaces: (subdomain: string, locale: string, multilingualEnabled: boolean) =>
    api.get<Space[]>(
      buildHelpCenterPath(
        subdomain,
        locale,
        multilingualEnabled,
        '/spaces',
        '/spaces',
      ),
    ),

  getSpaceNavigation: (
    subdomain: string,
    locale: string,
    spaceSlug: string,
    multilingualEnabled: boolean,
  ) =>
    api.get<NavItem[]>(
      buildHelpCenterPath(
        subdomain,
        locale,
        multilingualEnabled,
        `/spaces/${spaceSlug}/navigation`,
        `/spaces/${spaceSlug}/navigation`,
      ),
    ),

  getCollection: (
    subdomain: string,
    locale: string,
    collectionSlug: string,
    multilingualEnabled: boolean,
  ) =>
    api.get<CollectionPage>(
      buildHelpCenterPath(
        subdomain,
        locale,
        multilingualEnabled,
        `/c/${collectionSlug}`,
        `/c/${collectionSlug}`,
      ),
    ),

  getArticle: (
    subdomain: string,
    locale: string,
    articleKey: string,
    multilingualEnabled: boolean,
  ) =>
    api.get<ArticleDetail>(
      buildHelpCenterPath(
        subdomain,
        locale,
        multilingualEnabled,
        `/articles/${articleKey}`,
        `/articles/${articleKey}`,
      ),
    ),

  search: (
    subdomain: string,
    locale: string,
    query: string,
    multilingualEnabled: boolean,
    spaceSlug?: string,
  ) =>
    api.get<SearchResult[]>(
      `${buildHelpCenterPath(
        subdomain,
        locale,
        multilingualEnabled,
        '/search',
        '/search',
      )}?q=${encodeURIComponent(query)}&limit=20${spaceSlug ? `&space=${encodeURIComponent(spaceSlug)}` : ''}`,
    ),

  submitFeedback: (
    subdomain: string,
    locale: string,
    articleKey: string,
    multilingualEnabled: boolean,
    payload: { is_helpful: boolean; comment?: string },
  ) =>
    api.post(
      buildHelpCenterPath(
        subdomain,
        locale,
        multilingualEnabled,
        `/articles/${articleKey}/feedback`,
        `/articles/${articleKey}/feedback`,
      ),
      payload,
    ),

  getPreview: (subdomain: string, docId: string, token: string) =>
    api.get<PreviewArticleDetail>(
      `/hc/${subdomain}/preview/${docId}?token=${encodeURIComponent(token)}`,
    ),
}
