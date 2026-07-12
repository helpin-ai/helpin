import { useEffect, useLayoutEffect, useRef, useState } from 'react'
import { Sparkles, Undo2, X } from 'lucide-react'
import { useRewriteSupportDraft, useSendMessage, type SupportAIRewriteOperation } from '@helpin-ai/support-core'
import { cn } from '@mobile/lib/cn'
import { haptic } from '@mobile/lib/haptics'
import { Pressable } from '@mobile/ui/pressable'
import { SegmentedControl } from '@mobile/ui/segmented-control'
import { DEFAULT_DRAFT, useDraftStore, type ComposerMode } from './draft-store'
import type { FailedSend } from './failed-sends-reducer'
import { SendButton, type SendButtonState } from './send-button'
import { useTypingBroadcast } from './use-typing-broadcast'
import { AIToolsSheet } from './ai-tools-sheet'

/** How long the "Rewritten · Undo" bar stays before auto-dismissing. */
const UNDO_VISIBLE_MS = 6000

export interface ComposerProps {
  workspaceId: string
  conversationId: string
}

/**
 * `text-body` is 0.9375rem/1.25rem (20px line-height, see index.css) — the
 * textarea grows from 1 to `MAX_LINES` of that, then scrolls internally.
 */
const LINE_HEIGHT_PX = 20
const MAX_LINES = 6
const TEXTAREA_VERTICAL_PADDING_PX = 16 // py-2 (8px top + 8px bottom)
const MAX_TEXTAREA_HEIGHT_PX = LINE_HEIGHT_PX * MAX_LINES + TEXTAREA_VERTICAL_PADDING_PX
const MIN_TEXTAREA_HEIGHT_PX = LINE_HEIGHT_PX + TEXTAREA_VERTICAL_PADDING_PX

