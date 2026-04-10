export interface HelpCollection {
  id: string;
  name: string;
  slug: string;
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
  excerpt?: string;
  icon?: string;
}

export interface HelpArticle {
  id: string;
  title: string;
  slug: string;
  excerpt?: string;
  icon?: string;
  content_html?: string | null;
  public_path?: string;
}

function getApiBase(host: string): string {
  if (host.startsWith('http://') || host.startsWith('https://')) {
    return host.replace(/\/$/, '');
  }
  return `https://${host}`;
}

async function fetchHelpJSON<T>(host: string, widgetKey: string, path: string): Promise<T> {
  const response = await fetch(`${getApiBase(host)}${path}?widget_key=${encodeURIComponent(widgetKey)}`);
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

export function fetchHelpArticle(host: string, widgetKey: string, articleSlug: string): Promise<HelpArticle> {
  return fetchHelpJSON(host, widgetKey, `/widget/support/help/articles/${encodeURIComponent(articleSlug)}`);
}
