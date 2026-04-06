'use client';

import { useState, useMemo, useCallback, useRef, useEffect } from 'react';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Search01Icon, SmileIcon, Cancel01Icon } from '@/lib/icons';
import { cn } from '@/lib/utils';
import { loadEmojiCatalog, type EmojiCatalog } from '@helpin/widget-core';

interface EmojiPickerProps {
  onEmojiSelect: (emoji: string) => void;
  align?: 'start' | 'center' | 'end';
  side?: 'top' | 'right' | 'bottom' | 'left';
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

export function EmojiPicker({ onEmojiSelect, align = 'start', side = 'top' }: EmojiPickerProps) {
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState('');
  const [activeCategory, setActiveCategory] = useState<string>('smileys');
  const [emojiCatalog, setEmojiCatalog] = useState<EmojiCatalog | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);
  const searchInputRef = useRef<HTMLInputElement>(null);

  const displayedEmojis = useMemo<string[]>(() => {
    if (!emojiCatalog) {
      return [];
    }
    if (search.trim()) {
      return emojiCatalog.search(search);
    }
    const category = emojiCatalog.categories.find((c) => c.id === activeCategory);
    return category?.emojis || [];
  }, [activeCategory, emojiCatalog, search]);

  const handleSelect = useCallback(
    (emoji: string) => {
      onEmojiSelect(emoji);
      setOpen(false);
      setSearch('');
    },
    [onEmojiSelect],
  );

  useEffect(() => {
    if (open && emojiCatalog && searchInputRef.current) {
      searchInputRef.current.focus();
    }
  }, [emojiCatalog, open]);

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

  const handleRetry = useCallback(() => {
    setLoadError(null);
    setIsLoading(false);
  }, []);

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button
          variant="ghost"
          size="sm"
          className="h-7 w-7 p-0 text-muted-foreground hover:text-foreground"
          aria-label="Open emoji picker"
        >
          <SmileIcon className="h-4 w-4" />
        </Button>
      </PopoverTrigger>
      <PopoverContent
        className="w-[320px] p-0"
        align={align}
        side={side}
        sideOffset={8}
        onOpenAutoFocus={(e) => e.preventDefault()}
      >
        {isLoading ? (
          <div className="p-4 text-center text-sm text-muted-foreground">
            Loading emojis...
          </div>
        ) : loadError ? (
          <div className="space-y-3 p-4 text-center">
            <p className="text-sm text-muted-foreground">{loadError}</p>
            <Button type="button" variant="outline" size="sm" onClick={handleRetry}>
              Retry
            </Button>
          </div>
        ) : emojiCatalog ? (
          <>
            <div className="flex items-center gap-2 border-b p-2">
              <Search01Icon className="h-4 w-4 shrink-0 text-muted-foreground" />
              <Input
                ref={searchInputRef}
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                placeholder="Search emojis..."
                className="h-8 border-0 bg-transparent p-0 text-sm focus-visible:ring-0 focus-visible:ring-offset-0"
              />
              {search && (
                <button
                  type="button"
                  onClick={() => setSearch('')}
                  className="shrink-0 rounded p-0.5 hover:bg-muted"
                  aria-label="Clear search"
                >
                  <Cancel01Icon className="h-3.5 w-3.5 text-muted-foreground" />
                </button>
              )}
            </div>

            {search ? (
              <ScrollArea className="h-[240px]">
                <div className="flex flex-wrap gap-1 p-2">
                  {displayedEmojis.length > 0 ? (
                    displayedEmojis.map((emoji: string, i: number) => (
                      <button
                        type="button"
                        key={`${emoji}-${i}`}
                        onClick={() => handleSelect(emoji)}
                        className="flex h-8 w-8 items-center justify-center rounded-md text-xl transition-colors hover:bg-muted"
                        aria-label={`Insert ${emoji}`}
                        title={emoji}
                      >
                        {emoji}
                      </button>
                    ))
                  ) : (
                    <p className="w-full py-8 text-center text-sm text-muted-foreground">
                      No emojis found
                    </p>
                  )}
                </div>
              </ScrollArea>
            ) : (
              <>
                <div className="flex gap-1 border-b px-2 py-1">
                  {emojiCatalog.categories.map((category) => (
                    <button
                      type="button"
                      key={category.id}
                      onClick={() => setActiveCategory(category.id)}
                      className={cn(
                        'flex h-8 w-8 items-center justify-center rounded-md text-lg transition-colors',
                        activeCategory === category.id
                          ? 'bg-primary/10 text-primary'
                          : 'text-muted-foreground hover:bg-muted hover:text-foreground',
                      )}
                      title={category.label}
                      aria-label={category.label}
                    >
                      {CATEGORY_ICONS[category.id]}
                    </button>
                  ))}
                </div>

                <ScrollArea className="h-[200px]">
                  <div className="flex flex-wrap gap-1 p-2">
                    {displayedEmojis.map((emoji: string, i: number) => (
                      <button
                        type="button"
                        key={`${emoji}-${i}`}
                        onClick={() => handleSelect(emoji)}
                        className="flex h-8 w-8 items-center justify-center rounded-md text-xl transition-colors hover:bg-muted"
                        aria-label={`Insert ${emoji}`}
                        title={emoji}
                      >
                        {emoji}
                      </button>
                    ))}
                  </div>
                </ScrollArea>
              </>
            )}
          </>
        ) : (
          <div className="p-4 text-center text-sm text-muted-foreground">
            Loading emojis...
          </div>
        )}
      </PopoverContent>
    </Popover>
  );
}
