import { useEffect, useRef, useState, useMemo, memo } from 'react';
import { useLocation, useNavigate } from '@tanstack/react-router';
import { toast } from 'sonner';
import { Message01Icon, BotIcon, Loading01Icon, CheckmarkCircle02Icon, CancelCircleIcon, MoreHorizontalIcon } from '@/lib/icons';
import { Badge } from '@/components/ui/badge';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Button } from '@/components/ui/button';
import {
  useConversation,
  useConversationMessages,
  useChatSettings,
  useInboxScopes,
  useSupportTeammatePresence,
  useUpdateConversationStatus,
  useRunConversationAgent,
  useCreateTaskFromConversation,
  useMoveConversation,
  useDismissConversationTriage,
  useDeleteSupportMessage,
} from '@/hooks/queries/useSupport';
import { useWorkspaceAccess, useUpdateSupportTaskPreferences } from '@/hooks/queries/useSession';
import { useWorkspaceSettings } from '@/hooks/queries/useSettings';
import { useWorkspaceMembers } from '@/hooks/queries/useWorkspaces';
import { CreateTaskDialog } from './CreateTaskDialog';
import { agentService } from '@/lib/services/agentService';
// supportService import kept for non-presence HTTP calls
import { type AgentTypingState, useSupportPresenceStore } from '@/stores/supportPresenceStore';
import { useSupportInboxStore } from '@/stores/supportInboxStore';
import { useAuthStore } from '@/stores/authStore';
import { resolveTeamMemberAvatarSrc } from '@/lib/teamMemberAvatar';
import type { AgentRun, SupportMessage, ConversationStatus } from '@/lib/pmTypes';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { openTaskRoute } from '@/components/pm/task-detail/taskRouteNavigation';
import { getDayLabel, getEffectiveSenderType, isSameDay, getInitial } from './helpers';
import { MessageBubble } from './MessageBubble';
import { ReplyComposer } from './ReplyComposer';
import { EmptyState } from './EmptyState';
import { AgentRunsCard } from './AgentRunsCard';
import { ConversationActionsMenu } from './ConversationActionsMenu';
import { SupportInboxOnboarding } from './SupportInboxOnboarding';

interface MessageThreadProps {
  workspaceId: string;
  conversationId: string | null;
  showInboxOnboarding?: boolean;
  onWidgetSettingsClick?: () => void;
  onCreateConversationClick?: () => void;
}

const INITIAL_THREAD_ITEM_COUNT = 60;
const THREAD_HISTORY_HYDRATION_DELAY_MS = 120;
const RESTORE_SUPPORT_DRAFT_EVENT = 'support:restore-draft';

function TypingIndicatorBar({ conversationId }: { conversationId: string | null }) {
  const typingState = useSupportPresenceStore(
    (s) => (conversationId ? s.typingIndicators[conversationId] : false)
  );
  if (typingState === false || typingState === undefined) return null;
  return (
    <div className="flex justify-start mt-2 animate-in fade-in slide-in-from-left-2 duration-200">
      {/* Avatar placeholder matching customer bubble layout */}
      <div className="mr-2 flex w-7 shrink-0 flex-col justify-end">
        <div className="flex h-7 w-7 items-center justify-center rounded-full bg-blue-50 dark:bg-blue-950/30 shadow-sm">
          <span className="flex gap-0.5 text-sm leading-none text-blue-400">
            <span className="animate-bounce [animation-delay:0ms]">·</span>
            <span className="animate-bounce [animation-delay:150ms]">·</span>
            <span className="animate-bounce [animation-delay:300ms]">·</span>
          </span>
        </div>
      </div>
      <div className="max-w-[70%]">
        <div className="rounded-2xl rounded-bl-sm bg-blue-50 dark:bg-blue-950/30 px-3.5 py-2 text-sm leading-relaxed text-blue-900 dark:text-blue-100">
          {typingState ? (
            <p className="whitespace-pre-wrap italic opacity-60">{typingState}</p>
          ) : (
            <p className="italic opacity-50">typing…</p>
          )}
        </div>
      </div>
    </div>
  );
}

