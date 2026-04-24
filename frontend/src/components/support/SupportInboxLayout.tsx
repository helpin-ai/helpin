import { useCallback, useEffect, useMemo, useState } from 'react';
import { ArrowLeft02Icon } from '@/lib/icons';
import { useLocation, useNavigate, useParams } from '@tanstack/react-router';
import { Button } from '@/components/ui/button';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useAuthStore } from '@/stores/authStore';
import { useSupportInboxStore } from '@/stores/supportInboxStore';
import { useSupportMailboxes } from '@/hooks/queries/useSupport';
import { ConversationList } from './ConversationList';
import { MessageThread } from './MessageThread';
import { ConversationDetailSidebar } from './ConversationDetailSidebar';
import { CreateConversationDialog } from './CreateConversationDialog';
import { TeamInboxDialog } from './TeamInboxDialog';
import { buildSupportInboxSearch, navFilterFromView, normalizeSupportInboxRouteSearch } from '@/lib/supportInboxRouting';

export function SupportInboxLayout() {
  const workspace = useWorkspaceStore((s) => s.currentWorkspace);
  const user = useAuthStore((s) => s.user);
  const workspaceId = workspace?.id ?? '';
  const slug = workspace?.slug ?? '';
  const {
    navFilter,
    statusFilter,
    searchQuery,
    selectedMailboxId,
    selectedConversationId,
    activePanel,
    setActivePanel,
    setNavFilter,
    setSelectedMailboxId,
    setStatusFilter,
    setSearchQuery,
    selectConversation,
    createDialogOpen,
    setCreateDialogOpen,
    teamInboxDialogOpen,
    setTeamInboxDialogOpen,
    editMailboxId,
  } = useSupportInboxStore();
  const { data: mailboxes = [] } = useSupportMailboxes(workspaceId);
  const editMailbox = editMailboxId ? mailboxes.find((m) => m.id === editMailboxId) ?? null : null;
  const [showInboxOnboarding, setShowInboxOnboarding] = useState(false);
  const navigate = useNavigate();
  const location = useLocation();
  const params = useParams({ strict: false }) as { conversationId?: string };
  const routeConversationId = params.conversationId ?? null;
  const handleWidgetSettingsClick = useCallback(() => {
    if (!slug) return;
    void navigate({ to: '/w/$slug/settings/chat-general', params: { slug } });
  }, [navigate, slug]);

  const supportRouteSearch = useMemo(
    () => buildSupportInboxSearch({
      navFilter,
      selectedMailboxId,
      statusFilter,
      searchQuery,
    }),
    [navFilter, searchQuery, selectedMailboxId, statusFilter],
  );

  // Sync URL params → store on mount / URL change.
  useEffect(() => {
    const routeSearch = normalizeSupportInboxRouteSearch(location.search as Record<string, unknown>);
    const nextNavFilter = navFilterFromView(routeSearch.view);
    const nextMailboxId = routeSearch.inbox || 'all';
    const defaultStatusFilter = nextNavFilter === 'my_inbox' || nextNavFilter === 'unassigned' || nextNavFilter === 'mentions'
      ? 'open'
      : 'all';
    const nextStatusFilter = routeSearch.status || defaultStatusFilter;
    const nextSearchQuery = routeSearch.q || '';

    if (routeConversationId === 'inbox') {
      if (routeSearch.conversation) {
        void navigate({
          to: '/w/$slug/support/$conversationId',
          params: { slug, conversationId: routeSearch.conversation },
          search: buildSupportInboxSearch({
            navFilter: nextNavFilter,
            selectedMailboxId: nextMailboxId,
            statusFilter: nextStatusFilter,
            searchQuery: nextSearchQuery,
          }),
          replace: true,
        });
      } else {
        void navigate({
          to: '/w/$slug/support',
          params: { slug },
          search: buildSupportInboxSearch({
            navFilter: nextNavFilter,
            selectedMailboxId: nextMailboxId,
            statusFilter: nextStatusFilter,
            searchQuery: nextSearchQuery,
          }),
          replace: true,
        });
      }
      return;
    }

    const currentState = useSupportInboxStore.getState();
    if (nextNavFilter !== currentState.navFilter) {
      setNavFilter(nextNavFilter);
    }
    if (nextMailboxId !== useSupportInboxStore.getState().selectedMailboxId) {
      setSelectedMailboxId(nextMailboxId);
    }
    if (nextStatusFilter !== useSupportInboxStore.getState().statusFilter) {
      setStatusFilter(nextStatusFilter);
    }
    if (nextSearchQuery !== useSupportInboxStore.getState().searchQuery) {
      setSearchQuery(nextSearchQuery);
    }

    const nextConversationId = routeConversationId;
    if (nextConversationId !== useSupportInboxStore.getState().selectedConversationId) {
      selectConversation(nextConversationId);
    }
  }, [
    location.search,
    navigate,
    selectConversation,
    setNavFilter,
    setSearchQuery,
    setSelectedMailboxId,
    setStatusFilter,
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

      <CreateConversationDialog
        workspaceId={workspaceId}
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
