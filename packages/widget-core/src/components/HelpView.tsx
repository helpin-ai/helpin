import { FunctionComponent } from 'preact';
import { useEffect, useState } from 'preact/hooks';
import type { WidgetConfig } from '../types';
import { FileTextIcon, ChevronRightIcon, XIcon, SearchIcon } from './icons';
import { HelpSpaceView } from './HelpSpaceView';
import { fetchHelpSearchResults, type HelpSearchResult } from './helpApi';

interface HelpViewProps {
  config: WidgetConfig;
  host?: string;
  widgetKey?: string;
  onSelectSpace: (spaceSlug: string) => void;
  onSelectCollection: (collectionSlug: string) => void;
  onSelectArticle?: (articleKey: string) => void;
  onClose?: () => void;
}

export const HelpView: FunctionComponent<HelpViewProps> = ({
  config,
  host,
  widgetKey,
  onSelectSpace,
  onSelectCollection,
  onSelectArticle,
  onClose,
}) => {
  const helpSpaces = config.helpSpaces ?? [];
  const canBrowseDocs = !!host && !!widgetKey && helpSpaces.length > 0;
  const [searchQuery, setSearchQuery] = useState('');
  const [debouncedSearchQuery, setDebouncedSearchQuery] = useState('');
  const [searchResults, setSearchResults] = useState<HelpSearchResult[]>([]);
  const [isSearching, setIsSearching] = useState(false);
  const [searchError, setSearchError] = useState<string | null>(null);
  const normalizedSearchQuery = searchQuery.trim();
  const normalizedDebouncedSearchQuery = debouncedSearchQuery.trim();
  const showSearchResults = canBrowseDocs && normalizedSearchQuery.length >= 2;

  useEffect(() => {
    const timeout = window.setTimeout(() => {
      setDebouncedSearchQuery(normalizedSearchQuery);
    }, 250);

    return () => window.clearTimeout(timeout);
  }, [normalizedSearchQuery]);

  useEffect(() => {
    if (!showSearchResults || normalizedDebouncedSearchQuery.length < 2 || !host || !widgetKey) {
      setSearchResults([]);
      setIsSearching(false);
      setSearchError(null);
      return;
    }

    let cancelled = false;
    setIsSearching(true);
    setSearchError(null);

    fetchHelpSearchResults(host, widgetKey, normalizedDebouncedSearchQuery, 8)
      .then((results) => {
        if (!cancelled) {
          setSearchResults(results);
        }
      })
      .catch(() => {
        if (!cancelled) {
          setSearchError('Unable to search articles right now.');
          setSearchResults([]);
        }
      })
      .finally(() => {
        if (!cancelled) {
          setIsSearching(false);
        }
      });

    return () => {
      cancelled = true;
    };
  }, [host, normalizedDebouncedSearchQuery, showSearchResults, widgetKey]);

  return (
    <div className="helpin-help-view helpin-view-enter">
      <div className="helpin-help-header">
        <div className="helpin-help-header-spacer" />
        <span className="helpin-help-title">Help Center</span>
        {onClose ? (
          <button className="helpin-window-close-inline" onClick={onClose} aria-label="Close">
            <XIcon size={18} />
          </button>
        ) : (
          <div className="helpin-help-header-spacer" />
        )}
      </div>
      <div className="helpin-help-content">
        {canBrowseDocs && (
          <div className="helpin-help-search">
            <SearchIcon size={16} class="helpin-help-search-icon" />
            <input
              value={searchQuery}
              onInput={(event) => setSearchQuery((event.currentTarget as HTMLInputElement).value)}
              placeholder="Search help articles..."
              aria-label="Search help articles"
            />
            {isSearching ? (
              <span
                className="helpin-help-search-action helpin-help-search-spinner"
                role="status"
                aria-label="Searching articles"
              />
            ) : searchQuery.length > 0 ? (
              <button
                type="button"
                className="helpin-help-search-action helpin-help-search-clear"
                onClick={() => setSearchQuery('')}
                aria-label="Clear search"
              >
                <XIcon size={14} />
              </button>
            ) : (
              <span className="helpin-help-search-action" aria-hidden="true" />
            )}
          </div>
        )}

        {!canBrowseDocs && (
          <p className="helpin-help-empty">Articles are not available in this widget yet.</p>
        )}

        {showSearchResults && (
          <div className="helpin-help-list helpin-stagger-list">
            {isSearching && <p className="helpin-help-empty">Searching articles...</p>}
            {!isSearching && searchError && <p className="helpin-help-empty">{searchError}</p>}
            {!isSearching && !searchError && searchResults.length === 0 && (
              <p className="helpin-help-empty">No articles found.</p>
            )}
            {!isSearching && !searchError && searchResults.map((result) => (
              <button
                key={result.article_key}
                className="helpin-help-link"
                onClick={() => onSelectArticle?.(result.article_key)}
              >
                <FileTextIcon size={20} />
                <div className="helpin-help-link-text">
                  <span className="helpin-help-link-title">{result.title}</span>
                  <span className="helpin-help-link-desc">
                    {result.collection_name || result.excerpt || 'Help article'}
                  </span>
                </div>
                <ChevronRightIcon size={16} class="helpin-help-link-arrow" />
              </button>
            ))}
          </div>
        )}

        {!showSearchResults && canBrowseDocs && helpSpaces.length === 1 && host && widgetKey && (
          <div className="helpin-help-inline-section">
            <HelpSpaceView
              host={host}
              widgetKey={widgetKey}
              space={helpSpaces[0]}
              showBack={false}
              showHeader={false}
              animateDrilldown={false}
              onBack={() => {}}
              onSelectCollection={onSelectCollection}
            />
          </div>
        )}

        {!showSearchResults && canBrowseDocs && helpSpaces.length > 1 && (
          <div className="helpin-help-inline-section">
            <div className="helpin-help-list helpin-stagger-list">
              {helpSpaces.map((space) => (
                <button
                  key={space.id}
                  className="helpin-help-link"
                  onClick={() => onSelectSpace(space.slug)}
                >
                  <FileTextIcon size={20} />
                  <div className="helpin-help-link-text">
                    <span className="helpin-help-link-title">{space.name}</span>
                    <span className="helpin-help-link-desc">Browse collections and articles</span>
                  </div>
                  <ChevronRightIcon size={16} class="helpin-help-link-arrow" />
                </button>
              ))}
            </div>
          </div>
        )}
      </div>
    </div>
  );
};
