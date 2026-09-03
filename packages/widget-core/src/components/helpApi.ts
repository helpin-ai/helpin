export interface HelpCollection {
  id: string;
  name: string;
  slug: string;
  public_id: string;
  icon?: string;
  /** Parent collection id. null means top-level in the space. */
  parent_collection_id: string | null;
  /** Tree depth: 0 top-level, 1 child, 2 grandchild. */
  depth: number;
  article_count: number;
}

export interface HelpArticleSummary {
  id: string;
  title: string;
  slug: string;
  public_id: string;
  article_key: string;
  excerpt?: string;
  icon?: string;
}

export interface HelpArticle {
  id: string;
  title: string;
  slug: string;
  public_id: string;
  article_key: string;
  excerpt?: string;
  icon?: string;
  content_html?: string | null;
  public_path?: string;
}

export interface HelpSearchResult {
  id: string;
  title: string;
  slug: string;
  public_id: string;
  article_key: string;
  excerpt?: string;
  collection_name?: string | null;
  space_name?: string;
}

export function buildHelpArticleKey(slug: string, publicId?: string | null): string {
  const trimmedSlug = slug.trim().replace(/^\/+|\/+$/g, '');
  const trimmedPublicId = (publicId ?? '').trim().toLowerCase();
  if (trimmedSlug && trimmedPublicId) {
    return `${trimmedSlug}-${trimmedPublicId}`;
  }
  return trimmedSlug || trimmedPublicId;
}

export function buildHelpCollectionKey(slug: string, publicId?: string | null): string {
  const trimmedSlug = slug.trim().replace(/^\/+|\/+$/g, '');
  const trimmedPublicId = (publicId ?? '').trim().toLowerCase();
  if (trimmedSlug && trimmedPublicId) {
    return `${trimmedSlug}-${trimmedPublicId}`;
  }
  return trimmedSlug || trimmedPublicId;
}

function getApiBase(host: string): string {
  if (host.startsWith('http://') || host.startsWith('https://')) {
    return host.replace(/\/$/, '');
  }
  return `https://${host}`;
}

async function fetchHelpJSON<T>(host: string, widgetKey: string, path: string): Promise<T> {
  const separator = path.includes('?') ? '&' : '?';
  const response = await fetch(`${getApiBase(host)}${path}${separator}widget_key=${encodeURIComponent(widgetKey)}`);
  if (!response.ok) {
    throw new Error(`Request failed: ${response.status}`);
  }
  return response.json() as Promise<T>;
}

export function fetchHelpCollections(host: string, widgetKey: string, spaceSlug: string): Promise<HelpCollection[]> {
  return fetchHelpJSON(host, widgetKey, `/widget/support/help/spaces/${encodeURIComponent(spaceSlug)}/collections`);
}

export function fetchHelpArticles(host: string, widgetKey: string, collectionSlug: string): Promise<HelpArticleSummary[]> {
  return fetchHelpJSON(host, widgetKey, `/widget/support/help/collections/${encodeURIComponent(collectionSlug)}/articles`);
}

export function fetchHelpArticle(host: string, widgetKey: string, articleKey: string): Promise<HelpArticle> {
  return fetchHelpJSON(host, widgetKey, `/widget/support/help/articles/${encodeURIComponent(articleKey)}`);
}

/**
 * Searches help articles. `anonymousId` is the visitor's durable browser id when
 * the host SDK has one; it attributes the search to the visitor who made it so
 * self-service questions stay traceable to an account.
 */
export function fetchHelpSearchResults(
  host: string,
  widgetKey: string,
  query: string,
  limit = 8,
  anonymousId?: string,
	coverageSignal = false,
): Promise<HelpSearchResult[]> {
  const params = new URLSearchParams({
    q: query,
    limit: String(limit),
  });
  if (anonymousId) {
    params.set('anonymous_id', anonymousId);
  }
	if (coverageSignal) {
	  params.set('coverage_signal', '1');
	}
  return fetchHelpJSON(host, widgetKey, `/widget/support/help/search?${params.toString()}`);
}
