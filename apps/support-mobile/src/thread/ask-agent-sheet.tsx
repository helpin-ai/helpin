import { useEffect, useMemo, useRef, useState } from 'react'
import { Bot, Send, Sparkles, X } from 'lucide-react'
import {
  useEnsureConversationDockChat,
  useGenerateSupportDockChatTitle,
  useResolveSupportDockRunInteraction,
  useSendSupportDockChatMessage,
  useSupportDockChat,
  useSupportDockChatMessages,
  useSupportDockRunInteractions,
  supportService,
  type SupportAgentRunMessage,
  type SupportConversation,
} from '@helpin-ai/support-core'
import { toast } from 'sonner'

import { getUpgradeRequiredReason, type UpgradeRequiredReason } from '@mobile/lib/upgrade-required'
import { Pressable } from '@mobile/ui/pressable'
import { Sheet } from '@mobile/ui/sheet'
import { Spinner } from '@mobile/ui/spinner'
import { UpgradeRequiredSheet } from '@mobile/ui/upgrade-required-sheet'
import { RunInteractionCards } from './ai-run-approvals'
import { Markdown } from './markdown'
import {
  SUPPORT_AGENT_STARTERS,
  buildSupportDockPageContext,
  newSupportDockClientMessageId,
  supportAgentRunLabel,
} from './ask-agent'

