import { useEffect, useId, useRef, useState, type FormEvent, type KeyboardEvent } from 'react'
import { ArrowUp, ArrowUpRight, Check, Copy, FileText, LifeBuoy, RotateCcw, Sparkles, SquarePen, ThumbsDown, ThumbsUp, X } from 'lucide-react'
import { DocsLink } from '@/components/DocsLink'
import { useDocsContext } from '@/contexts/DocsContext'
import { useHelpAssistant } from '@/hooks/useHelpAssistant'
import { useRevealText } from '@/hooks/useRevealText'
import { renderAnswerMarkdown } from '@/lib/answerMarkdown'
import { helpAssistantService } from '@/lib/helpAssistant'
import type { AssistantMessage } from '@/lib/types'

/** A question to ask as the panel opens, optionally about the current article. */
export interface AskAIRequest {
  question?: string
  pageArticleId?: string
  pageTitle?: string
}

type HelpinWidget = (method: string, ...args: unknown[]) => void

function helpinWidget(): HelpinWidget | null {
  const candidate = (window as unknown as { helpin?: unknown }).helpin
  return typeof candidate === 'function' ? (candidate as HelpinWidget) : null
}

/**
 * AskAIPanel is the help center's Ask AI conversation: a side panel where
 * visitors ask questions and get answers from the published articles, with
 * sources, feedback, and a way to contact support.
 */
