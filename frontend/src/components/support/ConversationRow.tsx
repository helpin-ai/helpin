import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { useAuthStore } from '@/stores/authStore';
import { useSupportInboxStore } from '@/stores/supportInboxStore';
import { useWorkspaceMembers } from '@/hooks/queries/useWorkspaces';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import type { SupportConversation } from '@/lib/pmTypes';
import { timeAgo, getInitial } from './helpers';

const EMPTY_ARRAY: string[] = [];

// Generate a consistent color from a string (name, email, or anonymous ID)
const AVATAR_COLORS = [
  'bg-rose-100 text-rose-700 dark:bg-rose-900/30 dark:text-rose-400',
  'bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-400',
  'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400',
  'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400',
  'bg-teal-100 text-teal-700 dark:bg-teal-900/30 dark:text-teal-400',
  'bg-cyan-100 text-cyan-700 dark:bg-cyan-900/30 dark:text-cyan-400',
  'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400',
  'bg-indigo-100 text-indigo-700 dark:bg-indigo-900/30 dark:text-indigo-400',
  'bg-violet-100 text-violet-700 dark:bg-violet-900/30 dark:text-violet-400',
  'bg-pink-100 text-pink-700 dark:bg-pink-900/30 dark:text-pink-400',
];

function getAvatarColor(seed: string): string {
  let hash = 0;
  for (let i = 0; i < seed.length; i++) {
    hash = seed.charCodeAt(i) + ((hash << 5) - hash);
  }
  return AVATAR_COLORS[Math.abs(hash) % AVATAR_COLORS.length];
}

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
  // typing state is string (including empty '') when typing, false/undefined when not
  const isCustomerTyping = typeof typingState === 'string';
  const agentTypingMap = useSupportInboxStore((s) => s.agentTyping[conversation.id]);
  const agentTypingEntries = agentTypingMap ? Object.entries(agentTypingMap) : [];
  const isAgentTyping = agentTypingEntries.length > 0;
  const remoteViewingIds = useSupportInboxStore((s) => s.viewingAgents[conversation.id] || EMPTY_ARRAY);
  const currentUserId = useAuthStore((s) => s.user?.id);
  // Merge self into viewing list for the selected conversation (self events are filtered from WS)
  const viewingAgentIds = isSelected && currentUserId && !remoteViewingIds.includes(currentUserId)
    ? [...remoteViewingIds, currentUserId]
    : remoteViewingIds;
  const hasActivity = isCustomerTyping || isAgentTyping || viewingAgentIds.length > 0;

  return (
    <button
      type="button"
      onClick={onSelect}
      className={`w-full text-left border-b px-3 py-2.5 transition-colors hover:bg-muted/50 ${
        isSelected ? 'bg-muted border-l-2 border-l-primary' : ''
      }`}
    >
      <div className="flex items-start gap-2.5">
        <div className={`mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-full text-xs font-semibold ${getAvatarColor(conversation.customer_email || conversation.customer_name || conversation.id)}`}>
          {getInitial(displayName)}
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex items-center justify-between gap-2">
            <span className="truncate text-sm font-medium">{displayName}</span>
            <span className="shrink-0 text-[10px] text-muted-foreground">{timeAgo(conversation.updated_at)}</span>
          </div>
          <div className="mt-0.5 flex items-center gap-1.5">
            <p className="min-w-0 flex-1 truncate text-sm text-foreground/80">
              {isCustomerTyping ? (
                <span className="italic text-muted-foreground">{typingState || 'typing…'}</span>
              ) : isAgentTyping ? (
                <span className="italic text-primary/60">{agentTypingEntries[0][1] || 'typing…'}</span>
              ) : (
                conversation.last_message || conversation.subject
              )}
            </p>
            {hasActivity && (
              <div className="flex shrink-0 items-center gap-1">
                {(isCustomerTyping || isAgentTyping) && <TypingDotsPill />}
                {agentTypingEntries
                  .filter(([uid]) => !viewingAgentIds.includes(uid))
                  .map(([uid]) => (
                    <AgentAvatar key={uid} userId={uid} tooltip="responding" />
                  ))}
                {viewingAgentIds.map((uid) => (
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
}
