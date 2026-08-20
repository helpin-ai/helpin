import { useEffect, useMemo, useRef, useState } from 'react'
import { useParams, useRouter } from '@tanstack/react-router'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import {
  findConversationInListCache,
  flattenSupportMessagePages,
  isSupportConversationListQueryKey,
  useConversation,
  useConversationMessages,
  useMarkConversationRead,
  useDismissConversationTriage,
  useInboxScopes,
  useMoveConversation,
  useSupportInstallation,
  useSupportPresenceStore,
  useUpdateConversationStatus,
  type ConversationListCache,
  type ConversationStatus,
  type SupportConversation,
  type SupportMessage,
} from '@helpin-ai/support-core'
import { TopBar } from '@mobile/ui/top-bar'
import { OfflineBanner } from '@mobile/ui/offline-banner'
import { Pressable } from '@mobile/ui/pressable'
import { EmptyState } from '@mobile/ui/empty-state'
import { cn } from '@mobile/lib/cn'
import { workspacesService } from '@mobile/lib/services/workspaces-service'
import { useWorkspacePermissions } from '@mobile/lib/use-workspace-permissions'
import { useWorkspaceStore } from '@mobile/stores/workspace-store'
import { useAuthStore } from '@mobile/stores/auth-store'
import type { ShortcutVariableContext } from '@/components/support/shortcutVariables'
import type { MentionMember } from '@mobile/thread/mentions'
import { useResolvedTransitionStore } from '@mobile/stores/resolved-transition-store'
import { displayNameFor } from '@mobile/inbox/conversation-cell'
import { MessageList, type MessageListHandle, type TypingIndicatorState } from '@mobile/thread/message-list'
import { computeSupportReceipt, groupMessages } from '@mobile/thread/thread-helpers'
import { Composer } from '@mobile/thread/composer'
import { ContextSheet } from '@mobile/thread/context-sheet'
import { ConversationActionsSheet } from '@mobile/thread/conversation-actions-sheet'
import { buildTriageBanner } from '@mobile/thread/triage-banner'
import { MessageActionsSheet } from '@mobile/thread/message-actions-sheet'
import { useDraftStore } from '@mobile/thread/draft-store'
import { haptic } from '@mobile/lib/haptics'
import { CheckCircle2, MessageCircle, MoreHorizontal, Sparkles } from 'lucide-react'
import { toast } from 'sonner'

