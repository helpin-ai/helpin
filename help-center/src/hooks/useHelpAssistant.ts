import { useCallback, useEffect, useRef, useState } from 'react'
import { useDocsContext } from '@/contexts/DocsContext'
import { assistantVisitorId, helpAssistantService, storeAssistantSession, storedAssistantSession } from '@/lib/helpAssistant'
import { buildCanonicalArticlePath } from '@/lib/locale'
import { helpCenterService } from '@/lib/services'
import type { AssistantCitation, AssistantMessage } from '@/lib/types'

const POLL_MS = 1000
const PENDING_LABEL = 'Searching the help articles…'

export interface HelpAssistantState {
  messages: AssistantMessage[]
  /** Answers that arrived while the panel was open; they are revealed gradually. */
  freshAnswerIds: ReadonlySet<string>
  restoring: boolean
  busy: boolean
  notice: string | null
  turnsLeft: number | null
  send: (text: string, pageArticleId?: string) => void
  retry: () => void
  feedback: (messageId: string, helpful: boolean) => void
  reset: () => void
}

let localId = 0
const tempId = (prefix: string) => `${prefix}-local-${++localId}`

function pendingAnswer(): AssistantMessage {
  return { id: tempId('answer'), role: 'assistant', content: '', status: 'pending', progress_label: PENDING_LABEL, citations: [], created_at: new Date().toISOString() }
}

/**
 * useHelpAssistant runs an Ask AI conversation: it sends questions, polls the
 * session until each answer is written, and keeps the conversation for the
 * browser tab. When the assistant is busy or unavailable, the first question
 * falls back to the help center's one-shot AI answer.
 */
