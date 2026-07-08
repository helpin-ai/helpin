import type { ComposerMode } from './draft-store'

/** A send attempt that failed after `useSendMessage` rolled back its optimistic bubble — the composer keeps this around as a retry chip since the hook itself discards the failed attempt. */
export interface FailedSend {
  id: string
  content: string
  mode: ComposerMode
}

export type FailedSendsAction =
  | { type: 'add'; failedSend: FailedSend }
  | { type: 'remove'; id: string }

/** Pure reducer so the add/remove semantics are unit-testable without mounting the composer. */
export function failedSendsReducer(state: FailedSend[], action: FailedSendsAction): FailedSend[] {
  switch (action.type) {
    case 'add':
      return [...state, action.failedSend]
    case 'remove':
      return state.filter((failedSend) => failedSend.id !== action.id)
    default:
      return state
  }
}
