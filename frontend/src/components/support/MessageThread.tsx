import { useEffect, useRef, useState, useMemo, memo } from 'react';
import { toast } from 'sonner';
import { useConfirm } from '@/components/ui/confirm-dialog';
import { Message01Icon, BotIcon, Loading01Icon, MoreHorizontalIcon, CheckmarkCircle02Icon, CancelCircleIcon, Link01Icon, MailOpenIcon, Shield02Icon, Delete01Icon, PencilEdit01Icon } from '@/lib/icons';
import { Badge } from '@/components/ui/badge';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import {
  useConversation,
  useConversationMessages,
  useChatSettings,
  useInboxScopes,
  useSupportTeammatePresence,
  useUpdateConversationStatus,
  useRunConversationAgent,
  useMarkConversationUnread,
  useUpdateConversationSubject,
  useDeleteConversation,
  useMoveConversation,
  useDismissConversationTriage,
} from '@/hooks/queries/useSupport';
import { useWorkspaceMembers } from '@/hooks/queries/useWorkspaces';
import { agentService } from '@/lib/services/agentService';
// supportService import kept for non-presence HTTP calls
import { type AgentTypingState, useSupportPresenceStore } from '@/stores/supportPresenceStore';
import { useSupportInboxStore } from '@/stores/supportInboxStore';
import { useAuthStore } from '@/stores/authStore';
import type { AgentRun, SupportMessage, ConversationStatus } from '@/lib/pmTypes';
import { getDayLabel, getEffectiveSenderType, isSameDay, getInitial } from './helpers';
import { MessageBubble } from './MessageBubble';
import { ReplyComposer } from './ReplyComposer';
import { AgentRunsCard } from './AgentRunsCard';

