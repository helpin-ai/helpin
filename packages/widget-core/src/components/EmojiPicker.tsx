import { FunctionComponent } from 'preact';
import { useState, useMemo, useCallback, useEffect } from 'preact/hooks';
import type { EmojiCategory } from '@helpin-ai/shared';
import type { EmojiCatalog } from './emoji-catalog';
import { loadEmojiCatalog } from './emoji-loader';
import { SearchIcon, SmileIcon, XIcon } from './icons';

const EMOJI_CDN_BASE = 'https://cdn.jsdelivr.net/npm/emoji-datasource-apple@15.0.1/img/apple/64/';

interface EmojiPickerProps {
  onEmojiSelect: (emoji: string) => void;
}

const CATEGORY_ICONS: Record<string, string> = {
  smileys: '😀',
  animals: '🐾',
  food: '🍔',
  activities: '⚽',
  travel: '✈️',
  objects: '💡',
  symbols: '❤️',
  flags: '🏳️',
};

function getEmojiImageUrl(unicode: string): string {
  return EMOJI_CDN_BASE + unicode + '.png';
}

function emojiToCodepoint(emoji: string): string {
  return Array.from(emoji)
    .map((char) => char.codePointAt(0)?.toString(16) ?? '')
    .filter(Boolean)
    .join('-');
}

const EmojiOption: FunctionComponent<{
  emoji: string;
  onSelect: (emoji: string) => void;
}> = ({ emoji, onSelect }) => {
  const [imageFailed, setImageFailed] = useState(false);
  const codepoint = emojiToCodepoint(emoji);
  const imageURL = codepoint ? getEmojiImageUrl(codepoint) : '';

  return (
    <button
      type="button"
      className="helpin-emoji-btn"
      onClick={() => onSelect(emoji)}
      title={emoji}
      aria-label={`Insert ${emoji}`}
    >
      {!imageFailed && imageURL ? (
        <img
          src={imageURL}
          alt=""
          width={24}
          height={24}
          loading="lazy"
          aria-hidden="true"
          onError={() => setImageFailed(true)}
        />
      ) : null}
      <span
        aria-hidden="true"
        style={{ display: imageFailed || !imageURL ? 'inline' : 'none' }}
      >
        {emoji}
      </span>
    </button>
  );
};

export const EmojiPicker: FunctionComponent<EmojiPickerProps> = ({ onEmojiSelect }) => {
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState('');
  const [activeCategory, setActiveCategory] = useState('smileys');
  const [emojiCatalog, setEmojiCatalog] = useState<EmojiCatalog | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);

  useEffect(() => {
    if (!open || emojiCatalog || loadError) {
      return;
    }

    let cancelled = false;
    setIsLoading(true);
    setLoadError(null);

    loadEmojiCatalog()
      .then((catalog) => {
        if (cancelled) {
          return;
        }
        setEmojiCatalog(catalog);
      })
      .catch(() => {
        if (cancelled) {
          return;
        }
        setLoadError('Failed to load emojis');
      })
      .finally(() => {
        if (!cancelled) {
          setIsLoading(false);
        }
      });

    return () => {
      cancelled = true;
    };
  }, [emojiCatalog, loadError, open]);

  const displayedEmojis = useMemo(() => {
    if (!emojiCatalog) {
      return [];
    }
    if (search.trim()) {
      return emojiCatalog.search(search).slice(0, 60);
    }
    const category = emojiCatalog.categories.find((c: EmojiCategory) => c.id === activeCategory);
    return (category?.emojis || []).slice(0, 60);
  }, [activeCategory, emojiCatalog, search]);

  const handleSelect = useCallback(
    (emoji: string) => {
      onEmojiSelect(emoji);
      setOpen(false);
      setSearch('');
    },
    [onEmojiSelect],
  );

  const handleRetry = useCallback(() => {
    setLoadError(null);
    setIsLoading(false);
  }, []);

  return (
    <div className="helpin-emoji-picker">
      <button
        type="button"
        className="helpin-compose-tool-btn helpin-emoji-trigger"
        aria-label="Open emoji picker"
        onClick={() => setOpen(!open)}
      >
        <SmileIcon size={18} strokeWidth={1.75} />
      </button>

      {open && (
        <div className="helpin-emoji-dropdown">
          {isLoading ? (
            <div className="helpin-emoji-loading">Loading emojis...</div>
          ) : loadError ? (
            <div className="helpin-emoji-loading">
              <p>{loadError}</p>
              <button
                type="button"
                className="helpin-emoji-retry"
                onClick={handleRetry}
              >
                Retry
              </button>
            </div>
          ) : (
            <>
              <div className="helpin-emoji-search">
                <SearchIcon size={14} />
                <input
                  type="text"
                  value={search}
                  onInput={(e) => setSearch((e.target as HTMLInputElement).value)}
                  placeholder="Search emojis..."
                  className="helpin-emoji-search-input"
                  autoFocus
                />
                {search && (
                  <button
                    type="button"
                    className="helpin-emoji-search-clear"
                    onClick={() => setSearch('')}
                    aria-label="Clear search"
                  >
                    <XIcon size={12} />
                  </button>
                )}
              </div>

              {search ? (
                <div className="helpin-emoji-grid">
                  {displayedEmojis.length > 0 ? (
                    displayedEmojis.map((emoji: string, i: number) => (
                      <EmojiOption
                        key={`${emoji}-${i}`}
                        emoji={emoji}
                        onSelect={handleSelect}
                      />
                    ))
                  ) : (
                    <p className="helpin-emoji-empty">No emojis found</p>
                  )}
                </div>
              ) : (
                <>
                  <div className="helpin-emoji-categories">
                    {emojiCatalog?.categories.map((cat: EmojiCategory) => (
                      <button
                        key={cat.id}
                        type="button"
                        className={`helpin-emoji-category-btn ${activeCategory === cat.id ? 'helpin-emoji-category-btn--active' : ''}`}
                        onClick={() => setActiveCategory(cat.id)}
                        title={cat.label}
                        aria-label={cat.label}
                      >
                        {CATEGORY_ICONS[cat.id]}
                      </button>
                    ))}
                  </div>
                  <div className="helpin-emoji-grid">
                    {displayedEmojis.map((emoji: string, i: number) => (
                      <EmojiOption
                        key={`${emoji}-${i}`}
                        emoji={emoji}
                        onSelect={handleSelect}
                      />
                    ))}
                  </div>
                </>
              )}
            </>
          )}
        </div>
      )}
    </div>
  );
};
