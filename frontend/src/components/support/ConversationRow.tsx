import { memo, useMemo, useState, type KeyboardEvent } from 'react';
import { ArrowTurnBackwardIcon, BotIcon, CheckmarkCircle02Icon, Mail01Icon, Message01Icon, MoreHorizontalIcon } from '@/lib/icons';
import type { TicketSource } from '@/lib/pm-types/support';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { useSupportInboxStore } from '@/stores/supportInboxStore';
import { type AgentTypingState, useSupportPresenceStore } from '@/stores/supportPresenceStore';
import { useWorkspaceMembers } from '@/hooks/queries/useWorkspaces';
import { resolveTeamMemberAvatarSrc } from '@/lib/teamMemberAvatar';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import type { SupportConversation } from '@/lib/pmTypes';
import { timeAgo, getInitial, getAvatarColor } from './helpers';
import {
  getConversationRowVisualState,
  getSupportTagPillStyle,
  isNotePreview,
  stripNotePrefix,
} from './conversationRowVisual';
import { ConversationActionsMenu, type ConversationActionMoveOption } from './ConversationActionsMenu';

// Re-exported from the pure helper module so existing importers (tests, other
// surfaces) keep resolving these from './ConversationRow'.
export {
  getConversationRowVisualState,
  getSupportTagPillStyle,
  getVisibleSupportTagCount,
} from './conversationRowVisual';
export type { ConversationRowVisualState } from './conversationRowVisual';

const EMPTY_ARRAY: string[] = [];

type ChannelMeta = { icon: typeof Message01Icon; label: string };

const CHANNEL_META: Record<TicketSource, ChannelMeta | null> = {
  widget: { icon: Message01Icon, label: 'Live chat' },
  email: { icon: Mail01Icon, label: 'Email' },
  api: null,
  internal: null,
};

const ChannelIcon = memo(function ChannelIcon({ source }: { source?: TicketSource | null }) {
  if (!source) return null;
  const meta = CHANNEL_META[source];
  if (!meta) return null;
  const Icon = meta.icon;
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span
          aria-label={meta.label}
          className="inline-flex h-3.5 w-3.5 shrink-0 items-center justify-center text-muted-foreground/70"
        >
          <Icon className="h-3.5 w-3.5" />
        </span>
      </TooltipTrigger>
      <TooltipContent side="top">
        <span className="text-xs">{meta.label}</span>
      </TooltipContent>
    </Tooltip>
  );
});

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
  members: Array<{ user_id: string; full_name?: string | null; email?: string | null; avatar_url?: string | null; avatar_style?: string | null; avatar_seed?: string | null; avatar_background_mode?: string | null; avatar_background_color?: string | null }>;
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
  members: Array<{ user_id: string; full_name?: string | null; email?: string | null; avatar_url?: string | null; avatar_style?: string | null; avatar_seed?: string | null; avatar_background_mode?: string | null; avatar_background_color?: string | null }>
) {
  const member = members.find((m) => m.user_id === userId);
  const name = typingState?.name || member?.full_name || member?.email || 'Agent';
  const avatarUrl = typingState?.avatarUrl || resolveTeamMemberAvatarSrc({
    avatarUrl: member?.avatar_url,
    avatarStyle: member?.avatar_style,
    avatarSeed: member?.avatar_seed,
    avatarBackgroundMode: member?.avatar_background_mode,
    avatarBackgroundColor: member?.avatar_background_color,
    fallbackSeed: name,
  });
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
  const avatarUrl = avatarUrlOverride || resolveTeamMemberAvatarSrc({
    avatarUrl: member?.avatar_url,
    avatarStyle: member?.avatar_style,
    avatarSeed: member?.avatar_seed,
    avatarBackgroundMode: member?.avatar_background_mode,
    avatarBackgroundColor: member?.avatar_background_color,
    fallbackSeed: name,
  });

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
  workspaceId: string;
  conversation: SupportConversation;
  moveOptions: ConversationActionMoveOption[];
  onSelectConversation: (id: string, unreadCount?: number) => void;
  isTransitioningOut?: boolean;
}

const AIHandoffIndicator = memo(function AIHandoffIndicator() {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span
          aria-label="AI handed off to team"
          className="inline-flex h-4 w-4 shrink-0 items-center justify-center rounded-full bg-amber-100 text-amber-700 ring-1 ring-amber-200/80 dark:bg-amber-950/35 dark:text-amber-300 dark:ring-amber-800/60"
        >
          <BotIcon className="h-2.5 w-2.5" />
        </span>
      </TooltipTrigger>
      <TooltipContent side="left">
        <span className="text-xs">AI handed off to team</span>
      </TooltipContent>
    </Tooltip>
  );
});