interface MessageThreadProps {
  workspaceId: string;
  conversationId: string | null;
}

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
  member: { full_name?: string | null; email?: string | null; avatar_url?: string | null } | undefined,
  typing: AgentTypingState
) {
  return {
    name: typing.name || member?.full_name || member?.email || 'Agent',
    avatarUrl: typing.avatarUrl || member?.avatar_url || undefined,
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
    <div ref={separatorRef} className="sticky top-0 z-[1] flex items-center justify-center py-3">
      <span
        className={`relative rounded-full px-3 py-0.5 text-[10.5px] font-medium text-muted-foreground/70 ${
          isSticky ? 'bg-white dark:bg-background' : 'bg-muted'
        }`}
        style={{ border: 'none', boxShadow: 'none', outline: 'none' }}
      >
        {label}
      </span>
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

export function MessageThread({ workspaceId, conversationId }: MessageThreadProps) {
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const scrollAreaRef = useRef<HTMLDivElement>(null);
  const separatorRefs = useRef(new Map<number, HTMLDivElement>());
  const { data: conversation } = useConversation(workspaceId, conversationId);
  const { data: messages = [], isLoading } = useConversationMessages(workspaceId, conversationId);
  const { data: inboxScopes } = useInboxScopes(workspaceId);
  const { data: installation } = useChatSettings(workspaceId);
  useSupportTeammatePresence(workspaceId);
  const { data: members = [] } = useWorkspaceMembers(workspaceId);
  const updateStatus = useUpdateConversationStatus(workspaceId);
  const runAgent = useRunConversationAgent(workspaceId);
  const markUnread = useMarkConversationUnread(workspaceId);
  const updateSubject = useUpdateConversationSubject(workspaceId);
  const deleteConversation = useDeleteConversation(workspaceId);
  const confirm = useConfirm();
  const moveConversation = useMoveConversation(workspaceId);
  const dismissTriage = useDismissConversationTriage(workspaceId);
  const currentUser = useAuthStore((s) => s.user);
  const setSelectedMailboxId = useSupportInboxStore((s) => s.setSelectedMailboxId);

  const [agentRuns, setAgentRuns] = useState<AgentRun[]>([]);
  const [activeStickySeparator, setActiveStickySeparator] = useState<number | null>(null);
  const assignedAgentId = conversation?.assigned_agent_id ?? null;
  const memberAvatarByUserId = useMemo(() => {
    const map = new Map<string, string>();
    for (const member of members) {
      if (member.user_id && member.avatar_url) {
        map.set(member.user_id, member.avatar_url);
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
    if (conversation.status === 'closed' || conversation.status === 'spam') return null;
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
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
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
  }, [groupedMessages]);

  if (!conversationId) {
    return (
      <div className="flex flex-1 flex-col items-center justify-center gap-3 bg-muted/30 text-muted-foreground">
        <Message01Icon className="h-12 w-12 opacity-20" />
        <p className="text-sm">Select a conversation to view</p>
      </div>
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
              <h2 className="text-sm font-semibold text-muted-foreground">#{conversation.display_id}</h2>
            </div>
          </div>

          <div className="flex items-center gap-1.5 shrink-0">
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
                className="h-7 gap-1 text-xs"
                onClick={() => updateStatus.mutate({ conversationId: conversation.id, status: 'open' as ConversationStatus })}
              >
                <CancelCircleIcon className="h-3.5 w-3.5" />
                Unresolve
              </Button>
            ) : (
              <Button
                size="sm"
                variant="default"
                className="h-7 gap-1 text-xs"
                onClick={() => updateStatus.mutate({ conversationId: conversation.id, status: 'resolved' as ConversationStatus })}
              >
                <CheckmarkCircle02Icon className="h-3.5 w-3.5" />
                Resolve
              </Button>
            )}

            {/* More actions */}
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button variant="ghost" size="sm" className="h-7 w-7 p-0">
                  <MoreHorizontalIcon className="h-4 w-4" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                <DropdownMenuItem onClick={() => {
                  markUnread.mutate(conversation.id);
                  toast.success('Marked as unread');
                }}>
                  <MailOpenIcon className="h-4 w-4" />
                  Mark as unread
                </DropdownMenuItem>
                <DropdownMenuItem onClick={() => {
                  navigator.clipboard.writeText(window.location.href);
                  toast.success('Link copied to clipboard');
                }}>
                  <Link01Icon className="h-4 w-4" />
                  Copy link
                </DropdownMenuItem>
                <DropdownMenuItem onClick={() => {
                  const newSubject = window.prompt('Conversation subject:', conversation.subject);
                  if (newSubject !== null && newSubject.trim()) {
                    updateSubject.mutate({ conversationId: conversation.id, subject: newSubject.trim() });
                  }
                }}>
                  <PencilEdit01Icon className="h-4 w-4" />
                  Set conversation subject
                </DropdownMenuItem>
                {moveOptions.length > 0 && (
                  <>
                    <DropdownMenuSeparator />
                    {moveOptions.map((option) => (
                      <DropdownMenuItem
                        key={option!.id}
                        onClick={() => {
                          const nextMailboxId = option!.id === 'shared' ? null : option!.id;
                          moveConversation.mutate({
                            conversationId: conversation.id,
                            mailboxId: nextMailboxId,
                          }, {
                            onSuccess: () => {
                              setSelectedMailboxId(option!.id);
                              toast.success(`Moved to ${option!.name}`);
                            },
                          });
                        }}
                      >
                        <Message01Icon className="h-4 w-4" />
                        Move to {option!.name}
                      </DropdownMenuItem>
                    ))}
                  </>
                )}
                <DropdownMenuSeparator />
                <DropdownMenuItem
                  className="text-destructive focus:text-destructive"
                  onClick={() => {
                    updateStatus.mutate({ conversationId: conversation.id, status: 'spam' as ConversationStatus });
                    toast.success('Conversation marked as spam');
                  }}
                >
                  <Shield02Icon className="h-4 w-4" />
                  Mark as spam
                </DropdownMenuItem>
                <DropdownMenuItem
                  className="text-destructive focus:text-destructive"
                  onClick={async () => {
                    const ok = await confirm({
                      title: 'Delete conversation?',
                      description: 'This will permanently delete this conversation. This action cannot be undone.',
                      confirmText: 'Delete',
                      variant: 'destructive',
                    });
                    if (!ok) return;
                    deleteConversation.mutate(conversation.id, {
                      onSuccess: () => {
                        useSupportInboxStore.getState().selectConversation(null);
                        toast.success('Conversation deleted');
                      },
                    });
                  }}
                >
                  <Delete01Icon className="h-4 w-4" />
                  Delete conversation
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
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
        <div className="px-4 pb-4 pt-2">
          {isLoading && <MessageSkeleton />}
          {!isLoading && messages.length === 0 && (
            <div className="flex flex-col items-center justify-center gap-2 py-12 text-muted-foreground">
              <Message01Icon className="h-8 w-8 opacity-30" />
              <p className="text-sm">No messages yet. Start the conversation below.</p>
            </div>
          )}
          {groupedMessages.map((item, idx) => {
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
              <MessageBubble
                key={item.message.id}
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
            );
          })}
          <TypingIndicatorBar conversationId={conversationId} />
          <AgentTypingBubble conversationId={conversationId} workspaceId={workspaceId} />
          <div ref={messagesEndRef} />
        </div>
      </ScrollArea>

      {/* Reply composer */}
      {conversationId && (
        <ReplyComposer
          workspaceId={workspaceId}
          conversationId={conversationId}
          emailFallbackHint={emailFallbackHint}
        />
      )}
    </div>
  );
}
