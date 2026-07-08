// Stub for Task 4 (UI primitives) so Pressable compiles and can call `haptic()`.
// Task 5 replaces this with real Tauri haptic-feedback plugin calls.
export type HapticKind =
  | 'selection'
  | 'impactLight'
  | 'impactMedium'
  | 'notificationSuccess'
  | 'notificationError'

export function haptic(_k: HapticKind) {}
