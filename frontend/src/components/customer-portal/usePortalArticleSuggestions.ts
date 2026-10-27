import { useEffect, useState } from 'react'
import { customerPortalService, type CustomerPortalArticleSuggestion } from '@/lib/services/customerPortalService'

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

/**
 * usePortalArticleSuggestions searches the help center as a customer writes a
 * request. It waits for a pause in typing, cancels stale searches, and keeps
 * the last results on screen while the next search runs. A failed search keeps
 * what was shown: suggestions are a convenience, never a blocker.
 */
export function usePortalArticleSuggestions(slug: string, subject: string, message: string) {
  const text = portalSuggestionText(subject, message)
  const searchable = portalSuggestionSearchable(text)
  const [debounced, setDebounced] = useState(text)
  const [result, setResult] = useState<{ text: string; articles: CustomerPortalArticleSuggestion[] }>({ text: '', articles: [] })

  useEffect(() => {
    const timer = window.setTimeout(() => setDebounced(text), PORTAL_SUGGESTION_DEBOUNCE_MS)
    return () => window.clearTimeout(timer)
  }, [text])

  useEffect(() => {
    if (!portalSuggestionSearchable(debounced)) return undefined
    const controller = new AbortController()
    let current = true
    customerPortalService
      .suggestions(slug, debounced, controller.signal)
      .then((response) => {
        // A response that lands after newer text was typed is stale.
        if (current) setResult({ text: debounced, articles: response.articles })
      })
      .catch(() => {
        // Aborted or failed: keep the current suggestions.
      })
    return () => {
      current = false
      controller.abort()
    }
  }, [slug, debounced])

  return {
    articles: searchable ? result.articles : [],
    /** The text the shown articles were found for. */
    query: result.text,
  }
}
