import { useEffect } from 'react';
import { ArrowLeft02Icon } from '@/lib/icons';
import { useNavigate, useParams } from '@tanstack/react-router';
import { Button } from '@/components/ui/button';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useAuthStore } from '@/stores/authStore';
import { useSupportInboxStore } from '@/stores/supportInboxStore';
import { ConversationList } from './ConversationList';
import { MessageThread } from './MessageThread';
import { ConversationDetailSidebar } from './ConversationDetailSidebar';
import { CreateConversationDialog } from './CreateConversationDialog';
import { TeamInboxDialog } from './TeamInboxDialog';

export function SupportInboxLayout() {
  const workspace = useWorkspaceStore((s) => s.currentWorkspace);
  const user = useAuthStore((s) => s.user);
  const workspaceId = workspace?.id ?? '';
  const slug = workspace?.slug ?? '';
  const {
    selectedConversationId,
    activePanel,
    setActivePanel,
    selectConversation,
    createDialogOpen,
    setCreateDialogOpen,
    teamInboxDialogOpen,
    setTeamInboxDialogOpen,
  } = useSupportInboxStore();
  const navigate = useNavigate();
  const params = useParams({ strict: false }) as { conversationId?: string };

  // Sync URL param → store on mount / URL change
  useEffect(() => {
    if (params.conversationId && params.conversationId !== selectedConversationId) {
      selectConversation(params.conversationId);
    }
  }, [params.conversationId]);

  // Sync store → URL when conversation selection changes
  useEffect(() => {
    if (!slug) return;
    const urlConvId = params.conversationId || null;
    if (selectedConversationId && selectedConversationId !== urlConvId) {
      navigate({ to: '/w/$slug/support/$conversationId', params: { slug, conversationId: selectedConversationId }, replace: true });
    } else if (!selectedConversationId && urlConvId) {
      navigate({ to: '/w/$slug/support', params: { slug }, replace: true });
    }
  }, [selectedConversationId, slug]);

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
          <ConversationList workspaceId={workspaceId} userId={user?.id} />
        </div>

        {/* Panel 2: Message thread */}
        <div className={`${
          !selectedConversationId ? 'hidden md:flex' : 'flex'
        } min-w-0 min-h-0 flex-1`}>
          <MessageThread workspaceId={workspaceId} conversationId={selectedConversationId} />
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
      />
    </div>
  );
}
