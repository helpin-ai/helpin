import { FunctionComponent } from 'preact';
import { useEffect, useMemo, useState } from 'preact/hooks';
import { ChevronLeftIcon, ChevronRightIcon, FileTextIcon } from './icons';
import {
  buildHelpArticleKey,
  fetchHelpArticles,
  fetchHelpCollections,
  buildHelpCollectionKey,
  type HelpArticleSummary,
  type HelpCollection,
} from './helpApi';
import {
  buildHelpCollectionTree,
  findHelpCollectionBySlug,
  helpCollectionAncestorPath,
} from './helpTree';

interface HelpCollectionViewProps {
  host: string;
  widgetKey: string;
  /** Collection slug the user is currently viewing. */
  collectionSlug: string;
  /**
   * Space slug for the space this collection belongs to. When provided,
   * the view fetches the space's full collection list and renders any
   * child collections as drilldown tiles above the direct articles.
   */
  spaceSlug?: string;
  onBack: () => void;
  /**
   * Called when the user clicks a child collection tile. The widget
   * state owner (ChatWindow) updates activeCollectionSlug to the new
   * slug, which re-renders this view.
   */
  onSelectCollection?: (collectionSlug: string) => void;
  onSelectArticle: (articleKey: string) => void;
}

export const HelpCollectionView: FunctionComponent<HelpCollectionViewProps> = ({
  host,
  widgetKey,
  collectionSlug,
  spaceSlug,
  onBack,
  onSelectCollection,
  onSelectArticle,
}) => {
  const [articles, setArticles] = useState<HelpArticleSummary[]>([]);
  const [collections, setCollections] = useState<HelpCollection[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setIsLoading(true);
    setError(null);

    const articlesPromise = fetchHelpArticles(host, widgetKey, collectionSlug);
    // Fetch the space's full collection list so we can render nested
    // children alongside the direct articles. Skipped when no spaceSlug
    // is provided — the view degrades gracefully to articles-only.
    const collectionsPromise = spaceSlug
      ? fetchHelpCollections(host, widgetKey, spaceSlug).catch(() => [] as HelpCollection[])
      : Promise.resolve([] as HelpCollection[]);

    Promise.all([articlesPromise, collectionsPromise])
      .then(([articleData, collectionData]) => {
        if (!cancelled) {
          setArticles(articleData);
          setCollections(collectionData);
        }
      })
      .catch(() => {
        if (!cancelled) {
          setError('Unable to load articles right now.');
        }
      })
      .finally(() => {
        if (!cancelled) {
          setIsLoading(false);
        }
      });

    return () => {
      cancelled = true;
    };
  }, [collectionSlug, host, widgetKey, spaceSlug]);

  const tree = useMemo(() => buildHelpCollectionTree(collections), [collections]);
  const activeNode = useMemo(
    () => findHelpCollectionBySlug(tree, collectionSlug),
    [tree, collectionSlug],
  );
  const ancestorPath = useMemo(
    () => helpCollectionAncestorPath(tree, collectionSlug),
    [tree, collectionSlug],
  );
  const childCollections = activeNode?.children ?? [];

  const title = activeNode?.collection.name ?? 'Articles';
  const hasChildren = childCollections.length > 0;
  const hasArticles = articles.length > 0;

  return (
    <div className="helpin-help-view">
      <div className="helpin-help-header">
        <button className="helpin-help-back" onClick={onBack} aria-label="Back">
          <ChevronLeftIcon size={18} />
        </button>
        <div className="helpin-help-header-copy">
          <span className="helpin-help-title">{title}</span>
          {ancestorPath.length > 1 && (
            <p className="helpin-help-subtitle">
              {ancestorPath
                .slice(0, -1)
                .map((node) => node.collection.name)
                .join(' › ')}
            </p>
          )}
        </div>
        <div className="helpin-help-header-spacer" />
      </div>
      <div className="helpin-help-content">
        {isLoading && <p className="helpin-help-empty">Loading articles...</p>}
        {!isLoading && error && <p className="helpin-help-empty">{error}</p>}
        {!isLoading && !error && !hasChildren && !hasArticles && (
          <p className="helpin-help-empty">No published articles are available yet.</p>
        )}
        {!isLoading && !error && (hasChildren || hasArticles) && (
          <div className="helpin-help-list">
            {childCollections.map((child) => (
              <button
                key={child.collection.slug}
                className="helpin-help-link"
                onClick={() => onSelectCollection?.(buildHelpCollectionKey(child.collection.slug, child.collection.public_id))}
              >
                <FileTextIcon size={20} />
                <div className="helpin-help-link-text">
                  <span className="helpin-help-link-title">{child.collection.name}</span>
                  <span className="helpin-help-link-desc">
                    {child.collection.article_count} article
                    {child.collection.article_count === 1 ? '' : 's'}
                    {child.children.length > 0
                      ? ` · ${child.children.length} sub-collection${child.children.length === 1 ? '' : 's'}`
                      : ''}
                  </span>
                </div>
                <ChevronRightIcon size={16} class="helpin-help-link-arrow" />
              </button>
            ))}
            {articles.map((article) => (
              <button
                key={article.slug}
                className="helpin-help-link"
                onClick={() => onSelectArticle(article.article_key || buildHelpArticleKey(article.slug, article.public_id))}
              >
                <FileTextIcon size={20} />
                <div className="helpin-help-link-text">
                  <span className="helpin-help-link-title">{article.title}</span>
                  {article.excerpt && (
                    <span className="helpin-help-link-desc">{article.excerpt}</span>
                  )}
                </div>
                <ChevronRightIcon size={16} class="helpin-help-link-arrow" />
              </button>
            ))}
          </div>
        )}
      </div>
    </div>
  );
};
