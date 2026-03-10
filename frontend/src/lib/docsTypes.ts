// Docs module TypeScript interfaces — matching backend models in server/internal/model/docs.go

// ─── Doc type and status constants ──────────────────────────────────────────

export type DocType = 'wiki' | 'sop' | 'feature_doc' | 'product_spec' | 'support_article' | 'help_center_article';
export type DocStatus = 'draft' | 'published' | 'archived';
export type SpaceType = 'internal' | 'external_capable';
export type SpaceVisibility = 'workspace_wide' | 'team_only';
export type VersionType = 'manual' | 'auto' | 'publish' | 'revert';
export type LinkContext = 'attached' | 'mentioned' | 'created_from' | 'linked_in_content';
export type LinkedObjectType = 'epic' | 'story' | 'project' | 'objective' | 'sprint' | 'support_ticket';

// ─── Core models ────────────────────────────────────────────────────────────

export interface DocsSpace {
  id: string;
  workspace_id: string;
  team_id?: string;
  team_ids: string[];
  name: string;
  slug: string;
  icon?: string;
  visibility: SpaceVisibility;
  type: SpaceType;
  restrict_to_owners: boolean;
  default_review_days?: number;
  is_system: boolean;
  position: number;
  created_by: string;
  created_at: string;
  updated_at: string;
  deleted_at?: string;
}

export interface DocsCollection {
  id: string;
  space_id: string;
  workspace_id: string;
  name: string;
  description?: string;
  icon?: string;
  position: number;
  created_by: string;
  created_at: string;
  updated_at: string;
  deleted_at?: string;
}

export interface DocsDocument {
  id: string;
  workspace_id: string;
  space_id: string;
  collection_id?: string;
  title: string;
  doc_type: DocType;
  status: DocStatus;
  visibility: SpaceVisibility;
  owner_id?: string;
  team_id?: string;
  template_key?: string;
  excerpt?: string;
  icon?: string;
  tags: string[];
  is_pinned: boolean;
  is_publicly_shared: boolean;
  share_token?: string;
  is_locked: boolean;
  locked_by?: string;
  last_reviewed_at?: string;
  next_review_at?: string;
  published_at?: string;
  created_by: string;
  created_at: string;
  updated_at: string;
  deleted_at?: string;
}

export interface DocsContent {
  id: string;
  document_id: string;
  content: unknown;
  content_text: string;
  word_count: number;
  created_at: string;
  updated_at: string;
}

export interface DocsVersion {
  id: string;
  document_id: string;
  content: unknown;
  content_text: string;
  snapshot_label?: string;
  version_type: VersionType;
  word_count: number;
  created_by: string;
  created_at: string;
}

export interface DocsLink {
  id: string;
  workspace_id: string;
  document_id: string;
  linked_object_type: LinkedObjectType;
  linked_object_id: string;
  link_context: LinkContext;
  created_by: string;
  created_at: string;
}

export type HelpcenterThemeMode = 'light' | 'dark' | 'system';

export interface HelpcenterHeaderLink {
  label: string;
  url: string;
  external: boolean;
}

export interface HelpcenterFooterLink {
  label: string;
  url: string;
}

export interface HelpcenterFooterConfig {
  copyright_text: string;
  links: HelpcenterFooterLink[];
}

export interface HelpcenterHomepageConfig {
  hero_title: string;
  hero_subtitle: string;
  featured_space_ids: string[];
}

export interface HelpcenterSpaceNavConfig {
  order: string[];
  hidden: string[];
}

export interface DocsHelpcenterConfig {
  id: string;
  workspace_id: string;
  subdomain: string;
  custom_domain?: string;
  brand_name: string;
  brand_logo_url?: string;
  brand_color: string;
  favicon_url?: string;
  theme_mode: HelpcenterThemeMode;
  header_links: HelpcenterHeaderLink[];
  footer_config: HelpcenterFooterConfig;
  homepage_config: HelpcenterHomepageConfig;
  space_nav_config: HelpcenterSpaceNavConfig;
  search_placeholder?: string;
  is_published: boolean;
  seo_title?: string;
  seo_description?: string;
  support_email?: string;
  created_at: string;
  updated_at: string;
}

