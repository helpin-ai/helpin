import { useEffect, useMemo, useRef, useState } from 'react'
import { useParams, useRouter } from '@tanstack/react-router'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import {
  isSupportConversationListQueryKey,
  useConversation,
  useConversationMessages,
  useMarkConversationRead,
  useSupportPresenceStore,
  type ConversationListResponse,
  type ConversationStatus,
  type SupportConversation,
} from '@helpin-ai/support-core'
import { TopBar } from '@mobile/ui/top-bar'
import { Pressable } from '@mobile/ui/pressable'
import { Sheet } from '@mobile/ui/sheet'
import { EmptyState } from '@mobile/ui/empty-state'
import { cn } from '@mobile/lib/cn'
import { workspacesService } from '@mobile/lib/services/workspaces-service'
import { displayNameFor } from '@mobile/inbox/conversation-cell'
import { MessageList, type MessageListHandle, type TypingIndicatorState } from '@mobile/thread/message-list'
import { groupMessages } from '@mobile/thread/thread-helpers'
import { MessageCircle } from 'lucide-react'

const STATUS_LABELS: Record<ConversationStatus, string> = {
  open: 'Open',
  waiting_on_customer: 'Waiting on customer',
  resolved: 'Resolved',
  spam: 'Spam',
}

function assigneeLabel(conversation: SupportConversation): string | null {
  if (conversation.assigned_user_id || conversation.assigned_agent_id) return 'Assigned'
  if (conversation.ai_state === 'pending') return 'AI handling'
  return 'Unassigned'
}

/**
 * Reads any already-cached conversation list page (primed by the inbox
 * screen, Task 11) for this id so the header can paint before the dedicated
 * `useConversation` fetch resolves. `useConversation` does not read from the
 * list cache itself (verified: support-core/use-support-core.ts has no
 * `initialData`/`placeholderData` derived from the list query) — without
 * this, the header would show a skeleton on every open even when the
 * conversation was just visible in the inbox list.
 */
function useConversationFromListCache(
  workspaceId: string,
  conversationId: string | null,
): SupportConversation | undefined {
  const queryClient = useQueryClient()
  return useMemo(() => {
    if (!workspaceId || !conversationId) return undefined
    const lists = queryClient.getQueriesData<ConversationListResponse>({
      predicate: (query) => isSupportConversationListQueryKey(query.queryKey, workspaceId),
    })
    for (const [, data] of lists) {
      const found = data?.data?.find((c) => c.id === conversationId)
      if (found) return found
    }
    return undefined
    // eslint-disable-next-line react-hooks/exhaustive-deps -- one-shot cache read, not reactive to later cache writes
  }, [workspaceId, conversationId])
}

