import { memo } from 'react';
import { CheckCircle2 } from 'lucide-react';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { useAuthStore } from '@/stores/authStore';
import { useSupportInboxStore } from '@/stores/supportInboxStore';
import { type AgentTypingState, useSupportPresenceStore } from '@/stores/supportPresenceStore';
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

const AgentTypingActivity = memo(function AgentTypingActivity({
  viewingAgentIds,
  agentTypingMap,
  agentTypingEntries,
  members,
}: {
  viewingAgentIds: string[];
  agentTypingMap: Record<string, AgentTypingState> | undefined;
  agentTypingEntries: Array<[string, AgentTypingState]>;
  members: Array<{ user_id: string; full_name?: string | null; email?: string | null; avatar_url?: string | null }>;
}) {
  return (
    <div className="flex items-center gap-1.5">
      <TypingDotsPill />
      <div className="flex items-center flex-row-reverse">
        {viewingAgentIds.map((uid) => (
          <AgentAvatar
            key={uid}
            userId={uid}
            tooltip={agentTypingMap?.[uid] !== undefined ? 'typing' : 'viewing'}
            nameOverride={agentTypingMap?.[uid]?.name}
            avatarUrlOverride={agentTypingMap?.[uid]?.avatarUrl}
          />
        ))}
        {agentTypingEntries
          .filter(([uid]) => !viewingAgentIds.includes(uid))
          .map(([uid, typing]) => {
            const identity = resolveAgentIdentity(uid, typing, members);
            return (
              <AgentAvatar
                key={uid}
                userId={uid}
                tooltip={typing.content ? `typing: ${typing.content}` : 'typing'}
                nameOverride={identity.name}
                avatarUrlOverride={identity.avatarUrl}
              />
            );
          })}
      </div>
    </div>
  );
});

function resolveAgentIdentity(
  userId: string,
  typingState: AgentTypingState | undefined,
  members: Array<{ user_id: string; full_name?: string | null; email?: string | null; avatar_url?: string | null }>
) {
  const member = members.find((m) => m.user_id === userId);
  const name = typingState?.name || member?.full_name || member?.email || 'Agent';
  const avatarUrl = typingState?.avatarUrl || member?.avatar_url || undefined;
  return { name, avatarUrl };
}