export interface DocsHelpcenterArticle {
  id: string;
  document_id: string;
  seo_title?: string;
  seo_description?: string;
  helpful_count: number;
  not_helpful_count: number;
  view_count: number;
  public_published_at?: string;
  created_at: string;
  updated_at: string;
}

export interface DocsSlugAlias {
  id: string;
  workspace_id: string;
  document_id: string;
  old_slug: string;
  created_at: string;
}

// ─── Search result ──────────────────────────────────────────────────────────

export interface DocsSearchResult extends DocsDocument {
  rank: number;
}

// ─── Request DTOs ───────────────────────────────────────────────────────────

export interface CreateDocsSpaceRequest {
  team_id?: string;
  team_ids?: string[];
  name: string;
  slug?: string;
  icon?: string;
  visibility: SpaceVisibility;
  type: SpaceType;
  restrict_to_owners: boolean;
  default_review_days?: number;
}

export interface UpdateDocsSpaceRequest {
  name?: string;
  slug?: string;
  icon?: string;
  type?: SpaceType;
  visibility?: SpaceVisibility;
  restrict_to_owners?: boolean;
  default_review_days?: number;
  team_ids?: string[];
  set_team_ids?: boolean;
}

export interface CreateDocsCollectionRequest {
  name: string;
  description?: string;
  icon?: string;
}

export interface UpdateDocsCollectionRequest {
  name?: string;
  description?: string;
  icon?: string;
  position?: number;
}

export interface CreateDocsDocumentRequest {
  space_id: string;
  collection_id?: string;
  title: string;
  doc_type: DocType;
  owner_id?: string;
  template_key?: string;
  icon?: string;
  tags?: string[];
}

export interface UpdateDocsDocumentRequest {
  title?: string;
  collection_id?: string;
  owner_id?: string;
  template_key?: string;
  excerpt?: string;
  icon?: string;
  tags?: string[];
  is_pinned?: boolean;
}

export interface MoveDocsDocumentRequest {
  space_id: string;
  collection_id?: string;
}

export interface SaveDocsContentRequest {
  content: unknown;
}

export interface CreateDocsVersionRequest {
  snapshot_label?: string;
}

export interface UpdateDocsVersionRequest {
  snapshot_label?: string;
}

export interface CreateDocsLinkRequest {
  linked_object_type: LinkedObjectType;
  linked_object_id: string;
  link_context: LinkContext;
}

export interface UpdateDocsHelpcenterConfigRequest {
  subdomain?: string;
  custom_domain?: string;
  brand_name?: string;
  brand_logo_url?: string;
  brand_color?: string;
  favicon_url?: string;
  theme_mode?: HelpcenterThemeMode;
  header_links?: HelpcenterHeaderLink[];
  footer_config?: HelpcenterFooterConfig;
  homepage_config?: HelpcenterHomepageConfig;
  space_nav_config?: HelpcenterSpaceNavConfig;
  search_placeholder?: string;
  is_published?: boolean;
  seo_title?: string;
  seo_description?: string;
  support_email?: string;
}

export interface DocsArticleFeedbackRequest {
  is_helpful: boolean;
  comment?: string;
  session_id?: string;
}

export interface ToggleDocShareRequest {
  is_publicly_shared: boolean;
}

export interface PublicDocResponse {
  document: DocsDocument;
  content: DocsContent | null;
}

// ─── Display helpers ────────────────────────────────────────────────────────

export const DOC_TYPE_LABELS: Record<DocType, string> = {
  wiki: 'Wiki',
  sop: 'SOP',
  feature_doc: 'Feature Doc',
  product_spec: 'Product Spec',
  support_article: 'Support Article',
  help_center_article: 'Help Center Article',
};

export const DOC_STATUS_LABELS: Record<DocStatus, string> = {
  draft: 'Draft',
  published: 'Published',
  archived: 'Archived',
};

export const VERSION_TYPE_LABELS: Record<VersionType, string> = {
  manual: 'Manual',
  auto: 'Auto',
  publish: 'Published',
  revert: 'Reverted',
};
