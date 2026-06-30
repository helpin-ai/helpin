import { FunctionComponent } from 'preact';
import { useEffect, useState } from 'preact/hooks';
import { ChevronLeftIcon, ExternalLinkIcon, XIcon } from './icons';
import { fetchHelpArticle, type HelpArticle } from './helpApi';

interface HelpArticleViewProps {
  host: string;
  widgetKey: string;
  articleKey: string;
  onBack: () => void;
  onClose?: () => void;
}

export const HelpArticleView: FunctionComponent<HelpArticleViewProps> = ({
  host,
  widgetKey,
  articleKey,
  onBack,
  onClose,
}) => {
  const [article, setArticle] = useState<HelpArticle | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const articleExternalURL = article?.public_path
    ? (article.public_path.startsWith('http://') || article.public_path.startsWith('https://')
        ? article.public_path
        : `${host.startsWith('http://') || host.startsWith('https://') ? host.replace(/\/$/, '') : `https://${host}`}${article.public_path}`)
    : null;

  useEffect(() => {
    let cancelled = false;
    setIsLoading(true);
    setError(null);

    fetchHelpArticle(host, widgetKey, articleKey)
      .then((data) => {
        if (!cancelled) {
          setArticle(data);
        }
      })
      .catch(() => {
        if (!cancelled) {
          setError('Unable to load this article right now.');
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
  }, [articleKey, host, widgetKey]);

  return (
    <div className="helpin-article-view">
      <div className="helpin-article-header">
        <button className="helpin-help-back" onClick={onBack} aria-label="Back">
          <ChevronLeftIcon size={18} />
        </button>
        <div className="helpin-article-header-copy">
          <span className="helpin-help-title">{article?.title || 'Article'}</span>
        </div>
        <div className="helpin-article-header-actions">
          {articleExternalURL && (
            <a
              href={articleExternalURL}
              className="helpin-window-close-inline"
              target="_blank"
              rel="noreferrer"
              aria-label="Open article in Help Center"
              title="Open article in Help Center"
            >
              <ExternalLinkIcon size={16} />
            </a>
          )}
          {onClose ? (
            <button className="helpin-window-close-inline" onClick={onClose} aria-label="Close">
              <XIcon size={18} />
            </button>
          ) : (
            <div className="helpin-help-header-spacer" />
          )}
        </div>
      </div>
      <div className="helpin-article-content">
        {isLoading && <p className="helpin-help-empty">Loading article...</p>}
        {!isLoading && error && <p className="helpin-help-empty">{error}</p>}
        {!isLoading && !error && article?.content_html && (
          <div dangerouslySetInnerHTML={{ __html: article.content_html }} />
        )}
        {!isLoading && !error && !article?.content_html && (
          <p className="helpin-help-empty">This article does not have published content yet.</p>
        )}
      </div>
    </div>
  );
};