const AgentAvatar = memo(function AgentAvatar({
  userId,
  tooltip,
  nameOverride,
  avatarUrlOverride,
}: {
  userId: string;
  tooltip?: string;
  nameOverride?: string;
  avatarUrlOverride?: string;
}) {
  const wsId = useWorkspaceStore((s) => s.currentWorkspace?.id ?? '');
  const { data: members = [] } = useWorkspaceMembers(wsId);
  const member = members.find((m) => m.user_id === userId);
  const name = nameOverride || member?.full_name || member?.email || 'Agent';
  const avatarUrl = avatarUrlOverride || member?.avatar_url || undefined;

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <div className={`flex h-5 w-5 -ml-1.5 first:ml-0 items-center justify-center rounded-full text-[9px] font-medium ring-2 ring-background animate-in zoom-in-75 duration-300 ${getAvatarColor(userId)}`}>
          {avatarUrl ? (
            <img src={avatarUrl} alt={name} className="h-5 w-5 rounded-full object-cover" />
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
  const hasDraft = !!draftContent && !isSelected;
  const remoteViewingIds = useSupportPresenceStore((s) => s.viewingAgents[conversation.id] || EMPTY_ARRAY);
  const currentUserId = useAuthStore((s) => s.user?.id);
  const viewingAgentIds = isSelected && currentUserId && !remoteViewingIds.includes(currentUserId)
    ? [...remoteViewingIds, currentUserId]
    : remoteViewingIds;

  return (
    <button
      type="button"
      onClick={onSelect}
      className={`group relative w-full text-left px-3 py-2.5 transition-all duration-200 hover:bg-muted/50 ${
        isSelected
          ? 'bg-muted'
          : isUnread
            ? 'bg-blue-50/50 dark:bg-blue-950/20'
            : ''
      }`}
    >
      {/* Active indicator bar (Crisp-style ::before) */}
      <span
        className={`absolute left-0 top-1/2 -translate-y-1/2 w-[3px] rounded-r-full transition-all duration-200 ${
          isSelected
            ? 'h-8 bg-primary'
            : isUnread
              ? 'h-5 bg-blue-500'
              : 'h-0 bg-transparent'
        }`}
      />

      {/* Inset bottom separator (Crisp-style ::after) */}
      <span className="absolute bottom-0 left-4 right-4 h-px bg-border/60 group-last:hidden" />

      <div className="flex items-center gap-3">
        {/* Avatar */}
        <div className="relative shrink-0">
          <div className={`flex h-9 w-9 items-center justify-center rounded-full text-xs font-semibold ${getAvatarColor(conversation.customer_email || conversation.customer_name || conversation.id)}`}>
            {getInitial(displayName)}
          </div>
          {isVisitorOnline && (
            <span className="absolute -bottom-0.5 -right-0.5 h-3 w-3 rounded-full bg-green-400 ring-2 ring-background" />
          )}
        </div>

        {/* Content */}
        <div style={{ flex: 1, minWidth: 0, overflow: 'hidden' }}>
          {/* Headline: name + time */}
          <div className="grid items-center gap-2" style={{ gridTemplateColumns: '1fr auto' }}>
            <span className={`text-[13.5px] leading-tight overflow-hidden text-ellipsis whitespace-nowrap ${isUnread ? 'font-semibold text-foreground' : 'font-medium text-foreground/90'}`}>
              {displayName}
            </span>
            <span className="shrink-0 text-[11px] text-muted-foreground/70 tabular-nums">
              {timeAgo(conversation.updated_at)}
            </span>
          </div>

          {/* Context: message preview + activity */}
          <div className="grid items-center gap-1.5 mt-0.5" style={{ gridTemplateColumns: '1fr auto' }}>
            <p
              className={`text-sm m-0 leading-[18px] ${isUnread ? 'font-medium text-foreground/80' : 'text-muted-foreground'}`}
              style={{ display: '-webkit-box', WebkitLineClamp: 1, WebkitBoxOrient: 'vertical', overflow: 'hidden', textOverflow: 'ellipsis', maxHeight: '18px' }}
            >
              {isCustomerTyping ? (
                <span className="italic text-muted-foreground">{typingState || 'typing…'}</span>
              ) : isAgentTyping ? (
                <span className="italic text-blue-600/70 dark:text-blue-400/70">
                  {(() => {
                    const [uid, typing] = agentTypingEntries[0];
                    const { name } = resolveAgentIdentity(uid, typing, members);
                    const shortName = name.split(' ')[0] || 'Agent';
                    return `${shortName} is typing…`;
                  })()}
                </span>
              ) : hasDraft ? (
                <>
                  <span className="inline-block w-[3px] h-3.5 align-middle rounded-full bg-blue-500 mr-1.5" />
                  <span className="text-blue-600 dark:text-blue-400 font-medium">Draft: </span>
                  <span className="text-muted-foreground">{draftContent}</span>
                </>
              ) : conversation.last_message?.startsWith('Note: ') ? (
                <>
                  <span className="inline-block w-[3px] h-3.5 align-middle rounded-full bg-amber-500 mr-1.5" />
                  <span className="text-amber-600 dark:text-amber-400 font-medium">Note: </span>
                  <span className="text-muted-foreground">{conversation.last_message.slice(6)}</span>
                </>
              ) : (
                conversation.last_message || conversation.subject
              )}
            </p>

            {/* Activity indicators or status icon */}
            <div className="flex shrink-0 items-center">
              {isCustomerTyping ? (
                <TypingDotsPill />
              ) : isAgentTyping ? (
                <AgentTypingActivity
                  viewingAgentIds={viewingAgentIds}
                  agentTypingMap={agentTypingMap}
                  agentTypingEntries={agentTypingEntries}
                  members={members}
                />
              ) : isUnread ? (
                <span className="inline-flex h-[18px] min-w-[18px] items-center justify-center rounded-full bg-blue-600 px-1 text-[10px] font-semibold text-white animate-in zoom-in-75 duration-200">
                  {unreadCount > 99 ? '99+' : unreadCount}
                </span>
              ) : viewingAgentIds.length > 0 ? (
                <div className="flex items-center flex-row-reverse">
                  {viewingAgentIds.map((uid) => (
                    <AgentAvatar key={uid} userId={uid} tooltip="viewing" />
                  ))}
                </div>
              ) : (conversation.status === 'resolved' || conversation.status === 'closed') ? (
                <Tooltip>
                  <TooltipTrigger asChild>
                    <CheckCircle2 className="h-5 w-5 text-green-500" />
                  </TooltipTrigger>
                  <TooltipContent side="left"><span className="text-xs">Resolved</span></TooltipContent>
                </Tooltip>
              ) : null}
            </div>
          </div>
        </div>
      </div>
    </button>
  );
});
