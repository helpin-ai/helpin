import {
  BotIcon,
  Building03Icon,
  CheckListIcon,
  DollarCircleIcon,
  File01Icon,
  FolderKanbanIcon,
  Message01Icon,
  UserIcon,
} from '@/lib/icons';
import { UserAvatar } from '@/components/pm/UserAvatar';
import type { MentionSuggestionItem } from '@/components/pm/mentionSuggestions';
import { cn } from '@/lib/utils';
import type { DocsEntitySearchType } from '@/components/docs/entitySearch';

interface MentionSuggestionsListProps {
  items: MentionSuggestionItem[];
  selectedIndex: number;
  onSelect: (item: MentionSuggestionItem) => void;
  className?: string;
  /** Compact mode: single-line with name + @handle, no email. */
  compact?: boolean;
}

function entityMentionTypeLabel(type: DocsEntitySearchType) {
  if (type === 'epic') return 'Epic'
  if (type === 'support_conversation') return 'Conversation'
  if (type === 'deal') return 'Deal'
  if (type === 'contact') return 'Contact'
  if (type === 'company') return 'Company'
  if (type === 'story') return 'Story'
  if (type === 'document') return 'Doc'
  return 'Task'
}

function entityMentionTypePluralLabel(type: DocsEntitySearchType) {
  if (type === 'company') return 'companies'
  if (type === 'story') return 'stories'
  return `${entityMentionTypeLabel(type).toLowerCase()}s`
}

function mentionSectionLabel(item: MentionSuggestionItem) {
  if (item.type === 'member') return 'People'
  if (item.type === 'team') return 'Teams'
  if (item.type === 'agent') return 'Agents'
  return item.entityType ? entityMentionTypePluralLabel(item.entityType) : 'Entities'
}

function EntitySuggestionIcon({ entityType }: { entityType?: DocsEntitySearchType }) {
  if (entityType === 'epic') return <FolderKanbanIcon className="h-3 w-3" />
  if (entityType === 'support_conversation') return <Message01Icon className="h-3 w-3" />
  if (entityType === 'deal') return <DollarCircleIcon className="h-3 w-3" />
  if (entityType === 'contact') return <UserIcon className="h-3 w-3" />
  if (entityType === 'company') return <Building03Icon className="h-3 w-3" />
  if (entityType === 'document') return <File01Icon className="h-3 w-3" />
  return <CheckListIcon className="h-3 w-3" />
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
        const sectionLabel = mentionSectionLabel(item);
        const previous = items[index - 1];
        const showSection = !previous || mentionSectionLabel(previous) !== sectionLabel;
        const label = item.label || (item as MentionSuggestionItem & { name?: string }).name || item.handle;

        return (
          <div key={`${item.type}:${item.entityType ?? 'mention'}:${item.id}:${item.handle}`}>
            {showSection && (
              <div className="px-2.5 pb-1 pt-2 text-[10px] font-semibold uppercase tracking-wide text-muted-foreground first:pt-0">
                {sectionLabel}
              </div>
            )}
            <button
              data-mention-suggestion-type={item.type}
              data-mention-entity-type={item.entityType}
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
                <span data-mention-member-avatar>
                  <UserAvatar
                    name={label}
                    avatarUrl={item.avatarUrl}
                    className="h-6 w-6 shrink-0 text-[10px]"
                  />
                </span>
              ) : item.type === 'team' ? (
                <span data-mention-team-badge className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-emerald-500/15 text-emerald-600 dark:text-emerald-400">
                  <span className="text-[10px] font-semibold">{label.slice(0, 1).toUpperCase()}</span>
                </span>
              ) : item.type === 'entity' ? (
                <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-sky-500/15 text-sky-600 dark:text-sky-400">
                  <EntitySuggestionIcon entityType={item.entityType} />
                </span>
              ) : (
                <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-violet-500/15 text-violet-600 dark:text-violet-400">
                  <BotIcon className="h-3 w-3" />
                </span>
              )}
              <span className="min-w-0 flex-1">
                <span className="block truncate text-[13px] font-medium leading-tight text-foreground">
                  {label}
                </span>
                {(item.type !== 'member' || item.secondaryText) && (
                  <span className="block truncate text-xs leading-tight text-muted-foreground">
                    {item.type === 'agent' ? item.secondaryText : item.secondaryText || `@${item.handle}`}
                  </span>
                )}
              </span>
              {item.type === 'member' && item.handle && (
                <span className="shrink-0 text-xs text-muted-foreground/60">
                  @{item.handle}
                </span>
              )}
            </button>
          </div>
        );
      })}
    </div>
  );
}