function AgentTypingBubble({ conversationId, workspaceId }: { conversationId: string | null; workspaceId: string }) {
  const agentTypingMap = useSupportPresenceStore(
    (s) => (conversationId ? s.agentTyping[conversationId] : undefined)
  );
  const { data: members = [] } = useWorkspaceMembers(workspaceId);

  const entries = agentTypingMap ? Object.entries(agentTypingMap) : [];
  if (entries.length === 0) return null;

  return (
    <>
      {entries.map(([actorId, typing]) => {
        const member = members.find((m) => m.user_id === actorId);
        const { name, avatarUrl } = resolveAgentIdentity(member, typing);
        return (
          <div key={actorId} className="flex justify-end mt-2 animate-in fade-in slide-in-from-right-2 duration-200">
            <div className="max-w-[70%]">
              <div className="mb-1 pr-1 text-right">
                <span className="text-[11px] font-medium text-blue-600/70 dark:text-blue-400/70">
                  {name} is typing
                  <span className="inline-flex ml-0.5">
                    <span className="animate-bounce [animation-delay:0ms] [animation-duration:1s]">.</span>
                    <span className="animate-bounce [animation-delay:200ms] [animation-duration:1s]">.</span>
                    <span className="animate-bounce [animation-delay:400ms] [animation-duration:1s]">.</span>
                  </span>
                </span>
              </div>
              <div className="rounded-2xl rounded-br-sm bg-blue-100/60 px-3.5 py-2 text-sm leading-relaxed text-blue-600/70 dark:bg-blue-900/20 dark:text-blue-300/70">
                {typing.content ? (
                  <p className="whitespace-pre-wrap italic opacity-70">{typing.content}</p>
                ) : (
                  <span className="flex items-center gap-1.5 italic opacity-50">
                    <span className="flex gap-0.5">
                      <span className="animate-bounce [animation-delay:0ms]">·</span>
                      <span className="animate-bounce [animation-delay:150ms]">·</span>
                      <span className="animate-bounce [animation-delay:300ms]">·</span>
                    </span>
                    typing…
                  </span>
                )}
              </div>
            </div>
            <div className="ml-2 flex w-7 shrink-0 flex-col justify-end">
              {avatarUrl ? (
                <img src={avatarUrl} alt={name} title={name} className="h-7 w-7 rounded-full object-cover shadow-sm" />
              ) : (
                <div className="flex h-7 w-7 items-center justify-center rounded-full bg-blue-600 text-[11px] font-semibold text-white shadow-sm" title={name}>
                  {getInitial(name)}
                </div>
              )}
            </div>
          </div>
        );
      })}
    </>
  );
}

function resolveAgentIdentity(
  member: { full_name?: string | null; email?: string | null; avatar_url?: string | null; avatar_style?: string | null; avatar_seed?: string | null; avatar_background_mode?: string | null; avatar_background_color?: string | null } | undefined,
  typing: AgentTypingState
) {
  const name = typing.name || member?.full_name || member?.email || 'Agent';
  return {
    name,
    avatarUrl: typing.avatarUrl || resolveTeamMemberAvatarSrc({
      avatarUrl: member?.avatar_url,
      avatarStyle: member?.avatar_style,
      avatarSeed: member?.avatar_seed,
      avatarBackgroundMode: member?.avatar_background_mode,
      avatarBackgroundColor: member?.avatar_background_color,
      fallbackSeed: name,
    }),
  };
}

function DaySeparator({
  label,
  isSticky,
  separatorRef,
}: {
  label: string;
  isSticky: boolean;
  separatorRef?: (node: HTMLDivElement | null) => void;
}) {
  return (
    <div ref={separatorRef} className="sticky top-0 z-[1] my-5 flex items-center gap-3">
      <div className="h-px flex-1 bg-border/60" aria-hidden />
      <span
        className={`shrink-0 rounded-full px-3 py-0.5 text-[10.5px] font-medium text-muted-foreground/70 ${
          isSticky ? 'bg-white dark:bg-background' : 'bg-muted'
        }`}
        style={{ border: 'none', boxShadow: 'none', outline: 'none' }}
      >
        {label}
      </span>
      <div className="h-px flex-1 bg-border/60" aria-hidden />
    </div>
  );
}

