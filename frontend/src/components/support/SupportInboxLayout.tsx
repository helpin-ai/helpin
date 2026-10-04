import { useNotificationView } from '@/hooks/useNotificationView';
import { lazy, Suspense, useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useLocation, useNavigate, useParams } from '@tanstack/react-router';
import { Button } from '@/components/ui/button';
import { Sheet, SheetContent, SheetDescription, SheetTitle } from '@/components/ui/sheet';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useAuthStore } from '@/stores/authStore';
import { usePermissions, useWorkspaceAccess } from '@/hooks/queries';
import { useMinWidth } from '@/hooks/use-min-width';
import { supportInboxBuiltinViewKey, useSupportInboxStore } from '@/stores/supportInboxStore';
import { useConversation, useSupportInboxViews, useSupportMailboxes, useSupportRoutingUsage } from '@/hooks/queries/useSupport';
import { ConversationList, type SupportInboxEmptyState } from './ConversationList';
import { MessageThread } from './MessageThread';
import { buildSupportConversationPageContext } from './supportAgentContext';
import { supportSidebarWidthClass } from './supportSidebarLayout';
import { shouldClearConversationForMailbox } from './supportInboxSelection';
import { NewConversationDialog } from './NewConversationDialog';
import { TeamInboxDialog } from './TeamInboxDialog';
import { buildSupportInboxSearch, navFilterFromView, normalizeSupportInboxRouteSearch } from '@/lib/supportInboxRouting';
import { conversationListFiltersEqual, defaultAIStatesForNav, defaultAssignmentForNav, defaultConversationListFiltersForNav, defaultStatesForNav, parseSupportInboxViewFilters, statesEqual, stringArraysEqual, type ConversationAIStateFilter, type ConversationAssignmentFilter, type ConversationListFilters, type ConversationStateFilter } from '@/lib/supportInboxFilters';

const LazySupportAgentSidebar = lazy(() => import('./SupportAgentSidebar').then((module) => ({ default: module.SupportAgentSidebar })));
const LazyConversationDetailSidebar = lazy(() => import('./ConversationDetailSidebar').then((module) => ({ default: module.ConversationDetailSidebar })));

function parseRouteStates(value: string | undefined, navFilter: ReturnType<typeof navFilterFromView>): ConversationStateFilter[] {
  if (!value) return defaultStatesForNav(navFilter);
  const states = value.split(',').filter((state): state is ConversationStateFilter =>
    state === 'open' ||
    state === 'waiting_on_customer' ||
    state === 'resolved' ||
    state === 'spam'
  );
  return states.length > 0 ? states : defaultStatesForNav(navFilter);
}

function parseRouteStringList(value: string | undefined): string[] {
  if (!value) return [];
  return value.split(',').map((entry) => entry.trim()).filter(Boolean);
}

function parseRouteAssignments(value: string | undefined, navFilter: ReturnType<typeof navFilterFromView>): ConversationAssignmentFilter[] {
  if (!value) return defaultAssignmentForNav(navFilter);
  if (value === 'none') return [];
  return parseRouteStringList(value).filter((entry): entry is ConversationAssignmentFilter =>
    entry === 'me' ||
    entry === 'mentioned_me' ||
    entry === 'opened_by_me' ||
    entry === 'unassigned' ||
    entry === 'others'
  );
}

function parseRouteAIStates(value: string | undefined, legacySystemTags: string | undefined, navFilter: ReturnType<typeof navFilterFromView>): ConversationAIStateFilter[] {
  if (!value && !legacySystemTags) return defaultAIStatesForNav(navFilter);
  const values = [...parseRouteStringList(value), ...parseRouteStringList(legacySystemTags)];
  const states: ConversationAIStateFilter[] = [];
  for (const item of values) {
    const mapped =
      item === 'ai_handoff' || item === 'needs_human'
        ? 'handoff'
        : item === 'ai_resolved' || item === 'resolved_by_ai'
          ? 'resolved'
          : item === 'ai_handling' || item === 'ai-active' || item === 'ai_active'
            ? 'handling'
            : item;
    if (
      (mapped === 'handling' || mapped === 'handoff' || mapped === 'resolved') &&
      !states.includes(mapped)
    ) {
      states.push(mapped);
    }
  }
  return states;
}

