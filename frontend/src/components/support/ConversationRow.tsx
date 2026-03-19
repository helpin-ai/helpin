import { memo } from 'react';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { useAuthStore } from '@/stores/authStore';
import { useSupportInboxStore } from '@/stores/supportInboxStore';
import { useSupportPresenceStore } from '@/stores/supportPresenceStore';
import { useWorkspaceMembers } from '@/hooks/queries/useWorkspaces';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import type { SupportConversation } from '@/lib/pmTypes';
import { timeAgo, getInitial, getAvatarColor } from './helpers';

const EMPTY_ARRAY: string[] = [];

const TypingDotsPill = memo(function TypingDotsPill() {
  return (
    <span className="inline-flex items-center gap-0.5 rounded-full bg-foreground/10 px-2 py-0.5">
      <span className="h-1 w-1 rounded-full bg-foreground/50 animate-bounce [animation-delay:0ms]" />
      <span className="h-1 w-1 rounded-full bg-foreground/50 animate-bounce [animation-delay:150ms]" />
      <span className="h-1 w-1 rounded-full bg-foreground/50 animate-bounce [animation-delay:300ms]" />
    </span>
  );
});

const AgentAvatar = memo(function AgentAvatar({ userId, tooltip }: { userId: string; tooltip?: string }) {
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
});

interface ConversationRowProps {
  conversation: SupportConversation;
  isSelected: boolean;
  onSelect: () => void;
}

export const ConversationRow = memo(function ConversationRow({ conversation, isSelected, onSelect }: ConversationRowProps) {
  const visitorLabel = conversation.anonymous_id ? `Visitor #${conversation.anonymous_id.slice(0, 6)}` : 'Anonymous';
  const displayName = conversation.customer_name || conversation.customer_email || visitorLabel;
  const unreadCount = conversation.unread_count ?? 0;
  const isUnread = unreadCount > 0;
  const typingState = useSupportPresenceStore((s) => s.typingIndicators[conversation.id]);
  // typing state is string (including empty '') when typing, false/undefined when not
  const isCustomerTyping = typeof typingState === 'string';
  const agentTypingMap = useSupportPresenceStore((s) => s.agentTyping[conversation.id]);
  const agentTypingEntries = agentTypingMap ? Object.entries(agentTypingMap) : [];
  const isAgentTyping = agentTypingEntries.length > 0;
  const wsId = useWorkspaceStore((s) => s.currentWorkspace?.id ?? '');
  const { data: members = [] } = useWorkspaceMembers(wsId);
  const isVisitorOnline = useSupportPresenceStore((s) =>
    conversation.anonymous_id ? !!s.onlineVisitors[conversation.anonymous_id] : false
  );
  const draftContent = useSupportInboxStore((s) => s.drafts[conversation.id]);
  // Only show draft label when NOT actively viewing this conversation
  const hasDraft = !!draftContent && !isSelected;
  const remoteViewingIds = useSupportPresenceStore((s) => s.viewingAgents[conversation.id] || EMPTY_ARRAY);
  const currentUserId = useAuthStore((s) => s.user?.id);
  // Merge self into viewing list for the selected conversation (self events are filtered from WS)
  const viewingAgentIds = isSelected && currentUserId && !remoteViewingIds.includes(currentUserId)
    ? [...remoteViewingIds, currentUserId]
    : remoteViewingIds;
  const hasActivity = isUnread || isCustomerTyping || isAgentTyping || viewingAgentIds.length > 0;

  return (
    <button
      type="button"
      onClick={onSelect}
      className={`w-full text-left border-b px-3 py-2.5 transition-colors hover:bg-muted/50 ${
        isSelected ? 'bg-muted border-l-2 border-l-primary' : isUnread ? 'bg-blue-50/50 dark:bg-blue-950/20' : ''
      }`}
    >
      <div className="flex items-start gap-2.5">
        <div className="relative mt-0.5 shrink-0">
          <div className={`flex h-8 w-8 items-center justify-center rounded-full text-xs font-semibold ${getAvatarColor(conversation.customer_email || conversation.customer_name || conversation.id)}`}>
            {getInitial(displayName)}
          </div>
          {isVisitorOnline && (
            <span className="absolute -top-0.5 -left-0.5 h-2.5 w-2.5 rounded-full bg-green-400 ring-2 ring-background" />
          )}
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex items-center justify-between gap-2">
            <span className={`truncate max-w-[160px] text-sm ${isUnread ? 'font-semibold' : 'font-medium'}`}>{displayName}</span>
            <span className="shrink-0 text-[10px] text-muted-foreground">{timeAgo(conversation.updated_at)}</span>
          </div>
          <div className="mt-0.5 flex min-w-0 items-center gap-1.5">
            <p className={`min-w-0 flex-1 truncate text-sm ${isUnread ? 'font-medium text-foreground' : 'text-foreground/80'}`}>
              {isCustomerTyping ? (
                <span className="italic text-muted-foreground">{typingState || 'typing…'}</span>
              ) : isAgentTyping ? (
                <span className="italic text-blue-600/70 dark:text-blue-400/70">
                  {(() => {
                    const [uid, content] = agentTypingEntries[0];
                    const m = members.find((mb) => mb.user_id === uid);
                    const name = m?.full_name?.split(' ')[0] || 'Agent';
                    return content ? `${name}: ${content}` : `${name} is typing…`;
                  })()}
                </span>
              ) : hasDraft ? (
                <>
                  <span className="inline-block w-0.5 h-3 align-middle rounded-full bg-blue-500 mr-1" />
                  <span className="text-blue-600 dark:text-blue-400 font-medium">Draft: </span>
                  <span className="text-muted-foreground">{draftContent}</span>
                </>
              ) : conversation.last_message?.startsWith('Note: ') ? (
                <>
                  <span className="inline-block w-0.5 h-3 align-middle rounded-full bg-amber-500 mr-1" />
                  <span className="text-amber-600 dark:text-amber-400 font-medium">Note: </span>
                  <span className="text-muted-foreground">{conversation.last_message.slice(6)}</span>
                </>
              ) : (
                conversation.last_message || conversation.subject
              )}
            </p>
            {hasActivity && (
              <div className="flex shrink-0 items-center gap-1">
                {(isCustomerTyping || isAgentTyping) && <TypingDotsPill />}
                {isUnread && (
                  <span className="inline-flex h-[18px] min-w-[18px] items-center justify-center rounded-full bg-blue-600 px-1 text-[10px] font-semibold text-white">
                    {unreadCount > 99 ? '99+' : unreadCount}
                  </span>
                )}
                {!isUnread && agentTypingEntries
                  .filter(([uid]) => !viewingAgentIds.includes(uid))
                  .map(([uid]) => (
                    <AgentAvatar key={uid} userId={uid} tooltip="responding" />
                  ))}
                {!isUnread && viewingAgentIds.map((uid) => (
                  <AgentAvatar
                    key={uid}
                    userId={uid}
                    tooltip={agentTypingMap?.[uid] !== undefined ? 'responding' : 'viewing'}
                  />
                ))}
              </div>
            )}
          </div>
        </div>
      </div>
    </button>
  );
});
