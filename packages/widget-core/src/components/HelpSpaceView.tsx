import { FunctionComponent } from 'preact';
import { useEffect, useState } from 'preact/hooks';
import type { HelpSpace } from '@helpin/shared';
import { ChevronLeftIcon, ChevronRightIcon, FileTextIcon } from './icons';
import { fetchHelpCollections, type HelpCollection } from './helpApi';

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
        {!isLoading && !error && collections.length === 0 && (
          <p className="helpin-help-empty">No published collections are available yet.</p>
        )}
        {!isLoading && !error && collections.length > 0 && (
          <div className="helpin-help-list">
            {collections.map((collection) => (
              <button
                key={collection.slug}
                className="helpin-help-link"
                onClick={() => onSelectCollection(collection.slug)}
              >
                <FileTextIcon size={20} />
                <div className="helpin-help-link-text">
                  <span className="helpin-help-link-title">{collection.name}</span>
                  <span className="helpin-help-link-desc">
                    {collection.article_count} article{collection.article_count === 1 ? '' : 's'}
                  </span>
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
