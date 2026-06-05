import { useCallback, useEffect, useMemo, useState } from 'react';
import { ArrowLeft02Icon } from '@/lib/icons';
import { useLocation, useNavigate, useParams } from '@tanstack/react-router';
import { Button } from '@/components/ui/button';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useAuthStore } from '@/stores/authStore';
import { usePermissions, useWorkspaceAccess } from '@/hooks/queries';
import { supportInboxBuiltinViewKey, useSupportInboxStore } from '@/stores/supportInboxStore';
import { useSupportInboxViews, useSupportMailboxes } from '@/hooks/queries/useSupport';
import { ConversationList } from './ConversationList';
import { MessageThread } from './MessageThread';
import { ConversationDetailSidebar } from './ConversationDetailSidebar';
import { NewConversationDialog } from './NewConversationDialog';
import { TeamInboxDialog } from './TeamInboxDialog';
import { buildSupportInboxSearch, navFilterFromView, normalizeSupportInboxRouteSearch } from '@/lib/supportInboxRouting';
import { conversationListFiltersEqual, defaultAIStatesForNav, defaultAssignmentForNav, defaultConversationListFiltersForNav, defaultStatesForNav, parseSupportInboxViewFilters, statesEqual, stringArraysEqual, type ConversationAIStateFilter, type ConversationAssignmentFilter, type ConversationListFilters, type ConversationStateFilter } from '@/lib/supportInboxFilters';

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
    activePanel,
    setActivePanel,
    syncRouteState,
    selectConversation,
    createDialogOpen,
    setCreateDialogOpen,
    teamInboxDialogOpen,
    setTeamInboxDialogOpen,
    editMailboxId,
  } = useSupportInboxStore();
  const { data: mailboxes = [] } = useSupportMailboxes(workspaceId);
  const { data: access } = useWorkspaceAccess(workspaceId);
  const { isAdmin } = usePermissions(access);
  const editMailbox = editMailboxId ? mailboxes.find((m) => m.id === editMailboxId) ?? null : null;
  const [showInboxOnboarding, setShowInboxOnboarding] = useState(false);
  const navigate = useNavigate();
  const location = useLocation();
  const params = useParams({ strict: false }) as { conversationId?: string };
  const routeConversationId = params.conversationId ?? null;
  const routeSearch = useMemo(
    () => normalizeSupportInboxRouteSearch(location.search as Record<string, unknown>),
    [location.search],
  );
  const { data: routeCustomViews } = useSupportInboxViews(workspaceId, !!routeSearch.custom_view);
  const handleWidgetSettingsClick = useCallback(() => {
    if (!slug) return;
    void navigate({ to: '/w/$slug/settings/chat-general', params: { slug } });
  }, [navigate, slug]);

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

  // Sync store → URL when conversation selection changes
  useEffect(() => {
    if (!slug) return;
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
  }, [routeConversationId, selectedConversationId, slug, supportRouteSearch, navigate]);

  if (!workspace) {
    return <p className="p-4 text-sm text-muted-foreground">Workspace not found.</p>;
  }

  return (
    <div className="flex h-full flex-col">
      {/* Mobile back button */}
      {selectedConversationId && (activePanel === 'thread' || activePanel === 'detail') && (
        <div className="flex items-center border-b px-3 py-1.5 md:hidden">
          <Button
            variant="ghost"
            size="sm"
            className="h-8 gap-1.5 px-2 text-sm"
            onClick={() => {
              selectConversation(null);
              setActivePanel('list');
              navigate({ to: '/w/$slug/support', params: { slug }, search: supportRouteSearch, replace: true });
            }}
          >
            <ArrowLeft02Icon className="h-4 w-4" />
            Back
          </Button>
        </div>
      )}

      {/* Three-panel layout */}
      <div className="flex min-h-0 flex-1 overflow-hidden">
        {/* Panel 1: Conversation list */}
        <div className={`${
          selectedConversationId ? 'hidden md:flex' : 'flex'
        } w-full md:w-auto min-h-0`}>
          <ConversationList
            workspaceId={workspaceId}
            userId={user?.id}
            onOnboardingEmptyChange={setShowInboxOnboarding}
            onWidgetSettingsClick={handleWidgetSettingsClick}
            onCreateConversationClick={() => setCreateDialogOpen(true)}
            canCreateSharedViews={isAdmin}
          />
        </div>

        {/* Panel 2: Message thread */}
        <div className={`${
          !selectedConversationId ? 'hidden md:flex' : 'flex'
        } min-w-0 min-h-0 flex-1`}>
          <MessageThread
            workspaceId={workspaceId}
            conversationId={selectedConversationId}
            showInboxOnboarding={showInboxOnboarding}
            onWidgetSettingsClick={handleWidgetSettingsClick}
            onCreateConversationClick={() => setCreateDialogOpen(true)}
          />
        </div>

        {/* Panel 3: Detail sidebar - hidden on mobile & tablet */}
        <div className={`hidden ${selectedConversationId ? 'xl:flex' : ''}`}>
          <ConversationDetailSidebar workspaceId={workspaceId} conversationId={selectedConversationId} />
        </div>
      </div>

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
