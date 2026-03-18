import { Link } from '@tanstack/react-router';
import { ChevronLeft, ChevronRight, Mail, User, Clock } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { SidebarAssociations } from './SidebarAssociations';
import { useConversation } from '@/hooks/queries/useSupport';
import { useSupportInboxStore } from '@/stores/supportInboxStore';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { getInitial, getAvatarColor, formatTimestamp } from './helpers';

interface ConversationDetailSidebarProps {
  workspaceId: string;
  conversationId: string | null;
}

export function ConversationDetailSidebar({ workspaceId, conversationId }: ConversationDetailSidebarProps) {
  const { detailSidebarCollapsed, toggleDetailSidebar } = useSupportInboxStore();
  const workspace = useWorkspaceStore((s) => s.currentWorkspace);
  const { data: conversation } = useConversation(workspaceId, conversationId);
  if (detailSidebarCollapsed) {
    return (
      <div className="flex w-10 flex-col items-center border-l bg-muted/30 pt-2">
        <Button variant="ghost" size="sm" className="h-7 w-7 p-0" onClick={toggleDetailSidebar}>
          <ChevronLeft className="h-4 w-4" />
        </Button>
      </div>
    );
  }

  const displayName = conversation?.customer_name || conversation?.customer_email || (conversation?.anonymous_id ? `Visitor #${conversation.anonymous_id.slice(0, 6)}` : 'Anonymous');

  return (
    <div className="flex w-[300px] flex-col border-l bg-muted/30">
      {/* Header */}
      <div className="flex items-center justify-between border-b px-3 py-2">
        <h3 className="text-sm font-semibold">Details</h3>
        <Button variant="ghost" size="sm" className="h-7 w-7 p-0" onClick={toggleDetailSidebar}>
          <ChevronRight className="h-4 w-4" />
        </Button>
      </div>

      {!conversation ? (
        <div className="flex flex-1 items-center justify-center">
          <p className="text-xs text-muted-foreground">No conversation selected</p>
        </div>
      ) : (
        <div className="flex-1 overflow-y-auto">
          {/* ── Contact Card ─────────────────────────────── */}
          <div className="flex flex-col items-center gap-1.5 px-3 py-4 border-b">
            <div className={`flex h-12 w-12 items-center justify-center rounded-full text-base font-semibold ${getAvatarColor(conversation.customer_email || conversation.customer_name || conversation.id)}`}>
              {getInitial(conversation.customer_name || conversation.customer_email)}
            </div>
            <span className="text-sm font-semibold truncate max-w-full">{displayName}</span>
            {conversation.customer_email && conversation.customer_name && (
              <span className="flex items-center gap-1 text-xs text-muted-foreground truncate max-w-full">
                <Mail className="h-3 w-3 shrink-0" />
                {conversation.customer_email}
              </span>
            )}
            {conversation.crm_contact_id && workspace?.slug && (
              <Link
                to="/w/$slug/crm/contacts/$contactId"
                params={{ slug: workspace.slug, contactId: conversation.crm_contact_id }}
                className="inline-flex items-center gap-1 rounded-md bg-blue-50 px-2 py-0.5 text-[11px] font-medium text-blue-700 transition-colors hover:bg-blue-100 dark:bg-blue-950/30 dark:text-blue-400 mt-0.5"
              >
                <User className="h-3 w-3" />
                View CRM Contact
              </Link>
            )}
          </div>

          {/* ── Conversation Info ────────────────────────── */}
          <div className="border-b px-3 py-3 space-y-2">
            <div className="flex items-center justify-between text-xs">
              <span className="text-muted-foreground flex items-center gap-1"><Clock className="h-3 w-3" /> Created</span>
              <span>{formatTimestamp(conversation.created_at)}</span>
            </div>
            <div className="flex items-center justify-between text-xs">
              <span className="text-muted-foreground flex items-center gap-1"><Clock className="h-3 w-3" /> Updated</span>
              <span>{formatTimestamp(conversation.updated_at)}</span>
            </div>
          </div>

          {/* ── Links / Associations ─────────────────────── */}
          <SidebarAssociations
            workspaceId={workspaceId}
            conversationId={conversation.id}
          />
        </div>
      )}
    </div>
  );
}
