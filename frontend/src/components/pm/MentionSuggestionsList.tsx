import { Users } from 'lucide-react';
import { UserAvatar } from '@/components/pm/UserAvatar';
import type { MentionSuggestionItem } from '@/components/pm/mentionSuggestions';
import { cn } from '@/lib/utils';

interface MentionSuggestionsListProps {
  items: MentionSuggestionItem[];
  selectedIndex: number;
  onSelect: (item: MentionSuggestionItem) => void;
  className?: string;
  /** Compact mode: single-line with name + @handle, no email. */
  compact?: boolean;
}

export function MentionSuggestionsList({
  items,
  selectedIndex,
  onSelect,
  className,
  compact = false,
}: MentionSuggestionsListProps) {
  return (
    <div className={cn('space-y-0.5', className)} role="listbox" aria-label="Mention suggestions">
      {items.map((item, index) => {
        const selected = index === selectedIndex;

        return (
          <button
            key={`${item.type}:${item.id}:${item.handle}`}
            type="button"
            className={cn(
              'flex w-full items-center gap-2.5 rounded-md px-2 py-2 text-left transition-colors',
              selected ? 'bg-accent text-foreground' : 'hover:bg-accent/70',
            )}
            onMouseDown={(event) => {
              event.preventDefault();
              onSelect(item);
            }}
          >
            {item.type === 'member' ? (
              <UserAvatar
                name={item.label}
                avatarUrl={item.avatarUrl}
                className="h-7 w-7 shrink-0 text-[10px]"
              />
            ) : (
              <span className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-emerald-500/10 text-emerald-700 dark:text-emerald-300">
                <Users className="h-3.5 w-3.5" />
              </span>
            )}
            <span className="min-w-0 flex-1">
              {compact ? (
                <span className="flex items-center gap-1.5 truncate">
                  <span className="text-sm font-medium text-foreground">{item.label}</span>
                  <span className="text-xs text-muted-foreground">@{item.handle}</span>
                </span>
              ) : (
                <>
                  <span className="block truncate text-sm font-medium text-foreground">
                    {item.label}
                  </span>
                  <span className="block truncate text-xs text-muted-foreground">
                    @{item.handle}
                    {item.type === 'member' && item.secondaryText ? ` · ${item.secondaryText}` : ''}
                  </span>
                </>
              )}
            </span>
          </button>
        );
      })}
    </div>
  );
}
