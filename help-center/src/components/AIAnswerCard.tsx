import { useEffect, useRef, useState } from 'react'
import { Sparkles, ThumbsDown, ThumbsUp, ArrowRight } from 'lucide-react'
import { DocsLink } from '@/components/DocsLink'
import { useDocsContext } from '@/contexts/DocsContext'
import { buildCanonicalArticlePath, isMultilingualEnabled } from '@/lib/locale'
import { helpCenterService } from '@/lib/services'
import type { AIAnswerResponse } from '@/lib/types'

export const AI_ANSWER_MIN_QUERY_CHARS = 8

export type AIAnswerState =
  | { phase: 'idle' }
  | { phase: 'loading' }
  | { phase: 'done'; response: AIAnswerResponse }
  | { phase: 'rate_limited' }
  | { phase: 'error' }

/**
 * Shared AI-answer state machine: `ask` runs one generation (always from an
 * explicit user gesture — never on load, so crawlers cannot spend tokens),
 * `sendFeedback` records one thumbs vote per answer.
 */
export function useAIAnswer(locale: string, space?: string) {
  const { subdomain, config, enabledLocales } = useDocsContext()
  const multilingualEnabled = isMultilingualEnabled(enabledLocales)
  const [state, setState] = useState<AIAnswerState>({ phase: 'idle' })
  const [vote, setVote] = useState<'up' | 'down' | null>(null)
  const askSeq = useRef(0)

  const enabled = config?.ai_answers_enabled !== false

  const reset = () => {
    askSeq.current += 1
    setState({ phase: 'idle' })
    setVote(null)
  }

  const ask = async (query: string) => {
    if (!enabled || query.trim().length < AI_ANSWER_MIN_QUERY_CHARS) return
    const seq = ++askSeq.current
    setState({ phase: 'loading' })
    setVote(null)
    const res = await helpCenterService.askAnswer(
      subdomain,
      locale,
      multilingualEnabled,
      { query, space },
    )
    if (seq !== askSeq.current) return // superseded by a newer ask/reset
    if (res.error || !res.data) {
      setState(res.status === 429 ? { phase: 'rate_limited' } : { phase: 'error' })
      return
    }
    setState({ phase: 'done', response: res.data })
  }

  const sendFeedback = (isHelpful: boolean) => {
    if (state.phase !== 'done' || vote) return
    setVote(isHelpful ? 'up' : 'down')
    void helpCenterService.answerFeedback(
      subdomain,
      locale,
      multilingualEnabled,
      state.response.answer_id,
      { is_helpful: isHelpful },
    )
  }

  return { enabled, state, vote, ask, reset, sendFeedback }
}

interface AIAnswerPanelProps {
  locale: string
  state: AIAnswerState
  vote: 'up' | 'down' | null
  onFeedback: (isHelpful: boolean) => void
  onCitationClick?: () => void
  className?: string
}

