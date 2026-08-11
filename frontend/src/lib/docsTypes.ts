// Docs module TypeScript interfaces — matching backend models in server/internal/model/docs.go

// ─── Doc type and status constants ──────────────────────────────────────────

export type DocStatus = 'draft' | 'published' | 'archived';
export type SpaceType = 'internal' | 'external_capable';
export type SpaceVisibility = 'workspace_wide' | 'team_only';
export type VersionType = 'manual' | 'auto' | 'publish' | 'revert' | 'proposal_apply';
export type LinkContext = 'attached' | 'mentioned' | 'created_from' | 'linked_in_content';
export type LinkedObjectType = 'epic' | 'task' | 'story' | 'project' | 'objective' | 'sprint' | 'support_conversation' | 'deal' | 'contact' | 'company';

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
  /**
   * Parent collection in the same space. null means top-level.
   * The backend enforces a bounded tree with depth 0..2.
   */
  parent_collection_id: string | null;
  /** Tree depth: 0 for top-level, 1 for child, 2 for grandchild. */
  depth: number;
  name: string;
  slug: string;
  description?: string;
  icon?: string;
  position: number;
  sort_key: string;
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
  status: DocStatus;
  visibility: SpaceVisibility;
  owner_id?: string;
  team_id?: string;
  template_key?: string;
  excerpt?: string;
  icon?: string;
  tags: string[];
  position: number;
  sort_key: string;
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
  hc_slug?: string;
  hc_og_title?: string;
  hc_og_description?: string;
  hc_og_image_url?: string;
  hc_og_image_alt?: string;
  has_unpublished_changes?: boolean;
  live_published_at?: string;
  live_slug?: string;
  pending_change_proposal_count?: number;
}

export interface DocsContent {
  id: string;
  document_id: string;
  content: unknown;
  content_text: string;
  word_count: number;
  import_source_system?: string;
  created_at: string;
  updated_at: string;
}

export interface DocsFixFormattingResult {
  content: DocsContent;
  source_system: string;
  republished: boolean;
  warnings: number;
  html_block_fallbacks: number;
  normalized_note_blocks: number;
}

export interface DocsBlock {
  id: string;
  workspace_id: string;
  document_id: string;
  parent_id?: string | null;
  type: string;
  content: unknown;
  content_text?: string;
  sort_key: string;
  revision: number;
  authored_by?: string | null;
  last_edited_by?: string | null;
  created_at: string;
  updated_at: string;
  deleted_at?: string | null;
  agent_readable?: DocsBlockAgentProjection;
}

export interface DocsBlockAgentProjection {
  kind: string;
  block_id: string;
  text?: string;
  attrs?: Record<string, unknown>;
  entity_refs?: DocsBlockAgentRef[];
  citations?: DocsBlockAgentRef[];
  actions?: DocsBlockAgentAction[];
}

export interface DocsBlockAgentRef {
  type: string;
  id: string;
  title?: string;
  access?: 'granted' | 'redacted' | 'unknown';
  redacted?: boolean;
}

export interface DocsBlockAgentAction {
  type: string;
  label: string;
}

export interface DocsAISectionCandidate {
  id: string;
  workspace_id: string;
  document_id: string;
  block_id: string;
  agent_run_id?: string | null;
  status: 'ready' | 'approved' | 'rejected' | 'failed';
  current_content: unknown;
  candidate_content: unknown;
  candidate_text?: string;
  source_refs?: unknown;
  prompt?: string | null;
  prompt_hash?: string | null;
  model?: string | null;
  created_by: string;
  approved_by?: string | null;
  created_at: string;
  updated_at: string;
}

export interface AISectionCandidateResponse {
  candidate: DocsAISectionCandidate | null;
  agent_run?: import('./pmTypes').AgentRun;
  content?: DocsContent;
}

export interface DocsChangeProposalSource {
  type: 'conversation' | 'document' | 'url' | 'agent_run' | 'coverage_gap';
  id?: string;
  label: string;
  url?: string;
}

export interface DocsChangeProposal {
  id: string;
  workspace_id: string;
  document_id: string;
  block_id?: string | null;
  agent_id?: string | null;
  agent_run_id?: string | null;
  scope: 'document' | 'block';
  status: 'pending' | 'applied' | 'discarded' | string;
  revision?: number;
  summary: string;
  content_markdown: string;
  base_markdown?: string;
  content: unknown;
  sources?: DocsChangeProposalSource[];
  created_by: string;
  resolved_by?: string | null;
  resolved_at?: string | null;
  created_at: string;
  updated_at: string;
}

export interface DocsChangeProposalApplyResponse {
  proposal: DocsChangeProposal;
  content: DocsContent;
}

