import { ArrowLeft } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useAuthStore } from '@/stores/authStore';
import { useSupportInboxStore } from '@/stores/supportInboxStore';
import { ConversationList } from './ConversationList';
import { MessageThread } from './MessageThread';
import { ConversationDetailSidebar } from './ConversationDetailSidebar';
import { CreateConversationDialog } from './CreateConversationDialog';

export function SupportInboxLayout() {
  const workspace = useWorkspaceStore((s) => s.currentWorkspace);
  const user = useAuthStore((s) => s.user);
  const workspaceId = workspace?.id ?? '';
  const { selectedConversationId, activePanel, setActivePanel, selectConversation, createDialogOpen, setCreateDialogOpen } = useSupportInboxStore();

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
            <ArrowLeft className="h-4 w-4" />
            Back
          </Button>
        </div>
      )}

      {/* Three-panel layout */}
      <div className="flex min-h-0 flex-1 overflow-hidden">
        {/* Panel 1: Conversation list */}
        <div className={`${
          selectedConversationId ? 'hidden md:flex' : 'flex'
        } w-full md:w-auto`}>
          <ConversationList workspaceId={workspaceId} userId={user?.id} />
        </div>

        {/* Panel 2: Message thread */}
        <div className={`${
          !selectedConversationId ? 'hidden md:flex' : 'flex'
        } min-w-0 flex-1`}>
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
    </div>
  );
}
