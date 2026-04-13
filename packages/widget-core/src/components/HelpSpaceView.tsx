import { FunctionComponent } from 'preact';
import { useEffect, useMemo, useState } from 'preact/hooks';
import type { HelpSpace } from '@helpin-ai/shared';
import { ChevronLeftIcon, ChevronRightIcon, FileTextIcon } from './icons';
import { buildHelpCollectionKey, fetchHelpCollections, type HelpCollection } from './helpApi';
import { buildHelpCollectionTree } from './helpTree';

interface HelpSpaceViewProps {
  host: string;
  widgetKey: string;
  space: HelpSpace;
  showBack: boolean;
  showHeader?: boolean;
  onBack: () => void;
  onSelectCollection: (collectionSlug: string) => void;
}

export const HelpSpaceView: FunctionComponent<HelpSpaceViewProps> = ({
  host,
  widgetKey,
  space,
  showBack,
  showHeader = true,
  onBack,
  onSelectCollection,
}) => {
  const [collections, setCollections] = useState<HelpCollection[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Fold the flat collection list into a tree so we only show top-level
  // nodes at the space root. Users drill into child collections from
  // HelpCollectionView.
  const topLevelNodes = useMemo(() => buildHelpCollectionTree(collections), [collections]);

  useEffect(() => {
    let cancelled = false;
    setIsLoading(true);
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
        if (!cancelled) {
          setIsLoading(false);
        }
      });

    return () => {
      cancelled = true;
    };
  }, [host, widgetKey, space.slug]);

  return (
    <div className="helpin-help-view">
      {showHeader && (
        <div className="helpin-help-header">
          {showBack && (
            <button className="helpin-help-back" onClick={onBack} aria-label="Back">
              <ChevronLeftIcon size={18} />
            </button>
          )}
          <div>
            <span className="helpin-help-title">{space.name}</span>
            <p className="helpin-help-subtitle">Browse collections</p>
          </div>
        </div>
      )}
      <div className="helpin-help-content">
        {isLoading && <p className="helpin-help-empty">Loading collections...</p>}
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