export interface DocsReferenceItem {
  id: string;
  kind: 'doc_link' | 'entity_embed' | 'citation' | 'comment' | 'agent_run' | string;
  title: string;
  description?: string;
  document_id?: string;
  block_id?: string;
  entity_type?: string;
  entity_id?: string;
  reference_id?: string;
  reference_url?: string;
  status?: 'available' | 'unavailable' | string;
  access?: 'granted' | 'unavailable' | string;
  created_at?: string;
}

export interface DocsReferencesResponse {
  items: DocsReferenceItem[];
}

export interface DocsEntityRefRequest {
  entity_type: string;
  entity_id: string;
  label?: string;
  display_id?: string | number | null;
}

export interface DocsResolvedEntityRef {
  entity_type: string;
  entity_id: string;
  status: 'available' | 'unavailable';
  access: 'granted' | 'unavailable' | 'redacted';
  title: string;
  display_id?: string | number | null;
  meta?: string;
  state_label?: string;
  href?: string;
}

export interface ResolveDocsEntityRefsResponse {
  refs: DocsResolvedEntityRef[];
}

export interface DocsResolvedEmbed {
  url: string;
  provider: string;
  title: string;
  description?: string;
  image_url?: string | null;
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
  // Enriched by backend
  linked_object_name?: string;
  linked_object_display_id?: number;
  document_title?: string;
}

export type HelpcenterThemeMode = 'light' | 'dark' | 'system';
export type HelpcenterPublicUrlMode = 'hosted_subdomain' | 'custom_domain' | 'reverse_proxy';

export type HelpcenterHeaderLinkStyle = 'text' | 'button';

export interface HelpcenterHeaderLink {
  label: string;
  url: string;
  external: boolean;
  style: HelpcenterHeaderLinkStyle;
  position: number;
}

export interface HelpcenterFooterLink {
  label: string;
  url: string;
}

export type HelpcenterSocialPlatform =
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
  | 'website';

export interface HelpcenterSocialLink {
  platform: HelpcenterSocialPlatform;
  url: string;
  label?: string;
}

export interface HelpcenterFooterConfig {
  show_copyright?: boolean;
  copyright_text?: string;
  links?: HelpcenterFooterLink[];
  social_links?: HelpcenterSocialLink[];
}

export type HomepageFeaturedCardLinkType = 'space' | 'collection' | 'article' | 'url';

export interface HomepageFeaturedCard {
  title: string;
  description: string;
  icon: string;
  link_type: HomepageFeaturedCardLinkType;
  link_value: string;
  space_slug: string;
}

export interface HelpcenterHomepageConfig {
  hero_title: string;
  hero_subtitle: string;
  featured_cards: HomepageFeaturedCard[];
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
  public_url_mode: HelpcenterPublicUrlMode;
  reverse_proxy_host?: string;
  reverse_proxy_base_path?: string;
  brand_name: string;
  brand_logo_url?: string;
  brand_logo_dark_url?: string;
  brand_color: string;
  favicon_url?: string;
  theme_mode: HelpcenterThemeMode;
  header_links: HelpcenterHeaderLink[];
  footer_config: HelpcenterFooterConfig;
  homepage_config: HelpcenterHomepageConfig;
  space_nav_config: HelpcenterSpaceNavConfig;
  search_placeholder?: string;
  default_locale: string;
  enabled_locales: string[];
  protected_terms: string[];
  show_language_switcher: boolean;
  fallback_to_default_locale: boolean;
  is_published: boolean;
  chat_widget_enabled: boolean;
  ai_answers_enabled: boolean;
  seo_title?: string;
  seo_description?: string;
  og_title?: string;
  og_description?: string;
  og_image_url?: string;
  og_image_alt?: string;
  support_email?: string;
  created_at: string;
  updated_at: string;
}

export interface DocsHelpcenterArticle {
  id: string;
  document_id: string;
  seo_title?: string;
  seo_description?: string;
  og_title?: string;
  og_description?: string;
  og_image_url?: string;
  og_image_alt?: string;
  helpful_count: number;
  not_helpful_count: number;
  view_count: number;
  public_published_at?: string;
  created_at: string;
  updated_at: string;
}

export type DocsHelpcenterTranslationStatus = 'draft' | 'published' | 'needs_review';
export type DocsHelpcenterTranslationState = DocsHelpcenterTranslationStatus | 'missing';

export interface DocsHelpcenterSpaceTranslation {
  id: string;
  space_id: string;
  workspace_id: string;
  locale: string;
  name: string;
  slug?: string;
  description?: string;
  status: DocsHelpcenterTranslationStatus;
  source_updated_at?: string;
  source_synced: boolean;
  published_at?: string;
  created_at: string;
  updated_at: string;
}