/** Skeleton message bubbles shown while loading */
const MessageSkeleton = memo(function MessageSkeleton() {
  return (
    <div className="space-y-6 py-8">
      {/* Customer message group */}
      <div className="flex items-end gap-2" style={{ width: '55%' }}>
        <div className="h-7 w-7 shrink-0 animate-pulse rounded-full bg-muted" />
        <div className="flex flex-1 flex-col gap-1.5">
          <div className="h-10 animate-pulse rounded-2xl rounded-bl-sm bg-muted" />
          <div className="h-6 w-3/4 animate-pulse rounded-2xl bg-muted" />
        </div>
      </div>
      {/* Agent message group */}
      <div className="flex items-end gap-2 self-end ml-auto" style={{ width: '60%' }}>
        <div className="flex flex-1 flex-col items-end gap-1.5">
          <div className="h-14 w-full animate-pulse rounded-2xl rounded-br-sm bg-blue-100 dark:bg-blue-900/20" />
          <div className="h-8 w-4/5 animate-pulse rounded-2xl bg-blue-100 dark:bg-blue-900/20" />
        </div>
        <div className="h-7 w-7 shrink-0 animate-pulse rounded-full bg-blue-100 dark:bg-blue-900/20" />
      </div>
      {/* Another customer message */}
      <div className="flex items-end gap-2" style={{ width: '45%' }}>
        <div className="h-7 w-7 shrink-0 animate-pulse rounded-full bg-muted" />
        <div className="flex flex-1 flex-col gap-1.5">
          <div className="h-8 animate-pulse rounded-2xl rounded-bl-sm bg-muted" />
        </div>
      </div>
    </div>
  );
});

