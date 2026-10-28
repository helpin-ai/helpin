import { useCallback, type ReactNode } from 'react';
import { Button } from '@/components/ui/button';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from '@/components/ui/command';
import { PlusSignIcon } from '@/lib/icons';
import { cn } from '@/lib/utils';
import { SupportTagBadge } from './SupportTagPicker';
import type { SupportTag, SupportInboxScope } from '@/lib/pmTypes';
import type { NavFilter } from '@/stores/supportInboxStore';
import { CONVERSATION_PRIORITY_OPTIONS, type ConversationListFilters, type ConversationStateFilter, type ConversationAssignmentFilter, type ConversationAIStateFilter, type ConversationSortOrder } from '@/lib/supportInboxFilters';

function FilterPill({
  children,
  selected,
  onClick,
  disabled = false,
}: {
  children: ReactNode;
  selected: boolean;
  onClick: () => void;
  disabled?: boolean;
}) {
  return (
    <button
      type="button"
      className={cn(
        'inline-flex h-8 items-center gap-1.5 rounded-md border px-2.5 text-xs font-medium transition-colors disabled:opacity-50 disabled:cursor-not-allowed',
        selected
          ? 'border-border bg-primary/10 text-primary'
          : 'border-border bg-background text-foreground hover:bg-muted'
      )}
      disabled={disabled}
      aria-pressed={selected}
      title={disabled ? 'Enable SLA in Support settings to use this filter.' : undefined}
      onClick={onClick}
    >
      {children}
    </button>
  );
}

function FilterSection({ title, children }: { title: string; children: ReactNode }) {
  return (
    <div className="space-y-2">
      <div className="text-[11px] font-semibold uppercase tracking-normal text-muted-foreground">{title}</div>
      <div className="flex flex-wrap gap-2">{children}</div>
    </div>
  );
}

function TagFilterSelector({
  tags,
  selectedTagIds,
  onToggleTag,
}: {
  tags: SupportTag[];
  selectedTagIds: string[];
  onToggleTag: (tagId: string) => void;
}) {
  const selectedTags = tags.filter((tag) => selectedTagIds.includes(tag.id));

  return (
    <>
      {selectedTags.map((tag) => (
        <SupportTagBadge
          key={tag.id}
          name={tag.name}
          color={tag.color}
          onRemove={() => onToggleTag(tag.id)}
        />
      ))}
      <Popover>
        <PopoverTrigger asChild>
          <Button
            type="button"
            variant="outline"
            size="sm"
            className="h-5 gap-1 rounded-sm border-[0.5px] px-1.5 text-[11px] font-medium leading-none"
          >
            <PlusSignIcon className="h-3 w-3" />
            Add
          </Button>
        </PopoverTrigger>
        <PopoverContent align="start" className="w-64 p-0">
          <Command shouldFilter>
            <CommandInput placeholder="Search tags..." className="h-8 text-xs" />
            <CommandList className="max-h-56">
              <CommandEmpty className="py-3 text-center text-xs text-muted-foreground">
                No tags found
              </CommandEmpty>
              {tags.length > 0 && (
                <CommandGroup>
                  {tags.map((tag) => (
                    <CommandItem
                      key={tag.id}
                      value={tag.name}
                      data-checked={selectedTagIds.includes(tag.id)}
                      onSelect={() => onToggleTag(tag.id)}
                    >
                      <span
                        className="h-2 w-2 rounded-full"
                        style={{ backgroundColor: tag.color || 'var(--muted-foreground)' }}
                      />
                      <span className="truncate">{tag.name}</span>
                    </CommandItem>
                  ))}
                </CommandGroup>
              )}
            </CommandList>
          </Command>
        </PopoverContent>
      </Popover>
    </>
  );
}

