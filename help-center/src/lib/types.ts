// ─── Help Center Config ─────────────────────────────────────────────────────

export type HelpcenterThemeMode = 'light' | 'dark' | 'system'

export interface HelpCenterConfig {
  id: string
  workspace_id: string
  subdomain: string
  custom_domain: string | null
  brand_name: string
  brand_logo_url: string | null
  brand_color: string
  favicon_url: string | null
  theme_mode: HelpcenterThemeMode
  search_placeholder: string | null
  is_published: boolean
  seo_title: string | null
  seo_description: string | null
  support_email: string | null
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
  position: number
  article_count?: number
}

// ─── Articles ───────────────────────────────────────────────────────────────

export interface Article {
  id: string
  title: string
  slug: string
  excerpt: string | null
  icon: string | null
  status: string
  collection_id: string | null
  collection_name?: string
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

// ─── Navigation ─────────────────────────────────────────────────────────────

export interface NavItem {
  id: string
  name: string
  slug: string
  icon: string | null
  articles: NavArticle[]
}

export interface NavArticle {
  id: string
  title: string
  slug: string
}

// ─── Search ─────────────────────────────────────────────────────────────────

export interface SearchResult {
  id: string
  title: string
  slug: string
  excerpt: string | null
  collection_name: string | null
  space_slug: string
  space_name?: string
  highlights?: string[]
}

// ─── Router Context ─────────────────────────────────────────────────────────

export interface HelpCenterContext {
  subdomain: string
}