export function AskAgentSheet({ workspaceId, conversation, open, onOpenChange, canResolveInteractions }: {
  workspaceId: string
  conversation: SupportConversation
  open: boolean
  onOpenChange: (open: boolean) => void
  canResolveInteractions: boolean
}) {
  const ensureChat = useEnsureConversationDockChat(workspaceId)
  const [chatId, setChatId] = useState<string | null>(null)
  const [value, setValue] = useState('')
  const [olderMessages, setOlderMessages] = useState<SupportAgentRunMessage[]>([])
  const [nextBefore, setNextBefore] = useState<number | null | undefined>(undefined)
  const [loadingEarlier, setLoadingEarlier] = useState(false)
  const [upgradeReason, setUpgradeReason] = useState<UpgradeRequiredReason | null>(null)
  const ensureKeyRef = useRef<string | null>(null)
  const transcriptRef = useRef<HTMLDivElement | null>(null)
  const pageContext = useMemo(() => buildSupportDockPageContext(conversation), [conversation])

  useEffect(() => {
    setChatId(null)
    setOlderMessages([])
    setNextBefore(undefined)
    ensureKeyRef.current = null
  }, [conversation.id, workspaceId])

  useEffect(() => {
    const key = `${workspaceId}:${conversation.id}`
    if (!open || !workspaceId || chatId || ensureKeyRef.current === key) return
    ensureKeyRef.current = key
    ensureChat.mutate(conversation.id, {
      onSuccess: (chat) => setChatId(chat.id),
      onError: () => toast.error('Could not open Ask Agent'),
    })
  }, [chatId, conversation.id, ensureChat, open, workspaceId])

  const detailQuery = useSupportDockChat(workspaceId, chatId, open)
  const messagesQuery = useSupportDockChatMessages(workspaceId, chatId, open)
  const runActive = detailQuery.data?.run?.status === 'queued' || detailQuery.data?.run?.status === 'running'
  const interactionQuery = useSupportDockRunInteractions(
    workspaceId,
    chatId,
    open && canResolveInteractions && !!detailQuery.data?.run,
  )
  const resolveInteraction = useResolveSupportDockRunInteraction(workspaceId, chatId ?? '')
  const sendMessage = useSendSupportDockChatMessage(workspaceId, chatId ?? '')
  const generateTitle = useGenerateSupportDockChatTitle(workspaceId, chatId ?? '')
  const messages = useMemo(
    () => [...new Map(
      [...olderMessages, ...(messagesQuery.data?.messages ?? [])].map((message) => [message.id, message]),
    ).values()]
      .filter((message) => (message.role === 'user' || message.role === 'assistant') && message.content.trim())
      .sort((left, right) => {
        if (left.dock_chat_sequence != null && right.dock_chat_sequence != null) {
          return left.dock_chat_sequence - right.dock_chat_sequence
        }
        return Date.parse(left.created_at) - Date.parse(right.created_at)
      }),
    [messagesQuery.data?.messages, olderMessages],
  )

  useEffect(() => {
    if (nextBefore === undefined && messagesQuery.data) {
      setNextBefore(messagesQuery.data.next_before ?? null)
    }
  }, [messagesQuery.data, nextBefore])

  useEffect(() => {
    if (!open) return
    const frame = requestAnimationFrame(() => {
      if (transcriptRef.current) transcriptRef.current.scrollTop = transcriptRef.current.scrollHeight
    })
    return () => cancelAnimationFrame(frame)
  }, [messages.length, open, runActive])

  const retryEnsure = () => {
    ensureKeyRef.current = null
    ensureChat.mutate(conversation.id, {
      onSuccess: (chat) => setChatId(chat.id),
      onError: () => toast.error('Could not open Ask Agent'),
    })
  }

  const send = async (content: string) => {
    const trimmed = content.trim()
    if (!trimmed || !chatId || sendMessage.isPending) return
    const needsTitle = !(detailQuery.data?.chat.title ?? '').trim()
    try {
      await sendMessage.mutateAsync({
        clientMessageId: newSupportDockClientMessageId(),
        content: trimmed,
        pageContext,
      })
      setValue('')
      if (needsTitle) generateTitle.mutate({ content: trimmed, pageContext })
    } catch (error) {
      const reason = getUpgradeRequiredReason(error)
      if (reason) setUpgradeReason(reason)
      else toast.error('Could not send message to Ask Agent')
    }
  }

  const loadEarlier = async () => {
    if (!chatId || typeof nextBefore !== 'number' || loadingEarlier) return
    setLoadingEarlier(true)
    try {
      const response = await supportService.listSupportDockChatMessages(workspaceId, chatId, nextBefore)
      if (response.error || !response.data) throw new Error(response.error || 'Could not load earlier messages')
      setOlderMessages((current) => [...current, ...response.data!.messages])
      setNextBefore(response.data.next_before ?? null)
    } catch {
      toast.error('Could not load earlier Ask Agent messages')
    } finally {
      setLoadingEarlier(false)
    }
  }

  const status = detailQuery.data?.run?.status ?? detailQuery.data?.chat.active_run_status

  return (
    <>
      <Sheet open={open} onOpenChange={onOpenChange} title="Ask Agent" className="h-[88vh] max-h-[88vh]">
        <div className="flex min-h-0 flex-1 flex-col">
          <header className="flex shrink-0 items-center gap-3 border-b border-border/60 px-4 pb-3">
            <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-primary/10 text-primary"><Sparkles className="h-5 w-5" /></span>
            <div className="min-w-0 flex-1">
              <h2 className="text-headline text-foreground">Ask Agent</h2>
              <p role="status" className="truncate text-caption text-muted-foreground">{supportAgentRunLabel(status)}</p>
            </div>
            <Pressable aria-label="Close Ask Agent" onPress={() => onOpenChange(false)} className="flex h-9 w-9 items-center justify-center rounded-full active:bg-muted"><X className="h-5 w-5" /></Pressable>
          </header>

          <div ref={transcriptRef} className="min-h-0 flex-1 overflow-y-auto px-4 py-4">
            {!chatId && ensureChat.isPending && <div className="flex h-full items-center justify-center"><Spinner /></div>}
            {!chatId && ensureChat.isError && (
              <div className="flex h-full flex-col items-center justify-center gap-3 text-center">
                <Bot className="h-8 w-8 text-muted-foreground" />
                <p className="text-body font-medium">Ask Agent could not be opened</p>
                <Pressable onPress={retryEnsure} className="flex min-h-10 w-auto min-w-0 items-center rounded-full bg-primary px-4 text-footnote font-semibold text-primary-foreground">Try again</Pressable>
              </div>
            )}

            {chatId && messagesQuery.isPending && <div className="flex h-full items-center justify-center"><Spinner /></div>}
            {chatId && !messagesQuery.isPending && messages.length === 0 && (
              <div className="flex min-h-full flex-col justify-end pb-2">
                <div className="mx-auto max-w-sm text-center">
                  <span className="mx-auto flex h-12 w-12 items-center justify-center rounded-2xl bg-primary/10 text-primary"><Bot className="h-6 w-6" /></span>
                  <h3 className="mt-3 text-headline">What should I help with?</h3>
                  <p className="mt-1 text-footnote text-muted-foreground">I can use this conversation and your workspace context.</p>
                </div>
                <div className="mt-5 space-y-2">
                  {SUPPORT_AGENT_STARTERS.map((starter) => (
                    <Pressable key={starter.label} disabled={sendMessage.isPending} onPress={() => void send(starter.prompt)} className="flex min-h-11 w-full items-center rounded-2xl border border-border/70 bg-background px-4 text-left text-footnote font-medium shadow-sm disabled:opacity-50">
                      {starter.label}
                    </Pressable>
                  ))}
                </div>
              </div>
            )}

            {messages.length > 0 && (
              <div className="space-y-3">
                {typeof nextBefore === 'number' && (
                  <div className="flex justify-center pb-1">
                    <Pressable disabled={loadingEarlier} onPress={() => void loadEarlier()} className="flex min-h-9 w-auto min-w-0 items-center rounded-full border border-border px-4 text-footnote font-medium disabled:opacity-50">
                      {loadingEarlier ? <Spinner size={14} /> : 'Load earlier messages'}
                    </Pressable>
                  </div>
                )}
                {messages.map((message) => (
                  <div key={message.id} className={message.role === 'user' ? 'flex justify-end' : 'flex justify-start'}>
                    <div className={message.role === 'user' ? 'max-w-[86%] rounded-2xl rounded-br-md bg-primary px-3 py-2 text-primary-foreground' : 'max-w-[92%] rounded-2xl rounded-bl-md border border-border/70 bg-muted/40 px-3 py-2 text-foreground'}>
                      <Markdown className="selectable min-w-0 break-words text-body">{message.content}</Markdown>
                    </div>
                  </div>
                ))}
                {runActive && <div className="flex items-center gap-2 text-footnote text-muted-foreground"><Spinner size={14} />Agent is working…</div>}
              </div>
            )}

            {canResolveInteractions && (
              <div className="mt-4">
                <RunInteractionCards
                  inline
                  interactions={interactionQuery.data?.interactions ?? []}
                  onResolve={(variables, options) => resolveInteraction.mutate(variables, options)}
                  resolving={resolveInteraction.isPending}
                />
              </div>
            )}
          </div>

          <div className="shrink-0 border-t border-border/60 bg-background px-4 pb-[max(var(--safe-bottom),12px)] pt-3">
            <div className="flex items-end gap-2 rounded-2xl border border-input bg-background p-2 focus-within:ring-2 focus-within:ring-ring/30">
              <textarea
                aria-label="Message Ask Agent"
                value={value}
                onChange={(event) => setValue(event.target.value)}
                onKeyDown={(event) => {
                  if (event.key === 'Enter' && !event.shiftKey) {
                    event.preventDefault()
                    void send(value)
                  }
                }}
                rows={1}
                placeholder="Ask about this conversation…"
                disabled={!chatId || sendMessage.isPending}
                className="max-h-28 min-h-9 flex-1 resize-none bg-transparent px-2 py-2 text-body outline-none placeholder:text-muted-foreground disabled:opacity-50"
              />
              <Pressable aria-label="Send to Ask Agent" disabled={!chatId || !value.trim() || sendMessage.isPending} onPress={() => void send(value)} className="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-primary text-primary-foreground disabled:opacity-40">
                {sendMessage.isPending ? <Spinner size={16} /> : <Send className="h-4 w-4" />}
              </Pressable>
            </div>
          </div>
        </div>
      </Sheet>
      <UpgradeRequiredSheet reason={upgradeReason} onOpenChange={(nextOpen) => { if (!nextOpen) setUpgradeReason(null) }} />
    </>
  )
}