const AgentReplyIndicator = memo(function AgentReplyIndicator({ label }: { label: string }) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span
          aria-label={label}
          className="inline-flex h-3.5 w-3.5 shrink-0 items-center justify-center text-muted-foreground/75"
        >
          <ArrowTurnBackwardIcon className="h-3.5 w-3.5 -scale-y-100" />
        </span>
      </TooltipTrigger>
      <TooltipContent side="top">
        <span className="text-xs">{label}</span>
      </TooltipContent>
    </Tooltip>
  );
});

const CustomerReplyIndicator = memo(function CustomerReplyIndicator() {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span
          aria-label="Customer replied"
          className="inline-flex h-3.5 w-3.5 shrink-0 items-center justify-center"
        >
          <span className="h-1.5 w-1.5 rounded-full bg-blue-500" />
        </span>
      </TooltipTrigger>
      <TooltipContent side="top">
        <span className="text-xs">Customer replied</span>
      </TooltipContent>
    </Tooltip>
  );
});

const AIResolvedIndicator = memo(function AIResolvedIndicator() {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span
          aria-label="Resolved by AI"
          className="inline-flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-emerald-100 text-emerald-700 ring-1 ring-emerald-200/80 dark:bg-emerald-950/35 dark:text-emerald-300 dark:ring-emerald-800/60"
        >
          <BotIcon className="h-3 w-3" />
        </span>
      </TooltipTrigger>
      <TooltipContent side="left">
        <span className="text-xs">Resolved by AI</span>
      </TooltipContent>
    </Tooltip>
  );
});

const HUMAN_QUEUE_FLOW_STATES = new Set(['queued_for_human', 'after_hours_queue']);

const HumanQueueBadge = memo(function HumanQueueBadge({ waitSince }: { waitSince: string }) {
  return (
    <span
      aria-label={`Waiting for human, waiting ${timeAgo(waitSince)}`}
      className="inline-flex shrink-0 items-center gap-1 rounded-full border border-amber-200/70 bg-amber-50 px-1.5 py-0.5 text-[10px] font-medium leading-none text-amber-700 dark:border-amber-900/60 dark:bg-amber-950/30 dark:text-amber-300"
    >
      Waiting for human · {timeAgo(waitSince)}
    </span>
  );
});

const MAX_VISIBLE_USER_TAGS = 2;

const UserTagStrip = memo(function UserTagStrip({
  tags,
}: {
  tags: NonNullable<SupportConversation['tags']>;
}) {
  if (tags.length === 0) return null;
  const visibleTags = tags.slice(0, MAX_VISIBLE_USER_TAGS);
  const hiddenCount = Math.max(0, tags.length - visibleTags.length);

  return (
    <div className="mt-1 flex min-w-0 items-center gap-1 overflow-hidden">
      {visibleTags.map((tag) => (
        <span
          key={tag.id}
          className="inline-flex max-w-[7rem] shrink-0 items-center rounded-full border border-border/70 bg-background/70 px-1.5 py-0.5 text-[10px] font-medium leading-none text-muted-foreground"
          title={tag.name}
          style={getSupportTagPillStyle(tag.color)}
        >
          <span className="min-w-0 truncate">{tag.name}</span>
        </span>
      ))}
      {hiddenCount > 0 ? (
        <span className="inline-flex shrink-0 items-center rounded-full border border-border/70 bg-muted/60 px-1.5 py-0.5 text-[10px] font-medium leading-none text-muted-foreground">
          +{hiddenCount}
        </span>
      ) : null}
    </div>
  );
});