/** Presentational answer panel shared by the search page and the dialog. */
export function AIAnswerPanel({
  locale,
  state,
  vote,
  onFeedback,
  onCitationClick,
  className,
}: AIAnswerPanelProps) {
  const { enabledLocales } = useDocsContext()
  const multilingualEnabled = isMultilingualEnabled(enabledLocales)

  if (state.phase === 'idle') return null

  return (
    <section
      className={`rounded-lg border px-4 py-4 ${className ?? ''}`}
      style={{ borderColor: 'var(--hc-border)' }}
      aria-live="polite"
    >
      <div
        className="mb-2 flex items-center gap-2 text-xs font-medium uppercase tracking-wide"
        style={{ color: 'var(--hc-text-secondary)' }}
      >
        <Sparkles className={`h-3.5 w-3.5 ${state.phase === 'loading' ? 'animate-pulse' : ''}`} />
        {state.phase === 'loading' ? 'AI is looking for an answer…' : 'AI answer'}
      </div>

      {state.phase === 'loading' && (
        <div className="space-y-2 py-1" aria-label="Generating answer">
          <div className="h-3 w-11/12 animate-pulse rounded" style={{ background: 'var(--hc-border)' }} />
          <div className="h-3 w-9/12 animate-pulse rounded" style={{ background: 'var(--hc-border)' }} />
          <div className="h-3 w-7/12 animate-pulse rounded" style={{ background: 'var(--hc-border)' }} />
        </div>
      )}

      {state.phase === 'rate_limited' && (
        <p className="text-sm" style={{ color: 'var(--hc-text-secondary)' }}>
          The AI answer limit was reached — please try again later, or browse the
          articles below.
        </p>
      )}

      {state.phase === 'error' && (
        <p className="text-sm" style={{ color: 'var(--hc-text-secondary)' }}>
          An answer could not be generated right now. The articles below may help.
        </p>
      )}

      {state.phase === 'done' && state.response.status !== 'answered' && (
        <p className="text-sm" style={{ color: 'var(--hc-text-secondary)' }}>
          Our documentation does not confidently answer this — the articles below
          are the closest matches.
        </p>
      )}

      {state.phase === 'done' && state.response.status === 'answered' && (
        <>
          <p
            className="whitespace-pre-wrap text-sm leading-relaxed"
            style={{ color: 'var(--hc-text-primary)' }}
          >
            {state.response.answer}
          </p>

          {state.response.citations.length > 0 && (
            <div className="mt-3 flex flex-wrap gap-2">
              {state.response.citations.map((citation) => (
                <DocsLink
                  key={citation.document_id}
                  to={buildCanonicalArticlePath(
                    multilingualEnabled,
                    locale,
                    citation.slug,
                    citation.public_id,
                  )}
                  onClick={onCitationClick}
                  className="inline-flex items-center gap-1 rounded-full border px-2.5 py-1 text-xs transition-colors hover:opacity-80"
                  style={{ borderColor: 'var(--hc-border)', color: 'var(--hc-text-secondary)' }}
                >
                  {citation.title}
                  <ArrowRight className="h-3 w-3" />
                </DocsLink>
              ))}
            </div>
          )}

          <div
            className="mt-3 flex items-center gap-2 text-xs"
            style={{ color: 'var(--hc-text-secondary)' }}
          >
            <span>Was this helpful?</span>
            <button
              type="button"
              aria-label="Answer was helpful"
              disabled={vote !== null}
              onClick={() => onFeedback(true)}
              className="rounded p-1 transition-opacity hover:opacity-70 disabled:opacity-40"
              style={vote === 'up' ? { color: 'var(--hc-accent, currentColor)' } : undefined}
            >
              <ThumbsUp className="h-3.5 w-3.5" />
            </button>
            <button
              type="button"
              aria-label="Answer was not helpful"
              disabled={vote !== null}
              onClick={() => onFeedback(false)}
              className="rounded p-1 transition-opacity hover:opacity-70 disabled:opacity-40"
              style={vote === 'down' ? { color: 'var(--hc-accent, currentColor)' } : undefined}
            >
              <ThumbsDown className="h-3.5 w-3.5" />
            </button>
            {vote && <span>Thanks for the feedback.</span>}
          </div>
        </>
      )}
    </section>
  )
}

interface AIAnswerCardProps {
  locale: string
  query: string
  space?: string
}

/**
 * "Get an AI answer" card on the full search page: an explicit ask button
 * that expands into the shared answer panel.
 */
export function AIAnswerCard({ locale, query, space }: AIAnswerCardProps) {
  const ai = useAIAnswer(locale, space)
  const askedQueryRef = useRef('')

  // A new query resets the card back to the ask button.
  useEffect(() => {
    if (askedQueryRef.current && askedQueryRef.current !== query) {
      askedQueryRef.current = ''
      ai.reset()
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [query])

  if (!ai.enabled) return null
  if (!query || query.trim().length < AI_ANSWER_MIN_QUERY_CHARS) return null

  if (ai.state.phase === 'idle') {
    return (
      <button
        type="button"
        onClick={() => {
          askedQueryRef.current = query
          void ai.ask(query)
        }}
        className="mb-6 flex w-full items-center gap-2 rounded-lg border px-4 py-3 text-left text-sm transition-colors hover:opacity-90"
        style={{
          borderColor: 'var(--hc-border)',
          background: 'var(--hc-surface, transparent)',
          color: 'var(--hc-text-primary)',
        }}
      >
        <Sparkles className="h-4 w-4 shrink-0" style={{ color: 'var(--hc-accent, currentColor)' }} />
        <span>Get an AI answer for &ldquo;{query}&rdquo;</span>
        <ArrowRight className="ml-auto h-4 w-4 shrink-0 opacity-50" />
      </button>
    )
  }

  return (
    <AIAnswerPanel
      locale={locale}
      state={ai.state}
      vote={ai.vote}
      onFeedback={ai.sendFeedback}
      className="mb-6"
    />
  )
}
