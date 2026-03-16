import { Badge } from '@/components/ui/badge';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { useAuthStore } from '@/stores/authStore';
import { useSupportInboxStore } from '@/stores/supportInboxStore';
import { useWorkspaceMembers } from '@/hooks/queries/useWorkspaces';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import type { SupportConversation } from '@/lib/pmTypes';
import { STATUS_COLORS, STATUS_LABELS, PRIORITY_COLORS } from './constants';
import { timeAgo, getInitial } from './helpers';

const EMPTY_ARRAY: string[] = [];

function TypingDotsPill() {
  return (
    <span className="inline-flex items-center gap-0.5 rounded-full bg-foreground/10 px-2 py-0.5">
      <span className="h-1 w-1 rounded-full bg-foreground/50 animate-bounce [animation-delay:0ms]" />
      <span className="h-1 w-1 rounded-full bg-foreground/50 animate-bounce [animation-delay:150ms]" />
      <span className="h-1 w-1 rounded-full bg-foreground/50 animate-bounce [animation-delay:300ms]" />
    </span>
  );
}

function AgentAvatar({ userId, tooltip }: { userId: string; tooltip?: string }) {
  const wsId = useWorkspaceStore((s) => s.currentWorkspace?.id ?? '');
  const { data: members = [] } = useWorkspaceMembers(wsId);
  const member = members.find((m) => m.user_id === userId);
  const name = member?.full_name || member?.email || 'Agent';

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <div className="flex h-5 w-5 items-center justify-center rounded-full bg-primary text-[9px] font-medium text-primary-foreground ring-2 ring-background">
          {member?.avatar_url ? (
            <img src={member.avatar_url} alt={name} className="h-5 w-5 rounded-full object-cover" />
          ) : (
            getInitial(name)
          )}
        </div>
      </TooltipTrigger>
      <TooltipContent side="left">
        <span className="text-xs">{name}{tooltip ? ` — ${tooltip}` : ''}</span>
      </TooltipContent>
    </Tooltip>
  );
}

interface ConversationRowProps {
  conversation: SupportConversation;
  isSelected: boolean;
  onSelect: () => void;
}

export function ConversationRow({ conversation, isSelected, onSelect }: ConversationRowProps) {
  const displayName = conversation.customer_name || conversation.customer_email || 'Anonymous';
  const typingState = useSupportInboxStore((s) => s.typingIndicators[conversation.id]);
  const isCustomerTyping = typingState !== false && typingState !== undefined;
  const agentState = useSupportInboxStore((s) => s.agentTyping[conversation.id]);
  const remoteViewingIds = useSupportInboxStore((s) => s.viewingAgents[conversation.id] || EMPTY_ARRAY);
  const currentUserId = useAuthStore((s) => s.user?.id);
  // Merge self into viewing list for the selected conversation (self events are filtered from WS)
  const viewingAgentIds = isSelected && currentUserId && !remoteViewingIds.includes(currentUserId)
    ? [...remoteViewingIds, currentUserId]
    : remoteViewingIds;
  const hasActivity = isCustomerTyping || !!agentState || viewingAgentIds.length > 0;

  return (
    <button
      type="button"
      onClick={onSelect}
      className={`w-full text-left border-b px-3 py-2.5 transition-colors hover:bg-muted/50 ${
        isSelected ? 'bg-muted border-l-2 border-l-primary' : ''
      }`}
    >
      <div className="flex items-start gap-2.5">
        <div className="mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-primary/10 text-xs font-medium text-primary">
          {getInitial(displayName)}
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex items-center justify-between gap-2">
            <span className="truncate text-sm font-medium">{displayName}</span>
            <span className="shrink-0 text-[10px] text-muted-foreground">{timeAgo(conversation.updated_at)}</span>
          </div>
          <p className="mt-0.5 truncate text-sm text-foreground/80">
            {isCustomerTyping && typingState ? (
              <span className="italic text-muted-foreground">{typingState}</span>
            ) : agentState?.content ? (
              <span className="italic text-primary/60">{agentState.content}</span>
            ) : (
              conversation.subject
            )}
          </p>
          {/* Bottom row: badges left, typing + avatars right */}
          <div className="mt-1 flex items-center justify-between gap-1.5">
            <div className="flex items-center gap-1.5">
              <span className="text-[10px] text-muted-foreground">#{conversation.display_id}</span>
              <Badge variant="secondary" className={`text-[10px] px-1.5 py-0 leading-tight ${STATUS_COLORS[conversation.status]}`}>
                {STATUS_LABELS[conversation.status]}
              </Badge>
              <Badge variant="secondary" className={`text-[10px] px-1.5 py-0 leading-tight ${PRIORITY_COLORS[conversation.priority]}`}>
                {conversation.priority}
              </Badge>
            </div>

            {/* Right: typing indicator + agent avatars */}
            {hasActivity && (
              <div className="flex items-center gap-1">
                {(isCustomerTyping || agentState) && <TypingDotsPill />}
                {agentState && !viewingAgentIds.includes(agentState.actorId) && (
                  <AgentAvatar userId={agentState.actorId} tooltip="responding" />
                )}
                {viewingAgentIds.map((uid) => (
                  <AgentAvatar
                    key={uid}
                    userId={uid}
                    tooltip={agentState?.actorId === uid ? 'responding' : 'viewing'}
                  />
                ))}
              </div>
            )}
          </div>
        </div>
      </div>
    </button>
  );
}