export const ConversationRow = memo(function ConversationRow({
  workspaceId,
  conversation,
  moveOptions,
  onSelectConversation,
  isTransitioningOut = false,
}: ConversationRowProps) {
  const isSelected = useSupportInboxStore((s) => s.selectedConversationId === conversation.id);
  const visitorLabel = conversation.anonymous_id ? `Visitor #${conversation.anonymous_id.slice(0, 6)}` : 'Anonymous';
  const displayName = conversation.customer_name || conversation.customer_email || visitorLabel;
  const unreadCount = conversation.unread_count ?? 0;
  const visualState = getConversationRowVisualState(conversation, isSelected);
  const isUnread = visualState.isUnread;
  const [actionsOpen, setActionsOpen] = useState(false);
  const [actionsMounted, setActionsMounted] = useState(false);
  const [actionsDialogOpen, setActionsDialogOpen] = useState(false);
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
  const viewingAgentIds = remoteViewingIds;
  const availableMoveOptions = useMemo(
    () => moveOptions.filter((option) => option.id !== (conversation.mailbox_id ?? 'shared')),
    [conversation.mailbox_id, moveOptions]
  );
  const hasAIHandoff = (conversation.system_tags ?? []).includes('ai_handoff');
  const hasAIResolved = (conversation.system_tags ?? []).includes('ai_resolved') || conversation.flow_state === 'resolved_by_ai' || conversation.ai_state === 'resolved';
  const isWaitingForHuman = !!conversation.flow_state && HUMAN_QUEUE_FLOW_STATES.has(conversation.flow_state);
  const humanQueueWaitSince = conversation.customer_requested_human_at || conversation.ai_escalated_at || conversation.updated_at;
  const userTags = conversation.tags ?? [];
  const hasAgentReplyPreview = conversation.last_message_sender_type === 'user' || conversation.last_message_sender_type === 'agent';
  const hasCustomerReplyPreview = conversation.customer_awaiting_response
    ?? conversation.awaiting_reply
    ?? conversation.last_message_sender_type === 'customer';
  const agentReplyLabel = `${conversation.last_message_sender_display_name?.trim() || 'Agent'} replied`;
  const handleRowKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    if (event.target !== event.currentTarget) return;
    if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault();
      onSelectConversation(conversation.id, unreadCount);
    }
  };

  const handleActionsOpenChange = (open: boolean) => {
    setActionsOpen(open);
    setActionsMounted(open);
  };

  const showActionsMenu = actionsMounted || actionsOpen || actionsDialogOpen;
  const actionButton = (
    <button
      type="button"
      aria-label={`Open actions for ${displayName}`}
      aria-haspopup="menu"
      aria-expanded={actionsOpen}
      className="inline-flex h-6 w-6 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-muted hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/40"
      onClick={(event) => {
        event.stopPropagation();
        if (!showActionsMenu) {
          setActionsMounted(true);
          setActionsOpen(true);
        }
      }}
      onKeyDown={(event) => {
        event.stopPropagation();
      }}
    >
      <MoreHorizontalIcon className="h-3.5 w-3.5" />
    </button>
  );

  return (
    <div
      role="button"
      tabIndex={0}
      onClick={() => onSelectConversation(conversation.id, unreadCount)}
      onKeyDown={handleRowKeyDown}
      onMouseEnter={() => setActionsMounted(true)}
      onMouseLeave={() => {
        if (!actionsOpen && !actionsDialogOpen) {
          setActionsMounted(false);
        }
      }}
      onFocusCapture={() => setActionsMounted(true)}
      onBlurCapture={(event) => {
        if (!actionsOpen && !actionsDialogOpen && !event.currentTarget.contains(event.relatedTarget as Node | null)) {
          setActionsMounted(false);
        }
      }}
      data-transitioning-out={isTransitioningOut ? 'true' : undefined}
      data-conversation-id={conversation.id}
      className={`group relative w-full cursor-pointer px-3 py-2.5 text-left transition-all duration-200 hover:bg-muted/75 focus:outline-none focus-visible:ring-2 focus-visible:ring-primary/40 dark:hover:bg-muted/40 ${
        isTransitioningOut
          ? 'pointer-events-none bg-emerald-50/70 opacity-60 dark:bg-emerald-950/20'
          : isSelected
          ? 'bg-muted/80 dark:bg-muted/45'
          : ''
      }`}
    >
      {/* Active indicator bar (Crisp-style ::before) */}
      <span
        className={`absolute left-0 top-1/2 -translate-y-1/2 w-[3px] rounded-r-full transition-all duration-200 ${
          isSelected
            ? 'h-11 bg-primary/90'
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
            <span className="absolute -left-0.5 -top-0.5 h-2.5 w-2.5 rounded-full bg-green-400 ring-2 ring-background shadow-sm" />
          )}
        </div>

        {/* Content */}
        <div style={{ flex: 1, minWidth: 0, overflow: 'hidden' }}>
          {/* Headline: name + time */}
          <div className="grid items-center gap-2" style={{ gridTemplateColumns: '1fr auto' }}>
            <div className="flex min-w-0 items-center gap-1.5">
              <ChannelIcon source={conversation.source} />
              <span
                data-conversation-customer-name="true"
                className={`min-w-0 overflow-hidden text-ellipsis whitespace-nowrap text-[13.5px] leading-tight ${visualState.usesUnreadTypography ? 'font-semibold text-foreground' : 'font-medium text-foreground/90'}`}
              >
                {displayName}
              </span>
            </div>
            <div className="relative flex h-6 min-w-6 items-center justify-end">
              <div
                className={`flex shrink-0 items-center transition-opacity duration-150 ${
                  actionsOpen
                    ? 'opacity-0'
                    : 'group-hover:opacity-0 group-focus-within:opacity-0'
                }`}
              >
                <span className="text-[11px] text-muted-foreground/70 tabular-nums">
                  {timeAgo(conversation.list_last_activity_at ?? conversation.list_last_message_at ?? conversation.created_at)}
                </span>
              </div>
              <div
                className={`absolute inset-0 flex items-center justify-end transition-opacity duration-150 ${
                  actionsOpen
                    ? 'opacity-100'
                    : 'pointer-events-none opacity-0 group-hover:pointer-events-auto group-hover:opacity-100 group-focus-within:pointer-events-auto group-focus-within:opacity-100'
                }`}
              >
                {showActionsMenu ? (
                  <ConversationActionsMenu
                    workspaceId={workspaceId}
                    conversation={conversation}
                    moveOptions={availableMoveOptions}
                    open={actionsOpen}
                    onOpenChange={handleActionsOpenChange}
                    onSubjectDialogOpenChange={setActionsDialogOpen}
                    align="end"
                    trigger={actionButton}
                  />
                ) : actionButton}
              </div>
            </div>
          </div>

          {/* Context: message preview + activity */}
          <div className="grid items-center gap-1.5 mt-0.5" style={{ gridTemplateColumns: '1fr auto' }}>
            <p
              className={`flex min-w-0 items-center gap-1 text-sm m-0 leading-[18px] ${visualState.usesUnreadTypography ? 'font-medium text-foreground/80' : 'text-muted-foreground'}`}
              style={{ maxHeight: '18px' }}
            >
              {isCustomerTyping ? (
                <span className="min-w-0 truncate italic text-muted-foreground">{typingState || 'typing…'}</span>
              ) : isAgentTyping ? (
                <span className="min-w-0 truncate italic text-blue-600/70 dark:text-blue-400/70">
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
                  <span className="min-w-0 truncate text-muted-foreground">{draftContent}</span>
                </>
              ) : isNotePreview(conversation.last_message) ? (
                <>
                  <span className="font-medium text-amber-600 dark:text-amber-400">Note: </span>
                  <span className="min-w-0 truncate text-muted-foreground">{stripNotePrefix(conversation.last_message ?? '')}</span>
                </>
              ) : (
                <>
                  {hasAgentReplyPreview ? (
                    <AgentReplyIndicator label={agentReplyLabel} />
                  ) : hasCustomerReplyPreview ? (
                    <CustomerReplyIndicator />
                  ) : null}
                  <span className="min-w-0 truncate">{conversation.last_message || conversation.subject}</span>
                </>
              )}
            </p>

            {/* Activity indicators or status icon */}
            <div className="flex shrink-0 items-center gap-1">
              {hasAIHandoff && <AIHandoffIndicator />}
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
                <span className="inline-flex h-[18px] min-w-[18px] items-center justify-center rounded-full bg-red-500 px-1 text-[10px] font-semibold text-white animate-in zoom-in-75 duration-200">
                  {unreadCount > 99 ? '99+' : unreadCount}
                </span>
              ) : viewingAgentIds.length > 0 ? (
                <div className="flex items-center flex-row-reverse">
                  {viewingAgentIds.map((uid) => (
                    <AgentAvatar key={uid} userId={uid} tooltip="viewing" />
                  ))}
                </div>
              ) : hasAIResolved ? (
                <AIResolvedIndicator />
              ) : conversation.status === 'resolved' ? (
                <Tooltip>
                  <TooltipTrigger asChild>
                    <CheckmarkCircle02Icon className="h-5 w-5 text-green-500" />
                  </TooltipTrigger>
                  <TooltipContent side="left"><span className="text-xs">Resolved</span></TooltipContent>
                </Tooltip>
              ) : null}
            </div>
          </div>
          {isWaitingForHuman && humanQueueWaitSince ? (
            <div className="mt-1 flex min-w-0 items-center gap-1 overflow-hidden">
              <HumanQueueBadge waitSince={humanQueueWaitSince} />
            </div>
          ) : null}
          <UserTagStrip tags={userTags} />
        </div>
      </div>
    </div>
  );
});
