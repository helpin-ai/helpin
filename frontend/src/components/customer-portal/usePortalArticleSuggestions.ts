import { useCallback, useEffect, useRef, useState } from 'react'
import { customerPortalService, type CustomerPortalArticleSuggestion, type CustomerPortalIntake } from '@/lib/services/customerPortalService'

/** Wait for a pause in typing before searching. */
export const PORTAL_SUGGESTION_DEBOUNCE_MS = 500
const MESSAGE_CHARS = 500

/** portalSuggestionText is what a request form searches for: the subject and the start of the message. */
export function portalSuggestionText(subject: string, message: string) {
  return `${subject}\n${message.slice(0, MESSAGE_CHARS)}`.split(/\s+/).filter(Boolean).join(' ')
}

/** portalSuggestionSearchable mirrors the server: at least two words and 12 characters. */
export function portalSuggestionSearchable(text: string) {
  return text.length >= 12 && text.split(' ').length >= 2
}

/** newIntakeId returns a v4 UUID; randomUUID needs a secure context, getRandomValues does not. */
export function newIntakeId() {
  if (typeof crypto.randomUUID === 'function') return crypto.randomUUID()
  const bytes = crypto.getRandomValues(new Uint8Array(16))
  bytes[6] = (bytes[6] & 0x0f) | 0x40
  bytes[8] = (bytes[8] & 0x3f) | 0x80
  const hex = Array.from(bytes, (byte) => byte.toString(16).padStart(2, '0')).join('')
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`
}

/** Events are measurement only; a failure never affects the form. */
function recordEvent(slug: string, event: Parameters<typeof customerPortalService.suggestionEvent>[1]) {
  void customerPortalService.suggestionEvent(slug, event).catch(() => {})
}

/**
 * usePortalArticleSuggestions searches the help center as a customer writes a
 * request. It waits for a pause in typing, cancels stale searches, and keeps
 * the last results on screen while the next search runs. A failed search keeps
 * what was shown: suggestions are a convenience, never a blocker.
 *
 * Each form is one intake: the articles shown and opened are recorded so
 * reports can measure deflection, and `intake` goes with the request so
 * teammates see what the customer already read.
 */
export function usePortalArticleSuggestions(slug: string, subject: string, message: string) {
  const text = portalSuggestionText(subject, message)
  const searchable = portalSuggestionSearchable(text)
  const [intakeId] = useState(newIntakeId)
  const [debounced, setDebounced] = useState(text)
  const [result, setResult] = useState<{ text: string; articles: CustomerPortalArticleSuggestion[] }>({ text: '', articles: [] })
  const seen = useRef({ suggested: new Set<string>(), opened: new Set<string>(), lastShown: '' })
  const completedSearch = useRef<string | null>(null)

  useEffect(() => {
    const timer = window.setTimeout(() => setDebounced(text), PORTAL_SUGGESTION_DEBOUNCE_MS)
    return () => window.clearTimeout(timer)
  }, [text])

  useEffect(() => {
    if (!portalSuggestionSearchable(debounced)) return undefined
    completedSearch.current = null
    const controller = new AbortController()
    let current = true
    customerPortalService
      .suggestions(slug, debounced, controller.signal)
      .then((response) => {
        // A response that lands after newer text was typed is stale.
        if (!current) return
        completedSearch.current = response.search_completed ? debounced : null
        setResult({ text: debounced, articles: response.articles })
        const ids = response.articles.map((article) => article.id)
        ids.forEach((id) => seen.current.suggested.add(id))
        // Record each distinct set of articles once.
        const key = ids.join(',')
        if (ids.length > 0 && key !== seen.current.lastShown) {
          seen.current.lastShown = key
          recordEvent(slug, { intake_id: intakeId, event: 'shown', article_ids: ids })
        }
      })
      .catch(() => {
        // Aborted or failed: keep the current suggestions.
      })
    return () => {
      current = false
      controller.abort()
    }
  }, [slug, debounced, intakeId])

  const recordOpened = useCallback((article: CustomerPortalArticleSuggestion) => {
    seen.current.opened.add(article.id)
    recordEvent(slug, { intake_id: intakeId, event: 'opened', article_ids: [article.id] })
  }, [slug, intakeId])

  /** intake returns what to send with the request. */
  const intake = useCallback((): CustomerPortalIntake => ({
    intake_id: intakeId,
    suggested_article_ids: [...seen.current.suggested].slice(0, 25),
    opened_article_ids: [...seen.current.opened].slice(0, 25),
    search_completed: searchable && completedSearch.current === text,
  }), [intakeId, searchable, text])

  return {
    articles: searchable ? result.articles : [],
    /** The text the shown articles were found for. */
    query: result.text,
    recordOpened,
    intake,
  }
}
