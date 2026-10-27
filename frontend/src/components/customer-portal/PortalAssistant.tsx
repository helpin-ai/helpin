import { useEffect, useId, useRef, useState, type FormEvent, type KeyboardEvent } from 'react'
import { Link, useNavigate } from '@tanstack/react-router'
import type { AssistantMessage } from '@helpin-ai/shared'
import { QuietPrimaryAction, QuietTextAction } from '@/components/design-system/quiet'
import { useRevealText } from '@/hooks/useRevealText'
import { ArrowUpRight01Icon, Cancel01Icon, Copy01Icon, File01Icon, PencilEdit02Icon, SparklesIcon, ThumbsDownIcon, ThumbsUpIcon, Tick01Icon } from '@/lib/icons'
import { cn } from '@/lib/utils'
import { PortalActivityRow, PortalMessageContent } from './PortalConversation'
import { usePortalAssistant } from './usePortalAssistant'

/**
 * PortalAskAI is the portal's Ask AI entry: a header action that opens a side
 * panel where customers ask questions and get answers from the help articles
 * they may read. When the articles do not answer, it starts a support
 * request with the conversation filled in.
 */
export function PortalAskAI({ slug }: { slug: string }) {
  const [open, setOpen] = useState(false)
  return (
    <>
      <QuietTextAction onClick={() => setOpen(true)} aria-expanded={open}>
        <SparklesIcon className="size-3.5" aria-hidden="true" />
        Ask AI
      </QuietTextAction>
      {open ? <PortalAssistantPanel slug={slug} onClose={() => setOpen(false)} /> : null}
    </>
  )
}

