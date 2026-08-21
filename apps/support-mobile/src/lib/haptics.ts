import {
  impactFeedback,
  notificationFeedback,
  selectionFeedback,
} from '@tauri-apps/plugin-haptics'

export type HapticKind =
  | 'selection'
  | 'impactLight'
  | 'impactMedium'
  | 'notificationSuccess'
  | 'notificationError'

const isTauri = () => typeof window !== 'undefined' && '__TAURI_INTERNALS__' in window

// Fire-and-forget: callers never await this, and it must never throw. The
// plugin's commands resolve to a `{ status: 'ok' | 'error' }` result object
// for expected failures (e.g. no haptics hardware) and only reject when the
// underlying IPC call itself fails (e.g. plugin not registered) — either way
// we don't want a tap gesture to break because a vibration motor is missing,
// so both cases are swallowed here.
export function haptic(kind: HapticKind): void {
  if (!isTauri()) return
  const fire = async () => {
    switch (kind) {
      case 'selection':
        return selectionFeedback()
      case 'impactLight':
        return impactFeedback('light')
      case 'impactMedium':
        return impactFeedback('medium')
      case 'notificationSuccess':
        return notificationFeedback('success')
      case 'notificationError':
        return notificationFeedback('error')
    }
  }
  void fire().catch(() => {})
}
