import { UserGroupIcon } from '@/lib/icons';
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
  compact: _compact = false,
}: MentionSuggestionsListProps) {
  return (
    <div className={cn('space-y-px', className)} role="listbox" aria-label="Mention suggestions">
      {items.map((item, index) => {
        const selected = index === selectedIndex;

        return (
          <button
            key={`${item.type}:${item.id}:${item.handle}`}
            type="button"
            role="option"
            aria-selected={selected}
            className={cn(
              'flex w-full items-center gap-2.5 rounded-lg px-2.5 py-1.5 text-left transition-colors',
              selected
                ? 'bg-primary/10 text-foreground dark:bg-primary/15'
                : 'text-foreground/80 hover:bg-accent/60',
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
                className="h-6 w-6 shrink-0 text-[10px]"
              />
            ) : (
              <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-emerald-500/15 text-emerald-600 dark:text-emerald-400">
                <UserGroupIcon className="h-3 w-3" />
              </span>
            )}
            <span className="min-w-0 flex-1">
              <span className="block truncate text-[13px] font-medium leading-tight text-foreground">
                {item.label}
              </span>
              {item.type === 'team' && (
                <span className="block truncate text-xs leading-tight text-muted-foreground">
                  @{item.handle}
                </span>
              )}
            </span>
            {item.type === 'member' && item.handle && (
              <span className="shrink-0 text-xs text-muted-foreground/60">
                @{item.handle}
              </span>
            )}
          </button>
        );
      })}
    </div>
  );
}