export function ConversationScreen() {
  const { slug, conversationId } = useParams({ strict: false })
  const router = useRouter()

  const workspaceQuery = useQuery({
    queryKey: ['workspace', slug],
    queryFn: async () => {
      const { data, error } = await workspacesService.getBySlug(slug ?? '')
      if (error || !data) throw new Error(error ?? 'Failed to load workspace')
      return data
    },
    enabled: !!slug,
  })
  const workspaceId = workspaceQuery.data?.id ?? ''

  const cachedConversation = useConversationFromListCache(workspaceId, conversationId ?? null)
  const conversationQuery = useConversation(workspaceId, conversationId ?? null)
  const conversation = conversationQuery.data ?? cachedConversation

  const messagesQuery = useConversationMessages(workspaceId, conversationId ?? null)
  const messages = useMemo(() => messagesQuery.data ?? [], [messagesQuery.data])
  const items = useMemo(() => groupMessages(messages), [messages])

  const markRead = useMarkConversationRead(workspaceId)
  useEffect(() => {
    if (workspaceId && conversationId) markRead.mutate(conversationId)
    // eslint-disable-next-line react-hooks/exhaustive-deps -- fire once per (workspaceId, conversationId), not on every markRead identity change
  }, [workspaceId, conversationId])

  // Conversation-scoped presence slices: selecting the whole maps would
  // re-render this screen for presence churn in OTHER conversations. Each
  // selector returns the value for THIS conversation only (a boolean,
  // a primitive, and a per-conversation object reference), so zustand's
  // Object.is equality skips unrelated updates.
  const anonymousId = conversation?.anonymous_id
  const visitorOnline = useSupportPresenceStore((s) => !!(anonymousId && s.onlineVisitors[anonymousId]))
  const customerTypingValue = useSupportPresenceStore((s) =>
    conversationId ? s.typingIndicators[conversationId] : undefined,
  )
  const conversationAgentTyping = useSupportPresenceStore((s) =>
    conversationId ? s.agentTyping[conversationId] : undefined,
  )

  const typingIndicator: TypingIndicatorState | null = useMemo(() => {
    if (!conversationId) return null
    if (customerTypingValue !== undefined && customerTypingValue !== false) {
      return { align: 'left', label: `${conversation ? displayNameFor(conversation) : 'Customer'} is typing…` }
    }
    const firstTeammate = conversationAgentTyping ? Object.values(conversationAgentTyping)[0] : undefined
    if (firstTeammate) {
      return { align: 'right', label: `${firstTeammate.name ?? 'A teammate'} is typing…` }
    }
    return null
  }, [conversationId, customerTypingValue, conversationAgentTyping, conversation])

  const messageListRef = useRef<MessageListHandle>(null)
  const [showNewMessagePill, setShowNewMessagePill] = useState(false)
  const [contextSheetOpen, setContextSheetOpen] = useState(false)

  const handleBack = () => {
    if (router.history.canGoBack()) router.history.back()
    else router.navigate({ to: '/w/$slug/support', params: { slug: slug ?? '' } })
  }

  const customerName = conversation ? displayNameFor(conversation) : 'Conversation'
  const subtitle = conversation
    ? [STATUS_LABELS[conversation.status], assigneeLabel(conversation)].filter(Boolean).join(' · ')
    : undefined

  const showEmpty = !workspaceQuery.isPending && !conversationQuery.isPending && !conversation && conversationQuery.isError

  return (
    <div className="flex h-dvh flex-col bg-background">
      <TopBar
        title={customerName}
        subtitle={subtitle}
        onBack={handleBack}
        onTitlePress={() => setContextSheetOpen(true)}
        titleSlot={
          <>
            <span className="flex items-center gap-1.5">
              <span className="max-w-[180px] truncate text-headline">{customerName}</span>
              <span
                aria-hidden
                className={cn('h-2 w-2 shrink-0 rounded-full', visitorOnline ? 'bg-emerald-500' : 'bg-transparent')}
              />
            </span>
            {subtitle && <span className="max-w-[220px] truncate text-footnote text-muted-foreground">{subtitle}</span>}
          </>
        }
      />

      <div className="relative min-h-0 flex-1">
        {showEmpty ? (
          <EmptyState
            icon={<MessageCircle className="h-6 w-6" />}
            title="Couldn't load this conversation"
            body="Check your connection and try again."
          />
        ) : (
          <MessageList
            ref={messageListRef}
            items={items}
            loading={messagesQuery.isPending}
            typingIndicator={typingIndicator}
            onShowNewMessagePillChange={setShowNewMessagePill}
          />
        )}

        {showNewMessagePill && (
          <div className="pointer-events-none absolute inset-x-0 bottom-4 flex justify-center">
            <Pressable
              haptic="selection"
              onPress={() => messageListRef.current?.scrollToBottom('smooth')}
              className="pointer-events-auto flex h-auto min-h-0 w-auto min-w-0 items-center rounded-full bg-primary px-4 py-1.5 text-footnote font-medium text-primary-foreground shadow-lg"
            >
              New message
            </Pressable>
          </div>
        )}
      </div>

      {/*
        Composer seam (Task 14): render the composer bar as a sibling here,
        below the MessageList's flex-1 container. After a successful
        own-send, call `messageListRef.current?.scrollToBottom('smooth')` to
        satisfy "auto-scroll on own-sends". The "New message" pill above is
        positioned `bottom-4` against this screen's own edge as a
        placeholder — once the composer exists, give it a fixed height and
        move the pill to sit just above it (e.g. `bottom-[var(--composer-height)]`).
      */}

      <Sheet open={contextSheetOpen} onOpenChange={setContextSheetOpen}>
        <div className="px-4 pb-4">
          <p className="text-headline">{customerName}</p>
          {conversation?.customer_email && (
            <p className="mt-1 text-footnote text-muted-foreground">{conversation.customer_email}</p>
          )}
          {/* Task 15 replaces this placeholder with the full ContextSheet (customer profile, CRM/linked-task info, etc). */}
        </div>
      </Sheet>
    </div>
  )
}
