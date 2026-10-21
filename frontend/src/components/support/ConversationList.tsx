import { useMemo, useCallback, useState, memo, useEffect, type ReactNode, type UIEvent } from 'react';
import {
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
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import { useCreateSupportInboxView, useHasAnySupportConversation, useInfiniteConversations, useInboxScopes, useSupportInboxViews, useSupportTags, useUpdateSupportBuiltinInboxView, useUpdateSupportInboxView } from '@/hooks/queries/useSupport';
import { supportInboxBuiltinViewKey, useSupportInboxStore, type NavFilter } from '@/stores/supportInboxStore';
import { useSupportPresenceStore } from '@/stores/supportPresenceStore';
import { ConversationRow } from './ConversationRow';
import { EmptyState } from './EmptyState';
import {
  buildConversationListRequestFilters,
  buildSupportInboxViewFilters,
  defaultAIStatesForNav,
  defaultConversationListFiltersForNav,
  defaultStatesForNav,
  filterChangeCountFromBaseline,
  filterSupportConversations,
  parseSupportInboxViewFilters,
  statesEqual,
  stringArraysEqual,
  type ConversationAIStateFilter,
  type ConversationAssignmentFilter,
  type ConversationSortOrder,
  type ConversationStateFilter,
} from '@/lib/supportInboxFilters';
import type { SupportConversation, SupportInboxScope, SupportInboxView, SupportTag } from '@/lib/pmTypes';
import { cn } from '@/lib/utils';
import { SupportTagBadge } from './SupportTagPicker';
import { SupportInboxPanelHeader } from './SupportInboxPanelHeader';
import { workspaceSidebarSafeInsetClassName } from '@/components/design-system/quiet';
import { ConversationBulkToolbar } from './ConversationBulkToolbar';
import { useConversationSelection } from './useConversationSelection';

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

export type SupportInboxEmptyState = 'onboarding' | 'inbox-zero' | null;

interface ConversationListProps {
  workspaceId: string;
  userId?: string;
  autoSelectFirst?: boolean;
  onConversationOpen?: (conversationId: string) => void;
  onInboxEmptyStateChange?: (state: SupportInboxEmptyState) => void;
  onWidgetSettingsClick?: () => void;
  onCreateConversationClick?: () => void;
  onSearchClick?: () => void;
  onViewCreated?: (view: SupportInboxView) => void;
  canCreateSharedViews?: boolean;
  canEditConversations?: boolean;
}

export function ConversationList({
  workspaceId,
  userId,
  autoSelectFirst = true,
  onConversationOpen,
  onInboxEmptyStateChange,
  onWidgetSettingsClick,
  onCreateConversationClick,
  onSearchClick,
  onViewCreated,
  canCreateSharedViews = false,
  canEditConversations = false,
}: ConversationListProps) {
  const searchQuery = useSupportInboxStore((s) => s.searchQuery);
  const navFilter = useSupportInboxStore((s) => s.navFilter);
  const activeCustomViewId = useSupportInboxStore((s) => s.activeCustomViewId);
  const customViewDirty = useSupportInboxStore((s) => s.customViewDirty);
  const selectedMailboxId = useSupportInboxStore((s) => s.selectedMailboxId);
  const conversationListFilters = useSupportInboxStore((s) => s.conversationListFilters);
  const setConversationListFilter = useSupportInboxStore((s) => s.setConversationListFilter);
  const setConversationMailboxFilters = useSupportInboxStore((s) => s.setConversationMailboxFilters);
  const markCustomViewClean = useSupportInboxStore((s) => s.markCustomViewClean);
  const applyCustomView = useSupportInboxStore((s) => s.applyCustomView);
  const builtinViewFilters = useSupportInboxStore((s) => s.builtinViewFilters);
  const setBuiltinViewFilter = useSupportInboxStore((s) => s.setBuiltinViewFilter);
  const syncRouteState = useSupportInboxStore((s) => s.syncRouteState);
  const selectConversation = useSupportInboxStore((s) => s.selectConversation);
  const createCustomViewOpen = useSupportInboxStore((s) => s.createCustomViewOpen);
  const setCreateCustomViewOpen = useSupportInboxStore((s) => s.setCreateCustomViewOpen);
  const selectedConversationId = useSupportInboxStore((s) => s.selectedConversationId);
  const handoffConversationId = useSupportInboxStore((s) => s.conversationHandoff?.fromConversationId ?? null);
  const [saveViewOpen, setSaveViewOpen] = useState(false);
  const [filterPopoverOpen, setFilterPopoverOpen] = useState(false);
  const [saveViewName, setSaveViewName] = useState('');
  const [saveViewShared, setSaveViewShared] = useState(false);
  const wsSend = useSupportPresenceStore((s) => s.wsSend);
  const wsConnected = useSupportPresenceStore((s) => s.wsConnected);
  const createInboxView = useCreateSupportInboxView(workspaceId);
  const updateInboxView = useUpdateSupportInboxView(workspaceId);
  const updateBuiltinInboxView = useUpdateSupportBuiltinInboxView(workspaceId);
  const { data: inboxScopes } = useInboxScopes(workspaceId);
  const { data: customViews = [] } = useSupportInboxViews(workspaceId, !!activeCustomViewId);
  const { data: supportTags = [] } = useSupportTags(workspaceId);

  const handleSelect = useCallback((id: string, _unreadCount?: number) => {
    if (onConversationOpen) {
      onConversationOpen(id);
    } else {
      selectConversation(id);
    }
  }, [onConversationOpen, selectConversation]);

  const filters = useMemo(() => buildConversationListRequestFilters({
    navFilter,
    selectedMailboxId,
    searchQuery,
    listFilters: conversationListFilters,
  }), [conversationListFilters, navFilter, searchQuery, selectedMailboxId]);
  const selectionScope = JSON.stringify([workspaceId, userId, activeCustomViewId, navFilter, selectedMailboxId, conversationListFilters, filters]);
  const selection = useConversationSelection(selectionScope);
  const [actionScope, setActionScope] = useState<string | null>(null);
  if (actionScope !== null && actionScope !== selectionScope) setActionScope(null);
  const bulkActionPending = actionScope === selectionScope;
  const selectionActive = selection.items.size > 0 || selection.loading;
  const {
    data: response,
    isLoading,
    isFetching,
    isFetchingNextPage,
    hasNextPage,
    fetchNextPage,
    error,
  } = useInfiniteConversations(workspaceId, filters);
  const conversations = useMemo(
    () => response?.pages.flatMap((page) => page.data ?? []) ?? [],
    [response],
  );

  const filterConversations = useCallback((items: SupportConversation[]) => {
    const hasExplicitStateFilters = !statesEqual(conversationListFilters.states, defaultStatesForNav(navFilter));
    const hasExplicitAIStateFilters = !stringArraysEqual(conversationListFilters.aiStates, defaultAIStatesForNav(navFilter));
    const shouldSkipSidebarViewFilter = navFilter === 'mine' || hasExplicitStateFilters || ((navFilter === 'ai_active' || navFilter === 'resolved_by_ai') && hasExplicitAIStateFilters);
    return filterSupportConversations(items, {
      navFilter,
      mailboxScope: selectedMailboxId === 'all' && conversationListFilters.mailboxIds.length === 0 && navFilter === 'inbox' ? 'shared' : selectedMailboxId,
      mailboxScopes: conversationListFilters.mailboxIds,
      userId,
      searchQuery: '',
      sortOrder: conversationListFilters.sort,
      skipViewFilter: shouldSkipSidebarViewFilter,
      aiStates: conversationListFilters.aiStates,
    });
  }, [conversationListFilters.aiStates, conversationListFilters.mailboxIds, conversationListFilters.sort, conversationListFilters.states, navFilter, selectedMailboxId, userId]);
  const filteredConversations = useMemo(() => filterConversations(conversations), [conversations, filterConversations]);
  const allLoadedSelected = filteredConversations.length > 0 && filteredConversations.every((conversation) => selection.items.has(conversation.id));
  const totalMatches = response?.pages[0]?.total;
  const handleToggleSelection = useCallback((id: string) => {
    if (bulkActionPending || selection.loading) return;
    const conversation = filteredConversations.find((item) => item.id === id);
    if (conversation) selection.toggle(conversation);
  }, [bulkActionPending, filteredConversations, selection.loading, selection.toggle]);
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
  const activeCustomViewName = activeCustomViewId
    ? customViews.find((view) => view.id === activeCustomViewId)?.name ?? null
    : null;
  const listTitle = activeCustomViewName ?? (selectedMailboxName
    ? navFilter === 'inbox'
      ? `${selectedMailboxName} inbox`
      : `${selectedMailboxName}: ${viewTitle}`
    : viewTitle);
  const currentViewFilters = useMemo(() => buildSupportInboxViewFilters({
    navFilter,
    selectedMailboxId,
    searchQuery,
    listFilters: conversationListFilters,
  }), [conversationListFilters, navFilter, searchQuery, selectedMailboxId]);
  const currentBuiltinViewKey = activeCustomViewId ? null : supportInboxBuiltinViewKey(navFilter, selectedMailboxId);
  const builtinBaseline = useMemo(() => {
    const savedFilters = currentBuiltinViewKey ? builtinViewFilters[currentBuiltinViewKey] : undefined;
    if (savedFilters) {
      const parsed = parseSupportInboxViewFilters(savedFilters, navFilter);
      return {
        searchQuery: parsed.searchQuery,
        filters: parsed.listFilters,
      };
    }
    return {
      searchQuery: '',
      filters: defaultConversationListFiltersForNav(navFilter),
    };
  }, [builtinViewFilters, currentBuiltinViewKey, navFilter]);
  const defaultBaseline = useMemo(() => ({
    searchQuery: '',
    filters: defaultConversationListFiltersForNav(navFilter),
  }), [navFilter]);
  const builtinFilterChangeCount = activeCustomViewId ? 0 : filterChangeCountFromBaseline({
    searchQuery,
    listFilters: conversationListFilters,
    baselineSearchQuery: builtinBaseline.searchQuery,
    baselineFilters: builtinBaseline.filters,
  });
  const customFilterChangeCount = activeCustomViewId && customViewDirty ? filterChangeCountFromBaseline({
    searchQuery,
    listFilters: conversationListFilters,
    baselineSearchQuery: defaultBaseline.searchQuery,
    baselineFilters: defaultBaseline.filters,
  }) : 0;
  const activeFilterCount = activeCustomViewId ? (customViewDirty ? Math.max(1, customFilterChangeCount) : 0) : builtinFilterChangeCount;
  const canSaveCurrentView = activeCustomViewId ? customViewDirty : builtinFilterChangeCount > 0;
  const canUpdateCurrentView = activeCustomViewId ? customViewDirty : builtinFilterChangeCount > 0;
  const isSwitchingEmptyList = !!isFetching && !isFetchingNextPage && filteredConversations.length === 0;
  const shouldShowListSkeleton = isLoading || isSwitchingEmptyList;
  const shouldShowEmptyState = !shouldShowListSkeleton && filteredConversations.length === 0 && !error && !hasNextPage;
  const isDefaultInboxEmpty =
    shouldShowEmptyState &&
    navFilter === 'inbox' &&
    selectedMailboxId === 'all' &&
    !searchQuery.trim();
  // An empty Inbox only means "set up support" when the workspace has no
  // conversations at all; otherwise everything is resolved/waiting (inbox zero).
  const { data: hasAnyConversation } = useHasAnySupportConversation(workspaceId, isDefaultInboxEmpty);
  const inboxEmptyState: SupportInboxEmptyState = !isDefaultInboxEmpty || hasAnyConversation === undefined
    ? null
    : hasAnyConversation ? 'inbox-zero' : 'onboarding';
  const shouldShowOnboardingEmptyState = inboxEmptyState === 'onboarding';

  useEffect(() => {
    onInboxEmptyStateChange?.(inboxEmptyState);
  }, [onInboxEmptyStateChange, inboxEmptyState]);

  useEffect(() => {
    if (!autoSelectFirst || selectionActive || selectedConversationId || shouldShowListSkeleton || error || filteredConversations.length === 0) return;
    const firstConversation = filteredConversations[0];
    selectConversation(firstConversation.id);
  }, [autoSelectFirst, error, filteredConversations, selectConversation, selectedConversationId, selectionActive, shouldShowListSkeleton]);

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
      onSuccess: (view) => {
        setSaveViewOpen(false);
        setSaveViewName('');
        setSaveViewShared(false);
        onViewCreated?.(view);
      },
    });
  }, [
    canCreateSharedViews,
    conversationListFilters,
    createInboxView,
    navFilter,
    onViewCreated,
    saveViewName,
    saveViewShared,
    searchQuery,
    selectedMailboxId,
  ]);

  const handleUpdateView = useCallback(() => {
    if (!activeCustomViewId) {
      if (!currentBuiltinViewKey) return;
      updateBuiltinInboxView.mutate({
        view_key: currentBuiltinViewKey,
        filters: currentViewFilters,
      }, {
        onSuccess: (view) => {
          if (view.view_key) {
            setBuiltinViewFilter(view.view_key, view.filters);
          }
        },
      });
      return;
    }
    updateInboxView.mutate({
      id: activeCustomViewId,
      filters: currentViewFilters,
    }, {
      onSuccess: (view) => {
        if (view) {
          applyCustomView(view);
        } else {
          markCustomViewClean();
        }
      },
    });
  }, [
    activeCustomViewId,
    applyCustomView,
    currentBuiltinViewKey,
    currentViewFilters,
    markCustomViewClean,
    setBuiltinViewFilter,
    updateBuiltinInboxView,
    updateInboxView,
  ]);

  const handleResetFilters = useCallback(() => {
    if (activeCustomViewId) {
      const activeView = customViews.find((view) => view.id === activeCustomViewId);
      if (activeView) {
        applyCustomView(activeView);
        return;
      }
    }

    syncRouteState({
      navFilter,
      selectedMailboxId,
      statusFilter: 'all',
      searchQuery: builtinBaseline.searchQuery,
      activeCustomViewId: null,
      conversationListFilters: builtinBaseline.filters,
    });
  }, [
    activeCustomViewId,
    applyCustomView,
    builtinBaseline.filters,
    builtinBaseline.searchQuery,
    customViews,
    navFilter,
    selectedMailboxId,
    syncRouteState,
  ]);

  return (
    <div className="flex h-full w-full flex-col bg-muted/30 md:w-[300px] md:border-r dark:border-sidebar-border"
      onKeyDownCapture={(event) => {
        // Menus and dialogs handle their own Escape; a tooltip on a toolbar
        // button must not swallow the shortcut for clearing row selection.
        if (event.key !== 'Escape' || !selectionActive || bulkActionPending) return;
        if ((event.target as HTMLElement).closest('[role="menu"], [role="dialog"], [role="alertdialog"], [data-slot="popover-content"]')) return;
        event.preventDefault();
        event.stopPropagation();
        selection.clear();
      }}
    >
      {/* Filter toolbar */}
      <SupportInboxPanelHeader
        className={cn(
          workspaceSidebarSafeInsetClassName,
          'gap-1.5 bg-muted/30 px-2',
        )}
      >
        <TooltipProvider>
          {selectionActive ? <ConversationBulkToolbar
            key={selectionScope}
            workspaceId={workspaceId}
            conversations={[...selection.items.values()]}
            loadedCount={filteredConversations.length}
            allLoadedSelected={allLoadedSelected}
            hasMore={!!hasNextPage}
            loadingSelection={selection.loading}
            canEdit={canEditConversations}
            moveOptions={mailboxMoveOptions}
            onSelectLoaded={() => selection.selectLoaded(filteredConversations)}
            onClear={selection.clear}
            onBusyChange={(busy) => setActionScope((previous) => busy ? selectionScope : previous === selectionScope ? null : previous)}
            onCompleted={selection.removeCompleted}
          /> : <>
          <div className="min-w-0 flex-1 px-1.5 text-sm font-medium">
            <div className="truncate">{listTitle}</div>
          </div>
          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                variant="ghost"
                size="icon"
                className="h-7 w-7 shrink-0"
                aria-label="New conversation"
                onClick={onCreateConversationClick}
              >
                <PlusSignIcon className="h-3.5 w-3.5" />
              </Button>
            </TooltipTrigger>
            <TooltipContent side="bottom">
              <span className="text-xs">New conversation</span>
            </TooltipContent>
          </Tooltip>
          <Popover
            open={filterPopoverOpen || createCustomViewOpen}
            onOpenChange={(open) => {
              setFilterPopoverOpen(open);
              if (!open) setCreateCustomViewOpen(false);
            }}
          >
            <Tooltip>
              <TooltipTrigger asChild>
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
              </TooltipTrigger>
              <TooltipContent side="bottom">
                <span className="text-xs">Filter conversations</span>
              </TooltipContent>
            </Tooltip>
            <PopoverContent side="right" align="start" sideOffset={8} className="w-[380px] p-3">
              <div className="mb-3 flex items-center justify-between gap-3">
                <div className="min-w-0 text-sm font-semibold">
                  <span className="block truncate">{createCustomViewOpen ? 'New view' : `${listTitle} filters`}</span>
                </div>
                {activeFilterCount > 0 && (
                  <Button
                    variant="ghost"
                    size="sm"
                    className="h-7 px-2 text-xs"
                    onClick={handleResetFilters}
                  >
                    Reset
                  </Button>
                )}
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
                  {(createCustomViewOpen || canSaveCurrentView || canUpdateCurrentView) && (
                    <div className="space-y-2 border-t pt-3">
                      {!createCustomViewOpen && canUpdateCurrentView && (
                        <TooltipProvider>
                          <Tooltip>
                            <TooltipTrigger asChild>
                              <Button
                                variant="default"
                                size="sm"
                                className="w-full justify-center"
                                disabled={activeCustomViewId ? updateInboxView.isPending : updateBuiltinInboxView.isPending}
                                onClick={handleUpdateView}
                                title="Updates this view for you only."
                              >
                                Update view
                              </Button>
                            </TooltipTrigger>
                            <TooltipContent side="top">
                              <span className="text-xs">Updates this view for you only.</span>
                            </TooltipContent>
                          </Tooltip>
                        </TooltipProvider>
                      )}
                      <Button
                        variant="outline"
                        size="sm"
                        className="w-full justify-center"
                        onClick={() => {
                          setFilterPopoverOpen(false);
                          setCreateCustomViewOpen(false);
                          setSaveViewOpen(true);
                        }}
                      >
                        {createCustomViewOpen ? 'Create view' : 'Save as new view'}
                      </Button>
                    </div>
                  )}
                </div>
              </PopoverContent>
          </Popover>
          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                variant="ghost"
                size="icon"
                className="h-7 w-7 shrink-0"
                aria-label="Search conversations"
                onClick={onSearchClick}
              >
                <Search01Icon className="h-3.5 w-3.5" />
              </Button>
            </TooltipTrigger>
            <TooltipContent side="bottom">
              <span className="text-xs">Search conversations</span>
            </TooltipContent>
          </Tooltip>
          </>}
        </TooltipProvider>
      </SupportInboxPanelHeader>
      {selectionActive && hasNextPage && !selection.allMatches && (
        <div className="border-b border-border/60 px-3 py-1.5 text-xs">
          {selection.loading ? <span role="status" className="text-muted-foreground">Selecting conversations… {selection.loadedCount}</span> : (
            <button
              type="button"
              className="text-foreground underline underline-offset-2 disabled:opacity-50"
              disabled={bulkActionPending}
              onClick={() => void selection.selectAllMatches(workspaceId, filters, filterConversations)}
            >
              Select all{totalMatches ? ` ${totalMatches.toLocaleString()}` : ''} matching conversations
            </button>
          )}
        </div>
      )}
      <Dialog open={saveViewOpen} onOpenChange={setSaveViewOpen}>
        <DialogContent aria-describedby={undefined}>
          <DialogHeader>
            <DialogTitle>Save new view</DialogTitle>
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

      {/* Keep the scroll offset when activity sorting moves a row to the top.
          Browser scroll anchoring can otherwise follow that row and jump the list. */}
      <div
        className="flex-1 min-h-0 overflow-y-auto pb-16 [overflow-anchor:none]"
        data-support-conversation-scroll
        onScroll={handleListScroll}
      >
        {shouldShowListSkeleton && (
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
            isTransitioningOut={handoffConversationId === conversation.id}
            onToggleSelection={handleToggleSelection}
            isBulkSelected={selection.items.has(conversation.id)}
            selectionActive={selectionActive}
            selectionDisabled={bulkActionPending || selection.loading}
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