export interface DocsHelpcenterCollectionTranslation {
  id: string;
  collection_id: string;
  workspace_id: string;
  space_id: string;
  locale: string;
  name: string;
  description?: string;
  slug?: string;
  status: DocsHelpcenterTranslationStatus;
  source_updated_at?: string;
  source_synced: boolean;
  published_at?: string;
  created_at: string;
  updated_at: string;
}

export interface DocsHelpcenterArticleTranslation {
  id: string;
  document_id: string;
  workspace_id: string;
  space_id: string;
  collection_id?: string;
  locale: string;
  title: string;
  slug?: string;
  excerpt?: string;
  content: unknown;
  content_text: string;
  seo_title?: string;
  seo_description?: string;
  og_title?: string;
  og_description?: string;
  og_image_url?: string;
  og_image_alt?: string;
  status: DocsHelpcenterTranslationStatus;
  source_updated_at?: string;
  source_synced: boolean;
  published_at?: string;
  view_count: number;
  helpful_count: number;
  not_helpful_count: number;
  created_at: string;
  updated_at: string;
  has_unpublished_changes?: boolean;
  live_published_at?: string;
  live_slug?: string;
}

export type DocsHelpcenterLocalesConfig = Pick<
  DocsHelpcenterConfig,
  'default_locale' | 'enabled_locales' | 'show_language_switcher' | 'fallback_to_default_locale'
>;

export interface UpdateDocsHelpcenterLocalesRequest {
  default_locale: string;
  enabled_locales: string[];
  show_language_switcher: boolean;
  fallback_to_default_locale: boolean;
}

export interface UpsertDocsHelpcenterSpaceTranslationRequest {
  locale: string;
  name: string;
  slug?: string;
  description?: string;
  status?: DocsHelpcenterTranslationStatus;
}

export interface UpsertDocsHelpcenterCollectionTranslationRequest {
  locale: string;
  name: string;
  description?: string;
  slug?: string;
  status?: DocsHelpcenterTranslationStatus;
}

/**
 * Payload for POST /docs/helpcenter/translations/auto-translate-missing.
 * One request targets one locale; the frontend loops over every enabled
 * non-default locale when the admin kicks off a bulk auto-translate.
 */
export interface AutoTranslateMissingRequest {
  locale: string;
}

/**
 * One failed target in an auto-translate response. Kind is `space` or
 * `collection`, id identifies the entity, reason is a short human-
 * readable string the UI can show in the results modal.
 */
export interface AutoTranslateFailedItem {
  kind: 'space' | 'collection';
  id: string;
  reason: string;
}

/**
 * Response from POST /docs/helpcenter/translations/auto-translate-missing.
 * Carries the created rows plus any per-target failures so the frontend
 * can both splice new translations into local state and surface diagnostic
 * detail in the results modal.
 */
export interface AutoTranslateMissingResponse {
  locale: string;
  requested: number;
  spaces?: DocsHelpcenterSpaceTranslation[];
  collections?: DocsHelpcenterCollectionTranslation[];
  failed?: AutoTranslateFailedItem[];
}

export interface UpsertDocsHelpcenterArticleTranslationRequest {
  locale: string;
  title: string;
  slug?: string;
  excerpt?: string;
  content: unknown;
  seo_title?: string;
  seo_description?: string;
  og_title?: string;
  og_description?: string;
  og_image_url?: string;
  og_image_alt?: string;
  status?: DocsHelpcenterTranslationStatus;
}

export interface HelpcenterLocaleOption {
  value: string;
  label: string;
}

export const HELP_CENTER_LOCALE_OPTIONS: HelpcenterLocaleOption[] = [
  { value: 'en', label: 'English' },
  { value: 'fr', label: 'French' },
  { value: 'de', label: 'German' },
  { value: 'es', label: 'Spanish' },
  { value: 'it', label: 'Italian' },
  { value: 'pt', label: 'Portuguese' },
  { value: 'pt-br', label: 'Portuguese (Brazil)' },
  { value: 'nl', label: 'Dutch' },
  { value: 'sv', label: 'Swedish' },
  { value: 'da', label: 'Danish' },
  { value: 'no', label: 'Norwegian' },
  { value: 'fi', label: 'Finnish' },
  { value: 'pl', label: 'Polish' },
  { value: 'cs', label: 'Czech' },
  { value: 'ro', label: 'Romanian' },
  { value: 'tr', label: 'Turkish' },
  { value: 'ar', label: 'Arabic' },
  { value: 'he', label: 'Hebrew' },
  { value: 'ja', label: 'Japanese' },
  { value: 'ko', label: 'Korean' },
  { value: 'zh-cn', label: 'Chinese (Simplified)' },
  { value: 'zh-tw', label: 'Chinese (Traditional)' },
];

export function getHelpcenterLocaleLabel(locale: string): string {
  const normalized = locale.trim().toLowerCase();
  const match = HELP_CENTER_LOCALE_OPTIONS.find((option) => option.value === normalized);
  return match?.label ?? normalized.toUpperCase();
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
  default_review_days?: number;
}

