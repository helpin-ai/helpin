import { useCallback, useState } from 'react'

type PortalDraft = { subject: string; message: string }

function readDraft(storageKey: string, initial?: PortalDraft): PortalDraft {
  try {
    const stored = JSON.parse(window.sessionStorage.getItem(storageKey) ?? '{}') as { subject?: unknown; message?: unknown }
    const draft = {
      subject: typeof stored.subject === 'string' ? stored.subject : '',
      message: typeof stored.message === 'string' ? stored.message : '',
    }
    return !draft.subject && !draft.message && initial ? initial : draft
  } catch {
    return initial ?? { subject: '', message: '' }
  }
}

/** Clear this portal's drafts, including drafts saved before identity scoping. */
export function clearPortalDrafts(slug: string) {
  try {
    const prefix = `helpin.portal.${slug}.`
    for (let i = window.sessionStorage.length - 1; i >= 0; i--) {
      const key = window.sessionStorage.key(i)
      if (key?.startsWith(prefix) && key.endsWith('.draft')) window.sessionStorage.removeItem(key)
    }
  } catch {
    // Storage may be unavailable.
  }
}

/**
 * usePortalDraft keeps a request draft for this browser tab, so leaving the
 * form (for example to read a suggested article) does not lose it. Storage can
 * be unavailable; the draft then lives only in the form. An initial draft,
 * such as a help assistant conversation, applies only when none is saved.
 */
export function usePortalDraft(slug: string, customerID: string, initial?: PortalDraft) {
  const storageKey = `helpin.portal.${slug}.${customerID}.new-request.draft`
  const [stored, setStored] = useState(() => ({ storageKey, draft: readDraft(storageKey, initial) }))
  let draft = stored.draft
  if (stored.storageKey !== storageKey) {
    draft = readDraft(storageKey)
    setStored({ storageKey, draft })
  }
  const setDraft = useCallback((next: PortalDraft) => {
    setStored({ storageKey, draft: next })
    try {
      window.sessionStorage.setItem(storageKey, JSON.stringify(next))
    } catch {
      // The draft still lives in the form.
    }
  }, [storageKey])
  const clearDraft = useCallback(() => {
    setStored({ storageKey, draft: { subject: '', message: '' } })
    try {
      window.sessionStorage.removeItem(storageKey)
    } catch {
      // Nothing stored.
    }
  }, [storageKey])
  return { draft, setDraft, clearDraft }
}