interface SupportViewFiltersProps {
  conversationListFilters: ConversationListFilters;
  navFilter: NavFilter;
  selectedMailboxId: string;
  setConversationListFilter: <K extends keyof ConversationListFilters>(key: K, value: ConversationListFilters[K]) => void;
  setConversationMailboxFilters: (ids: string[]) => void;
  supportTags: SupportTag[];
  teamInboxFilterOptions: Pick<SupportInboxScope, 'id' | 'name'>[];
  slaEnabled: boolean;
}

export function SupportViewFilters({ conversationListFilters, navFilter, selectedMailboxId, setConversationListFilter, setConversationMailboxFilters, supportTags, teamInboxFilterOptions, slaEnabled }: SupportViewFiltersProps) {
  const toggleStateFilter = useCallback((state: ConversationStateFilter) => {
    const currentStates = conversationListFilters.states;
    const nextStates = currentStates.includes(state)
      ? currentStates.filter((value) => value !== state)
      : [...currentStates, state];
    if (nextStates.length === 0) return;
    setConversationListFilter('states', nextStates);
  }, [conversationListFilters.states, setConversationListFilter]);

  const toggleAssignmentFilter = useCallback((assignment: ConversationAssignmentFilter) => {
    const currentAssignments = conversationListFilters.assignment;
    const nextAssignments = currentAssignments.includes(assignment)
      ? currentAssignments.filter((value) => value !== assignment)
      : [...currentAssignments, assignment];
    if (navFilter === 'mine' && nextAssignments.length === 0) return;
    setConversationListFilter('assignment', nextAssignments);
  }, [conversationListFilters.assignment, navFilter, setConversationListFilter]);

  const toggleMailboxFilter = useCallback((mailboxId: string) => {
    if (mailboxId === 'all') {
      setConversationMailboxFilters(navFilter === 'inbox' ? ['all'] : []);
      return;
    }
    const currentMailboxIds = conversationListFilters.mailboxIds.length > 0
      ? conversationListFilters.mailboxIds
      : selectedMailboxId !== 'all'
        ? [selectedMailboxId]
        : navFilter === 'inbox'
          ? ['shared']
          : [];
    if (currentMailboxIds.length === 0) {
      setConversationMailboxFilters([mailboxId]);
      return;
    }
    const scopedMailboxIds = currentMailboxIds.filter((value) => value !== 'all');
    const nextMailboxIds = scopedMailboxIds.includes(mailboxId)
      ? scopedMailboxIds.filter((value) => value !== mailboxId)
      : [...scopedMailboxIds, mailboxId];
    if (nextMailboxIds.length === 0) return;
    setConversationMailboxFilters(nextMailboxIds);
  }, [conversationListFilters.mailboxIds, navFilter, selectedMailboxId, setConversationMailboxFilters]);

  const toggleTagFilter = useCallback((tagId: string) => {
    const currentTags = conversationListFilters.tagIds;
    const nextTags = currentTags.includes(tagId)
      ? currentTags.filter((value) => value !== tagId)
      : [...currentTags, tagId];
    setConversationListFilter('tagIds', nextTags);
  }, [conversationListFilters.tagIds, setConversationListFilter]);

  const toggleAIStateFilter = useCallback((aiState: ConversationAIStateFilter) => {
    const currentStates = conversationListFilters.aiStates;
    const nextStates = currentStates.includes(aiState)
      ? currentStates.filter((value) => value !== aiState)
      : [...currentStates, aiState];
    setConversationListFilter('aiStates', nextStates);
  }, [conversationListFilters.aiStates, setConversationListFilter]);

  return <div className="space-y-4">
    <FilterSection title="State">
      {[
        ['Open', 'open'],
        ['Waiting', 'waiting_on_customer'],
        ['Resolved', 'resolved'],
        ['Spam', 'spam'],
      ].map(([label, value]) => (
        <FilterPill
          key={value}
          selected={conversationListFilters.states.includes(value as ConversationStateFilter)}
          onClick={() => toggleStateFilter(value as ConversationStateFilter)}
        >
          {label}
        </FilterPill>
      ))}
    </FilterSection>
    <FilterSection title="Priority">
      {CONVERSATION_PRIORITY_OPTIONS.map(({ value, label }) => (
        <FilterPill
          key={value}
          selected={(conversationListFilters.priorities ?? []).includes(value)}
          onClick={() => {
            const selected = conversationListFilters.priorities ?? [];
            setConversationListFilter('priorities', selected.includes(value)
              ? selected.filter(priority => priority !== value)
              : [...selected, value]);
          }}
        >
          {label}
        </FilterPill>
      ))}
    </FilterSection>
    <FilterSection title="AI state">
      {([
        ['AI handling', 'handling'],
        ['AI handoff', 'handoff'],
        ['AI resolved', 'resolved'],
      ] as Array<[string, ConversationAIStateFilter]>).map(([label, value]) => (
        <FilterPill
          key={value}
          selected={conversationListFilters.aiStates.includes(value)}
          onClick={() => toggleAIStateFilter(value)}
        >
          {label}
        </FilterPill>
      ))}
    </FilterSection>
    <FilterSection title="Assignment">
      {[
        ['Assigned to me', 'me'],
        ['Mentioned me', 'mentioned_me'],
        ['Opened by me', 'opened_by_me'],
        ['Unassigned', 'unassigned'],
        ['Assigned to others', 'others'],
      ].map(([label, value]) => (
        <FilterPill
          key={value}
          selected={conversationListFilters.assignment.includes(value as ConversationAssignmentFilter)}
          onClick={() => toggleAssignmentFilter(value as ConversationAssignmentFilter)}
        >
          {label}
        </FilterPill>
      ))}
    </FilterSection>
    <FilterSection title="Tags">
      <TagFilterSelector
        tags={supportTags}
        selectedTagIds={conversationListFilters.tagIds}
        onToggleTag={toggleTagFilter}
      />
    </FilterSection>
    <FilterSection title="Team inbox">
      <FilterPill
        selected={
          conversationListFilters.mailboxIds.includes('all') ||
          (selectedMailboxId === 'all' && conversationListFilters.mailboxIds.length === 0 && navFilter !== 'inbox')
        }
        onClick={() => toggleMailboxFilter('all')}
      >
        All
      </FilterPill>
      <FilterPill
        selected={
          conversationListFilters.mailboxIds.length > 0
            ? conversationListFilters.mailboxIds.includes('shared')
            : selectedMailboxId === 'all' && navFilter === 'inbox'
        }
        onClick={() => toggleMailboxFilter('shared')}
      >
        Main inbox
      </FilterPill>
      {teamInboxFilterOptions.map((mailbox) => (
        <FilterPill
          key={mailbox.id}
          selected={
            conversationListFilters.mailboxIds.length > 0
              ? conversationListFilters.mailboxIds.includes(mailbox.id)
              : selectedMailboxId === mailbox.id
          }
          onClick={() => toggleMailboxFilter(mailbox.id)}
        >
          {mailbox.name}
        </FilterPill>
      ))}
    </FilterSection>
    <FilterSection title="SLA">
      <FilterPill
        disabled={!slaEnabled}
        selected={Boolean(conversationListFilters.slaBreached)}
        onClick={() => setConversationListFilter('slaBreached', !conversationListFilters.slaBreached)}
      >
        Breached only
      </FilterPill>
    </FilterSection>
    <FilterSection title="Sort">
      {[
        ['Newest', 'newest'],
        ['Oldest', 'oldest'],
        ['Next SLA due', 'sla'],
      ].map(([label, value]) => (
        <FilterPill
          key={value}
          disabled={value === 'sla' && !slaEnabled}
          selected={conversationListFilters.sort === value}
          onClick={() => setConversationListFilter('sort', value as ConversationSortOrder)}
        >
          {label}
        </FilterPill>
      ))}
    </FilterSection>
  </div>;
}
