import { useEffect, useLayoutEffect, useMemo, useRef, useState, type ChangeEvent } from 'react'
import { AtSign, FileText, LoaderCircle, Mail, Paperclip, RotateCcw, Smile, Sparkles, Undo2, X, Zap } from 'lucide-react'
import { toast } from 'sonner'
import {
  useDeleteSupportAttachment,
  useRewriteSupportDraft,
  useSendMessage,
  useSupportCannedResponses,
  useUpdateConversationEmailRecipients,
  useUploadSupportAttachment,
  type SupportAIRewriteOperation,
  type SupportCannedResponse,
  type SupportConversation,
} from '@helpin-ai/support-core'
import { filterShortcuts, stripShortcutContent } from '@/components/support/shortcutFiltering'
import { resolveShortcutVariables, type ShortcutVariableContext } from '@/components/support/shortcutVariables'
import { cn } from '@mobile/lib/cn'
import { haptic } from '@mobile/lib/haptics'
import { getUpgradeRequiredReason, type UpgradeRequiredReason } from '@mobile/lib/upgrade-required'
import { Pressable } from '@mobile/ui/pressable'
import { UpgradeRequiredSheet } from '@mobile/ui/upgrade-required-sheet'
import { DEFAULT_DRAFT, useDraftStore, type ComposerMode } from './draft-store'
import type { FailedSend } from './failed-sends-reducer'
import { SendButton, type SendButtonState } from './send-button'
import { useTypingBroadcast } from './use-typing-broadcast'
import { Avatar } from '@mobile/ui/avatar'
import { AIToolsSheet } from './ai-tools-sheet'
import { CannedResponsesSheet } from './canned-responses-sheet'
import { EmailConfirmSheet } from './email-confirm-sheet'
import { cannedToPlainText, detectShortcutToken, replaceRange } from './canned-shortcuts'
import { detectMentionToken, mentionSuggestions, type MentionMember, type MentionSuggestion, type MentionToken } from './mentions'
import { teammatePresenceDotClass, teammatePresenceLabel } from './teammate-presence'
import { EmojiPickerSheet } from './emoji-picker-sheet'

/** How long the "Rewritten · Undo" bar stays before auto-dismissing. */
const UNDO_VISIBLE_MS = 6000

const emailConfirmSkipKey = (workspaceId: string) => `support_email_confirm_skip:${workspaceId}`
function isEmailConfirmSkipped(workspaceId: string): boolean {
  try {
    return localStorage.getItem(emailConfirmSkipKey(workspaceId)) === '1'
  } catch {
    return false
  }
}
function persistEmailConfirmSkip(workspaceId: string): void {
  try {
    localStorage.setItem(emailConfirmSkipKey(workspaceId), '1')
  } catch {
    /* storage unavailable — skip is session-only */
  }
}

export interface ComposerProps {
  workspaceId: string
  conversationId: string
  /** Context for resolving canned-response variables ({{customer.first_name}}, …). */
  variableContext?: ShortcutVariableContext
  /** Teammates mentionable in internal notes (empty disables @mentions). */
  mentionMembers?: MentionMember[]
  /** True when a reply will be delivered by email (offline widget visitor) — triggers a send confirm. */
  willSendAsEmail?: boolean
  /** Current recipient state drives email delivery metadata and copied-email confirmation. */
  conversation?: Pick<
    SupportConversation,
    | 'id'
    | 'customer_email'
    | 'email_cc'
    | 'primary_recipient_state'
    | 'suggested_primary_recipient_email'
    | 'suggested_primary_recipient_name'
  >
  /** Shortcut mutations are support.admin-only even though insertion is support.read. */
  canManageShortcuts?: boolean
}

/**
 * `text-body` is 0.875rem/1.1875rem (19px line-height, see index.css) — the
 * textarea grows from 1 to `MAX_LINES` of that, then scrolls internally.
 */
const LINE_HEIGHT_PX = 19
const MAX_LINES = 6
const TEXTAREA_VERTICAL_PADDING_PX = 16 // py-2 (8px top + 8px bottom)
const MAX_TEXTAREA_HEIGHT_PX = LINE_HEIGHT_PX * MAX_LINES + TEXTAREA_VERTICAL_PADDING_PX
const MIN_TEXTAREA_HEIGHT_PX = 56

const SENT_STATE_MS = 400

const MAX_ATTACHMENT_SIZE = 10 * 1024 * 1024
const ATTACHMENT_ACCEPT = 'image/*,.pdf,.doc,.docx,.txt,.csv,.xls,.xlsx,.zip,.gz,.tar,.md'

interface PendingAttachment {
  localId: string
  file: File
  status: 'uploading' | 'done' | 'error'
  attachmentId?: string
  previewUrl?: string
}

