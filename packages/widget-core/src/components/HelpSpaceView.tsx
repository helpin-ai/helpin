import { FunctionComponent } from 'preact';
import { useEffect, useMemo, useState } from 'preact/hooks';
import type { HelpSpace } from '@helpin-ai/shared';
import { ChevronLeftIcon, ChevronRightIcon, FileTextIcon, XIcon } from './icons';
import { buildHelpCollectionKey, fetchHelpCollections, type HelpCollection } from './helpApi';
import { buildHelpCollectionTree } from './helpTree';

interface HelpSpaceViewProps {
  host: string;
  widgetKey: string;
  space: HelpSpace;
  showBack: boolean;
  showHeader?: boolean;
  animateDrilldown?: boolean;
  onBack: () => void;
  onClose?: () => void;
  onSelectCollection: (collectionSlug: string) => void;
}

export const HelpSpaceView: FunctionComponent<HelpSpaceViewProps> = ({
  host,
  widgetKey,
  space,
  showBack,
  showHeader = true,
  animateDrilldown = true,
  onBack,
  onClose,
  onSelectCollection,
}) => {
  const [collections, setCollections] = useState<HelpCollection[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [showLoadingSkeleton, setShowLoadingSkeleton] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Fold the flat collection list into a tree so we only show top-level
  // nodes at the space root. Users drill into child collections from
  // HelpCollectionView.
  const topLevelNodes = useMemo(() => buildHelpCollectionTree(collections), [collections]);

  useEffect(() => {
    let cancelled = false;
    const loadingSkeletonTimer = window.setTimeout(() => {
      if (!cancelled) {
        setShowLoadingSkeleton(true);
      }
    }, 150);

    setIsLoading(true);
    setShowLoadingSkeleton(false);
    setError(null);

    fetchHelpCollections(host, widgetKey, space.slug)
      .then((data) => {
        if (!cancelled) {
          setCollections(data);
        }
      })
      .catch(() => {
        if (!cancelled) {
          setError('Unable to load collections right now.');
        }
      })
      .finally(() => {
        window.clearTimeout(loadingSkeletonTimer);
        if (!cancelled) {
          setIsLoading(false);
          setShowLoadingSkeleton(false);
        }
      });

    return () => {
      cancelled = true;
      window.clearTimeout(loadingSkeletonTimer);
    };
  }, [host, widgetKey, space.slug]);

  return (
    <div className={`helpin-help-view ${animateDrilldown ? 'helpin-help-drilldown-view' : ''}`}>
      {showHeader && (
        <div className="helpin-help-header">
          {showBack && (
            <button className="helpin-help-back" onClick={onBack} aria-label="Back">
              <ChevronLeftIcon size={18} />
            </button>
          )}
          <div className="helpin-help-header-copy">
            <span className="helpin-help-title">{space.name}</span>
            <p className="helpin-help-subtitle">Browse collections</p>
          </div>
          {onClose ? (
            <button className="helpin-window-close-inline" onClick={onClose} aria-label="Close">
              <XIcon size={18} />
            </button>
          ) : (
            <div className="helpin-help-header-spacer" />
          )}
        </div>
      )}
      <div className="helpin-help-content">
        {isLoading && !showLoadingSkeleton && (
          <div className="helpin-help-loading-reserve" aria-hidden="true" />
        )}
        {isLoading && showLoadingSkeleton && (
          <div className="helpin-help-loading-list" role="status" aria-label="Loading collections">
            {[0, 1, 2].map((item) => (
              <div className="helpin-help-link-skeleton" key={item}>
                <span className="helpin-help-link-skeleton-icon" />
                <span className="helpin-help-link-skeleton-copy">
                  <span className="helpin-help-link-skeleton-line helpin-help-link-skeleton-line-title" />
                  <span className="helpin-help-link-skeleton-line helpin-help-link-skeleton-line-desc" />
                </span>
              </div>
            ))}
          </div>
        )}
        {!isLoading && error && <p className="helpin-help-empty">{error}</p>}
        {!isLoading && !error && topLevelNodes.length === 0 && (
          <p className="helpin-help-empty">No published collections are available yet.</p>
        )}
        {!isLoading && !error && topLevelNodes.length > 0 && (
          <div className="helpin-help-list">
            {topLevelNodes.map((node) => {
              const totalArticles = sumDescendantArticles(node);
              const childCount = node.children.length;
              const descParts: string[] = [];
              if (totalArticles > 0) {
                descParts.push(`${totalArticles} article${totalArticles === 1 ? '' : 's'}`);
              }
              if (childCount > 0) {
                descParts.push(`${childCount} sub-collection${childCount === 1 ? '' : 's'}`);
              }
              return (
                <button
                  key={node.collection.slug}
                  className="helpin-help-link"
                  onClick={() => onSelectCollection(buildHelpCollectionKey(node.collection.slug, node.collection.public_id))}
                >
                  <FileTextIcon size={20} />
                  <div className="helpin-help-link-text">
                    <span className="helpin-help-link-title">{node.collection.name}</span>
                    <span className="helpin-help-link-desc">
                      {descParts.join(' · ')}
                    </span>
                  </div>
                  <ChevronRightIcon size={16} class="helpin-help-link-arrow" />
                </button>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
};

/** Sums the article counts of a node plus every descendant. */
function sumDescendantArticles(node: { collection: HelpCollection; children: Array<{ collection: HelpCollection; children: unknown[] }> }): number {
  let total = node.collection.article_count;
  for (const child of node.children) {
    total += sumDescendantArticles(child as { collection: HelpCollection; children: Array<{ collection: HelpCollection; children: unknown[] }> });
  }
  return total;
}
