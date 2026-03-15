import { useState } from 'react';
import { Link } from '@tanstack/react-router';
import { Bot, ChevronLeft, ChevronRight, Loader2, User } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Separator } from '@/components/ui/separator';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { AssociationsPanel } from '@/components/pm/AssociationsPanel';
import {
  useConversation,
  useUpdateConversationStatus,
  useAssignAgent,
  useRunConversationAgent,
  useSupportAgents,
} from '@/hooks/queries/useSupport';
import { useSupportInboxStore } from '@/stores/supportInboxStore';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { STATUS_LABELS, PRIORITY_LABELS } from './constants';
import { formatTimestamp } from './helpers';
import type { ConversationStatus } from '@/lib/pmTypes';

interface ConversationDetailSidebarProps {
  workspaceId: string;
  conversationId: string | null;
}

export function ConversationDetailSidebar({ workspaceId, conversationId }: ConversationDetailSidebarProps) {
  const { detailSidebarCollapsed, toggleDetailSidebar } = useSupportInboxStore();
  const workspace = useWorkspaceStore((s) => s.currentWorkspace);
  const { data: conversation } = useConversation(workspaceId, conversationId);
  const { data: supportAgents = [] } = useSupportAgents(workspaceId);

  const updateStatus = useUpdateConversationStatus(workspaceId);
  const assignAgent = useAssignAgent(workspaceId);
  const runAgent = useRunConversationAgent(workspaceId);

  const [assignAgentId, setAssignAgentId] = useState('');

  const handleAssign = async () => {
    if (!conversationId || !assignAgentId) return;
    await assignAgent.mutateAsync({ conversationId, agentId: assignAgentId });
    setAssignAgentId('');
  };

  const assignedAgent = conversation?.assigned_agent_id
    ? supportAgents.find((a) => a.id === conversation.assigned_agent_id)
    : null;

  if (detailSidebarCollapsed) {
    return (
      <div className="flex w-10 flex-col items-center border-l bg-muted/30 pt-2">
        <Button variant="ghost" size="sm" className="h-7 w-7 p-0" onClick={toggleDetailSidebar}>
          <ChevronLeft className="h-4 w-4" />
        </Button>
      </div>
    );
  }

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
          <div className="space-y-4 p-3">
            {/* Status */}
            <div>
              <label className="mb-1 block text-xs font-medium text-muted-foreground">Status</label>
              <Select
                value={conversation.status}
                onValueChange={(v) =>
                  updateStatus.mutate({ conversationId: conversation.id, status: v as ConversationStatus })
                }
              >
                <SelectTrigger className="h-8 text-xs">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {Object.entries(STATUS_LABELS).map(([value, label]) => (
                    <SelectItem key={value} value={value}>{label}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            {/* Priority */}
            <div>
              <label className="mb-1 block text-xs font-medium text-muted-foreground">Priority</label>
              <div className="rounded-md border px-2.5 py-1.5 text-xs capitalize">
                {PRIORITY_LABELS[conversation.priority]}
              </div>
            </div>

            {/* Assignee */}
            <div>
              <label className="mb-1 block text-xs font-medium text-muted-foreground">Assignee</label>
              {assignedAgent ? (
                <div className="flex items-center gap-2 rounded-md border px-2.5 py-1.5 text-xs">
                  <Bot className="h-3.5 w-3.5 text-muted-foreground" />
                  <span>{assignedAgent.name}</span>
                </div>
              ) : (
                <p className="text-xs text-muted-foreground italic">Unassigned</p>
              )}
              <div className="mt-1.5 flex items-center gap-1">
                <Select value={assignAgentId} onValueChange={setAssignAgentId} disabled={supportAgents.length === 0}>
                  <SelectTrigger className="h-7 flex-1 text-xs">
                    <SelectValue placeholder="Select agent" />
                  </SelectTrigger>
                  <SelectContent>
                    {supportAgents.map((agent) => (
                      <SelectItem key={agent.id} value={agent.id}>{agent.name}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <Button size="sm" variant="outline" className="h-7 text-xs" disabled={!assignAgentId} onClick={handleAssign}>
                  Assign
                </Button>
              </div>
              {conversation.assigned_agent_id && (
                <Button
                  size="sm"
                  variant="outline"
                  className="mt-1.5 h-7 w-full gap-1 text-xs"
                  disabled={runAgent.isPending}
                  onClick={() => runAgent.mutate(conversation.id)}
                >
                  {runAgent.isPending ? <Loader2 className="h-3 w-3 animate-spin" /> : <Bot className="h-3 w-3" />}
                  Run Agent
                </Button>
              )}
            </div>

            {/* Source */}
            <div>
              <label className="mb-1 block text-xs font-medium text-muted-foreground">Source</label>
              <div className="rounded-md border px-2.5 py-1.5 text-xs capitalize">
                {conversation.source}
              </div>
            </div>

            <Separator />

            {/* Customer info */}
            <div>
              <label className="mb-1 block text-xs font-medium text-muted-foreground">Customer</label>
              <div className="space-y-1 text-xs">
                {conversation.customer_name && <p>{conversation.customer_name}</p>}
                {conversation.customer_email && (
                  <p className="text-muted-foreground">{conversation.customer_email}</p>
                )}
                {conversation.crm_contact_id && workspace?.slug && (
                  <Link
                    to="/w/$slug/crm/contacts/$contactId"
                    params={{ slug: workspace.slug, contactId: conversation.crm_contact_id }}
                    className="inline-flex items-center gap-1 rounded-md bg-blue-50 px-1.5 py-0.5 text-[11px] font-medium text-blue-700 transition-colors hover:bg-blue-100 dark:bg-blue-950/30 dark:text-blue-400"
                  >
                    <User className="h-3 w-3" />
                    View CRM Contact
                  </Link>
                )}
              </div>
            </div>

            <Separator />

            {/* Associations */}
            <div>
              <AssociationsPanel
                objectType="support_conversation"
                objectId={conversation.id}
                workspaceId={workspaceId}
              />
            </div>

            <Separator />

            {/* Timestamps */}
            <div className="space-y-1.5 text-xs text-muted-foreground">
              <div className="flex justify-between">
                <span>Created</span>
                <span>{formatTimestamp(conversation.created_at)}</span>
              </div>
              <div className="flex justify-between">
                <span>Updated</span>
                <span>{formatTimestamp(conversation.updated_at)}</span>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