const attachmentLabel = (count: number) => `${count} attachment${count === 1 ? '' : 's'}`

/**
 * NOTE: the conversation screen mounts this with `key={conversationId}` so
 * the transient local state here (`phase`, `sendingRef`, the sent-timer)
 * resets on every conversation switch — everything that must survive
 * navigation (text, mode, failed-send chips) lives in `useDraftStore`,
 * keyed by conversation.
 *
 * Scroll-on-send is NOT wired from here: the optimistic append from
 * `useSendMessage` grows the message list's content, and the list's own
 * append effect + pinned-to-bottom ResizeObserver (message-list.tsx) already
 * scroll to the new bubble in the correct order (after the append exists),
 * so an extra imperative call from the composer would be redundant at best
 * and premature (pre-append) at worst.
 */
const EMPTY_VARIABLE_CONTEXT: ShortcutVariableContext = {}

const EMPTY_MEMBERS: MentionMember[] = []

function normalizeRecipientEmails(values: string[], excluded: string[] = []): string[] {
  const blocked = new Set(excluded.map((value) => value.trim().toLowerCase()).filter(Boolean))
  const seen = new Set<string>()
  return values.flatMap((value) => {
    const email = value.trim()
    const key = email.toLowerCase()
    if (!email || blocked.has(key) || seen.has(key)) return []
    seen.add(key)
    return [email]
  })
}

