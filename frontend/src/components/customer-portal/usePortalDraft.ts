import { useCallback, useState } from 'react'

/**
 * usePortalDraft keeps a request draft for this browser tab, so leaving the
 * form (for example to read a suggested article) does not lose it. Storage can
 * be unavailable; the draft then lives only in the form.
 */
export function usePortalDraft(key: string) {
  const storageKey = `helpin.portal.${key}.draft`
  const [draft, setDraftState] = useState<{ subject: string; message: string }>(() => {
    try {
      const stored = JSON.parse(window.sessionStorage.getItem(storageKey) ?? '{}') as { subject?: unknown; message?: unknown }
      return {
        subject: typeof stored.subject === 'string' ? stored.subject : '',
        message: typeof stored.message === 'string' ? stored.message : '',
      }
    } catch {
      return { subject: '', message: '' }
    }
  })
  const setDraft = useCallback((next: { subject: string; message: string }) => {
    setDraftState(next)
    try {
      window.sessionStorage.setItem(storageKey, JSON.stringify(next))
    } catch {
      // The draft still lives in the form.
    }
  }, [storageKey])
  const clearDraft = useCallback(() => {
    try {
      window.sessionStorage.removeItem(storageKey)
    } catch {
      // Nothing stored.
    }
  }, [storageKey])
  return { draft, setDraft, clearDraft }
}