const STATUS_LABELS: Record<ConversationStatus, string> = {
  open: 'Open',
  waiting_on_customer: 'Waiting on customer',
  resolved: 'Resolved',
  spam: 'Spam',
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
    const lists = queryClient.getQueriesData<ConversationListCache>({
      predicate: (query) => isSupportConversationListQueryKey(query.queryKey, workspaceId),
    })
    for (const [, data] of lists) {
      const found = findConversationInListCache(data, conversationId)
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
  const { accessQuery, canReadSupport, canEditSupport, canReadPM, canEditPM } = useWorkspacePermissions(workspaceId)
  const supportWorkspaceId = canReadSupport ? workspaceId : ''
  const accessDenied = accessQuery.isSuccess && !canReadSupport

  // Mirrors InboxScreen's effect: keeps `useWorkspaceStore` populated even
  // when this screen is reached directly (e.g. a future deep link) rather
  // than via the inbox, since the root-level realtime mount
  // (router.tsx's RootRealtimeMount) reads workspaceId from that store.
  const setCurrentWorkspace = useWorkspaceStore((s) => s.setCurrentWorkspace)
  useEffect(() => {
    if (workspaceQuery.data) {
      setCurrentWorkspace({
        id: workspaceQuery.data.id,
        slug: workspaceQuery.data.slug,
        name: workspaceQuery.data.name,
      })
    }
  }, [workspaceQuery.data, setCurrentWorkspace])

  const cachedConversation = useConversationFromListCache(supportWorkspaceId, conversationId ?? null)
  const conversationQuery = useConversation(supportWorkspaceId, conversationId ?? null)
  const conversation = conversationQuery.data ?? cachedConversation

  const messagesQuery = useConversationMessages(supportWorkspaceId, conversationId ?? null)
  const messages = useMemo(() => flattenSupportMessagePages(messagesQuery.data), [messagesQuery.data])
  const items = useMemo(() => groupMessages(messages), [messages])
  const receipt = useMemo(() => computeSupportReceipt(messages, conversation), [messages, conversation])

  const agentUser = useAuthStore((s) => s.user)
  const workspaceName = useWorkspaceStore((s) => s.currentWorkspace?.name)
  // Match the web composer: mentions use all workspace-assignable identities,
  // while the narrower conversation-assignee query remains assignment-only.
  const assignableQuery = useQuery({
    queryKey: ['workspace', workspaceId, 'assignable-members'],
    queryFn: async () => {
      const { data, error } = await workspacesService.listAssignableMembers(workspaceId)
      if (error || !data) throw new Error(error ?? 'Failed to load teammates')
      return data
    },
    enabled: !!workspaceId && canEditSupport,
    staleTime: 60_000,
  })
  const mentionMembers = useMemo<MentionMember[]>(
    () =>
      (assignableQuery.data ?? []).map((member) => ({
        id: member.id,
        user_id: member.user_id,
        email: member.email,
        display_name: member.display_name,
        avatar_url: member.avatar_url,
      })),
    [assignableQuery.data],
  )

  // Email-fallback: a reply to a widget conversation whose visitor is offline is
  // delivered by email (when the widget has email fallback enabled). Mirrors the
  // web thread's `emailFallbackHint`; drives the composer's send confirm.
  const installationQuery = useSupportInstallation(supportWorkspaceId)
  const isVisitorOnline = useSupportPresenceStore((s) =>
    conversation?.anonymous_id ? !!s.onlineVisitors[conversation.anonymous_id] : false,
  )
  const willSendAsEmail = useMemo(() => {
    const email = conversation?.customer_email?.trim()
    if (!conversation || !installationQuery.data?.settings?.email_fallback_enabled || !email) return false
    if (conversation.email_unsubscribed) return false
    if (conversation.status === 'resolved' || conversation.status === 'spam') return false
    if (conversation.anonymous_id && isVisitorOnline) return false
    return true
  }, [conversation, installationQuery.data, isVisitorOnline])
  const variableContext = useMemo<ShortcutVariableContext>(
    () => ({
      customer: { fullName: conversation?.customer_name, email: conversation?.customer_email },
      agent: { fullName: agentUser?.full_name, email: agentUser?.email },
      workspaceName,
      conversationSubject: conversation?.subject,
    }),
    [conversation?.customer_name, conversation?.customer_email, conversation?.subject, agentUser?.full_name, agentUser?.email, workspaceName],
  )

  const markRead = useMarkConversationRead(workspaceId)
  useEffect(() => {
    if (canReadSupport && workspaceId && conversationId) markRead.mutate(conversationId)
    // eslint-disable-next-line react-hooks/exhaustive-deps -- fire once per accessible (workspaceId, conversationId), not on every markRead identity change
  }, [canReadSupport, workspaceId, conversationId])

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
  const [actionsSheetOpen, setActionsSheetOpen] = useState(false)
  const [selectedMessage, setSelectedMessage] = useState<SupportMessage | null>(null)
  const setDraftText = useDraftStore((state) => state.setText)
  const setDraftMode = useDraftStore((state) => state.setMode)
  const updateStatus = useUpdateConversationStatus(workspaceId)
  const moveConversation = useMoveConversation(workspaceId)
  const dismissTriage = useDismissConversationTriage(workspaceId)
  const inboxScopes = useInboxScopes(supportWorkspaceId, !!conversation?.triage)

  const triageBanner = useMemo(() => buildTriageBanner(conversation, inboxScopes.data), [conversation, inboxScopes.data])

  useEffect(() => {
    setSelectedMessage(null)
  }, [conversationId])

  const restoreReplyDraft = (text: string) => {
    if (!conversationId) return
    setDraftMode(conversationId, 'reply')
    setDraftText(conversationId, text)
    requestAnimationFrame(() => messageListRef.current?.scrollToBottom('smooth'))
  }

  const handleBack = () => {
    if (router.history.canGoBack()) router.history.back()
    else router.navigate({ to: '/w/$slug/support', params: { slug: slug ?? '' } })
  }

  const customerName = conversation ? displayNameFor(conversation) : 'Conversation'
  // Web-style: the subject is the title; the customer + status is the subtitle.
  // Falls back to the customer name when there's no subject (common for chat).
  const conversationTitle = conversation?.subject?.trim() || customerName
  const subtitle = conversation
    ? [customerName, STATUS_LABELS[conversation.status]].filter(Boolean).join(' · ')
    : undefined
  const isResolved = conversation?.status === 'resolved'

  const markResolved = useResolvedTransitionStore((s) => s.markResolved)
  const handleToggleResolve = () => {
    if (!canEditSupport || !conversation || !conversationId || updateStatus.isPending) return
    const resolving = !isResolved
    updateStatus.mutate(
      { conversationId, status: resolving ? 'resolved' : 'open' },
      {
        onSuccess: () => {
          // Resolving clears the conversation from the queue: hand it to the
          // inbox to animate out + offer Undo, then pop back to the list.
          // Reopening just flips the status in place (stay on the thread).
          if (resolving) {
            haptic('notificationSuccess')
            markResolved(conversationId)
            handleBack()
          }
        },
        onError: () => toast.error('Could not update conversation'),
      },
    )
  }

  const showError =
    !workspaceQuery.isPending &&
    !accessQuery.isPending &&
    (workspaceQuery.isError ||
      accessQuery.isError ||
      (!conversationQuery.isPending && !conversation && conversationQuery.isError))

  return (
    <div className="flex h-dvh flex-col bg-background">
      <TopBar
        title={conversationTitle}
        subtitle={subtitle}
        onBack={handleBack}
        titleAlign="left"
        onTitlePress={() => setContextSheetOpen(true)}
        titleSlot={<span className="max-w-[200px] truncate text-headline">{conversationTitle}</span>}
        trailing={
          conversation && canEditSupport && (
            <>
              <Pressable
                aria-label={isResolved ? 'Reopen conversation' : 'Resolve conversation'}
                haptic="selection"
                onPress={handleToggleResolve}
                disabled={updateStatus.isPending}
                className="flex h-9 w-9 items-center justify-center rounded-full active:bg-muted disabled:opacity-40"
              >
                <CheckCircle2 className={cn('h-6 w-6', isResolved ? 'text-green-500' : 'text-muted-foreground')} />
              </Pressable>
              <Pressable
                aria-label="Conversation actions"
                haptic="selection"
                onPress={() => setActionsSheetOpen(true)}
                className="flex h-9 w-9 items-center justify-center rounded-full active:bg-muted"
              >
                <MoreHorizontal className="h-5 w-5 text-muted-foreground" />
              </Pressable>
            </>
          )
        }
      />

      <OfflineBanner />

      {triageBanner && canEditSupport && conversation && (
        <div className="border-b border-amber-200/70 bg-amber-50/80 px-4 py-3 dark:border-amber-900/60 dark:bg-amber-950/25">
          <div className="flex items-start gap-2">
            <Sparkles className="mt-0.5 h-4 w-4 shrink-0 text-amber-600 dark:text-amber-400" />
            <div className="min-w-0 flex-1">
              <p className="text-footnote font-semibold text-foreground">Suggested inbox: {triageBanner.mailboxName}</p>
              <p className="mt-0.5 text-caption text-muted-foreground">
                {triageBanner.source}{triageBanner.confidence ? ` · ${triageBanner.confidence} confidence` : ''}
              </p>
              {triageBanner.reason && <p className="mt-1 line-clamp-2 text-footnote text-muted-foreground">{triageBanner.reason}</p>}
            </div>
          </div>
          <div className="mt-2 flex justify-end gap-2">
            <Pressable
              disabled={moveConversation.isPending || dismissTriage.isPending}
              onPress={() => moveConversation.mutate(
                { conversationId: conversation.id, mailboxId: triageBanner.mailboxId },
                {
                  onSuccess: () => toast.success(`Moved to ${triageBanner.mailboxName}`),
                  onError: () => toast.error('Could not move conversation'),
                },
              )}
              className="flex min-h-9 w-auto min-w-0 items-center rounded-full bg-primary px-4 text-footnote font-semibold text-primary-foreground disabled:opacity-50"
            >
              Move
            </Pressable>
            <Pressable
              disabled={moveConversation.isPending || dismissTriage.isPending}
              onPress={() => dismissTriage.mutate(conversation.id, {
                onSuccess: () => toast.success('Routing suggestion dismissed'),
                onError: () => toast.error('Could not dismiss suggestion'),
              })}
              className="flex min-h-9 w-auto min-w-0 items-center rounded-full px-4 text-footnote font-semibold text-muted-foreground active:bg-muted disabled:opacity-50"
            >
              Dismiss
            </Pressable>
          </div>
        </div>
      )}

      {conversation && (
        <Pressable
          aria-label="Open conversation details"
          onPress={() => setContextSheetOpen(true)}
          className="flex w-full flex-col items-start border-b border-border/60 bg-background px-4 pb-3 pt-2 text-left"
        >
          <span className="line-clamp-2 text-title text-foreground">{conversationTitle}</span>
          <span className="mt-2 flex max-w-full items-center gap-2 text-footnote text-muted-foreground">
            <span className="flex min-w-0 items-center gap-1.5">
              {visitorOnline && <span aria-hidden className="h-1.5 w-1.5 shrink-0 rounded-full bg-emerald-500" />}
              <span className="truncate font-medium text-foreground/80">{customerName}</span>
            </span>
            <span aria-hidden className="text-border">•</span>
            <span className="shrink-0 rounded-full bg-muted px-2 py-0.5">{STATUS_LABELS[conversation.status]}</span>
          </span>
        </Pressable>
      )}

      <div className="relative min-h-0 flex-1">
        {accessDenied ? (
          <EmptyState
            icon={<MessageCircle className="h-6 w-6" />}
            title="Support access unavailable"
            body="Ask a workspace admin to grant you access to the Support module."
          />
        ) : showError ? (
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
            hasEarlier={!!messagesQuery.hasNextPage}
            loadingEarlier={messagesQuery.isFetchingNextPage}
            loadEarlierError={messagesQuery.isFetchNextPageError}
            onLoadEarlier={() => void messagesQuery.fetchNextPage()}
            messageCount={messages.length}
            oldestMessageId={messages[0]?.id}
            newestMessageId={messages[messages.length - 1]?.id}
            typingIndicator={typingIndicator}
            receiptMessageId={receipt.receiptMessageId}
            receiptStatus={receipt.receiptStatus}
            onShowNewMessagePillChange={setShowNewMessagePill}
            onMessageActions={setSelectedMessage}
          />
        )}

        {/* Persistent aria-live region (same rationale as OfflineBanner): the
            pill itself mounts/unmounts with `showNewMessagePill`, so the live
            region needs to survive that to reliably announce "New message". */}
        <div role="status" aria-live="polite">
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
      </div>

      {/* key={conversationId}: hard-remount the composer on conversation
          switch so its transient local state (send phase, sent-timer) can
          never leak across conversations — persistent state (draft text,
          mode, failed-send chips) lives in useDraftStore keyed by
          conversation. Scroll-on-own-send is handled by MessageList's
          append effect reacting to the optimistic append, not wired from
          the composer. */}
      {workspaceId && conversationId && canEditSupport && (
        <Composer
          key={conversationId}
          workspaceId={workspaceId}
          conversationId={conversationId}
          variableContext={variableContext}
          mentionMembers={mentionMembers}
          willSendAsEmail={willSendAsEmail}
        />
      )}
      {workspaceId && conversationId && canReadSupport && !accessQuery.isPending && !canEditSupport && (
        <div className="border-t border-border/60 bg-background px-4 pb-[max(var(--safe-bottom),12px)] pt-3 text-center text-footnote text-muted-foreground">
          Read-only access
        </div>
      )}

      <ContextSheet
        workspaceId={supportWorkspaceId}
        conversationId={canReadSupport ? conversationId ?? null : null}
        open={contextSheetOpen}
        onOpenChange={setContextSheetOpen}
        canEdit={canEditSupport}
      />

      {conversation && canEditSupport && (
        <ConversationActionsSheet
          open={actionsSheetOpen}
          onOpenChange={setActionsSheetOpen}
          workspaceId={workspaceId}
          workspaceSlug={slug ?? ''}
          conversation={conversation}
          canReadPM={canReadPM}
          canCreateTask={canEditPM}
          defaultTeamId={accessQuery.data?.membership.support_default_team_id ?? accessQuery.data?.team_memberships[0]?.team_id}
          onLeave={handleBack}
        />
      )}

      {selectedMessage && (
        <MessageActionsSheet
          open
          onOpenChange={(nextOpen) => {
            if (!nextOpen) setSelectedMessage(null)
          }}
          workspaceId={workspaceId}
          message={selectedMessage}
          currentUserId={agentUser?.id}
          canEditSupport={canEditSupport}
          onRestoreDraft={restoreReplyDraft}
        />
      )}
    </div>
  )
}