export function Composer({
  workspaceId,
  conversationId,
  variableContext = EMPTY_VARIABLE_CONTEXT,
  mentionMembers = EMPTY_MEMBERS,
  willSendAsEmail = false,
  conversation,
  canManageShortcuts = false,
}: ComposerProps) {
  const draft = useDraftStore((state) => state.drafts[conversationId] ?? DEFAULT_DRAFT)
  const setText = useDraftStore((state) => state.setText)
  const setMode = useDraftStore((state) => state.setMode)
  const clearDraft = useDraftStore((state) => state.clearDraft)
  const addFailedSend = useDraftStore((state) => state.addFailedSend)
  const removeFailedSend = useDraftStore((state) => state.removeFailedSend)

  const sendMessage = useSendMessage(workspaceId, conversationId)
  const uploadAttachment = useUploadSupportAttachment(workspaceId, conversationId)
  const deleteAttachment = useDeleteSupportAttachment(workspaceId)
  const updateEmailRecipients = useUpdateConversationEmailRecipients(workspaceId)
  const [phase, setPhase] = useState<'idle' | 'sending' | 'sent'>('idle')
  const [pendingAttachments, setPendingAttachments] = useState<PendingAttachment[]>([])
  const pendingAttachmentsRef = useRef<PendingAttachment[]>([])
  const fileInputRef = useRef<HTMLInputElement>(null)
  const mountedRef = useRef(true)

  const isNote = draft.mode === 'note'
  const primaryRecipientUnconfirmed = conversation?.primary_recipient_state === 'unconfirmed'
  const replyRecipient = variableContext.customer?.fullName?.trim().split(/\s+/)[0]
  const replyPlaceholder = replyRecipient ? `Reply to ${replyRecipient}…` : 'Reply…'
  // Broadcast "agent is typing" to teammates + the customer while composing a
  // reply (never in note mode). Emits `stop` on send, switch-to-note, or leave.
  const { notifyTyping, stopTyping } = useTypingBroadcast(conversationId, !isNote)
  useEffect(() => {
    if (isNote) stopTyping()
  }, [isNote, stopTyping])

  // AI writing tools: rewrite the draft, non-destructively (Undo restores the
  // pre-rewrite text). `undoText` holds the text to restore while the bar shows.
  const rewriteDraft = useRewriteSupportDraft(workspaceId, conversationId)
  const [aiSheetOpen, setAiSheetOpen] = useState(false)
  const [emojiSheetOpen, setEmojiSheetOpen] = useState(false)
  const [busyOperation, setBusyOperation] = useState<SupportAIRewriteOperation | null>(null)
  const [undoText, setUndoText] = useState<string | null>(null)
  const [upgradeReason, setUpgradeReason] = useState<UpgradeRequiredReason | null>(null)
  const undoTimeoutRef = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)
  const aiAssistedRef = useRef(false)
  useEffect(() => () => { if (undoTimeoutRef.current) clearTimeout(undoTimeoutRef.current) }, [])

  async function handleRewrite(operation: SupportAIRewriteOperation) {
    const source = draft.text.trim()
    if (!source || busyOperation) return
    setBusyOperation(operation)
    try {
      const result = await rewriteDraft.mutateAsync({ content: source, operation })
      setText(conversationId, result.content)
      aiAssistedRef.current = true
      setUndoText(source)
      if (undoTimeoutRef.current) clearTimeout(undoTimeoutRef.current)
      undoTimeoutRef.current = setTimeout(() => setUndoText(null), UNDO_VISIBLE_MS)
      haptic('notificationSuccess')
      setAiSheetOpen(false)
    } catch (error) {
      const reason = getUpgradeRequiredReason(error)
      if (reason) {
        setAiSheetOpen(false)
        setUpgradeReason(reason)
      } else {
        toast.error('Could not rewrite draft')
      }
      haptic('notificationError')
    } finally {
      setBusyOperation(null)
    }
  }

  function handleUndoRewrite() {
    if (undoText === null) return
    setText(conversationId, undoText)
    aiAssistedRef.current = false
    setUndoText(null)
    if (undoTimeoutRef.current) clearTimeout(undoTimeoutRef.current)
    haptic('impactLight')
  }

  // Canned responses: insert via a searchable sheet (⚡ button) or inline by
  // typing `!code`. Variables like {{customer.first_name}} are resolved from
  // the conversation + the signed-in agent + workspace.
  const cannedQuery = useSupportCannedResponses(workspaceId)
  const cannedResponses = cannedQuery.data ?? []
  const [cannedSheetOpen, setCannedSheetOpen] = useState(false)
  const [cursor, setCursor] = useState(0)
  const pendingCaretRef = useRef<number | null>(null)

  // The active `!token` under the caret drives the inline suggestion list.
  const activeToken = useMemo(() => detectShortcutToken(draft.text, cursor), [draft.text, cursor])
  const inlineSuggestions = useMemo(
    () => (activeToken ? filterShortcuts(cannedResponses, activeToken.query, 6) : []),
    [activeToken, cannedResponses],
  )

  function insertCanned(response: SupportCannedResponse, range?: { start: number; end: number }) {
    const resolved = cannedToPlainText(resolveShortcutVariables(response.content, variableContext))
    const from = range?.start ?? cursor
    const to = range?.end ?? cursor
    const edit = replaceRange(draft.text, from, to, resolved)
    setText(conversationId, edit.text)
    pendingCaretRef.current = edit.cursor
    setCursor(edit.cursor)
    if (!isNote) notifyTyping(edit.text)
    haptic('selection')
  }

  // @mentions — internal notes only, teammate handles inserted as plain text
  // (`@handle `), which the note renderer highlights (see splitMentionSegments).
  const mentionToken = useMemo(
    () => (isNote ? detectMentionToken(draft.text, cursor) : null),
    [isNote, draft.text, cursor],
  )
  const mentionItems = useMemo(
    () => (mentionToken ? mentionSuggestions(mentionToken.query, mentionMembers, 6) : []),
    [mentionToken, mentionMembers],
  )

  function insertMention(item: MentionSuggestion, token: MentionToken) {
    const edit = replaceRange(draft.text, token.start, token.end, `@${item.handle}`)
    setText(conversationId, edit.text)
    pendingCaretRef.current = edit.cursor
    setCursor(edit.cursor)
    haptic('selection')
  }

  function startMention() {
    const token = cursor > 0 && !/\s/.test(draft.text[cursor - 1] ?? '') ? ' @' : '@'
    const nextText = draft.text.slice(0, cursor) + token + draft.text.slice(cursor)
    const nextCursor = cursor + token.length
    setText(conversationId, nextText)
    pendingCaretRef.current = nextCursor
    setCursor(nextCursor)
  }

  function insertEmoji(emoji: string) {
    const nextText = draft.text.slice(0, cursor) + emoji + draft.text.slice(cursor)
    const nextCursor = cursor + emoji.length
    setText(conversationId, nextText)
    pendingCaretRef.current = nextCursor
    setCursor(nextCursor)
    if (!isNote) notifyTyping(nextText)
  }

  const textareaRef = useRef<HTMLTextAreaElement>(null)
  const sentTimeoutRef = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)
  // Synchronous re-entrancy lock: `phase` is async React state, so two taps
  // landing before a re-render could both see phase !== 'sending' and
  // double-send. This ref flips before any await and is the source of truth
  // for "a send is in flight right now".
  const sendingRef = useRef(false)

  // Auto-grow 1..MAX_LINES: reset to 'auto' first so shrinking (e.g. after
  // clearing the draft on send) is measured correctly, then clamp scrollHeight
  // to the 6-line cap. Runs in useLayoutEffect (before paint) so the height
  // change is applied in the same frame as the keystroke that caused it —
  // an effect running after paint would show one frame at the old height
  // first, which reads as a "jump".
  useLayoutEffect(() => {
    const el = textareaRef.current
    if (!el) return
    el.style.height = 'auto'
    el.style.height = `${Math.min(el.scrollHeight, MAX_TEXTAREA_HEIGHT_PX)}px`
    // Restore the caret after a programmatic insert (canned response), so the
    // cursor lands right after the inserted text instead of at the end.
    if (pendingCaretRef.current !== null) {
      const caret = pendingCaretRef.current
      pendingCaretRef.current = null
      el.focus()
      el.setSelectionRange(caret, caret)
    }
  }, [draft.text])

  useEffect(
    () => () => {
      if (sentTimeoutRef.current) clearTimeout(sentTimeoutRef.current)
    },
    [],
  )

  function updatePendingAttachments(updater: (current: PendingAttachment[]) => PendingAttachment[]) {
    const next = updater(pendingAttachmentsRef.current)
    pendingAttachmentsRef.current = next
    if (mountedRef.current) setPendingAttachments(next)
  }

  useEffect(() => {
    mountedRef.current = true
    return () => {
      mountedRef.current = false
      for (const attachment of pendingAttachmentsRef.current) {
        if (attachment.previewUrl) URL.revokeObjectURL(attachment.previewUrl)
      }
      pendingAttachmentsRef.current = []
    }
  }, [])

  async function uploadPendingAttachment(localId: string, file: File) {
    updatePendingAttachments((current) => current.map((item) =>
      item.localId === localId ? { ...item, status: 'uploading' } : item,
    ))
    try {
      const result = await uploadAttachment.mutateAsync(file)
      // The user may remove the chip (or leave the conversation) while the
      // storage request is in flight. Delete the now-confirmed orphan instead
      // of silently retaining a server record that can never be sent.
      if (!pendingAttachmentsRef.current.some((item) => item.localId === localId)) {
        await deleteAttachment.mutateAsync(result.id).catch(() => undefined)
        return
      }
      updatePendingAttachments((current) => current.map((item) =>
        item.localId === localId
          ? { ...item, status: 'done', attachmentId: result.id }
          : item,
      ))
    } catch {
      updatePendingAttachments((current) => current.map((item) =>
        item.localId === localId ? { ...item, status: 'error' } : item,
      ))
      toast.error(`Could not upload ${file.name}`)
      haptic('notificationError')
    }
  }

  function handleFileInput(event: ChangeEvent<HTMLInputElement>) {
    const files = Array.from(event.target.files ?? [])
    event.target.value = ''
    for (const file of files) {
      if (file.size > MAX_ATTACHMENT_SIZE) {
        toast.error(`${file.name} is larger than 10 MB`)
        continue
      }
      const localId = crypto.randomUUID()
      const previewUrl = file.type.startsWith('image/') ? URL.createObjectURL(file) : undefined
      updatePendingAttachments((current) => [
        ...current,
        { localId, file, previewUrl, status: 'uploading' },
      ])
      void uploadPendingAttachment(localId, file)
    }
  }

  function retryAttachment(attachment: PendingAttachment) {
    void uploadPendingAttachment(attachment.localId, attachment.file)
  }

  function removeAttachment(attachment: PendingAttachment) {
    updatePendingAttachments((current) => current.filter((item) => item.localId !== attachment.localId))
    if (attachment.previewUrl) URL.revokeObjectURL(attachment.previewUrl)
    if (attachment.attachmentId) {
      void deleteAttachment.mutateAsync(attachment.attachmentId).catch(() => {
        toast.error('Could not remove uploaded file')
      })
    }
  }

  function clearSentAttachments(attachmentIds: string[]) {
    const sent = new Set(attachmentIds)
    updatePendingAttachments((current) => current.filter((attachment) => {
      if (!attachment.attachmentId || !sent.has(attachment.attachmentId)) return true
      if (attachment.previewUrl) URL.revokeObjectURL(attachment.previewUrl)
      return false
    }))
  }

  const trimmed = draft.text.trim()
  const doneAttachmentIds = pendingAttachments
    .filter((attachment) => attachment.status === 'done' && attachment.attachmentId)
    .map((attachment) => attachment.attachmentId!)
  const hasUploadingAttachment = pendingAttachments.some((attachment) => attachment.status === 'uploading')
  const canSend = (trimmed.length > 0 || doneAttachmentIds.length > 0)
    && !hasUploadingAttachment
    && (isNote || !primaryRecipientUnconfirmed)
  const buttonState: SendButtonState =
    phase === 'sending' ? 'sending' : phase === 'sent' ? 'sent' : canSend ? 'active' : 'disabled'

  async function attemptSend(
    content: string,
    mode: ComposerMode,
    attachmentIds: string[],
    options: { retryId?: string; aiAssisted?: boolean } = {},
  ) {
    if (sendingRef.current) return
    sendingRef.current = true
    // A new send during the 400ms 'sent' display is legitimate — cancel the
    // stale timer so it can't fire mid-flight and flip this send's 'sending'
    // state back to 'idle' while the request is still outstanding.
    if (sentTimeoutRef.current) clearTimeout(sentTimeoutRef.current)
    setPhase('sending')
    const primaryEmail = conversation?.customer_email?.trim() ?? ''
    const ccEmails = normalizeRecipientEmails(conversation?.email_cc ?? [], [primaryEmail])
    try {
      await sendMessage.mutateAsync({
        content,
        is_internal: mode === 'note',
        ...(mode === 'reply' && options.aiAssisted ? { ai_assisted: true } : {}),
        ...(mode === 'reply' && primaryEmail ? { channels: ['email' as const] } : {}),
        ...(mode === 'reply' && ccEmails.length > 0 ? { cc_emails: ccEmails } : {}),
        ...(attachmentIds.length > 0 ? { attachment_ids: attachmentIds } : {}),
      })
      haptic('notificationSuccess')
      if (options.retryId) removeFailedSend(conversationId, options.retryId)
      clearSentAttachments(attachmentIds)
      setPhase('sent')
      sentTimeoutRef.current = setTimeout(() => setPhase('idle'), SENT_STATE_MS)
    } catch {
      haptic('notificationError')
      setPhase('idle')
      // A fresh send (no retryId) needs a new chip; a retry's chip is already
      // in the list — leave it there so the user can retry again.
      if (!options.retryId) {
        addFailedSend(conversationId, {
          id: crypto.randomUUID(), content, mode,
          ...(attachmentIds.length > 0 ? { attachmentIds } : {}),
          ...(options.aiAssisted ? { aiAssisted: true } : {}),
        })
      }
    } finally {
      sendingRef.current = false
    }
  }

  // Email-fallback send confirm: a reply to an offline widget visitor goes out
  // as an email — confirm before sending (unless the agent opted out).
  const [emailConfirmOpen, setEmailConfirmOpen] = useState(false)
  const [dontAskAgain, setDontAskAgain] = useState(false)
  const [emailConfirmSkipped, setEmailConfirmSkipped] = useState(() => isEmailConfirmSkipped(workspaceId))
  const pendingSendRef = useRef<{
    content: string
    mode: ComposerMode
    attachmentIds: string[]
    aiAssisted: boolean
  } | null>(null)

  function commitSend(content: string, mode: ComposerMode, attachmentIds: string[], aiAssisted: boolean) {
    // Clear the draft the instant a send is confirmed — mirrors the optimistic
    // bubble appearing instantly. If it later fails, the content isn't lost: it
    // lives on in the failedSends retry chip (persisted with the draft).
    clearDraft(conversationId)
    aiAssistedRef.current = false
    stopTyping()
    void attemptSend(content, mode, attachmentIds, { aiAssisted })
  }

  function handleSendPress() {
    if (!canSend || sendingRef.current) return
    const content = trimmed
    const mode = draft.mode
    const attachmentIds = doneAttachmentIds
    const aiAssisted = aiAssistedRef.current
    if (mode === 'reply' && willSendAsEmail && !emailConfirmSkipped) {
      pendingSendRef.current = { content, mode, attachmentIds, aiAssisted }
      setEmailConfirmOpen(true)
      return
    }
    commitSend(content, mode, attachmentIds, aiAssisted)
  }

  function handleConfirmEmailSend() {
    const pending = pendingSendRef.current
    pendingSendRef.current = null
    setEmailConfirmOpen(false)
    if (dontAskAgain) {
      persistEmailConfirmSkip(workspaceId)
      setEmailConfirmSkipped(true)
    }
    if (pending) commitSend(pending.content, pending.mode, pending.attachmentIds, pending.aiAssisted)
  }

  function handleRetry(failedSend: FailedSend) {
    void attemptSend(failedSend.content, failedSend.mode, failedSend.attachmentIds ?? [], {
      retryId: failedSend.id,
      aiAssisted: failedSend.aiAssisted,
    })
  }

  async function confirmCurrentPrimary() {
    if (!conversation) return
    try {
      await updateEmailRecipients.mutateAsync({
        conversationId: conversation.id,
        payload: {
          confirm_primary: true,
          cc_emails: normalizeRecipientEmails(conversation.email_cc ?? [], [conversation.customer_email ?? '']),
        },
      })
      toast.success('Primary recipient confirmed')
      haptic('notificationSuccess')
    } catch {
      toast.error('Could not confirm primary recipient')
      haptic('notificationError')
    }
  }

  async function makeSuggestedPrimary() {
    const suggestedEmail = conversation?.suggested_primary_recipient_email?.trim()
    if (!conversation || !suggestedEmail) return
    try {
      await updateEmailRecipients.mutateAsync({
        conversationId: conversation.id,
        payload: {
          primary_recipient_email: suggestedEmail,
          primary_recipient_name: conversation.suggested_primary_recipient_name ?? undefined,
          confirm_primary: true,
        },
      })
      toast.success('Primary recipient updated')
      haptic('notificationSuccess')
    } catch {
      toast.error('Could not update primary recipient')
      haptic('notificationError')
    }
  }

  return (
    <div className="flex flex-col">
      {draft.failedSends.length > 0 && (
        <div className="flex flex-col gap-1.5 px-3 pb-2">
          {draft.failedSends.map((failedSend) => (
            <div
              key={failedSend.id}
              className="flex items-center gap-2 rounded-lg border border-destructive/40 bg-destructive/10 px-2.5 py-1.5 text-footnote text-destructive"
            >
              <span className="min-w-0 flex-1 truncate">
                {failedSend.content || attachmentLabel(failedSend.attachmentIds?.length ?? 0)}
              </span>
              <button
                type="button"
                onClick={() => handleRetry(failedSend)}
                className="shrink-0 font-medium underline underline-offset-2"
              >
                Retry
              </button>
              <button
                type="button"
                aria-label="Dismiss failed message"
                onClick={() => removeFailedSend(conversationId, failedSend.id)}
                className="shrink-0 rounded-full p-0.5 opacity-60 active:opacity-100"
              >
                <X className="h-3 w-3" />
              </button>
            </div>
          ))}
        </div>
      )}

      <div
        className="border-t border-border/50 bg-background pb-[max(var(--safe-bottom),8px)] pt-2 transition-[margin-bottom] duration-150 ease-out mb-[var(--keyboard-inset)]"
      >
        {undoText !== null && (
          <div className="flex items-center gap-2 px-3 pt-2 text-footnote text-muted-foreground">
            <Sparkles className="h-3.5 w-3.5 shrink-0 text-primary" />
            <span className="min-w-0 flex-1 truncate">Draft rewritten</span>
            <button
              type="button"
              onClick={handleUndoRewrite}
              className="flex shrink-0 items-center gap-1 font-medium text-primary underline-offset-2 active:underline"
            >
              <Undo2 className="h-3.5 w-3.5" />
              Undo
            </button>
          </div>
        )}

        <div
          className={cn(
            'mx-3 overflow-hidden rounded-[24px] border shadow-[0_8px_30px_rgba(15,23,42,0.08)] transition-colors',
            isNote
              ? 'border-amber-300/70 bg-amber-50 dark:border-amber-800/70 dark:bg-amber-950/30'
              : 'border-border/80 bg-card',
          )}
        >
          {primaryRecipientUnconfirmed && !isNote && (
            <div className="space-y-2 border-b border-amber-200 bg-amber-50 px-3 py-3 text-amber-900 dark:border-amber-900/60 dark:bg-amber-950/35 dark:text-amber-100">
              <div className="flex items-start gap-2">
                <Mail className="mt-0.5 h-4 w-4 shrink-0" />
                <p className="text-footnote">Support was copied on this email. Confirm who should receive replies.</p>
              </div>
              <div className="flex flex-wrap gap-2 pl-6">
                {conversation?.suggested_primary_recipient_email?.trim() && (
                  <Pressable
                    disabled={updateEmailRecipients.isPending}
                    onPress={() => void makeSuggestedPrimary()}
                    className="h-auto min-h-8 w-auto min-w-0 rounded-full bg-amber-600 px-3 text-caption font-semibold text-white"
                  >
                    Use {conversation.suggested_primary_recipient_email.trim()}
                  </Pressable>
                )}
                {conversation?.customer_email?.trim() && (
                  <Pressable
                    disabled={updateEmailRecipients.isPending}
                    onPress={() => void confirmCurrentPrimary()}
                    className="h-auto min-h-8 w-auto min-w-0 rounded-full border border-amber-300 bg-background/70 px-3 text-caption font-semibold text-foreground"
                  >
                    Keep {conversation.customer_email.trim()}
                  </Pressable>
                )}
              </div>
            </div>
          )}
          <div className="flex items-center gap-1 px-3 pb-1 pt-2.5">
            <Pressable
              haptic="selection"
              aria-label="Reply"
              aria-pressed={!isNote}
              onPress={() => setMode(conversationId, 'reply')}
              className="flex h-10 min-h-0 min-w-[60px] items-center justify-center"
            >
              <span
                className={cn(
                  'rounded-full px-3 py-1 text-caption font-semibold transition-colors',
                  !isNote ? 'bg-primary/10 text-primary' : 'text-muted-foreground active:bg-muted',
                )}
              >
                Reply
              </span>
            </Pressable>
            <Pressable
              haptic="selection"
              aria-label="Note"
              aria-pressed={isNote}
              onPress={() => setMode(conversationId, 'note')}
              className="flex h-10 min-h-0 min-w-[60px] items-center justify-center"
            >
              <span
                className={cn(
                  'rounded-full px-3 py-1 text-caption font-semibold transition-colors',
                  isNote
                    ? 'bg-amber-200/70 text-amber-800 dark:bg-amber-900/50 dark:text-amber-300'
                    : 'text-muted-foreground active:bg-muted',
                )}
              >
                Note
              </span>
            </Pressable>
          </div>

          {pendingAttachments.length > 0 && (
            <div className="flex gap-2 overflow-x-auto px-3 pb-2">
              {pendingAttachments.map((attachment) => (
                <div
                  key={attachment.localId}
                  className={cn(
                    'relative flex h-16 min-w-0 max-w-52 items-center gap-2 rounded-xl border bg-background px-2 pr-8',
                    attachment.status === 'error' ? 'border-destructive/50' : 'border-border/60',
                  )}
                >
                  {attachment.previewUrl ? (
                    <img src={attachment.previewUrl} alt="" className="h-11 w-11 shrink-0 rounded-lg object-cover" />
                  ) : (
                    <span className="flex h-11 w-11 shrink-0 items-center justify-center rounded-lg bg-muted">
                      <FileText className="h-5 w-5 text-muted-foreground" />
                    </span>
                  )}
                  <span className="min-w-0 flex-1">
                    <span className="block truncate text-caption font-medium">{attachment.file.name}</span>
                    <span className={cn(
                      'mt-0.5 flex items-center gap-1 text-caption',
                      attachment.status === 'error' ? 'text-destructive' : 'text-muted-foreground',
                    )}>
                      {attachment.status === 'uploading' && <LoaderCircle className="h-3 w-3 animate-spin" />}
                      {attachment.status === 'uploading' ? 'Uploading' : attachment.status === 'error' ? 'Upload failed' : 'Ready'}
                    </span>
                  </span>
                  {attachment.status === 'error' && (
                    <button
                      type="button"
                      aria-label={`Retry upload ${attachment.file.name}`}
                      disabled={phase === 'sending'}
                      onClick={() => retryAttachment(attachment)}
                      className="absolute bottom-1.5 right-1.5 rounded-full p-1 text-destructive active:bg-destructive/10 disabled:opacity-40"
                    >
                      <RotateCcw className="h-3.5 w-3.5" />
                    </button>
                  )}
                  <button
                    type="button"
                    aria-label={`Remove ${attachment.file.name}`}
                    disabled={phase === 'sending'}
                    onClick={() => removeAttachment(attachment)}
                    className="absolute right-1.5 top-1.5 rounded-full bg-background/90 p-1 text-muted-foreground shadow-sm active:bg-muted disabled:opacity-40"
                  >
                    <X className="h-3.5 w-3.5" />
                  </button>
                </div>
              ))}
            </div>
          )}

        {mentionToken ? (
          <div className="mx-3 mb-1 max-h-44 overflow-y-auto rounded-xl border border-border/60 bg-background shadow-lg">
            {mentionItems.length > 0 ? (
              mentionItems.map((item) => (
                <button
                  key={item.id}
                  type="button"
                  onClick={() => insertMention(item, mentionToken)}
                  className="flex w-full items-center gap-2.5 border-b border-border/40 px-3 py-2 text-left last:border-0 active:bg-muted"
                >
                  <span className="relative shrink-0">
                    <Avatar name={item.label} src={item.avatarUrl ?? undefined} size={28} />
                    {item.presenceStatus && <span aria-hidden className={`absolute bottom-0 right-0 h-2 w-2 rounded-full border-2 border-background ${teammatePresenceDotClass(item.presenceStatus)}`} />}
                  </span>
                  <span className="min-w-0 flex-1">
                    <span className="block truncate text-footnote font-medium">{item.label}</span>
                    <span className="block truncate text-caption text-muted-foreground">@{item.handle}{item.presenceStatus ? ` · ${teammatePresenceLabel(item.presenceStatus)}` : ''}</span>
                  </span>
                </button>
              ))
            ) : (
              <div className="px-3 py-3 text-footnote text-muted-foreground">
                {mentionMembers.length === 0 ? 'No teammates available' : 'No teammates match this mention'}
              </div>
            )}
          </div>
        ) : activeToken && inlineSuggestions.length > 0 ? (
          <div className="mx-3 mb-1 max-h-44 overflow-y-auto rounded-xl border border-border/60 bg-background shadow-lg">
            {inlineSuggestions.map((response) => (
              <button
                key={response.id}
                type="button"
                onClick={() => insertCanned(response, { start: activeToken.start, end: activeToken.end })}
                className="flex w-full flex-col gap-0.5 border-b border-border/40 px-3 py-2 text-left last:border-0 active:bg-muted"
              >
                <span className="flex items-center gap-1.5 text-footnote font-semibold text-primary">
                  <Zap className="h-3 w-3" />
                  {response.short_code}
                </span>
                <span className="line-clamp-1 text-footnote text-muted-foreground">
                  {stripShortcutContent(response.content)}
                </span>
              </button>
            ))}
          </div>
        ) : null}
          <textarea
            ref={textareaRef}
            rows={1}
            value={draft.text}
            onChange={(event) => {
              const next = event.target.value
              setText(conversationId, next)
              setCursor(event.target.selectionStart ?? next.length)
              if (!isNote) notifyTyping(next)
            }}
            onSelect={(event) => setCursor(event.currentTarget.selectionStart ?? 0)}
            placeholder={isNote ? 'Internal note… (@ to mention)' : replyPlaceholder}
            style={{ minHeight: MIN_TEXTAREA_HEIGHT_PX, maxHeight: MAX_TEXTAREA_HEIGHT_PX }}
            className="w-full resize-none overflow-y-auto bg-transparent px-3.5 py-2 text-body text-foreground outline-none placeholder:text-muted-foreground"
          />
          <div className="flex items-center gap-1 px-2.5 pb-2.5">
            <input
              ref={fileInputRef}
              type="file"
              multiple
              accept={ATTACHMENT_ACCEPT}
              aria-label="Choose attachments"
              onChange={handleFileInput}
              className="hidden"
            />
            <Pressable
              aria-label="Open emoji picker"
              haptic="selection"
              disabled={phase === 'sending'}
              onPress={() => setEmojiSheetOpen(true)}
              className="flex h-9 w-9 items-center justify-center rounded-full text-muted-foreground active:bg-muted disabled:opacity-40"
            >
              <Smile className="h-5 w-5" />
            </Pressable>
            <Pressable
              aria-label="Attach files"
              haptic="selection"
              disabled={phase === 'sending'}
              onPress={() => fileInputRef.current?.click()}
              className="flex h-9 w-9 items-center justify-center rounded-full text-muted-foreground active:bg-muted disabled:opacity-40"
            >
              <Paperclip className="h-5 w-5" />
            </Pressable>
            {isNote && (
              <Pressable
                aria-label="Mention teammate"
                haptic="selection"
                disabled={phase === 'sending'}
                onPress={startMention}
                className="flex h-9 w-9 items-center justify-center rounded-full text-amber-700 active:bg-amber-100 disabled:opacity-40 dark:text-amber-300 dark:active:bg-amber-900/40"
              >
                <AtSign className="h-5 w-5" />
              </Pressable>
            )}
            <Pressable
              aria-label="Canned responses"
              haptic="selection"
              disabled={phase === 'sending'}
              onPress={() => setCannedSheetOpen(true)}
              className="flex h-9 w-9 items-center justify-center rounded-full text-muted-foreground active:bg-muted disabled:opacity-40"
            >
              <Zap className="h-5 w-5" />
            </Pressable>
            <Pressable
              aria-label="AI writing tools"
              haptic="selection"
              disabled={trimmed.length === 0 || phase === 'sending' || busyOperation !== null}
              onPress={() => setAiSheetOpen(true)}
              className="flex h-9 w-9 items-center justify-center rounded-full text-primary active:bg-primary/10 disabled:opacity-40"
            >
              <Sparkles className="h-5 w-5" />
            </Pressable>
            <div className="ml-auto">
              <SendButton state={buttonState} onPress={handleSendPress} />
            </div>
          </div>
        </div>
      </div>

      <AIToolsSheet
        open={aiSheetOpen}
        onOpenChange={setAiSheetOpen}
        busyOperation={busyOperation}
        onSelect={(operation) => void handleRewrite(operation)}
      />

      <CannedResponsesSheet
        workspaceId={workspaceId}
        open={cannedSheetOpen}
        onOpenChange={setCannedSheetOpen}
        responses={cannedResponses}
        loading={cannedQuery.isPending}
        canManage={canManageShortcuts}
        onSelect={(response) => {
          insertCanned(response)
          setCannedSheetOpen(false)
        }}
      />

      <EmojiPickerSheet
        open={emojiSheetOpen}
        onOpenChange={setEmojiSheetOpen}
        onSelect={insertEmoji}
      />

      <UpgradeRequiredSheet
        reason={upgradeReason}
        onOpenChange={(open) => {
          if (!open) setUpgradeReason(null)
        }}
      />

      <EmailConfirmSheet
        open={emailConfirmOpen}
        onOpenChange={setEmailConfirmOpen}
        customerEmail={variableContext.customer?.email ?? undefined}
        dontAskAgain={dontAskAgain}
        onToggleDontAskAgain={() => setDontAskAgain((value) => !value)}
        onConfirm={handleConfirmEmailSend}
      />
    </div>
  )
}
