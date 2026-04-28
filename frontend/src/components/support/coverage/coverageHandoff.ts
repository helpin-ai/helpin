import type { JSONContent } from '@tiptap/react'

type StorageLike = Pick<Storage, 'getItem' | 'setItem' | 'removeItem'>

export function coverageHandoffKey(gapId: string, suggestionId: string) {
  return `coverage-gap-draft:${gapId}:${suggestionId}`
}

function defaultStorage(): StorageLike | null {
  if (typeof window === 'undefined') return null
  return window.sessionStorage
}

export function storeCoverageHandoffContent(
  gapId: string,
  suggestionId: string,
  content: unknown,
  storage: StorageLike | null = defaultStorage(),
) {
  if (!storage || !content) return
  storage.setItem(coverageHandoffKey(gapId, suggestionId), JSON.stringify(content))
}

export function loadCoverageHandoffContent(
  gapId: string | undefined,
  suggestionId: string | undefined,
  storage: StorageLike | null = defaultStorage(),
): JSONContent | null {
  if (!storage || !gapId || !suggestionId) return null
  const key = coverageHandoffKey(gapId, suggestionId)
  const raw = storage.getItem(key)
  if (!raw) return null
  storage.removeItem(key)
  try {
    return JSON.parse(raw) as JSONContent
  } catch {
    return null
  }
}