export function MessageThread({
  workspaceId,
  conversationId,
  showInboxOnboarding,
  onWidgetSettingsClick,
  onCreateConversationClick,
}: MessageThreadProps) {
  const navigate = useNavigate();
  const location = useLocation();
  const workspaceSlug = useWorkspaceStore((s) => s.currentWorkspace?.slug ?? '');
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const scrollAreaRef = useRef<HTMLDivElement>(null);
  const separatorRefs = useRef(new Map<number, HTMLDivElement>());
  const { data: conversation, isFetched: conversationFetched } = useConversation(workspaceId, conversationId);
  const { data: messages = [], isLoading } = useConversationMessages(workspaceId, conversationId);
  const { data: inboxScopes } = useInboxScopes(workspaceId);
  const { data: installation } = useChatSettings(workspaceId);
  useSupportTeammatePresence(workspaceId);
  const { data: members = [] } = useWorkspaceMembers(workspaceId);
  const updateStatus = useUpdateConversationStatus(workspaceId);
  const runAgent = useRunConversationAgent(workspaceId);
  const createTaskFromConversation = useCreateTaskFromConversation(workspaceId);
  const moveConversation = useMoveConversation(workspaceId);
  const dismissTriage = useDismissConversationTriage(workspaceId);
  const deleteMessage = useDeleteSupportMessage(workspaceId, conversationId);
  const { data: access } = useWorkspaceAccess(workspaceId);
  const { data: wsSettings } = useWorkspaceSettings(workspaceId);
  const updatePreferences = useUpdateSupportTaskPreferences(workspaceId);
  const currentUser = useAuthStore((s) => s.user);
  const setSelectedMailboxId = useSupportInboxStore((s) => s.setSelectedMailboxId);

  const [showCreateTaskDialog, setShowCreateTaskDialog] = useState(false);
  const [agentRuns, setAgentRuns] = useState<AgentRun[]>([]);
  const [activeStickySeparator, setActiveStickySeparator] = useState<number | null>(null);
  const [composerReady, setComposerReady] = useState(false);
  const [historyHydrated, setHistoryHydrated] = useState(true);
  const assignedAgentId = conversation?.assigned_agent_id ?? null;
  const memberAvatarByUserId = useMemo(() => {
    const map = new Map<string, string>();
    for (const member of members) {
      const avatarSrc = resolveTeamMemberAvatarSrc({
        avatarUrl: member.avatar_url,
        avatarStyle: member.avatar_style,
        avatarSeed: member.avatar_seed,
        avatarBackgroundMode: member.avatar_background_mode,
        avatarBackgroundColor: member.avatar_background_color,
        fallbackSeed: member.full_name ?? member.email,
      });
      if (member.user_id && avatarSrc) {
        map.set(member.user_id, avatarSrc);
      }
    }
    return map;
  }, [members]);

  const moveOptions = useMemo(() => {
    const options = [inboxScopes?.shared_inbox, ...(inboxScopes?.mailboxes ?? [])].filter(Boolean);
    return options.filter((option) => option!.id !== (conversation?.mailbox_id ?? 'shared'));
  }, [conversation?.mailbox_id, inboxScopes]);

  const triageBanner = useMemo(() => {
    const triage = conversation?.triage;
    if (!conversation || !triage || !triage.suggested_mailbox_id) return null;

    const suggestedMailboxId = triage.suggested_mailbox_id;
    const suggestedMailbox = [inboxScopes?.shared_inbox, ...(inboxScopes?.mailboxes ?? [])]
      .filter(Boolean)
      .find((option) => option!.id === suggestedMailboxId);

    const mailboxName = suggestedMailbox?.name ?? 'Selected Inbox';
    const sourceLabel = triage.classifier_source === 'rule' ? 'Routing Rule' : 'AI Triage';
    const confidenceLabel = triage.confidence != null
      ? `${Math.round(triage.confidence * 100)}% confidence`
      : null;

    if (
      triage.status === 'suggested'
      && !triage.locked_at
      && suggestedMailboxId !== (conversation.mailbox_id ?? null)
    ) {
      return {
        kind: 'suggested' as const,
        mailboxId: suggestedMailboxId,
        mailboxName,
        sourceLabel,
        confidenceLabel,
        reason: triage.reason?.trim() || null,
      };
    }

    if (triage.status === 'auto_moved') {
      return {
        kind: 'auto_moved' as const,
        mailboxId: suggestedMailboxId,
        mailboxName,
        sourceLabel,
        confidenceLabel,
        reason: triage.reason?.trim() || null,
      };
    }

    return null;
  }, [conversation, inboxScopes]);

  useEffect(() => {
    let cancelled = false;

    if (!conversationId || !assignedAgentId) {
      queueMicrotask(() => {
        if (!cancelled) {
          setAgentRuns([]);
        }
      });
      return () => {
        cancelled = true;
      };
    }

    void (async () => {
      const res = await agentService.listRuns(workspaceId, assignedAgentId);
      if (cancelled || res.error) return;
      setAgentRuns(
        (res.data?.data ?? []).filter(
          (run) => run.target_type === 'support_conversation' && run.target_id === conversationId
        )
      );
    })();

    return () => {
      cancelled = true;
    };
  }, [workspaceId, conversationId, assignedAgentId]);

  const isVisitorOnline = useSupportPresenceStore((s) =>
    conversation?.anonymous_id ? !!s.onlineVisitors[conversation.anonymous_id] : false
  );

  const emailFallbackHint = useMemo(() => {
    const settings = installation?.settings;
    const email = conversation?.customer_email?.trim();
    if (!conversation || !settings?.email_fallback_enabled || !email) return null;
    if (conversation.email_unsubscribed) return null;
    if (conversation.status === 'resolved' || conversation.status === 'spam') return null;
    if (conversation.anonymous_id && isVisitorOnline) return null;

    return {
      email,
    };
  }, [conversation, installation, isVisitorOnline]);

  useEffect(() => {
    const handleAgentRunEvent = (event: Event) => {
      const detail = (event as CustomEvent<{ parent_type?: string; parent_id?: string }>).detail;
      if (detail?.parent_type !== 'support_conversation' || detail.parent_id !== conversationId || !assignedAgentId) return;

      void (async () => {
        const res = await agentService.listRuns(workspaceId, assignedAgentId);
        if (res.error) return;
        setAgentRuns(
          (res.data?.data ?? []).filter(
            (run) => run.target_type === 'support_conversation' && run.target_id === conversationId
          )
        );
      })();
    };

    window.addEventListener('agent_run-updated', handleAgentRunEvent);
    return () => {
      window.removeEventListener('agent_run-updated', handleAgentRunEvent);
    };
  }, [workspaceId, conversationId, assignedAgentId]);

  // Broadcast viewing presence via WebSocket (server tracks state, cleans up on disconnect)
  const wsSend = useSupportPresenceStore((s) => s.wsSend);
  const wsConnected = useSupportPresenceStore((s) => s.wsConnected);
  useEffect(() => {
    if (!conversationId || !wsSend || !wsConnected) return;
    wsSend('support:viewing:start', { conversation_id: conversationId });
    return () => {
      wsSend('support:viewing:stop', { conversation_id: conversationId });
    };
  }, [conversationId, wsSend, wsConnected]);

  useEffect(() => {
    const handleKeyDown = async (event: KeyboardEvent) => {
      if (!(event.metaKey || event.ctrlKey) || event.key.toLowerCase() !== 'z' || event.shiftKey || event.altKey) return;
      const target = event.target as HTMLElement | null;
      if (target?.closest('input, textarea, [contenteditable="true"]')) return;
      if (!conversationId || deleteMessage.isPending) return;

      const now = Date.now();
      const latest = [...messages].reverse().find((message) => {
        if (message.sender_type !== 'user' || message.sender_user_id !== currentUser?.id || message.is_internal) return false;
        if (!message.cancellable_until) return false;
        return Date.parse(message.cancellable_until) > now;
      });
      if (!latest) return;

      event.preventDefault();
      const result = await deleteMessage.mutateAsync({ messageId: latest.id, undo: true });
      if (result.markdown) {
        window.dispatchEvent(new CustomEvent(RESTORE_SUPPORT_DRAFT_EVENT, {
          detail: { conversationId, markdown: result.markdown },
        }));
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [conversationId, currentUser?.id, deleteMessage, messages]);

  useEffect(() => {
    if (!conversationId) {
      setComposerReady(false);
      return;
    }

    setComposerReady(false);
    let timeout = 0;
    const frame = window.requestAnimationFrame(() => {
      timeout = window.setTimeout(() => setComposerReady(true), 0);
    });

    return () => {
      window.cancelAnimationFrame(frame);
      if (timeout) window.clearTimeout(timeout);
    };
  }, [conversationId]);

  useEffect(() => {
    if (messages.length === 0) return;
    const frame = window.requestAnimationFrame(() => {
      messagesEndRef.current?.scrollIntoView({ block: 'end' });
    });
    return () => window.cancelAnimationFrame(frame);
  }, [messages]);

  const handleApproveRun = async (runId: string) => {
    await agentService.approveRun(workspaceId, runId, { send_message: true });
    if (!conversationId || !assignedAgentId) {
      setAgentRuns([]);
      return;
    }

    const res = await agentService.listRuns(workspaceId, assignedAgentId);
    if (res.error) return;
    setAgentRuns(
      (res.data?.data ?? []).filter(
        (run) => run.target_type === 'support_conversation' && run.target_id === conversationId
      )
    );
  };

  const handleCreateTask = async () => {
    if (!conversationId || !workspaceSlug) return;

    const dismissed = access?.membership?.support_task_dialog_dismissed;
    const savedTeamId = access?.membership?.support_default_team_id;

    if (!dismissed) {
      setShowCreateTaskDialog(true);
      return;
    }

    const created = await createTaskFromConversation.mutateAsync({
      conversationId,
      teamId: savedTeamId,
    });
    toast.success(`Created ${created.task_key ?? 'task'}`, {
      description: created.summary || created.task_name,
    });
    openTaskRoute(navigate as never, location as never, workspaceSlug, created.task_id);
  };

  const handleCreateTaskConfirm = async (teamId: string, dismissDialog: boolean) => {
    if (!conversationId || !workspaceSlug) return;

    // Create task first — only persist preferences after success
    const created = await createTaskFromConversation.mutateAsync({
      conversationId,
      teamId,
    });

    // Task succeeded — now save team preference and dismissal
    await updatePreferences.mutateAsync({
      support_default_team_id: teamId,
      support_task_dialog_dismissed: dismissDialog,
    });

    setShowCreateTaskDialog(false);
    toast.success(`Created ${created.task_key ?? 'task'}`, {
      description: created.summary || created.task_name,
    });
    openTaskRoute(navigate as never, location as never, workspaceSlug, created.task_id);
  };

  // Group messages with day separators and consecutive sender detection
  // Find the last outbound reply message (for read receipt display)
  const receiptMessageId = useMemo(() => {
    for (let i = messages.length - 1; i >= 0; i--) {
      const msg = messages[i];
      if (!msg.is_internal && msg.sender_type !== 'customer' && (!msg.message_type || msg.message_type === 'reply')) {
        return msg.id;
      }
    }
    return null;
  }, [messages]);

  // Derive delivered/read status from conversation's contact_last_seen_at cursor
  const receiptStatus = useMemo<'delivered' | 'delivered_email' | 'read' | 'read_email' | null>(() => {
    if (!receiptMessageId || !conversation) return null;
    const msg = messages.find((m) => m.id === receiptMessageId);
    if (!msg) return null;
    if (msg.email_read_at) return 'read_email';
    const seen = conversation.contact_last_seen_at;
    if (conversation.source === 'widget' && seen && new Date(seen) >= new Date(msg.created_at)) return 'read';
    if (msg.email_notified_at) return 'delivered_email';
    if (conversation.source === 'widget') return 'delivered';
    return null;
  }, [receiptMessageId, conversation, messages]);

  const groupedMessages = useMemo(() => {
    const items: Array<{ type: 'separator'; label: string } | { type: 'message'; message: SupportMessage; isConsecutive: boolean; isLastInGroup: boolean }> = [];
    let lastDate: string | null = null;

    messages.forEach((msg, idx) => {
      // Insert day separator if new day
      if (!lastDate || !isSameDay(lastDate, msg.created_at)) {
        items.push({ type: 'separator', label: getDayLabel(msg.created_at) });
        lastDate = msg.created_at;
      }

      // Check if consecutive (same sender within 2 minutes, same type)
      const prev = idx > 0 ? messages[idx - 1] : null;
      const currentSenderType = getEffectiveSenderType(msg);
      const prevSenderType = prev ? getEffectiveSenderType(prev) : null;
      const isConsecutive = prev !== null
        && prevSenderType === currentSenderType
        && prev.is_internal === msg.is_internal
        && isSameDay(prev.created_at, msg.created_at)
        && (new Date(msg.created_at).getTime() - new Date(prev.created_at).getTime()) < 120000;

      // Check if this is the last message in a consecutive group
      const next = idx < messages.length - 1 ? messages[idx + 1] : null;
      const nextSenderType = next ? getEffectiveSenderType(next) : null;
      const isLastInGroup = next === null
        || nextSenderType !== currentSenderType
        || next.is_internal !== msg.is_internal
        || !isSameDay(msg.created_at, next.created_at)
        || (new Date(next.created_at).getTime() - new Date(msg.created_at).getTime()) >= 120000;

      items.push({ type: 'message', message: msg, isConsecutive, isLastInGroup });
    });

    return items;
  }, [messages]);

  useEffect(() => {
    if (!conversationId || groupedMessages.length <= INITIAL_THREAD_ITEM_COUNT) {
      setHistoryHydrated(true);
      return;
    }

    setHistoryHydrated(false);
    let timeout = 0;
    const frame = window.requestAnimationFrame(() => {
      timeout = window.setTimeout(() => setHistoryHydrated(true), THREAD_HISTORY_HYDRATION_DELAY_MS);
    });

    return () => {
      window.cancelAnimationFrame(frame);
      if (timeout) window.clearTimeout(timeout);
    };
  }, [conversationId, groupedMessages.length]);

  const visibleGroupedMessages = useMemo(() => {
    if (historyHydrated || groupedMessages.length <= INITIAL_THREAD_ITEM_COUNT) {
      return groupedMessages;
    }

    const start = Math.max(0, groupedMessages.length - INITIAL_THREAD_ITEM_COUNT);
    let firstSeparatorBeforeWindow: (typeof groupedMessages)[number] | undefined;
    for (let i = start - 1; i >= 0; i -= 1) {
      if (groupedMessages[i]?.type === 'separator') {
        firstSeparatorBeforeWindow = groupedMessages[i];
        break;
      }
    }
    const visibleItems = groupedMessages.slice(start);

    if (firstSeparatorBeforeWindow && visibleItems[0]?.type !== 'separator') {
      return [firstSeparatorBeforeWindow, ...visibleItems];
    }

    return visibleItems;
  }, [groupedMessages, historyHydrated]);

  useEffect(() => {
    const viewport = scrollAreaRef.current?.querySelector('[data-slot="scroll-area-viewport"]') as HTMLDivElement | null;
    if (!viewport) return;

    let frame = 0;
    const updateActiveStickySeparator = () => {
      frame = 0;
      const scrollTop = viewport.scrollTop;
      let nextActive: number | null = null;

      for (const [index, node] of separatorRefs.current.entries()) {
        if (node.offsetTop < scrollTop) {
          if (nextActive === null || index > nextActive) {
            nextActive = index;
          }
        }
      }

      setActiveStickySeparator((current) => (current === nextActive ? current : nextActive));
    };

    const onScroll = () => {
      if (frame) return;
      frame = window.requestAnimationFrame(updateActiveStickySeparator);
    };

    updateActiveStickySeparator();
    viewport.addEventListener('scroll', onScroll, { passive: true });

    return () => {
      viewport.removeEventListener('scroll', onScroll);
      if (frame) {
        window.cancelAnimationFrame(frame);
      }
    };
  }, [visibleGroupedMessages]);

  // Treat a stale conversation id (e.g., previous selection that no longer
  // matches the active filter, or a deleted conversation) the same as no
  // selection. Wait until the fetch settled so we don't flash during load.
  const noSelection = !conversationId || (conversationFetched && !conversation);
  if (noSelection) {
    if (showInboxOnboarding && onWidgetSettingsClick && onCreateConversationClick) {
      return (
        <SupportInboxOnboarding
          onWidgetSettingsClick={onWidgetSettingsClick}
          onCreateConversationClick={onCreateConversationClick}
        />
      );
    }

    return (
      <EmptyState
        icon={Message01Icon}
        title="Select a conversation"
        subtitle="Pick one from the list to view messages and reply."
        background="muted"
      />
    );
  }

  return (
    <div className="flex flex-1 flex-col min-w-0 min-h-0">
      {/* Topbar with subtle bottom shadow (Crisp-style) */}
      {conversation && (
        <div className="relative z-10 flex items-center justify-between border-b px-4 py-2.5 bg-background">
          {/* Gradient shadow below topbar */}
          <div className="absolute top-full left-0 right-0 h-1.5 bg-gradient-to-b from-black/[0.025] to-transparent pointer-events-none" />

          <div className="flex items-center gap-2 min-w-0">
            <div className="min-w-0">
              <h2 className="truncate text-sm font-medium text-foreground">
                <span className="font-semibold text-muted-foreground">#{conversation.display_id}</span>
                <span className="mx-1 text-muted-foreground">-</span>
                <span>{conversation.subject}</span>
              </h2>
            </div>
          </div>

          <div className="flex items-center gap-1.5 shrink-0">
            <Button
              size="sm"
              variant="outline"
              className="h-7 gap-1 text-xs"
              disabled={createTaskFromConversation.isPending}
              onClick={handleCreateTask}
            >
              {createTaskFromConversation.isPending ? <Loading01Icon className="h-3 w-3 animate-spin" /> : <CheckmarkCircle02Icon className="h-3.5 w-3.5" />}
              Create Task
            </Button>

            {/* Run Agent */}
            {conversation.assigned_agent_id && (
              <Button
                size="sm"
                variant="outline"
                className="h-7 gap-1 text-xs"
                disabled={runAgent.isPending}
                onClick={() => runAgent.mutate(conversation.id)}
              >
                {runAgent.isPending ? <Loading01Icon className="h-3 w-3 animate-spin" /> : <BotIcon className="h-3 w-3" />}
                Run
              </Button>
            )}

            {/* Resolve / Unresolve */}
            {conversation.status === 'resolved' ? (
              <Button
                size="sm"
                variant="outline"
                className="h-7 gap-1 border-red-200 bg-red-50 text-red-700 hover:bg-red-100 hover:text-red-800 dark:border-red-900/60 dark:bg-red-950/30 dark:text-red-300 dark:hover:bg-red-950/50"
                onClick={() => updateStatus.mutate({ conversationId: conversation.id, status: 'open' as ConversationStatus })}
              >
                <CancelCircleIcon className="h-3.5 w-3.5" />
                Unresolve
              </Button>
            ) : (
              <Button
                size="sm"
                variant="default"
                className="h-7 gap-1 bg-emerald-600 text-white hover:bg-emerald-700 dark:bg-emerald-600 dark:hover:bg-emerald-700"
                onClick={() => updateStatus.mutate({ conversationId: conversation.id, status: 'resolved' as ConversationStatus })}
              >
                <CheckmarkCircle02Icon className="h-3.5 w-3.5" />
                Resolve
              </Button>
            )}

            {/* More actions */}
            <ConversationActionsMenu
              workspaceId={workspaceId}
              conversation={conversation}
              moveOptions={moveOptions.map((option) => ({ id: option!.id, name: option!.name }))}
              align="end"
              onConversationMoved={(option) => {
                setSelectedMailboxId(option.id);
              }}
              onConversationDeleted={() => {
                if (!workspaceSlug) return;
                void navigate({ to: '/w/$slug/support', params: { slug: workspaceSlug }, replace: true });
              }}
              trigger={(
                <Button variant="ghost" size="sm" className="h-7 w-7 p-0" aria-label="Open conversation actions">
                  <MoreHorizontalIcon className="h-4 w-4" />
                </Button>
              )}
            />
          </div>
        </div>
      )}

      {conversation && triageBanner && (
        <div className={`border-b px-4 py-3 ${
          triageBanner.kind === 'suggested'
            ? 'bg-amber-50/70 dark:bg-amber-950/20'
            : 'bg-emerald-50/70 dark:bg-emerald-950/20'
        }`}>
          <div className="flex items-start justify-between gap-4">
            <div className="min-w-0">
              <div className="flex flex-wrap items-center gap-2">
                <p className="text-sm font-medium">
                  {triageBanner.kind === 'suggested'
                    ? `Suggested inbox: ${triageBanner.mailboxName}`
                    : `Moved to ${triageBanner.mailboxName}`}
                </p>
                <Badge variant="secondary">{triageBanner.sourceLabel}</Badge>
                {triageBanner.confidenceLabel && <Badge variant="outline">{triageBanner.confidenceLabel}</Badge>}
              </div>
              {triageBanner.reason && (
                <p className="mt-1 text-sm text-muted-foreground">
                  Reason: {triageBanner.reason}
                </p>
              )}
            </div>

            {triageBanner.kind === 'suggested' && (
              <div className="flex shrink-0 items-center gap-2">
                <Button
                  size="sm"
                  onClick={() => {
                    moveConversation.mutate({
                      conversationId: conversation.id,
                      mailboxId: triageBanner.mailboxId,
                    }, {
                      onSuccess: () => {
                        setSelectedMailboxId(triageBanner.mailboxId);
                        toast.success(`Moved to ${triageBanner.mailboxName}`);
                      },
                    });
                  }}
                  disabled={moveConversation.isPending || dismissTriage.isPending}
                >
                  Move
                </Button>
                <Button
                  size="sm"
                  variant="ghost"
                  onClick={() => {
                    dismissTriage.mutate(conversation.id, {
                      onSuccess: () => {
                        toast.success('Routing suggestion dismissed');
                      },
                    });
                  }}
                  disabled={moveConversation.isPending || dismissTriage.isPending}
                >
                  Dismiss
                </Button>
              </div>
            )}
          </div>
        </div>
      )}

      {/* Agent runs — hidden for AI-first conversations (ai_state is set) */}
      {agentRuns.length > 0 && !conversation?.ai_state && (
        <div className="border-b px-4 py-2">
          <AgentRunsCard
            workspaceId={workspaceId}
            agentRuns={agentRuns}
            onApprove={handleApproveRun}
          />
        </div>
      )}

      {/* Messages area with light background (Crisp-style) */}
      <ScrollArea ref={scrollAreaRef} className="flex-1 min-h-0 bg-muted/20">
        <div className="px-4 pb-10 pt-2">
          {isLoading && <MessageSkeleton />}
          {!isLoading && messages.length === 0 && (
            <EmptyState
              icon={Message01Icon}
              title="No messages yet"
              subtitle="Start the conversation using the reply below."
            />
          )}
          {visibleGroupedMessages.map((item, idx) => {
            if (item.type === 'separator') {
              return (
                <DaySeparator
                  key={`sep-${idx}`}
                  label={item.label}
                  isSticky={activeStickySeparator === idx}
                  separatorRef={(node) => {
                    if (node) {
                      separatorRefs.current.set(idx, node);
                    } else {
                      separatorRefs.current.delete(idx);
                    }
                  }}
                />
              );
            }
            return (
              <div
                key={item.message.id}
                className="support-thread-message"
              >
                <MessageBubble
                  message={item.message}
                  isConsecutive={item.isConsecutive}
                  isLastInGroup={item.isLastInGroup}
                  source={conversation?.source}
                  receiptStatus={item.message.id === receiptMessageId ? receiptStatus : undefined}
                  customerDisplayName={conversation?.customer_name || conversation?.customer_email}
                  fallbackAvatarUrl={
                    (item.message.sender_user_id ? memberAvatarByUserId.get(item.message.sender_user_id) : undefined)
                    ?? ((item.message.sender_display_name === currentUser?.full_name || item.message.sender_display_name === currentUser?.email)
                      ? currentUser?.avatar_url
                      : undefined)
                  }
                />
              </div>
            );
          })}
          <TypingIndicatorBar conversationId={conversationId} />
          <AgentTypingBubble conversationId={conversationId} workspaceId={workspaceId} />
          <div ref={messagesEndRef} />
        </div>
      </ScrollArea>

      {/* Soft gradient fade between thread and composer */}
      <div className="pointer-events-none h-3 -mt-3 relative z-10 bg-gradient-to-t from-background to-transparent" />

      {/* Reply composer — show during loading (cache may still populate) and
          after a successful load. Only hide when the fetch settled AND the
          conversation didn't load (stale/deleted id) to avoid offering a
          reply for a conversation that doesn't exist. */}
      {composerReady && conversationId && (conversation || !conversationFetched) && (
        <ReplyComposer
          workspaceId={workspaceId}
          conversationId={conversationId}
          emailFallbackHint={emailFallbackHint}
        />
      )}

      <CreateTaskDialog
        open={showCreateTaskDialog}
        onOpenChange={setShowCreateTaskDialog}
        teams={wsSettings?.teams ?? []}
        defaultTeamId={access?.membership?.support_default_team_id ?? access?.team_memberships?.[0]?.team_id}
        isPending={createTaskFromConversation.isPending}
        onConfirm={handleCreateTaskConfirm}
      />
    </div>
  );
}
