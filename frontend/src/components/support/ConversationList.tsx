import { useMemo, useCallback, useState, memo, useEffect, type ReactNode, type UIEvent } from 'react';
import {
  Cancel01Icon,
  FilterHorizontalIcon,
  Message01Icon,
  PlusSignIcon,
  Search01Icon,
  Settings02Icon,
} from '@/lib/icons';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Label } from '@/components/ui/label';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Switch } from '@/components/ui/switch';
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from '@/components/ui/command';
import { useCreateSupportInboxView, useInfiniteConversations, useInboxScopes, useMarkConversationRead, useSupportTags } from '@/hooks/queries/useSupport';
import { useSupportInboxStore, type NavFilter } from '@/stores/supportInboxStore';
import { useSupportPresenceStore } from '@/stores/supportPresenceStore';
import { ConversationRow } from './ConversationRow';
import { EmptyState } from './EmptyState';
import {
  buildConversationListRequestFilters,
  buildSupportInboxViewFilters,
  defaultAIStatesForNav,
  defaultAssignmentForNav,
  defaultStatesForNav,
  filterSupportConversations,
  hasConversationListChanges,
  statesEqual,
  stringArraysEqual,
  type ConversationAIStateFilter,
  type ConversationAssignmentFilter,
  type ConversationListFilters,
  type ConversationSortOrder,
  type ConversationStateFilter,
} from '@/lib/supportInboxFilters';
import type { SupportInboxScope, SupportTag } from '@/lib/pmTypes';
import { cn } from '@/lib/utils';
import { SupportTagBadge } from './SupportTagPicker';

const SkeletonRow = memo(function SkeletonRow() {
  return (
    <div className="flex items-center gap-3 px-3 py-2.5">
      <div className="h-9 w-9 shrink-0 animate-pulse rounded-full bg-muted" />
      <div className="flex-1 space-y-1.5">
        <div className="flex items-center justify-between gap-2">
          <div className="h-3.5 w-28 animate-pulse rounded bg-muted" />
          <div className="h-3 w-10 animate-pulse rounded bg-muted" />
        </div>
        <div className="h-3 w-40 animate-pulse rounded bg-muted" />
      </div>
    </div>
  );
});

function emptyCopyForNavFilter(navFilter: string, searchQuery: string): { title: string; subtitle: string } {
  if (searchQuery.trim()) {
    return {
      title: 'No matching conversations',
      subtitle: 'Try another customer, subject, or email.',
    };
  }
  switch (navFilter) {
    case 'mine':
      return {
        title: 'Nothing for you',
        subtitle: 'Assigned conversations and mentions will appear here.',
      };
    case 'waiting':
      return {
        title: 'No waiting conversations',
        subtitle: 'Conversations waiting on a customer will appear here.',
      };
    case 'resolved':
      return {
        title: 'No resolved conversations',
        subtitle: 'Resolved conversations will appear here.',
      };
    case 'spam':
      return {
        title: 'No spam',
        subtitle: 'Spam conversations will appear here.',
      };
    case 'ai_active':
      return {
        title: 'No active AI conversations',
        subtitle: 'AI-handled conversations will appear here.',
      };
    case 'resolved_by_ai':
      return {
        title: 'No AI resolutions yet',
        subtitle: 'Resolved AI conversations will appear here.',
      };
    case 'inbox':
    default:
      return {
        title: 'Inbox is clear',
        subtitle: 'New conversations needing a teammate will appear here.',
      };
  }
}

function titleForView(navFilter: NavFilter): string {
  switch (navFilter) {
    case 'inbox':
      return 'Inbox';
    case 'mine':
      return 'Mine';
    case 'waiting':
      return 'Waiting';
    case 'resolved':
      return 'Resolved';
    case 'spam':
      return 'Spam';
    case 'ai_active':
      return 'AI Handling';
    case 'resolved_by_ai':
      return 'AI Resolved';
  }
}

function filterCount(filters: ConversationListFilters, navFilter: NavFilter, searchQuery: string): number {
  let count = 0;
  if (searchQuery.trim()) count += 1;
  if (!statesEqual(filters.states, defaultStatesForNav(navFilter))) count += 1;
  if (!stringArraysEqual(filters.assignment, defaultAssignmentForNav(navFilter))) count += 1;
  if (filters.mailboxIds.length > 0) count += 1;
  if (filters.tagIds.length > 0) count += 1;
  if (!stringArraysEqual(filters.aiStates, defaultAIStatesForNav(navFilter))) count += 1;
  if (filters.sort !== 'newest') count += 1;
  return count;
}