function PortalAssistantPanel({ slug, onClose }: { slug: string; onClose: () => void }) {
  const assistant = usePortalAssistant(slug)
  const navigate = useNavigate()
  const [draft, setDraft] = useState('')
  const inputRef = useRef<HTMLTextAreaElement>(null)
  const scrollRef = useRef<HTMLDivElement>(null)
  const stickToBottom = useRef(true)
  const titleId = useId()

  useEffect(() => {
    inputRef.current?.focus()
    const onKey = (event: globalThis.KeyboardEvent) => {
      if (event.key === 'Escape') onClose()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [onClose])

  // Follow the conversation unless the customer scrolled up.
  useEffect(() => {
    const element = scrollRef.current
    if (element && stickToBottom.current) element.scrollTop = element.scrollHeight
  })

  const submit = (event?: FormEvent) => {
    event?.preventDefault()
    const text = draft.trim()
    if (!text || assistant.busy) return
    assistant.send(text)
    setDraft('')
    stickToBottom.current = true
  }

  const onInputKey = (event: KeyboardEvent<HTMLTextAreaElement>) => {
    if (event.key === 'Enter' && !event.shiftKey && !event.nativeEvent.isComposing) {
      event.preventDefault()
      submit()
    }
  }

  const contactSupport = () => {
    const question = [...assistant.messages].reverse().find((message) => message.role === 'user')?.content ?? ''
    const transcript = assistant.messages
      .filter((message) => message.status !== 'pending' && message.content)
      .map((message) => `${message.role === 'user' ? 'Me' : 'Help assistant'}: ${message.content}`)
      .join('\n\n')
    onClose()
    void navigate({
      to: '/portal/$slug/new',
      params: { slug },
      search: { subject: question.slice(0, 120), message: `${question}\n\n---\nI asked the help assistant first:\n\n${transcript}`.slice(0, 8000), ...(assistant.sessionId ? { assistant_session: assistant.sessionId } : {}) } as never,
    })
  }

  const empty = assistant.messages.length === 0 && !assistant.restoring

  return (
    <aside role="dialog" aria-labelledby={titleId} className="fixed inset-y-0 right-0 z-50 flex w-full flex-col border-l border-quiet-divider-strong bg-background shadow-2xl sm:w-[440px]">
      <header className="flex h-14 flex-none items-center gap-3 border-b border-quiet-divider-strong px-4">
        <span className="flex size-7 items-center justify-center rounded-full bg-quiet-action text-quiet-action-ink" aria-hidden="true">
          <SparklesIcon className="size-3.5" />
        </span>
        <div className="min-w-0 flex-1">
          <h2 id={titleId} className="text-sm font-semibold leading-tight">Ask AI</h2>
          <p className="truncate text-[12px] text-quiet-text-tertiary">Answers from the help articles</p>
        </div>
        <button type="button" onClick={() => { assistant.reset(); inputRef.current?.focus() }} disabled={assistant.messages.length === 0} aria-label="New chat" title="New chat" className="rounded-md p-1.5 text-quiet-text-tertiary hover:bg-quiet-row-hover hover:text-quiet-text-primary disabled:pointer-events-none disabled:opacity-40">
          <PencilEdit02Icon className="size-4" />
        </button>
        <button type="button" onClick={onClose} aria-label="Close Ask AI" title="Close" className="rounded-md p-1.5 text-quiet-text-tertiary hover:bg-quiet-row-hover hover:text-quiet-text-primary">
          <Cancel01Icon className="size-4" />
        </button>
      </header>

      <div
        ref={scrollRef}
        onScroll={(event) => {
          const element = event.currentTarget
          stickToBottom.current = element.scrollHeight - element.scrollTop - element.clientHeight < 48
        }}
        className="flex-1 overflow-y-auto px-4 py-5"
        aria-live="polite"
        aria-busy={assistant.busy}
      >
        {assistant.restoring ? (
          <p role="status" className="text-sm text-quiet-text-tertiary">Loading your conversation…</p>
        ) : empty ? (
          <div className="flex h-full flex-col items-center justify-center px-4 text-center">
            <span className="mb-3 flex size-10 items-center justify-center rounded-full bg-quiet-icon-well text-quiet-text-secondary" aria-hidden="true">
              <SparklesIcon className="size-5" />
            </span>
            <p className="text-sm font-medium">How can I help?</p>
            <p className="mt-1 max-w-[280px] text-sm text-quiet-text-tertiary">Ask a question. Answers come from the help articles, with links to the sources.</p>
          </div>
        ) : (
          <ol className="space-y-6">
            {assistant.messages.map((message) => (
              <li key={message.id}>
                {message.role === 'user' ? (
                  <div className="flex justify-end">
                    <p className="max-w-[85%] whitespace-pre-wrap break-words rounded-2xl rounded-br-md bg-quiet-icon-well px-3.5 py-2 text-sm">{message.content}</p>
                  </div>
                ) : (
                  <PortalAssistantReply slug={slug} message={message} fresh={assistant.freshAnswerIds.has(message.id)} onFeedback={assistant.feedback} onRetry={assistant.retry} onContactSupport={contactSupport} />
                )}
              </li>
            ))}
          </ol>
        )}
      </div>

      <div className="flex-none border-t border-quiet-divider-strong px-4 pb-4 pt-3">
        {assistant.notice ? <p role="status" className="mb-2 text-[12px] text-quiet-text-tertiary">{assistant.notice}</p> : null}
        <form onSubmit={submit} className="flex items-end gap-2 rounded-xl border border-quiet-divider-strong px-3 py-2 focus-within:border-quiet-text-tertiary">
          <textarea
            ref={inputRef}
            value={draft}
            onChange={(event) => setDraft(event.target.value)}
            onKeyDown={onInputKey}
            rows={1}
            maxLength={1000}
            placeholder={assistant.messages.length ? 'Ask a follow-up…' : 'Ask a question…'}
            aria-label="Ask AI a question"
            className="max-h-36 min-h-6 flex-1 resize-none bg-transparent py-1 text-sm outline-none [field-sizing:content] placeholder:text-quiet-text-tertiary"
          />
          <QuietPrimaryAction type="submit" disabled={!draft.trim() || assistant.busy} aria-label="Send question" className="h-8 px-3">Send</QuietPrimaryAction>
        </form>
        <p className="mt-2 text-center text-[11px] text-quiet-text-tertiary">
          {assistant.turnsLeft !== null && assistant.turnsLeft <= 3 ? `${assistant.turnsLeft} questions left in this chat. ` : ''}AI can make mistakes. Check the sources.
        </p>
      </div>
    </aside>
  )
}

function PortalAssistantReply({ slug, message, fresh, onFeedback, onRetry, onContactSupport }: {
  slug: string
  message: AssistantMessage
  fresh: boolean
  onFeedback: (messageId: string, helpful: boolean) => void
  onRetry: () => void
  onContactSupport: () => void
}) {
  const { visible, revealing } = useRevealText(message.content, fresh)
  const [copied, setCopied] = useState(false)

  if (message.status === 'pending') {
    return <ul><PortalActivityRow label={message.progress_label || 'Thinking…'} /></ul>
  }
  if (message.status === 'failed') {
    return (
      <div role="alert" className="flex items-center gap-3 text-sm text-quiet-text-tertiary">
        <span>Something went wrong while answering.</span>
        <QuietTextAction onClick={onRetry}>Try again</QuietTextAction>
      </div>
    )
  }

  const voted = message.helpful !== undefined && message.helpful !== null
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(message.content)
      setCopied(true)
      window.setTimeout(() => setCopied(false), 1500)
    } catch {
      setCopied(false)
    }
  }

  return (
    <div className="text-sm leading-relaxed" aria-busy={revealing}>
      <PortalMessageContent content={visible} markdown />
      {!revealing && message.citations.length > 0 ? (
        <div className="mt-3">
          <p className="mb-1.5 text-[12px] font-medium text-quiet-text-tertiary">Sources</p>
          <ul className="space-y-1">
            {message.citations.map((citation) => {
              const className = 'group inline-flex max-w-full items-center gap-1.5 rounded-md border border-quiet-divider-strong px-2 py-1 text-[12px] hover:bg-quiet-row-hover'
              const body = (
                <>
                  <File01Icon className="size-3.5 flex-none text-quiet-text-tertiary" aria-hidden="true" />
                  <span className="truncate">{citation.title}</span>
                  <ArrowUpRight01Icon className="size-3 flex-none text-quiet-text-tertiary" aria-hidden="true" />
                </>
              )
              return (
                <li key={citation.article_id}>
                  {citation.portal_article_id ? (
                    <Link to="/portal/$slug/help/$articleId" params={{ slug, articleId: citation.portal_article_id }} className={className}>{body}</Link>
                  ) : citation.url ? (
                    <a href={citation.url} target="_blank" rel="noopener noreferrer" className={className}>{body}</a>
                  ) : (
                    <span className={className}>{body}</span>
                  )}
                </li>
              )
            })}
          </ul>
        </div>
      ) : null}
      {!revealing && message.status === 'handed_off' ? (
        <QuietPrimaryAction className="mt-3" onClick={onContactSupport}>Contact support</QuietPrimaryAction>
      ) : null}
      {!revealing ? (
        <div className="mt-2 flex items-center gap-1 text-quiet-text-tertiary">
          <button type="button" onClick={() => void copy()} aria-label={copied ? 'Copied' : 'Copy answer'} title="Copy" className="rounded p-1 hover:bg-quiet-row-hover hover:text-quiet-text-primary">
            {copied ? <Tick01Icon className="size-3.5" /> : <Copy01Icon className="size-3.5" />}
          </button>
          <button type="button" onClick={() => onFeedback(message.id, true)} disabled={voted} aria-label="Helpful" aria-pressed={message.helpful === true} title="Helpful" className={cn('rounded p-1 hover:bg-quiet-row-hover hover:text-quiet-text-primary disabled:hover:bg-transparent', message.helpful === true && 'text-quiet-accent')}>
            <ThumbsUpIcon className="size-3.5" />
          </button>
          <button type="button" onClick={() => onFeedback(message.id, false)} disabled={voted} aria-label="Not helpful" aria-pressed={message.helpful === false} title="Not helpful" className={cn('rounded p-1 hover:bg-quiet-row-hover hover:text-quiet-text-primary disabled:hover:bg-transparent', message.helpful === false && 'text-quiet-accent')}>
            <ThumbsDownIcon className="size-3.5" />
          </button>
          {voted ? <span className="ml-1 text-[12px]">Thanks for the feedback.</span> : null}
        </div>
      ) : null}
    </div>
  )
}