const SENT_STATE_MS = 400

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
export function Composer({ workspaceId, conversationId }: ComposerProps) {
  const draft = useDraftStore((state) => state.drafts[conversationId] ?? DEFAULT_DRAFT)
  const setText = useDraftStore((state) => state.setText)
  const setMode = useDraftStore((state) => state.setMode)
  const clearDraft = useDraftStore((state) => state.clearDraft)
  const addFailedSend = useDraftStore((state) => state.addFailedSend)
  const removeFailedSend = useDraftStore((state) => state.removeFailedSend)

  const sendMessage = useSendMessage(workspaceId, conversationId)
  const [phase, setPhase] = useState<'idle' | 'sending' | 'sent'>('idle')

  const isNote = draft.mode === 'note'
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
  const [busyOperation, setBusyOperation] = useState<SupportAIRewriteOperation | null>(null)
  const [undoText, setUndoText] = useState<string | null>(null)
  const undoTimeoutRef = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)
  useEffect(() => () => { if (undoTimeoutRef.current) clearTimeout(undoTimeoutRef.current) }, [])

  async function handleRewrite(operation: SupportAIRewriteOperation) {
    const source = draft.text.trim()
    if (!source || busyOperation) return
    setBusyOperation(operation)
    try {
      const result = await rewriteDraft.mutateAsync({ content: source, operation })
      setText(conversationId, result.content)
      setUndoText(source)
      if (undoTimeoutRef.current) clearTimeout(undoTimeoutRef.current)
      undoTimeoutRef.current = setTimeout(() => setUndoText(null), UNDO_VISIBLE_MS)
      haptic('notificationSuccess')
      setAiSheetOpen(false)
    } catch {
      haptic('notificationError')
    } finally {
      setBusyOperation(null)
    }
  }

  function handleUndoRewrite() {
    if (undoText === null) return
    setText(conversationId, undoText)
    setUndoText(null)
    if (undoTimeoutRef.current) clearTimeout(undoTimeoutRef.current)
    haptic('impactLight')
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
  }, [draft.text])

  useEffect(
    () => () => {
      if (sentTimeoutRef.current) clearTimeout(sentTimeoutRef.current)
    },
    [],
  )

  const trimmed = draft.text.trim()
  const buttonState: SendButtonState =
    phase === 'sending' ? 'sending' : phase === 'sent' ? 'sent' : trimmed.length === 0 ? 'disabled' : 'active'

  async function attemptSend(content: string, mode: ComposerMode, retryId?: string) {
    if (sendingRef.current) return
    sendingRef.current = true
    // A new send during the 400ms 'sent' display is legitimate — cancel the
    // stale timer so it can't fire mid-flight and flip this send's 'sending'
    // state back to 'idle' while the request is still outstanding.
    if (sentTimeoutRef.current) clearTimeout(sentTimeoutRef.current)
    setPhase('sending')
    try {
      await sendMessage.mutateAsync({ content, is_internal: mode === 'note' })
      haptic('notificationSuccess')
      if (retryId) removeFailedSend(conversationId, retryId)
      setPhase('sent')
      sentTimeoutRef.current = setTimeout(() => setPhase('idle'), SENT_STATE_MS)
    } catch {
      haptic('notificationError')
      setPhase('idle')
      // A fresh send (no retryId) needs a new chip; a retry's chip is already
      // in the list — leave it there so the user can retry again.
      if (!retryId) {
        addFailedSend(conversationId, { id: crypto.randomUUID(), content, mode })
      }
    } finally {
      sendingRef.current = false
    }
  }

  function handleSendPress() {
    if (trimmed.length === 0 || sendingRef.current) return
    const content = trimmed
    const mode = draft.mode
    // Clear the draft the instant a send is confirmed by the user (tapping
    // Send) — mirrors the optimistic bubble appearing instantly. If the send
    // later fails, the content isn't lost: it lives on in the failedSends
    // retry chip below (persisted alongside the draft), not back in the
    // (now-empty) input.
    clearDraft(conversationId)
    stopTyping()
    void attemptSend(content, mode)
  }

  function handleRetry(failedSend: FailedSend) {
    void attemptSend(failedSend.content, failedSend.mode, failedSend.id)
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
              <span className="min-w-0 flex-1 truncate">{failedSend.content}</span>
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
        className={cn(
          'border-t border-border/60 pb-[max(var(--safe-bottom),8px)] transition-[margin-bottom] duration-150 ease-out mb-[var(--keyboard-inset)]',
          isNote ? 'bg-amber-500/10' : 'bg-background',
        )}
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

        <div className="flex items-center gap-1 px-3 pt-2 pb-1.5">
          <SegmentedControl<ComposerMode>
            segments={[
              { value: 'reply', label: 'Reply' },
              { value: 'note', label: 'Note' },
            ]}
            value={draft.mode}
            onChange={(mode) => setMode(conversationId, mode)}
            className="w-40"
          />
          <div className="ml-auto flex items-center gap-0.5">
            <Pressable
              aria-label="AI writing tools"
              haptic="selection"
              disabled={trimmed.length === 0 || phase === 'sending' || busyOperation !== null}
              onPress={() => setAiSheetOpen(true)}
              className="flex h-9 w-9 items-center justify-center rounded-full text-primary active:bg-primary/10 disabled:opacity-40"
            >
              <Sparkles className="h-5 w-5" />
            </Pressable>
          </div>
        </div>
        <div className="flex items-end gap-2 px-3 pb-2">
          <textarea
            ref={textareaRef}
            rows={1}
            value={draft.text}
            onChange={(event) => {
              const next = event.target.value
              setText(conversationId, next)
              if (!isNote) notifyTyping(next)
            }}
            placeholder={isNote ? 'Internal note…' : 'Reply…'}
            style={{ minHeight: MIN_TEXTAREA_HEIGHT_PX, maxHeight: MAX_TEXTAREA_HEIGHT_PX }}
            className="flex-1 resize-none overflow-y-auto rounded-2xl border border-input bg-background px-3 py-2 text-body text-foreground outline-none placeholder:text-muted-foreground"
          />
          <SendButton state={buttonState} onPress={handleSendPress} />
        </div>
      </div>

      <AIToolsSheet
        open={aiSheetOpen}
        onOpenChange={setAiSheetOpen}
        busyOperation={busyOperation}
        onSelect={(operation) => void handleRewrite(operation)}
      />
    </div>
  )
}
