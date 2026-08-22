// Module-level arbitration between the inbox's two pointer-gesture
// recognizers (row swipes and pull-to-refresh), which both observe the same
// bubbling pointer stream. Whichever recognizer crosses its own intent
// threshold first claims the gesture; the other backs off until the pointer
// is released. A claim is a simple mutual-exclusion token, not a queue —
// there is only ever one active pointer gesture on a phone screen.

export type GestureKind = 'row-swipe' | 'pull'

let active: GestureKind | null = null

/**
 * Try to claim the current pointer gesture for `kind`. Returns false when a
 * different kind already owns it (the caller must abandon its gesture).
 * Re-claiming the same kind while it is active succeeds (idempotent).
 */
export function claimGesture(kind: GestureKind): boolean {
  if (active !== null && active !== kind) return false
  active = kind
  return true
}

/** Release the claim. Only the owning kind can release; others are a no-op. */
export function releaseGesture(kind: GestureKind): void {
  if (active === kind) active = null
}

/** The kind currently owning the pointer gesture, or null when idle. */
export function activeGesture(): GestureKind | null {
  return active
}