function FilterPill({
  children,
  selected,
  onClick,
}: {
  children: ReactNode;
  selected: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      className={cn(
        'inline-flex h-8 items-center gap-1.5 rounded-md border px-2.5 text-xs font-medium transition-colors',
        selected
          ? 'border-border bg-primary/10 text-primary'
          : 'border-border bg-background text-foreground hover:bg-muted'
      )}
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

interface ConversationListProps {
  workspaceId: string;
  userId?: string;
  onOnboardingEmptyChange?: (isEmpty: boolean) => void;
  onWidgetSettingsClick?: () => void;
  onCreateConversationClick?: () => void;
  canCreateSharedViews?: boolean;
}

export function ConversationList({
  workspaceId,
  userId,
  onOnboardingEmptyChange,
  onWidgetSettingsClick,
  onCreateConversationClick,
  canCreateSharedViews = false,
}: ConversationListProps) {
  const searchQuery = useSupportInboxStore((s) => s.searchQuery);
  const setSearchQuery = useSupportInboxStore((s) => s.setSearchQuery);
  const navFilter = useSupportInboxStore((s) => s.navFilter);
  const selectedMailboxId = useSupportInboxStore((s) => s.selectedMailboxId);
  const setMailboxFilter = useSupportInboxStore((s) => s.setMailboxFilter);
  const conversationListFilters = useSupportInboxStore((s) => s.conversationListFilters);
  const setConversationListFilter = useSupportInboxStore((s) => s.setConversationListFilter);
  const setConversationMailboxFilters = useSupportInboxStore((s) => s.setConversationMailboxFilters);
  const resetConversationListFilters = useSupportInboxStore((s) => s.resetConversationListFilters);
  const selectConversation = useSupportInboxStore((s) => s.selectConversation);
  const [searchExpanded, setSearchExpanded] = useState(false);
  const [saveViewOpen, setSaveViewOpen] = useState(false);
  const [saveViewName, setSaveViewName] = useState('');
  const [saveViewShared, setSaveViewShared] = useState(false);
  const wsSend = useSupportPresenceStore((s) => s.wsSend);
  const wsConnected = useSupportPresenceStore((s) => s.wsConnected);
  const markConversationRead = useMarkConversationRead(workspaceId);
  const createInboxView = useCreateSupportInboxView(workspaceId);
  const { data: inboxScopes } = useInboxScopes(workspaceId);
  const { data: supportTags = [] } = useSupportTags(workspaceId);

  const handleSelect = useCallback((id: string, unreadCount?: number) => {
    selectConversation(id);
    if ((unreadCount ?? 0) > 0) {
      window.requestAnimationFrame(() => {
        window.setTimeout(() => markConversationRead.mutate(id), 0);
      });
    }
  }, [markConversationRead, selectConversation]);

  const filters = useMemo(() => buildConversationListRequestFilters({
    navFilter,
    selectedMailboxId,
    searchQuery,
    listFilters: conversationListFilters,
  }), [conversationListFilters, navFilter, searchQuery, selectedMailboxId]);
  const {
    data: response,
    isLoading,
    isFetchingNextPage,
    hasNextPage,
    fetchNextPage,
    error,
  } = useInfiniteConversations(workspaceId, filters);
  const conversations = useMemo(
    () => response?.pages.flatMap((page) => page.data ?? []) ?? [],
    [response],
  );

  const filteredConversations = useMemo(() => {
    const hasExplicitStateFilters = !statesEqual(conversationListFilters.states, defaultStatesForNav(navFilter));
    const hasExplicitAIStateFilters = !stringArraysEqual(conversationListFilters.aiStates, defaultAIStatesForNav(navFilter));
    const shouldSkipSidebarViewFilter = hasExplicitStateFilters || ((navFilter === 'ai_active' || navFilter === 'resolved_by_ai') && hasExplicitAIStateFilters);
    return filterSupportConversations(conversations, {
      navFilter,
      mailboxScope: selectedMailboxId === 'all' && conversationListFilters.mailboxIds.length === 0 && navFilter === 'inbox' ? 'shared' : selectedMailboxId,
      mailboxScopes: conversationListFilters.mailboxIds,
      userId,
      searchQuery: '',
      sortOrder: conversationListFilters.sort,
      skipViewFilter: shouldSkipSidebarViewFilter,
      aiStates: conversationListFilters.aiStates,
    });
  }, [conversationListFilters.aiStates, conversationListFilters.mailboxIds, conversationListFilters.sort, conversationListFilters.states, conversations, navFilter, selectedMailboxId, userId]);
  const mailboxMoveOptions = useMemo(
    () => [inboxScopes?.shared_inbox, ...(inboxScopes?.mailboxes ?? [])].filter(Boolean) as SupportInboxScope[],
    [inboxScopes]
  );
  const teamInboxFilterOptions = inboxScopes?.mailboxes ?? [];
  const selectedMailboxName = useMemo(() => {
    if (selectedMailboxId === 'all') return null;
    if (selectedMailboxId === inboxScopes?.shared_inbox?.id || selectedMailboxId === 'shared') {
      return inboxScopes?.shared_inbox?.name ?? 'Shared Inbox';
    }
    return inboxScopes?.mailboxes?.find((mailbox) => mailbox.id === selectedMailboxId)?.name ?? null;
  }, [inboxScopes, selectedMailboxId]);
  const viewTitle = titleForView(navFilter);
  const listTitle = selectedMailboxName
    ? navFilter === 'inbox'
      ? `${selectedMailboxName} inbox`
      : `${selectedMailboxName}: ${viewTitle}`
    : viewTitle;
  const activeFilterCount = filterCount(conversationListFilters, navFilter, searchQuery);
  const canSaveCurrentView = hasConversationListChanges({
    navFilter,
    searchQuery,
    listFilters: conversationListFilters,
  });
  const shouldShowEmptyState = !isLoading && filteredConversations.length === 0 && !error && !hasNextPage;
  const shouldShowOnboardingEmptyState =
    shouldShowEmptyState &&
    navFilter === 'inbox' &&
    selectedMailboxId === 'all' &&
    !searchQuery.trim();

  useEffect(() => {
    onOnboardingEmptyChange?.(shouldShowOnboardingEmptyState);
  }, [onOnboardingEmptyChange, shouldShowOnboardingEmptyState]);

  useEffect(() => {
    if (!wsSend || !wsConnected || filteredConversations.length === 0) return;
    wsSend('support:presence:sync', {
      conversation_ids: filteredConversations.map((conversation) => conversation.id),
    });
  }, [filteredConversations, wsConnected, wsSend]);

  const handleListScroll = useCallback((event: UIEvent<HTMLDivElement>) => {
    if (!hasNextPage || isFetchingNextPage) {
      return;
    }
    const target = event.currentTarget;
    const distanceFromBottom = target.scrollHeight - target.scrollTop - target.clientHeight;
    if (distanceFromBottom <= 80) {
      fetchNextPage();
    }
  }, [fetchNextPage, hasNextPage, isFetchingNextPage]);

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

  const handleSaveView = useCallback(() => {
    const name = saveViewName.trim();
    if (!name) return;
    createInboxView.mutate({
      name,
      is_shared: canCreateSharedViews && saveViewShared,
      filters: buildSupportInboxViewFilters({
        navFilter,
        selectedMailboxId,
        searchQuery,
        listFilters: conversationListFilters,
      }),
    }, {
      onSuccess: () => {
        setSaveViewOpen(false);
        setSaveViewName('');
        setSaveViewShared(false);
      },
    });
  }, [
    canCreateSharedViews,
    conversationListFilters,
    createInboxView,
    navFilter,
    saveViewName,
    saveViewShared,
    searchQuery,
    selectedMailboxId,
  ]);

  return (
    <div className="flex h-full w-[300px] flex-col border-r bg-background dark:border-sidebar-border dark:bg-sidebar">
      {/* Filter toolbar */}
      <div
        className="relative z-10 flex items-center gap-1.5 border-b border-border/60 bg-background/85 px-2 py-1.5 supports-[backdrop-filter]:bg-background/75 dark:border-sidebar-border dark:bg-sidebar/90 dark:supports-[backdrop-filter]:bg-sidebar/80"
        style={{ backdropFilter: 'blur(8px) saturate(160%)' }}
      >
        {searchExpanded ? (
          <div className="flex flex-1 items-center gap-1">
            <Search01Icon className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
            <Input
              autoFocus
              className="h-7 flex-1 border-0 bg-transparent px-1 text-sm shadow-none focus-visible:ring-0 dark:text-sidebar-foreground dark:placeholder:text-sidebar-foreground/60"
              placeholder="Search conversations..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Escape') {
                  setSearchQuery('');
                  setSearchExpanded(false);
                }
              }}
            />
            <Button
              variant="ghost"
              size="icon"
              className="h-6 w-6 shrink-0"
              onClick={() => { setSearchQuery(''); setSearchExpanded(false); }}
            >
              <Cancel01Icon className="h-3.5 w-3.5" />
            </Button>
          </div>
        ) : (
          <>
            <div className="min-w-0 flex-1 px-1.5 text-sm font-medium">
              <div className="truncate">{listTitle}</div>
            </div>
            <Popover>
              <PopoverTrigger asChild>
                <Button
                  variant="ghost"
                  size="icon"
                  className={cn('relative h-7 w-7 shrink-0', activeFilterCount > 0 && 'text-primary')}
                  aria-label="Conversation filters"
                >
                  <FilterHorizontalIcon className="h-3.5 w-3.5" />
                  {activeFilterCount > 0 && (
                    <span className="absolute right-0.5 top-0.5 h-1.5 w-1.5 rounded-full bg-primary" />
                  )}
                </Button>
              </PopoverTrigger>
              <PopoverContent side="right" align="start" sideOffset={8} className="w-[380px] p-3">
                <div className="mb-3 flex items-center justify-between gap-3">
                  <div className="min-w-0 text-sm font-semibold">
                    <span className="block truncate">{listTitle} filters</span>
                  </div>
                  <Button
                    variant="ghost"
                    size="sm"
                    className="h-7 px-2 text-xs"
                    onClick={() => {
                      resetConversationListFilters();
                      setMailboxFilter('all');
                      setSearchQuery('');
                    }}
                  >
                    Reset
                  </Button>
                </div>
                <div className="space-y-4">
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
                  <FilterSection title="Sort">
                    {[
                      ['Newest', 'newest'],
                      ['Oldest', 'oldest'],
                    ].map(([label, value]) => (
                      <FilterPill
                        key={value}
                        selected={conversationListFilters.sort === value}
                        onClick={() => setConversationListFilter('sort', value as ConversationSortOrder)}
                      >
                        {label}
                      </FilterPill>
                    ))}
                  </FilterSection>
                  {canSaveCurrentView && (
                    <div className="border-t pt-3">
                      <Button
                        variant="outline"
                        size="sm"
                        className="w-full justify-center"
                        onClick={() => setSaveViewOpen(true)}
                      >
                        Save as view
                      </Button>
                    </div>
                  )}
                </div>
              </PopoverContent>
            </Popover>
            <Button
              variant="ghost"
              size="icon"
              className="h-7 w-7 shrink-0"
              onClick={() => setSearchExpanded(true)}
            >
              <Search01Icon className="h-3.5 w-3.5" />
            </Button>
          </>
        )}
      </div>
      <Dialog open={saveViewOpen} onOpenChange={setSaveViewOpen}>
        <DialogContent aria-describedby={undefined}>
          <DialogHeader>
            <DialogTitle>Save view</DialogTitle>
          </DialogHeader>
          <div className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="support-view-name">Name</Label>
              <Input
                id="support-view-name"
                autoFocus
                value={saveViewName}
                onChange={(event) => setSaveViewName(event.target.value)}
                onKeyDown={(event) => {
                  if (event.key === 'Enter') {
                    event.preventDefault();
                    handleSaveView();
                  }
                }}
              />
            </div>
            {canCreateSharedViews && (
              <div className="flex items-center justify-between rounded-md border px-3 py-2">
                <Label htmlFor="support-view-shared" className="text-sm font-medium">
                  Shared with workspace
                </Label>
                <Switch
                  id="support-view-shared"
                  checked={saveViewShared}
                  onCheckedChange={setSaveViewShared}
                />
              </div>
            )}
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setSaveViewOpen(false)}>
              Cancel
            </Button>
            <Button onClick={handleSaveView} disabled={!saveViewName.trim() || createInboxView.isPending}>
              Save
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Conversation list */}
      <div
        className="flex-1 min-h-0 overflow-y-auto"
        data-support-conversation-scroll
        onScroll={handleListScroll}
      >
        {isLoading && (
          <div>
            <SkeletonRow />
            <SkeletonRow />
            <SkeletonRow />
            <SkeletonRow />
            <SkeletonRow />
          </div>
        )}
        {error && (
          <p className="p-4 text-sm text-destructive">{String(error)}</p>
        )}
        {shouldShowEmptyState && (
          <EmptyState
            icon={Message01Icon}
            title={emptyCopyForNavFilter(navFilter, searchQuery).title}
            subtitle={emptyCopyForNavFilter(navFilter, searchQuery).subtitle}
            actions={
              shouldShowOnboardingEmptyState && onWidgetSettingsClick && onCreateConversationClick ? (
                <>
                  <Button size="sm" className="w-full justify-center" onClick={onWidgetSettingsClick}>
                    <Settings02Icon className="h-4 w-4" />
                    Install widget
                  </Button>
                  <Button size="sm" variant="outline" className="w-full justify-center" onClick={onCreateConversationClick}>
                    <PlusSignIcon className="h-4 w-4" />
                    Create test
                  </Button>
                </>
              ) : undefined
            }
            actionsClassName="w-full md:hidden"
          />
        )}
        {filteredConversations.map((conversation) => (
          <ConversationRow
            key={conversation.id}
            workspaceId={workspaceId}
            conversation={conversation}
            moveOptions={mailboxMoveOptions}
            onSelectConversation={handleSelect}
          />
        ))}
        {hasNextPage && (
          <div className="flex justify-center px-3 py-3">
            <Button
              variant="ghost"
              size="sm"
              className="text-xs"
              onClick={() => fetchNextPage()}
              disabled={isFetchingNextPage}
            >
              {isFetchingNextPage ? 'Loading...' : 'Load more'}
            </Button>
          </div>
        )}
      </div>
    </div>
  );
}