export function AskAIPanel({ request, onClose }: { request: AskAIRequest | null; onClose: () => void }) {
  const { config, subdomain, basepath } = useDocsContext()
  const assistant = useHelpAssistant(true)
  const [draft, setDraft] = useState('')
  const [page, setPage] = useState<AskAIRequest | null>(null)
  const [handoffNote, setHandoffNote] = useState<string | null>(null)
  const inputRef = useRef<HTMLTextAreaElement>(null)
  const scrollRef = useRef<HTMLDivElement>(null)
  const stickToBottom = useRef(true)
  const titleId = useId()
  const brand = config.brand_name || 'our'
  const accent = config.brand_color || undefined

  // Keep the chat widget launcher out of the panel while it is open.
  useEffect(() => {
    helpinWidget()?.('hide')
    return () => helpinWidget()?.('show')
  }, [])

  // A request from a page or the search dialog: ask it, or keep its page.
  const handled = useRef<AskAIRequest | null>(null)
  useEffect(() => {
    if (!request || handled.current === request) return
    handled.current = request
    if (request.pageArticleId) setPage(request)
    if (request.question) assistant.send(request.question, request.pageArticleId)
    // assistant.send changes with every answer; only a new request asks again.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [request])

  useEffect(() => {
    inputRef.current?.focus()
  }, [])

  useEffect(() => {
    const onKey = (event: globalThis.KeyboardEvent) => {
      if (event.key === 'Escape') onClose()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [onClose])

  // Follow the conversation unless the reader scrolled up.
  useEffect(() => {
    const element = scrollRef.current
    if (element && stickToBottom.current) element.scrollTop = element.scrollHeight
  })

  const submit = (event?: FormEvent) => {
    event?.preventDefault()
    const text = draft.trim()
    if (!text || assistant.busy) return
    assistant.send(text, page?.pageArticleId)
    setDraft('')
    stickToBottom.current = true
  }

  const onInputKey = (event: KeyboardEvent<HTMLTextAreaElement>) => {
    if (event.key === 'Enter' && !event.shiftKey && !event.nativeEvent.isComposing) {
      event.preventDefault()
      submit()
    }
  }

  const contactSupport = async () => {
    const question = [...assistant.messages].reverse().find((message) => message.role === 'user')?.content ?? ''
    const transcript = assistant.messages
      .filter((message) => message.status !== 'pending' && message.content)
      .map((message) => `${message.role === 'user' ? 'Me' : 'Help assistant'}: ${message.content}`)
      .join('\n\n')
    const portal = await helpAssistantService.portalSlug(subdomain)
    if (portal.data?.slug) {
      const params = new URLSearchParams({ subject: question.slice(0, 120), message: `${question}\n\n---\nI asked the help assistant first:\n\n${transcript}`.slice(0, 8000) })
      window.location.assign(`${basepath}/requests/new?${params.toString()}`)
      return
    }
    const widget = helpinWidget()
    if (widget) {
      onClose()
      widget('show')
      widget('openNewMessage', question)
      return
    }
    setHandoffNote(config.support_email ? `Email ${config.support_email} and the team will help you.` : 'Contact the team through your usual support channel.')
  }

  const empty = assistant.messages.length === 0 && !assistant.restoring
  const lowTurns = assistant.turnsLeft !== null && assistant.turnsLeft <= 3

  return (
    <aside
      role="dialog"
      aria-labelledby={titleId}
      className="fixed inset-y-0 right-0 z-50 flex w-full flex-col border-l border-border bg-background shadow-2xl animate-in slide-in-from-right duration-200 sm:w-[440px]"
    >
      <header className="flex h-14 flex-none items-center gap-3 border-b border-border px-4">
        <span className="flex size-7 items-center justify-center rounded-full text-white" style={{ background: accent ?? 'var(--color-primary)' }} aria-hidden="true">
          <Sparkles className="size-3.5" />
        </span>
        <div className="min-w-0 flex-1">
          <h2 id={titleId} className="text-sm font-semibold leading-tight">Ask AI</h2>
          <p className="truncate text-xs text-muted-foreground">Answers from {brand === 'our' ? 'our' : `${brand}'s`} help articles</p>
        </div>
        <button type="button" onClick={() => { assistant.reset(); setPage(null); setHandoffNote(null); inputRef.current?.focus() }} disabled={assistant.messages.length === 0} aria-label="New chat" title="New chat" className="rounded-md p-1.5 text-muted-foreground transition-colors hover:bg-muted hover:text-foreground disabled:pointer-events-none disabled:opacity-40">
          <SquarePen className="size-4" />
        </button>
        <button type="button" onClick={onClose} aria-label="Close Ask AI" title="Close" className="rounded-md p-1.5 text-muted-foreground transition-colors hover:bg-muted hover:text-foreground">
          <X className="size-4" />
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
          <p role="status" className="text-sm text-muted-foreground">Loading your conversation…</p>
        ) : empty ? (
          <EmptyState page={page} onAsk={(question) => assistant.send(question, page?.pageArticleId)} accent={accent} />
        ) : (
          <ol className="space-y-6">
            {assistant.messages.map((message) => (
              <li key={message.id}>
                {message.role === 'user'
                  ? <UserBubble text={message.content} />
                  : <AssistantReply message={message} fresh={assistant.freshAnswerIds.has(message.id)} accent={accent} onFeedback={assistant.feedback} onRetry={assistant.retry} onContactSupport={() => void contactSupport()} />}
              </li>
            ))}
          </ol>
        )}
        {handoffNote ? <p className="mt-4 rounded-lg border border-border bg-muted/40 px-3 py-2 text-sm">{handoffNote}</p> : null}
      </div>

      <div className="flex-none border-t border-border px-4 pb-4 pt-3">
        {assistant.notice ? <p role="status" className="mb-2 text-xs text-muted-foreground">{assistant.notice}</p> : null}
        {page?.pageTitle ? (
          <p className="mb-2 flex items-center gap-1.5 truncate text-xs text-muted-foreground">
            <FileText className="size-3.5 flex-none" aria-hidden="true" />
            <span className="truncate">Asking about “{page.pageTitle}”</span>
            <button type="button" onClick={() => setPage(null)} className="ml-auto flex-none underline-offset-2 hover:underline">Remove</button>
          </p>
        ) : null}
        <form onSubmit={submit} className="flex items-end gap-2 rounded-xl border border-border bg-background px-3 py-2 shadow-sm focus-within:ring-2 focus-within:ring-ring/30">
          <textarea
            ref={inputRef}
            value={draft}
            onChange={(event) => setDraft(event.target.value)}
            onKeyDown={onInputKey}
            rows={1}
            maxLength={1000}
            placeholder={assistant.messages.length ? 'Ask a follow-up…' : 'Ask a question…'}
            aria-label="Ask AI a question"
            className="max-h-36 min-h-6 flex-1 resize-none bg-transparent py-1 text-sm outline-none [field-sizing:content] placeholder:text-muted-foreground"
          />
          <button
            type="submit"
            disabled={!draft.trim() || assistant.busy}
            aria-label="Send question"
            className="flex size-8 flex-none items-center justify-center rounded-full text-white transition-opacity disabled:opacity-35"
            style={{ background: accent ?? 'var(--color-primary)' }}
          >
            <ArrowUp className="size-4" />
          </button>
        </form>
        <p className="mt-2 text-center text-[11px] text-muted-foreground">
          {lowTurns ? `${assistant.turnsLeft} questions left in this chat. ` : ''}AI can make mistakes. Check the sources.
        </p>
      </div>
    </aside>
  )
}

function EmptyState({ page, onAsk, accent }: { page: AskAIRequest | null; onAsk: (question: string) => void; accent?: string }) {
  const suggestions = page?.pageTitle
    ? [`Summarize “${page.pageTitle}”`, 'What are the steps on this page?']
    : ['How do I get started?', 'What can I do here?']
  return (
    <div className="flex h-full flex-col items-center justify-center px-4 text-center">
      <span className="mb-3 flex size-10 items-center justify-center rounded-full bg-muted" style={accent ? { color: accent } : undefined} aria-hidden="true">
        <Sparkles className="size-5" />
      </span>
      <p className="text-sm font-medium">How can I help?</p>
      <p className="mt-1 max-w-[280px] text-sm text-muted-foreground">Ask a question. Answers come from the help articles, with links to the sources.</p>
      <div className="mt-5 flex w-full max-w-[320px] flex-col gap-2">
        {suggestions.map((suggestion) => (
          <button key={suggestion} type="button" onClick={() => onAsk(suggestion)} className="rounded-lg border border-border px-3 py-2 text-left text-sm transition-colors hover:bg-muted">
            {suggestion}
          </button>
        ))}
      </div>
    </div>
  )
}

function UserBubble({ text }: { text: string }) {
  return (
    <div className="flex justify-end">
      <p className="max-w-[85%] whitespace-pre-wrap break-words rounded-2xl rounded-br-md bg-muted px-3.5 py-2 text-sm">{text}</p>
    </div>
  )
}

function AssistantReply({ message, fresh, accent, onFeedback, onRetry, onContactSupport }: {
  message: AssistantMessage
  fresh: boolean
  accent?: string
  onFeedback: (messageId: string, helpful: boolean) => void
  onRetry: () => void
  onContactSupport: () => void
}) {
  const { visible, revealing } = useRevealText(message.content, fresh)
  const [copied, setCopied] = useState(false)

  if (message.status === 'pending') {
    return (
      <div className="flex items-center gap-2 text-sm text-muted-foreground" role="status">
        <Sparkles className="size-3.5 animate-pulse" style={accent ? { color: accent } : undefined} aria-hidden="true" />
        <span>{message.progress_label || 'Thinking…'}</span>
        <span className="inline-flex gap-0.5" aria-hidden="true">
          <span className="size-1 animate-bounce rounded-full bg-current [animation-delay:0ms]" />
          <span className="size-1 animate-bounce rounded-full bg-current [animation-delay:150ms]" />
          <span className="size-1 animate-bounce rounded-full bg-current [animation-delay:300ms]" />
        </span>
      </div>
    )
  }

  if (message.status === 'failed') {
    return (
      <div className="flex items-center gap-3 text-sm text-muted-foreground" role="alert">
        <span>Something went wrong while answering.</span>
        <button type="button" onClick={onRetry} className="inline-flex items-center gap-1 font-medium text-foreground underline-offset-2 hover:underline">
          <RotateCcw className="size-3.5" aria-hidden="true" /> Try again
        </button>
      </div>
    )
  }

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
    <div className="text-sm">
      <div className="leading-relaxed text-foreground [&_ol]:my-2 [&_ol]:list-decimal [&_ol]:pl-5 [&_p+p]:mt-2 [&_ul]:my-2 [&_ul]:list-disc [&_ul]:pl-5" aria-busy={revealing}>
        {renderAnswerMarkdown(visible)}
      </div>

      {!revealing && message.citations.length > 0 ? (
        <div className="mt-3">
          <p className="mb-1.5 text-xs font-medium text-muted-foreground">Sources</p>
          <ul className="space-y-1">
            {message.citations.map((citation) => (
              <li key={citation.article_id}>
                <CitationLink title={citation.title} url={citation.url} />
              </li>
            ))}
          </ul>
        </div>
      ) : null}

      {!revealing && message.status === 'handed_off' ? (
        <button type="button" onClick={onContactSupport} className="mt-3 inline-flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-sm font-medium text-white transition-[filter] hover:brightness-95" style={{ background: accent ?? 'var(--color-primary)' }}>
          <LifeBuoy className="size-4" aria-hidden="true" /> Contact support
        </button>
      ) : null}

      {!revealing ? (
        <div className="mt-2 flex items-center gap-1 text-muted-foreground">
          <button type="button" onClick={() => void copy()} aria-label={copied ? 'Copied' : 'Copy answer'} title="Copy" className="rounded p-1 transition-colors hover:bg-muted hover:text-foreground">
            {copied ? <Check className="size-3.5" /> : <Copy className="size-3.5" />}
          </button>
          <button type="button" onClick={() => onFeedback(message.id, true)} disabled={message.helpful !== undefined && message.helpful !== null} aria-label="Helpful" aria-pressed={message.helpful === true} title="Helpful" className="rounded p-1 transition-colors hover:bg-muted hover:text-foreground disabled:hover:bg-transparent" style={message.helpful === true ? { color: accent } : undefined}>
            <ThumbsUp className="size-3.5" />
          </button>
          <button type="button" onClick={() => onFeedback(message.id, false)} disabled={message.helpful !== undefined && message.helpful !== null} aria-label="Not helpful" aria-pressed={message.helpful === false} title="Not helpful" className="rounded p-1 transition-colors hover:bg-muted hover:text-foreground disabled:hover:bg-transparent" style={message.helpful === false ? { color: accent } : undefined}>
            <ThumbsDown className="size-3.5" />
          </button>
          {message.helpful !== undefined && message.helpful !== null ? <span className="ml-1 text-xs">Thanks for the feedback.</span> : null}
        </div>
      ) : null}
    </div>
  )
}

function CitationLink({ title, url }: { title: string; url?: string }) {
  const className = 'group inline-flex max-w-full items-center gap-1.5 rounded-md border border-border px-2 py-1 text-xs transition-colors hover:bg-muted'
  const body = (
    <>
      <FileText className="size-3.5 flex-none text-muted-foreground" aria-hidden="true" />
      <span className="truncate">{title}</span>
      <ArrowUpRight className="size-3 flex-none text-muted-foreground transition-transform group-hover:-translate-y-px group-hover:translate-x-px" aria-hidden="true" />
    </>
  )
  if (!url) return <span className={className}>{body}</span>
  if (/^https?:\/\//.test(url)) {
    return <a href={url} className={className}>{body}</a>
  }
  return <DocsLink to={url} className={className}>{body}</DocsLink>
}