export interface UpdateDocsSpaceRequest {
  name?: string;
  slug?: string;
  icon?: string;
  type?: SpaceType;
  visibility?: SpaceVisibility;
  default_review_days?: number;
  team_ids?: string[];
  set_team_ids?: boolean;
}

export interface CreateDocsCollectionRequest {
  name: string;
  description?: string;
  icon?: string;
  /**
   * Optional parent collection id. When omitted or empty the collection
   * is created at the top of the space. Must live in the same space and
   * respect the depth cap.
   */
  parent_collection_id?: string | null;
}

export interface UpdateDocsCollectionRequest {
  name?: string;
  description?: string;
  icon?: string;
  position?: number;
  /**
   * Tri-state reparent sentinel:
   *   - undefined -> leave the parent unchanged
   *   - ""        -> reparent to the top of the space (top-level)
   *   - "id"      -> reparent under that collection in the same space
   * The backend rejects self-parenting, cycles, and depth overflows.
   */
  parent_collection_id?: string | null;
}

export interface DocsCollectionDeleteImpact {
  collection_id: string;
  collection_name: string;
  space_id: string;
  collection_count: number;
  document_count: number;
  archived_document_count: number;
  published_document_count: number;
  public_document_count: number;
}

export interface CreateDocsDocumentRequest {
  space_id: string;
  collection_id?: string;
  title: string;
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

export interface PublishDocsDocumentRequest {
  slug?: string;
  published_content?: unknown;
}

export interface PublishDocsHelpcenterArticleTranslationRequest {
  locale: string;
  slug?: string;
  published_content?: unknown;
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
  public_url_mode?: HelpcenterPublicUrlMode;
  reverse_proxy_host?: string;
  reverse_proxy_base_path?: string;
  brand_name?: string;
  brand_logo_url?: string;
  brand_logo_dark_url?: string;
  brand_color?: string;
  favicon_url?: string;
  theme_mode?: HelpcenterThemeMode;
  header_links?: HelpcenterHeaderLink[];
  footer_config?: HelpcenterFooterConfig;
  homepage_config?: HelpcenterHomepageConfig;
  space_nav_config?: HelpcenterSpaceNavConfig;
  search_placeholder?: string;
  protected_terms?: string[];
  is_published?: boolean;
  chat_widget_enabled?: boolean;
  ai_answers_enabled?: boolean;
  seo_title?: string;
  seo_description?: string;
  og_title?: string;
  og_description?: string;
  og_image_url?: string;
  og_image_alt?: string;
  support_email?: string;
}

export interface UpdateDocsHelpcenterArticleMetadataRequest {
  og_title?: string;
  og_description?: string;
  og_image_url?: string;
  og_image_alt?: string;
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

// ─── Reorder DTOs ───────────────────────────────────────────────────────────

export interface ReorderDocsSpacesRequest {
  section: SpaceType;
  space_ids: string[];
}

export interface ReorderDocsCollectionsRequest {
  collection_ids: string[];
  /**
   * Parent collection whose sibling bucket is being reordered.
   * undefined / null / "" means the top-level bucket in the space.
   * Collection IDs that do not belong to this bucket are silently
   * ignored by the backend.
   */
  parent_collection_id?: string | null;
}

export interface ReorderDocsDocumentsRequest {
  collection_id?: string;
  document_ids: string[];
}

export interface DocsSpaceDeleteImpact {
  space_id: string;
  space_name: string;
  collection_count: number;
  document_count: number;
  archived_document_count: number;
  published_document_count: number;
  public_document_count: number;
}

export interface ReorderDocsChildItem {
  kind: 'collection' | 'article';
  id: string;
}

/**
 * Cross-type reorder payload. Reassigns positions to a mixed list of
 * collections and articles that share the same parent (or the space
 * root when parent_collection_id is nil/empty). Positions are assigned
 * sequentially by index across both types in one server transaction.
 */
export interface ReorderDocsChildrenRequest {
  parent_collection_id?: string | null;
  items: ReorderDocsChildItem[];
}

// ─── Move (sort-key based) ──────────────────────────────────────────────────

export interface MoveDocsItemRef {
  type: 'doc' | 'collection';
  id: string;
}

export interface MoveDocsBucketRef {
  space_id: string;
  parent_collection_id?: string | null;
}

export interface MoveDocsPositionRef {
  before?: MoveDocsItemRef | null;
  after?: MoveDocsItemRef | null;
}

export interface MoveDocsItemRequest {
  item: MoveDocsItemRef;
  target_bucket: MoveDocsBucketRef;
  position: MoveDocsPositionRef;
}

// ─── Display helpers ────────────────────────────────────────────────────────

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
  proposal_apply: 'Applied',
};
