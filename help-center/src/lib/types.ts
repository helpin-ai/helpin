import type { QueryClient } from '@tanstack/react-query'

// ─── Help Center Config ─────────────────────────────────────────────────────

export type HelpcenterThemeMode = 'light' | 'dark' | 'system'

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

export interface FooterConfig {
  copyright_text?: string
  links?: FooterLink[]
}

export interface HelpCenterConfig {
  id: string
  workspace_id: string
  subdomain: string
  custom_domain: string | null
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
  seo_title: string | null
  seo_description: string | null
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
  articles: NavArticle[]
}

export interface NavArticle {
  id: string
  title: string
  slug: string
  public_id: string
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
}

// ─── Router Context ─────────────────────────────────────────────────────────

export interface HelpCenterContext {
  queryClient: QueryClient
}
