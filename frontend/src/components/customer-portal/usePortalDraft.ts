import { useCallback, useState } from 'react'

type PortalDraft = {
  subject: string
  message: string
  /** The chosen request type and its field values, as typed. */
  requestTypeId?: string
  fields?: Record<string, string | string[]>
  /** People to copy on the request. */
  cc?: string[]
}

function readFields(value: unknown): Record<string, string | string[]> {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return {}
  return Object.fromEntries(Object.entries(value).filter((entry): entry is [string, string | string[]] =>
    typeof entry[1] === 'string' || (Array.isArray(entry[1]) && entry[1].every((item) => typeof item === 'string'))))
}

function readDraft(storageKey: string, initial?: PortalDraft): PortalDraft {
  try {
    const stored = JSON.parse(window.sessionStorage.getItem(storageKey) ?? '{}') as { subject?: unknown; message?: unknown; requestTypeId?: unknown; fields?: unknown; cc?: unknown }
    const draft: PortalDraft = {
      subject: typeof stored.subject === 'string' ? stored.subject : '',
      message: typeof stored.message === 'string' ? stored.message : '',
    }
    if (typeof stored.requestTypeId === 'string') draft.requestTypeId = stored.requestTypeId
    if (stored.fields !== undefined) draft.fields = readFields(stored.fields)
    if (Array.isArray(stored.cc)) draft.cc = stored.cc.filter((item): item is string => typeof item === 'string')
    // A prefill replaces an empty draft but keeps who it copies.
    return !draft.subject && !draft.message && !draft.requestTypeId && initial ? { ...initial, ...(draft.cc ? { cc: draft.cc } : {}) } : draft
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