export function useHelpAssistant(enabled: boolean): HelpAssistantState {
  const { subdomain, locale, multilingualEnabled } = useDocsContext()
  const [sessionId, setSessionId] = useState<string | null>(null)
  const [messages, setMessages] = useState<AssistantMessage[]>([])
  const [freshAnswerIds, setFreshAnswerIds] = useState<ReadonlySet<string>>(new Set())
  const [restoring, setRestoring] = useState(false)
  const [notice, setNotice] = useState<string | null>(null)
  const [turnsLeft, setTurnsLeft] = useState<number | null>(null)
  const generation = useRef(0)
  const visitorId = useRef<string>('')
  const lastQuestion = useRef<{ text: string; pageArticleId?: string } | null>(null)

  const visitor = () => (visitorId.current ||= assistantVisitorId())

  // Restore this tab's conversation the first time the panel opens.
  const restored = useRef(false)
  useEffect(() => {
    if (!enabled || restored.current) return
    restored.current = true
    const stored = storedAssistantSession(subdomain)
    if (!stored) return
    const current = generation.current
    setRestoring(true)
    void helpAssistantService.session(subdomain, stored, visitor()).then((result) => {
      if (current !== generation.current) return
      setRestoring(false)
      if (result.data) {
        setSessionId(result.data.id)
        setMessages(result.data.messages)
        setTurnsLeft(result.data.turns_left)
      } else {
        storeAssistantSession(subdomain, null)
      }
    })
  }, [enabled, subdomain])

  const busy = messages.some((message) => message.status === 'pending')

  // Poll while an answer is being written.
  useEffect(() => {
    if (!sessionId || !busy) return
    const current = generation.current
    let cancelled = false
    let timer: number | undefined
    const tick = async () => {
      const result = await helpAssistantService.session(subdomain, sessionId, visitor())
      if (cancelled || current !== generation.current) return
      if (result.data) {
        const next = result.data.messages
        setMessages((previous) => {
          const wasPending = new Set(previous.filter((m) => m.status === 'pending').map((m) => m.id))
          const arrived = next.filter((m) => m.role === 'assistant' && m.status !== 'pending' && (wasPending.has(m.id) || !previous.some((p) => p.id === m.id)))
          if (arrived.length) setFreshAnswerIds((ids) => new Set([...ids, ...arrived.map((m) => m.id)]))
          return next
        })
        setTurnsLeft(result.data.turns_left)
        if (!result.data.messages.some((m) => m.status === 'pending')) return
      }
      timer = window.setTimeout(() => void tick(), POLL_MS)
    }
    timer = window.setTimeout(() => void tick(), POLL_MS)
    return () => {
      cancelled = true
      window.clearTimeout(timer)
    }
  }, [busy, sessionId, subdomain])

  // fallbackAnswer answers one question with the help center's one-shot AI
  // answer when the assistant cannot take it.
  const fallbackAnswer = useCallback(async (text: string, answerId: string, current: number) => {
    const result = await helpCenterService.askAnswer(subdomain, locale, multilingualEnabled, { query: text })
    if (current !== generation.current) return
    const response = result.data
    const answered = response?.status === 'answered' && response.answer
    const citations: AssistantCitation[] = answered
      ? response.citations.map((citation) => ({
          article_id: citation.document_id,
          title: citation.title,
          url: buildCanonicalArticlePath(multilingualEnabled, locale, citation.slug, citation.public_id),
        }))
      : []
    setMessages((previous) => previous.map((message) => message.id !== answerId ? message : {
      ...message,
      status: answered ? 'answered' : 'handed_off',
      progress_label: undefined,
      content: answered ? response.answer ?? '' : 'I could not find this in the help articles. Contact support and the team will help you.',
      citations,
    }))
    if (answered) setFreshAnswerIds((ids) => new Set([...ids, answerId]))
  }, [locale, multilingualEnabled, subdomain])

  const send = useCallback((raw: string, pageArticleId?: string) => {
    const text = raw.trim()
    if (!text || busy) return
    lastQuestion.current = { text, pageArticleId }
    setNotice(null)
    const question: AssistantMessage = { id: tempId('question'), role: 'user', content: text, status: 'answered', citations: [], created_at: new Date().toISOString() }
    const answer = pendingAnswer()
    setMessages((previous) => [...previous, question, answer])
    const current = generation.current
    const ask = async (session: string | null): Promise<void> => {
      const result = await helpAssistantService.ask(subdomain, { visitor_id: visitor(), session_id: session ?? undefined, message: text, page_article_id: pageArticleId })
      if (current !== generation.current) return
      if (result.data) {
        const turn = result.data
        setSessionId(turn.session_id)
        storeAssistantSession(subdomain, turn.session_id)
        setTurnsLeft(turn.turns_left)
        setMessages((previous) => previous.map((message) =>
          message.id === question.id ? { ...message, id: turn.question_id } : message.id === answer.id ? { ...message, id: turn.answer_id } : message))
        return
      }
      if (result.status === 404 && session) {
        // The stored conversation is gone (for example after signing in); start over.
        setSessionId(null)
        storeAssistantSession(subdomain, null)
        return ask(null)
      }
      if (result.status === 409) {
        setMessages((previous) => previous.filter((message) => message.id !== question.id && message.id !== answer.id))
        setNotice(result.error?.includes('full') ? 'This conversation is full. Start a new chat to keep asking.' : 'Wait for the current answer to finish.')
        return
      }
      if (result.status === 429 || result.status === 503) {
        setNotice(result.status === 429 ? 'The assistant has reached its limit for now, so this is a quick answer.' : 'The assistant is busy, so this is a quick answer.')
        return fallbackAnswer(text, answer.id, current)
      }
      setMessages((previous) => previous.map((message) => message.id === answer.id ? { ...message, status: 'failed', progress_label: undefined } : message))
    }
    void ask(sessionId)
  }, [busy, fallbackAnswer, sessionId, subdomain])

  const retry = useCallback(() => {
    const last = lastQuestion.current
    if (!last || busy) return
    // Drop the failed exchange; the question is asked again.
    setMessages((previous) => {
      const index = previous.length - 2
      return index >= 0 && previous[index + 1]?.status === 'failed' ? previous.slice(0, index) : previous
    })
    send(last.text, last.pageArticleId)
  }, [busy, send])

  const feedback = useCallback((messageId: string, helpful: boolean) => {
    setMessages((previous) => previous.map((message) => message.id === messageId ? { ...message, helpful } : message))
    if (!sessionId || messageId.includes('-local-')) return
    void helpAssistantService.feedback(subdomain, messageId, { visitor_id: visitor(), session_id: sessionId, helpful })
  }, [sessionId, subdomain])

  const reset = useCallback(() => {
    generation.current++
    setSessionId(null)
    setMessages([])
    setFreshAnswerIds(new Set())
    setNotice(null)
    setTurnsLeft(null)
    setRestoring(false)
    lastQuestion.current = null
    storeAssistantSession(subdomain, null)
  }, [subdomain])

  return { messages, freshAnswerIds, restoring, busy, notice, turnsLeft, send, retry, feedback, reset }
}