export function SupportInboxLayout() {
  const workspace = useWorkspaceStore((s) => s.currentWorkspace);
  const user = useAuthStore((s) => s.user);
  const workspaceId = workspace?.id ?? '';
  const slug = workspace?.slug ?? '';
  const {
    navFilter,
    statusFilter,
    searchQuery,
    conversationListFilters,
    activeCustomViewId,
    customViewDirty,
    builtinViewFilters,
    selectedMailboxId,
    selectedConversationId,
    syncRouteState,
    selectConversation,
    createDialogOpen,
    setCreateDialogOpen,
    teamInboxDialogOpen,
    setTeamInboxDialogOpen,
    editMailboxId,
    detailSidebarMode,
    detailSidebarCollapsed,
    setDetailSidebarMode,
  } = useSupportInboxStore();
  const { data: mailboxes = [] } = useSupportMailboxes(workspaceId);
  const { data: routingUsage } = useSupportRoutingUsage(workspaceId);
  const { data: selectedConversation } = useConversation(workspaceId, selectedConversationId);
  const supportAgentContext = useMemo(
    () => buildSupportConversationPageContext(selectedConversation ?? (selectedConversationId ? { id: selectedConversationId } : null)),
    [selectedConversation, selectedConversationId],
  );
  const { data: access } = useWorkspaceAccess(workspaceId);
  const { isAdmin } = usePermissions(access);
  const editMailbox = editMailboxId ? mailboxes.find((m) => m.id === editMailboxId) ?? null : null;
  const [inboxEmptyState, setInboxEmptyState] = useState<SupportInboxEmptyState>(null);
  const navigate = useNavigate();
  const location = useLocation();
  const params = useParams({ strict: false }) as { conversationId?: string };
  const routeConversationId = params.conversationId ?? null;
  const isMultiPanel = useMinWidth(768);
  useNotificationView(workspaceId, 'support_conversation', isMultiPanel ? selectedConversationId : routeConversationId);
  const showDetailSidebar = useMinWidth(1280) && !!selectedConversationId;
  const [mobileDetailsOpen, setMobileDetailsOpen] = useState(false);
  const mobileOpenedFromInboxRef = useRef(false);
  const previousRouteConversationIdRef = useRef(routeConversationId);
  const routeSearch = useMemo(
    () => normalizeSupportInboxRouteSearch(location.search as Record<string, unknown>),
    [location.search],
  );
  const { data: routeCustomViews } = useSupportInboxViews(workspaceId, !!routeSearch.custom_view);
  const handleWidgetSettingsClick = useCallback(() => {
    if (!slug) return;
    void navigate({ to: '/w/$slug/settings/chat-general', params: { slug } });
  }, [navigate, slug]);
  const handleRoutingSettingsClick = useCallback(() => {
    if (!slug) return;
    void navigate({ to: '/w/$slug/settings/inboxes-routing', params: { slug }, search: { tab: 'routing', create_inbox: false } });
  }, [navigate, slug]);
  const handleSupportSearchClick = useCallback(() => {
    if (!slug) return;
    if (typeof window !== 'undefined') {
      const returnTo = `${window.location.pathname}${window.location.search}`;
      const supportPath = `/w/${slug}/support`;
      if (returnTo.startsWith(supportPath) && !returnTo.startsWith(`${supportPath}/search`)) {
        window.sessionStorage.setItem(`support-search-return:${slug}`, returnTo);
      }
    }
    void navigate({ to: '/w/$slug/support/search', params: { slug }, search: {} as never });
  }, [navigate, slug]);
  const handleCloseAgentSidebar = useCallback(() => {
    setDetailSidebarMode('details');
    window.setTimeout(() => {
      document.querySelector<HTMLButtonElement>('[aria-label="Ask agents about this conversation"]')?.focus();
    }, 0);
  }, [setDetailSidebarMode]);

  const supportRouteSearch = useMemo(
    () => {
      const savedBuiltinFilters = activeCustomViewId
        ? null
        : builtinViewFilters[supportInboxBuiltinViewKey(navFilter, selectedMailboxId)];
      const savedBuiltinState = savedBuiltinFilters
        ? parseSupportInboxViewFilters(savedBuiltinFilters, navFilter)
        : null;
      const matchesSavedBuiltinView = !activeCustomViewId &&
        statusFilter === 'all' &&
        searchQuery.trim() === (savedBuiltinState?.searchQuery ?? '') &&
        conversationListFiltersEqual(
          conversationListFilters,
          savedBuiltinState?.listFilters ?? defaultConversationListFiltersForNav(navFilter),
        );
      return buildSupportInboxSearch({
        navFilter,
        selectedMailboxId,
        statusFilter,
        searchQuery,
        activeCustomViewId,
        listFilters: conversationListFilters,
        includeFilterParams: activeCustomViewId ? customViewDirty : !matchesSavedBuiltinView,
      });
    },
    [activeCustomViewId, builtinViewFilters, conversationListFilters, customViewDirty, navFilter, searchQuery, selectedMailboxId, statusFilter],
  );

  const handleOpenMobileConversation = useCallback((conversationId: string) => {
    if (!slug) return;
    mobileOpenedFromInboxRef.current = true;
    setMobileDetailsOpen(false);
    void navigate({
      to: '/w/$slug/support/$conversationId',
      params: { slug, conversationId },
      search: supportRouteSearch,
    });
  }, [navigate, slug, supportRouteSearch]);

  const handleCloseMobileConversation = useCallback(() => {
    if (!slug) return;
    setMobileDetailsOpen(false);
    selectConversation(null);

    if (mobileOpenedFromInboxRef.current && typeof window !== 'undefined') {
      mobileOpenedFromInboxRef.current = false;
      window.history.back();
      return;
    }

    void navigate({
      to: '/w/$slug/support',
      params: { slug },
      search: supportRouteSearch,
      replace: true,
    });
  }, [navigate, selectConversation, slug, supportRouteSearch]);

  // Sync URL params → store on mount / URL change.
  useEffect(() => {
    const nextNavFilter = navFilterFromView(routeSearch.view);
    let nextMailboxId = routeSearch.team_inbox || routeSearch.inbox || 'all';
    let effectiveNavFilter = nextNavFilter;
    if (!routeSearch.view) {
      if (routeSearch.status === 'waiting_on_customer') effectiveNavFilter = 'waiting';
      if (routeSearch.status === 'resolved') effectiveNavFilter = 'resolved';
      if (routeSearch.status === 'spam') effectiveNavFilter = 'spam';
    }
    const nextStatusFilter = routeSearch.status || 'all';
    let nextSearchQuery = routeSearch.q || '';
    let nextConversationListFilters: ConversationListFilters = {
      states: parseRouteStates(routeSearch.states, effectiveNavFilter),
      assignment: parseRouteAssignments(routeSearch.assigned_to, effectiveNavFilter),
      mailboxIds: parseRouteStringList(routeSearch.mailbox_ids),
      tagIds: parseRouteStringList(routeSearch.tag_ids),
      aiStates: parseRouteAIStates(routeSearch.ai, routeSearch.system_tags, effectiveNavFilter),
      sort: routeSearch.sort === 'oldest' ? 'oldest' : 'newest',
    };
    const hasExplicitRouteFilters = !!(
      routeSearch.status ||
      routeSearch.q ||
      routeSearch.states ||
      routeSearch.assigned_to ||
      routeSearch.mailbox_ids ||
      routeSearch.tag_ids ||
      routeSearch.ai ||
      routeSearch.system_tags ||
      routeSearch.sort
    );
    if (routeSearch.custom_view && !hasExplicitRouteFilters) {
      const customView = (routeCustomViews ?? []).find((view) => view.id === routeSearch.custom_view);
      if (customView) {
        const savedState = parseSupportInboxViewFilters(customView.filters, effectiveNavFilter);
        effectiveNavFilter = savedState.navFilter;
        nextMailboxId = savedState.selectedMailboxId;
        nextSearchQuery = savedState.searchQuery;
        nextConversationListFilters = savedState.listFilters;
      }
    } else if (!routeSearch.custom_view && !hasExplicitRouteFilters) {
      const savedFilters = builtinViewFilters[supportInboxBuiltinViewKey(effectiveNavFilter, nextMailboxId)];
      if (savedFilters) {
        const savedState = parseSupportInboxViewFilters(savedFilters, effectiveNavFilter);
        nextSearchQuery = savedState.searchQuery;
        nextConversationListFilters = savedState.listFilters;
      }
    }

    if (routeConversationId === 'inbox') {
      if (routeSearch.conversation) {
        void navigate({
          to: '/w/$slug/support/$conversationId',
          params: { slug, conversationId: routeSearch.conversation },
          search: buildSupportInboxSearch({
            navFilter: effectiveNavFilter,
            selectedMailboxId: nextMailboxId,
            statusFilter: nextStatusFilter,
            searchQuery: nextSearchQuery,
            activeCustomViewId: routeSearch.custom_view,
            listFilters: nextConversationListFilters,
          }),
          replace: true,
        });
      } else {
        void navigate({
          to: '/w/$slug/support',
          params: { slug },
          search: buildSupportInboxSearch({
            navFilter: effectiveNavFilter,
            selectedMailboxId: nextMailboxId,
            statusFilter: nextStatusFilter,
            searchQuery: nextSearchQuery,
            activeCustomViewId: routeSearch.custom_view,
            listFilters: nextConversationListFilters,
          }),
          replace: true,
        });
      }
      return;
    }

    const currentState = useSupportInboxStore.getState();
    if (
      effectiveNavFilter !== currentState.navFilter ||
      nextMailboxId !== currentState.selectedMailboxId ||
      nextStatusFilter !== currentState.statusFilter ||
      nextSearchQuery !== currentState.searchQuery ||
      (routeSearch.custom_view ?? null) !== currentState.activeCustomViewId ||
      !statesEqual(nextConversationListFilters.states, currentState.conversationListFilters.states) ||
      !stringArraysEqual(nextConversationListFilters.assignment, currentState.conversationListFilters.assignment) ||
      !stringArraysEqual(nextConversationListFilters.mailboxIds, currentState.conversationListFilters.mailboxIds) ||
      !stringArraysEqual(nextConversationListFilters.tagIds, currentState.conversationListFilters.tagIds) ||
      !stringArraysEqual(nextConversationListFilters.aiStates, currentState.conversationListFilters.aiStates) ||
      nextConversationListFilters.sort !== currentState.conversationListFilters.sort
    ) {
      syncRouteState({
        navFilter: effectiveNavFilter,
        selectedMailboxId: nextMailboxId,
        statusFilter: nextStatusFilter,
        searchQuery: nextSearchQuery,
        activeCustomViewId: routeSearch.custom_view,
        conversationListFilters: nextConversationListFilters,
      });
    }

    const nextConversationId = routeConversationId;
    if (nextConversationId !== useSupportInboxStore.getState().selectedConversationId) {
      selectConversation(nextConversationId);
    }
  }, [
    routeSearch,
    routeCustomViews,
    builtinViewFilters,
    navigate,
    selectConversation,
    syncRouteState,
    slug,
    routeConversationId,
  ]);

  useEffect(() => {
    const previousConversationId = previousRouteConversationIdRef.current;
    previousRouteConversationIdRef.current = routeConversationId;
    if (isMultiPanel || !previousConversationId || routeConversationId) return;

    setMobileDetailsOpen(false);
    mobileOpenedFromInboxRef.current = false;
    const frame = window.requestAnimationFrame(() => {
      Array.from(document.querySelectorAll<HTMLElement>('[data-conversation-id]'))
        .find((element) => element.dataset.conversationId === previousConversationId)
        ?.focus();
    });
    return () => window.cancelAnimationFrame(frame);
  }, [isMultiPanel, routeConversationId]);

  // Sync store → URL when conversation selection changes
  useEffect(() => {
    if (!slug || !isMultiPanel) return;
    if (selectedConversationId && selectedConversationId !== routeConversationId) {
      let timeout = 0;
      const frame = window.requestAnimationFrame(() => {
        timeout = window.setTimeout(() => {
          const currentSelectedConversationId = useSupportInboxStore.getState().selectedConversationId;
          if (!currentSelectedConversationId || currentSelectedConversationId === routeConversationId) {
            return;
          }

          navigate({
            to: '/w/$slug/support/$conversationId',
            params: { slug, conversationId: currentSelectedConversationId },
            search: supportRouteSearch,
            replace: true,
          });
        }, 0);
      });
      return () => {
        window.cancelAnimationFrame(frame);
        if (timeout) window.clearTimeout(timeout);
      };
    }
  }, [isMultiPanel, routeConversationId, selectedConversationId, slug, supportRouteSearch, navigate]);

  // A team inbox can become empty after its final conversation is moved or
  // resolved. Do not leave a thread from a different mailbox selected beside
  // that empty list.
  useEffect(() => {
    if (!selectedConversationId || !selectedConversation) return;
    if (!shouldClearConversationForMailbox(selectedMailboxId, selectedConversation.mailbox_id)) return;

    selectConversation(null);
    void navigate({
      to: '/w/$slug/support',
      params: { slug },
      search: supportRouteSearch,
      replace: true,
    });
  }, [navigate, selectConversation, selectedConversation, selectedConversationId, selectedMailboxId, slug, supportRouteSearch]);

  if (!workspace) {
    return <p className="p-4 text-sm text-muted-foreground">Workspace not found.</p>;
  }

  return (
    <div className="flex h-full flex-col">
      {routingUsage?.triage_enabled && routingUsage.exhausted ? (
        <div className="flex flex-col gap-2 border-b border-amber-200 bg-amber-50 px-4 py-2 text-sm text-amber-900 sm:flex-row sm:items-center sm:justify-between dark:border-amber-900/60 dark:bg-amber-950/30 dark:text-amber-200">
          <span>AI routing limit reached for today. Manual rules still run; AI routing resumes after the daily reset.</span>
          <Button
            type="button"
            variant="outline"
            size="sm"
            className="h-7 w-fit border-amber-300 bg-amber-50 text-amber-950 hover:bg-amber-100 dark:border-amber-800 dark:bg-amber-950/20 dark:text-amber-100 dark:hover:bg-amber-900/40"
            onClick={handleRoutingSettingsClick}
          >
            Manage routing
          </Button>
        </div>
      ) : null}

      {/* Three-panel layout */}
      <div className="flex min-h-0 flex-1 overflow-hidden">
        {/* Panel 1: Conversation list */}
        <div className="flex min-h-0 w-full md:w-auto">
          <ConversationList
            workspaceId={workspaceId}
            userId={user?.id}
            autoSelectFirst={isMultiPanel}
            onConversationOpen={isMultiPanel ? undefined : handleOpenMobileConversation}
            onInboxEmptyStateChange={setInboxEmptyState}
            onWidgetSettingsClick={handleWidgetSettingsClick}
            onCreateConversationClick={() => setCreateDialogOpen(true)}
            onSearchClick={handleSupportSearchClick}
            onViewCreated={(view) => {
              useSupportInboxStore.getState().applyCustomView(view);
              const next = useSupportInboxStore.getState();
              void navigate({
                to: '/w/$slug/support',
                params: { slug },
                search: buildSupportInboxSearch({
                  navFilter: next.navFilter,
                  selectedMailboxId: next.selectedMailboxId,
                  statusFilter: next.statusFilter,
                  searchQuery: next.searchQuery,
                  activeCustomViewId: view.id,
                  listFilters: next.conversationListFilters,
                  includeFilterParams: false,
                }),
              });
            }}
            canCreateSharedViews={isAdmin}
          />
        </div>

        {/* Panel 2: Message thread */}
        <div className="hidden min-h-0 min-w-0 flex-1 md:flex">
          <MessageThread
            workspaceId={workspaceId}
            conversationId={selectedConversationId}
            inboxEmptyState={inboxEmptyState}
            onWidgetSettingsClick={handleWidgetSettingsClick}
            onCreateConversationClick={() => setCreateDialogOpen(true)}
          />
        </div>

        {/* Panel 3: Detail sidebar - hidden on mobile & tablet */}
        {showDetailSidebar ? <div
          className={`relative hidden shrink-0 overflow-hidden border-l border-border/70 transition-[width,flex-basis] duration-200 ease-out motion-reduce:transition-none ${
            selectedConversationId ? 'xl:flex' : ''
          } ${supportSidebarWidthClass(detailSidebarMode, detailSidebarCollapsed)}`}
        >
          <div
            aria-hidden={detailSidebarMode === 'agents' || undefined}
            inert={detailSidebarMode === 'agents'}
            className={`absolute inset-0 transition-[opacity,transform] duration-200 ease-out motion-reduce:transition-none ${
              detailSidebarMode === 'agents' ? 'pointer-events-none -translate-x-2 opacity-0' : 'translate-x-0 opacity-100'
            }`}
          >
            {detailSidebarMode !== 'agents' ? (
              <Suspense fallback={null}>
                <LazyConversationDetailSidebar workspaceId={workspaceId} conversationId={selectedConversationId} />
              </Suspense>
            ) : null}
          </div>
          {supportAgentContext && detailSidebarMode === 'agents' ? (
            <div
              aria-hidden={detailSidebarMode !== 'agents' || undefined}
              inert={detailSidebarMode !== 'agents'}
              className={`absolute inset-0 transition-[opacity,transform] duration-200 ease-out motion-reduce:transition-none ${
                detailSidebarMode === 'agents' ? 'translate-x-0 opacity-100' : 'pointer-events-none translate-x-2 opacity-0'
              }`}
            >
              <Suspense fallback={null}>
                <LazySupportAgentSidebar
                  context={supportAgentContext}
                  active={detailSidebarMode === 'agents'}
                  onBack={handleCloseAgentSidebar}
                />
              </Suspense>
            </div>
          ) : null}
        </div> : null}
      </div>

      <Sheet
        open={!isMultiPanel && !!routeConversationId}
        onOpenChange={(open) => {
          if (!open && routeConversationId) {
            handleCloseMobileConversation();
          }
        }}
      >
        <SheetContent
          side="right"
          showCloseButton={false}
          className="gap-0 border-0 p-0 shadow-none data-[side=right]:!w-full data-[side=right]:!max-w-none"
        >
          <SheetTitle className="sr-only">Conversation</SheetTitle>
          <SheetDescription className="sr-only">
            Read and reply to the selected support conversation.
          </SheetDescription>
          <MessageThread
            workspaceId={workspaceId}
            conversationId={routeConversationId}
            presentation="mobile-sheet"
            onBackToInbox={handleCloseMobileConversation}
            onOpenDetails={() => setMobileDetailsOpen(true)}
            onCreateConversationClick={() => setCreateDialogOpen(true)}
          />
        </SheetContent>
      </Sheet>

      <Sheet open={!isMultiPanel && !!routeConversationId && mobileDetailsOpen} onOpenChange={setMobileDetailsOpen}>
        <SheetContent
          side="right"
          showCloseButton={false}
          className="gap-0 border-0 p-0 shadow-none data-[side=right]:!w-full data-[side=right]:!max-w-none"
        >
          <SheetTitle className="sr-only">Conversation details</SheetTitle>
          <SheetDescription className="sr-only">
            Customer identity, assignment, tags, and conversation context.
          </SheetDescription>
          <Suspense fallback={null}>
            <LazyConversationDetailSidebar
              workspaceId={workspaceId}
              conversationId={routeConversationId}
              presentation="mobile-sheet"
              onClose={() => setMobileDetailsOpen(false)}
            />
          </Suspense>
        </SheetContent>
      </Sheet>

      <NewConversationDialog
        workspaceId={workspaceId}
        workspaceSlug={slug}
        open={createDialogOpen}
        onOpenChange={setCreateDialogOpen}
      />
      <TeamInboxDialog
        workspaceId={workspaceId}
        open={teamInboxDialogOpen}
        onOpenChange={setTeamInboxDialogOpen}
        mailbox={editMailbox}
      />
    </div>
  );
}
