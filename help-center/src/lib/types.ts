import type { QueryClient } from '@tanstack/react-query'

// ─── Help Center Config ─────────────────────────────────────────────────────

export type HelpcenterThemeMode = 'light' | 'dark' | 'system'
export type HelpcenterPublicUrlMode = 'hosted_subdomain' | 'custom_domain' | 'reverse_proxy'

export type HeaderLinkStyle = 'text' | 'button'

export interface HeaderLink {
  label: string
  url: string
  external: boolean
  style: HeaderLinkStyle
  position: number
}

export interface FooterLink {
  label: string
  url: string
}

export type FooterSocialPlatform =
  | 'x'
  | 'twitter'
  | 'linkedin'
  | 'github'
  | 'youtube'
  | 'facebook'
  | 'instagram'
  | 'discord'
  | 'slack'
  | 'rss'
  | 'website'

export interface FooterSocialLink {
  platform: FooterSocialPlatform
  url: string
  label?: string
}

export interface FooterConfig {
  show_copyright?: boolean
  copyright_text?: string
  links?: FooterLink[]
  social_links?: FooterSocialLink[]
}

export interface HelpCenterConfig {
  public_widget_url?: string
  public_sdk_url?: string
  id: string
  workspace_id: string
  subdomain: string
  custom_domain: string | null
  public_url_mode?: HelpcenterPublicUrlMode | null
  reverse_proxy_host?: string | null
  reverse_proxy_base_path?: string | null
  brand_name: string
  brand_logo_url: string | null
  brand_logo_dark_url: string | null
  brand_color: string
  favicon_url: string | null
  theme_mode: HelpcenterThemeMode
  search_placeholder: string | null
  default_locale: string
  enabled_locales: string[]
  show_language_switcher: boolean
  fallback_to_default_locale: boolean
  is_published: boolean
  chat_widget_enabled?: boolean
  ai_answers_enabled?: boolean
  support_widget_key?: string | null
  seo_title: string | null
  seo_description: string | null
  og_title?: string | null
  og_description?: string | null
  og_image_url?: string | null
  og_image_alt?: string | null
  support_email: string | null
  header_links?: HeaderLink[]
  footer_config?: FooterConfig
  homepage_config?: HomepageConfig
}

// ─── Homepage Config ────────────────────────────────────────────────────────

export type HomepageFeaturedCardLinkType = 'space' | 'collection' | 'article' | 'url'

export interface HomepageFeaturedCard {
  title: string
  description: string
  icon: string
  link_type: HomepageFeaturedCardLinkType
  link_value: string
  space_slug: string
  public_id?: string
}

export interface HomepageConfig {
  hero_title?: string
  hero_subtitle?: string
  featured_cards?: HomepageFeaturedCard[]
}

// ─── Spaces ─────────────────────────────────────────────────────────────────

export interface Space {
  id: string
  name: string
  slug: string
  icon: string | null
  description: string | null
}

// API references

export interface APIReferenceSummary {
  id: string
  space_id: string
  name: string
  slug: string
  api_version: string
  operation_count: number
}

export interface APIReference extends APIReferenceSummary {
  openapi_version: string
  specification: Record<string, unknown>
  published_at?: string
}

// ─── Collections (Categories) ───────────────────────────────────────────────

export interface Collection {
  id: string
  space_id: string
  name: string
  description: string | null
  icon: string | null
  slug: string
  public_id: string
  position: number
  article_count?: number
}

// ─── Articles ───────────────────────────────────────────────────────────────

export interface Article {
  id: string
  title: string
  slug: string
  public_id: string
  locale?: string
  requested_locale?: string
  is_fallback?: boolean
  excerpt: string | null
  icon: string | null
  status: string
  space_slug?: string
  collection_id: string | null
  collection_name?: string
  collection_slug?: string | null
  collection_public_id?: string | null
  published_at: string | null
  seo_title: string | null
  seo_description: string | null
  og_title?: string | null
  og_description?: string | null
  og_image_url?: string | null
  og_image_alt?: string | null
  helpful_count: number
  not_helpful_count: number
  view_count: number
}

export interface ArticleDetail extends Article {
  content: Record<string, unknown> | null
  content_html?: string
}

export interface CollectionPage {
  collection: NavItem
  articles: NavArticle[]
  space_slug?: string
  alternate_paths?: Record<string, string>
}

export interface PreviewArticleDetail {
  id: string
  title: string
  excerpt?: string
  icon?: string
  status: string
  collection_id?: string
  collection_name?: string
  space_name: string
  space_slug: string
  content_html: string
}

// ─── Navigation ─────────────────────────────────────────────────────────────

/**
 * NavItem is a collection node returned by the public space navigation
 * endpoint. The backend returns a flat list; callers fold it into a tree
 * using parent_collection_id + depth.
 */
export interface NavItem {
  id: string
  name: string
  slug: string
  public_id: string
  icon: string | null
  /** Parent collection id. null means top-level. */
  parent_collection_id: string | null
  /** Tree depth: 0 for top-level, 1 for child, 2 for grandchild. */
  depth: number
  position: number
  articles: NavArticle[]
}

export interface NavArticle {
  id: string
  title: string
  slug: string
  public_id: string
  position: number
  published_at?: string | null
}

/**
 * NavTreeNode is a client-side folding of NavItem[] into a nested tree.
 * Each node owns its direct child NavItems plus its direct articles.
 */
export interface NavTreeNode {
  item: NavItem
  children: NavTreeNode[]
}

// ─── Search ─────────────────────────────────────────────────────────────────

export interface SearchMatch {
  entry_type: 'title' | 'excerpt' | 'heading' | 'body'
  section_title?: string | null
  anchor?: string | null
  snippet: string
}

export interface SearchResult {
  id: string
  title: string
  slug: string
  public_id: string
  locale?: string
  requested_locale?: string
  is_fallback?: boolean
  excerpt: string | null
  collection_id?: string | null
  collection_name: string | null
  collection_slug?: string | null
  collection_public_id?: string | null
  /**
   * Human-readable localized ancestor breadcrumb such as
   * "Root / Middle / Current". Present only when the article lives in
   * a nested collection; null for top-level or uncategorized articles.
   */
  collection_ancestor_path?: string | null
  space_slug: string
  space_name?: string
  highlights?: string[]
  matches?: SearchMatch[]
}

// ─── Router Context ─────────────────────────────────────────────────────────

export interface HelpCenterContext {
  queryClient: QueryClient
}

export interface AIAnswerCitation {
  document_id: string
  title: string
  slug: string
  public_id: string
  space_slug: string
  collection_slug?: string | null
  snippet?: string
}

export interface AIAnswerResponse {
  answer_id: string
  status: 'answered' | 'insufficient_evidence'
  answer?: string
  citations: AIAnswerCitation[]
  confidence?: number
  cached: boolean
}

export interface HelpCenterBootstrap {
  config: HelpCenterConfig
  locale: string
  spaces: Space[]
}
