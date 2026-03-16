import { useEffect, useRef, useCallback, useState, useMemo } from 'react';
import { MessageSquare, Bot, Loader2, MoreHorizontal, CheckCircle2, Clock, XCircle, User } from 'lucide-react';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import {
  useConversation,
  useConversationMessages,
  useUpdateConversationStatus,
  useRunConversationAgent,
} from '@/hooks/queries/useSupport';
import { useWorkspaceMembers } from '@/hooks/queries/useWorkspaces';
import { agentService } from '@/lib/services/agentService';
import { supportService } from '@/lib/services/supportService';
import { useSupportInboxStore } from '@/stores/supportInboxStore';
import type { AgentRun, SupportMessage, ConversationStatus } from '@/lib/pmTypes';
import { STATUS_COLORS, STATUS_LABELS, PRIORITY_COLORS, PRIORITY_LABELS } from './constants';
import { getDayLabel, isSameDay, getInitial } from './helpers';
import { MessageBubble } from './MessageBubble';
import { ReplyComposer } from './ReplyComposer';
import { AgentRunsCard } from './AgentRunsCard';

interface MessageThreadProps {
  workspaceId: string;
  conversationId: string | null;
}

function TypingIndicatorBar({ conversationId }: { conversationId: string | null }) {
  const typingState = useSupportInboxStore(
    (s) => (conversationId ? s.typingIndicators[conversationId] : false)
  );
  if (typingState === false || typingState === undefined) return null;
  return (
    <div className="flex justify-start mt-2 animate-in fade-in duration-200">
      {/* Avatar placeholder matching customer bubble layout */}
      <div className="mr-2 flex w-7 shrink-0 flex-col justify-end">
        <div className="flex h-7 w-7 items-center justify-center rounded-full bg-muted">
          <span className="flex gap-0.5 text-sm leading-none text-muted-foreground">
            <span className="animate-bounce [animation-delay:0ms]">·</span>
            <span className="animate-bounce [animation-delay:150ms]">·</span>
            <span className="animate-bounce [animation-delay:300ms]">·</span>
          </span>
        </div>
      </div>
      <div className="max-w-[70%]">
        <div className="rounded-2xl rounded-bl-sm bg-muted px-3.5 py-2 text-sm leading-relaxed text-foreground">
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
  const agentTypingMap = useSupportInboxStore(
    (s) => (conversationId ? s.agentTyping[conversationId] : undefined)
  );
  const { data: members = [] } = useWorkspaceMembers(workspaceId);

  const entries = agentTypingMap ? Object.entries(agentTypingMap) : [];
  if (entries.length === 0) return null;

  return (
    <>
      {entries.map(([actorId, content]) => {
        const member = members.find((m) => m.user_id === actorId);
        const name = member?.full_name || member?.email || 'Agent';
        return (
          <div key={actorId} className="flex justify-end mt-2 animate-in fade-in duration-200">
            <div className="max-w-[70%]">
              <div className="mb-1 pr-1 text-right">
                <span className="text-[11px] font-medium text-muted-foreground">{name}</span>
              </div>
              <div className="rounded-2xl rounded-br-sm bg-primary/40 px-3.5 py-2 text-sm leading-relaxed text-primary-foreground">
                {content ? (
                  <p className="whitespace-pre-wrap italic opacity-70">{content}</p>
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
              <div className="flex h-7 w-7 items-center justify-center rounded-full bg-primary/10 text-primary">
                <User className="h-3.5 w-3.5" />
              </div>
            </div>
          </div>
        );
      })}
    </>
  );
}

function DaySeparator({ label }: { label: string }) {
  return (
    <div className="flex items-center gap-3 py-4">
      <div className="h-px flex-1 bg-border" />
      <span className="text-[11px] font-medium text-muted-foreground">{label}</span>
      <div className="h-px flex-1 bg-border" />
    </div>
  );
}

export function MessageThread({ workspaceId, conversationId }: MessageThreadProps) {
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const { data: conversation } = useConversation(workspaceId, conversationId);
  const { data: messages = [], isLoading } = useConversationMessages(workspaceId, conversationId);
  const updateStatus = useUpdateConversationStatus(workspaceId);
  const runAgent = useRunConversationAgent(workspaceId);

  const [agentRuns, setAgentRuns] = useState<AgentRun[]>([]);

  const loadAgentRuns = useCallback(async () => {
    if (!conversationId || !conversation?.assigned_agent_id) {
      setAgentRuns([]);
      return;
    }
    const res = await agentService.listRuns(workspaceId, conversation.assigned_agent_id);
    if (res.error) return;
    setAgentRuns(
      (res.data?.data ?? []).filter(
        (run) => run.target_type === 'support_conversation' && run.target_id === conversationId
      )
    );
  }, [workspaceId, conversationId, conversation?.assigned_agent_id]);

  useEffect(() => {
    loadAgentRuns();
  }, [loadAgentRuns]);

  // Broadcast viewing presence to other agents
  const viewingRef = useRef<string | null>(null);
  useEffect(() => {
    if (!conversationId) return;
    // Prevent re-sending if already viewing this conversation
    if (viewingRef.current === conversationId) return;
    const prevConvId = viewingRef.current;
    viewingRef.current = conversationId;
    // Stop viewing previous conversation
    if (prevConvId) {
      supportService.sendViewingPresence(workspaceId, prevConvId, false).catch(() => {});
    }
    // Start viewing new conversation
    supportService.sendViewingPresence(workspaceId, conversationId, true).then((res) => {
      if (import.meta.env.DEV) console.debug('[viewing] sent viewing:true', conversationId, res.error ? `ERROR: ${res.error}` : 'ok');
    });
    // Heartbeat every 15s so other agents discover presence quickly
    const interval = setInterval(() => {
      supportService.sendViewingPresence(workspaceId, conversationId, true).catch(() => {});
    }, 15_000);
    return () => {
      clearInterval(interval);
      viewingRef.current = null;
      supportService.sendViewingPresence(workspaceId, conversationId, false).catch(() => {});
    };
  }, [workspaceId, conversationId]);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages]);

  const handleApproveRun = async (runId: string) => {
    await agentService.approveRun(workspaceId, runId, { send_message: true });
    await loadAgentRuns();
  };

  // Group messages with day separators and consecutive sender detection
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
      const isConsecutive = prev !== null
        && prev.sender_type === msg.sender_type
        && prev.is_internal === msg.is_internal
        && isSameDay(prev.created_at, msg.created_at)
        && (new Date(msg.created_at).getTime() - new Date(prev.created_at).getTime()) < 120000;

      // Check if this is the last message in a consecutive group
      const next = idx < messages.length - 1 ? messages[idx + 1] : null;
      const isLastInGroup = next === null
        || next.sender_type !== msg.sender_type
        || next.is_internal !== msg.is_internal
        || !isSameDay(msg.created_at, next.created_at)
        || (new Date(next.created_at).getTime() - new Date(msg.created_at).getTime()) >= 120000;

      items.push({ type: 'message', message: msg, isConsecutive, isLastInGroup });
    });

    return items;
  }, [messages]);

  if (!conversationId) {
    return (
      <div className="flex flex-1 flex-col items-center justify-center gap-3 text-muted-foreground">
        <MessageSquare className="h-12 w-12 opacity-20" />
        <p className="text-sm">Select a conversation to view</p>
      </div>
    );
  }

  return (
    <div className="flex flex-1 flex-col min-w-0 min-h-0">
      {/* Action header bar */}
      {conversation && (
        <div className="flex items-center justify-between border-b px-4 py-2.5">
          <div className="flex items-center gap-3 min-w-0">
            {/* Customer avatar */}
            <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-primary/10 text-sm font-semibold text-primary">
              {getInitial(conversation.customer_name)}
            </div>
            <div className="min-w-0">
              <div className="flex items-center gap-2">
                <h2 className="truncate text-sm font-semibold">{conversation.subject}</h2>
                <span className="shrink-0 text-[10px] text-muted-foreground">#{conversation.display_id}</span>
              </div>
              <p className="truncate text-xs text-muted-foreground">
                {conversation.customer_name || 'Anonymous'}
                {conversation.customer_email && ` · ${conversation.customer_email}`}
              </p>
            </div>
          </div>

          <div className="flex items-center gap-1.5 shrink-0">
            <Badge variant="secondary" className={`text-[10px] ${STATUS_COLORS[conversation.status]}`}>
              {STATUS_LABELS[conversation.status]}
            </Badge>
            <Badge variant="secondary" className={`text-[10px] ${PRIORITY_COLORS[conversation.priority]}`}>
              {PRIORITY_LABELS[conversation.priority]}
            </Badge>

            {/* Quick actions */}
            {conversation.assigned_agent_id && (
              <Button
                size="sm"
                variant="outline"
                className="h-7 gap-1 text-xs ml-1"
                disabled={runAgent.isPending}
                onClick={() => runAgent.mutate(conversation.id)}
              >
                {runAgent.isPending ? <Loader2 className="h-3 w-3 animate-spin" /> : <Bot className="h-3 w-3" />}
                Run
              </Button>
            )}

            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button variant="ghost" size="sm" className="h-7 w-7 p-0">
                  <MoreHorizontal className="h-4 w-4" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                <DropdownMenuItem onClick={() => updateStatus.mutate({ conversationId: conversation.id, status: 'resolved' as ConversationStatus })}>
                  <CheckCircle2 className="h-4 w-4" />
                  Resolve
                </DropdownMenuItem>
                <DropdownMenuItem onClick={() => updateStatus.mutate({ conversationId: conversation.id, status: 'waiting' as ConversationStatus })}>
                  <Clock className="h-4 w-4" />
                  Set Waiting
                </DropdownMenuItem>
                <DropdownMenuItem onClick={() => updateStatus.mutate({ conversationId: conversation.id, status: 'closed' as ConversationStatus })}>
                  <XCircle className="h-4 w-4" />
                  Close
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
        </div>
      )}

      {/* Agent runs (if any) */}
      {agentRuns.length > 0 && (
        <div className="border-b px-4 py-2">
          <AgentRunsCard
            workspaceId={workspaceId}
            agentRuns={agentRuns}
            onApprove={handleApproveRun}
          />
        </div>
      )}

      {/* Messages with day separators */}
      <ScrollArea className="flex-1 min-h-0">
        <div className="px-4 pb-4">
          {isLoading && (
            <div className="flex items-center justify-center py-8">
              <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
            </div>
          )}
          {!isLoading && messages.length === 0 && (
            <div className="flex flex-col items-center justify-center gap-2 py-12 text-muted-foreground">
              <MessageSquare className="h-8 w-8 opacity-30" />
              <p className="text-sm">No messages yet. Start the conversation below.</p>
            </div>
          )}
          {groupedMessages.map((item, idx) => {
            if (item.type === 'separator') {
              return <DaySeparator key={`sep-${idx}`} label={item.label} />;
            }
            return (
              <MessageBubble
                key={item.message.id}
                message={item.message}
                isConsecutive={item.isConsecutive}
                isLastInGroup={item.isLastInGroup}
                source={conversation?.source}
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
        <ReplyComposer workspaceId={workspaceId} conversationId={conversationId} />
      )}
    </div>
  );
}
