import { FunctionComponent } from 'preact';
import { useEffect, useState } from 'preact/hooks';
import { ChevronLeftIcon, ChevronRightIcon, FileTextIcon } from './icons';
import { fetchHelpArticles, type HelpArticleSummary } from './helpApi';

interface HelpCollectionViewProps {
  host: string;
  widgetKey: string;
  collectionSlug: string;
  onBack: () => void;
  onSelectArticle: (articleSlug: string) => void;
}

export const HelpCollectionView: FunctionComponent<HelpCollectionViewProps> = ({
  host,
  widgetKey,
  collectionSlug,
  onBack,
  onSelectArticle,
}) => {
  const [articles, setArticles] = useState<HelpArticleSummary[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setIsLoading(true);
    setError(null);

    fetchHelpArticles(host, widgetKey, collectionSlug)
      .then((data) => {
        if (!cancelled) {
          setArticles(data);
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
  }, [collectionSlug, host, widgetKey]);

  return (
    <div className="helpin-help-view">
      <div className="helpin-help-header">
        <button className="helpin-help-back" onClick={onBack} aria-label="Back">
          <ChevronLeftIcon size={18} />
        </button>
        <div className="helpin-help-header-copy">
          <span className="helpin-help-title">Articles</span>
          <p className="helpin-help-subtitle">Select an article to read</p>
        </div>
        <div className="helpin-help-header-spacer" />
      </div>
      <div className="helpin-help-content">
        {isLoading && <p className="helpin-help-empty">Loading articles...</p>}
        {!isLoading && error && <p className="helpin-help-empty">{error}</p>}
        {!isLoading && !error && articles.length === 0 && (
          <p className="helpin-help-empty">No published articles are available yet.</p>
        )}
        {!isLoading && !error && articles.length > 0 && (
          <div className="helpin-help-list">
            {articles.map((article) => (
              <button
                key={article.slug}
                className="helpin-help-link"
                onClick={() => onSelectArticle(article.slug)}
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
