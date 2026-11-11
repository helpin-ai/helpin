import { isMacPlatform } from '@/lib/keyboardShortcuts'

/** Inbox keyboard shortcuts, dispatched to the components that own each action. */
export type SupportShortcutAction =
  | 'next'
  | 'previous'
  | 'resolve'
  | 'snooze'
  | 'priority'
  | 'assign'
  | 'tags'
  | 'reply'
  | 'note'
  | 'cheatsheet'

export const SUPPORT_SHORTCUT_EVENT = 'support:shortcut'

export interface SupportShortcutDetail {
  action: SupportShortcutAction
  conversationId: string | null
  /** Set by the component that carried the action out. */
  handled: boolean
}

/**
 * A single key only moves or opens something, so a stray key while typing
 * changes nothing. Resolve acts at once, so it also needs the Command key
 * (Ctrl outside Apple devices).
 */
export const SUPPORT_SHORTCUTS: { key: string; withCommand?: boolean; action: SupportShortcutAction; label: string }[] = [
  { key: 'j', action: 'next', label: 'Next conversation' },
  { key: 'k', action: 'previous', label: 'Previous conversation' },
  { key: 'r', action: 'reply', label: 'Reply' },
  { key: 'n', action: 'note', label: 'Write an internal note' },
  { key: 'e', withCommand: true, action: 'resolve', label: 'Resolve' },
  { key: 'z', action: 'snooze', label: 'Snooze' },
  { key: 'a', action: 'assign', label: 'Assign' },
  { key: 'p', action: 'priority', label: 'Set priority' },
  { key: 't', action: 'tags', label: 'Add or remove tags' },
  { key: '?', action: 'cheatsheet', label: 'Show keyboard shortcuts' },
]

const ACTIONS_BY_KEY = new Map(SUPPORT_SHORTCUTS.filter((shortcut) => !shortcut.withCommand).map((shortcut) => [shortcut.key, shortcut.action]))
const COMMAND_ACTIONS_BY_KEY = new Map(SUPPORT_SHORTCUTS.filter((shortcut) => shortcut.withCommand).map((shortcut) => [shortcut.key, shortcut.action]))

/** The keys of a shortcut as this computer names them: "⌘E" on Apple devices, "Ctrl+E" elsewhere. */
export function supportShortcutLabel(action: SupportShortcutAction, isMac = isMacPlatform()): string {
  const shortcut = SUPPORT_SHORTCUTS.find((item) => item.action === action)
  if (!shortcut) return ''
  const key = shortcut.key.toUpperCase()
  if (!shortcut.withCommand) return key
  return isMac ? `⌘${key}` : `Ctrl+${key}`
}

/** Whether focus is somewhere keys type text or drive a widget. */
export function isShortcutBlockedTarget(target: EventTarget | null): boolean {
  if (!(target instanceof Element)) return false
  return !!target.closest(
    'input, textarea, select, [contenteditable=""], [contenteditable="true"], [role="dialog"], [role="alertdialog"], [role="menu"], [role="listbox"], [role="combobox"], [data-slot="popover-content"]',
  )
}

/** The shortcut a key press triggers, or null when it should type or do nothing. */
export function supportShortcutForEvent(
  event: Pick<KeyboardEvent, 'key' | 'metaKey' | 'ctrlKey' | 'altKey' | 'shiftKey' | 'target' | 'defaultPrevented' | 'isComposing' | 'repeat'>,
  isMac = isMacPlatform(),
): SupportShortcutAction | null {
  if (event.defaultPrevented || event.isComposing || event.altKey) return null
  // Ctrl on Apple devices and the Windows key elsewhere belong to the system.
  if (isMac ? event.ctrlKey : event.metaKey) return null
  if (isShortcutBlockedTarget(event.target)) return null
  const withCommand = isMac ? event.metaKey : event.ctrlKey
  // Shifted letters are left alone; "?" is Shift+/ on most layouts.
  const action = withCommand
    ? (event.shiftKey ? undefined : COMMAND_ACTIONS_BY_KEY.get(event.key.toLowerCase()))
    : ACTIONS_BY_KEY.get(event.key)
  if (!action) return null
  // Holding a key repeats list movement only.
  if (event.repeat && action !== 'next' && action !== 'previous') return null
  return action
}

/** Sends a shortcut to the inbox and reports whether anything handled it. */
export function dispatchSupportShortcut(action: SupportShortcutAction, conversationId: string | null): boolean {
  const detail: SupportShortcutDetail = { action, conversationId, handled: false }
  window.dispatchEvent(new CustomEvent<SupportShortcutDetail>(SUPPORT_SHORTCUT_EVENT, { detail }))
  return detail.handled
}

/** Subscribes to shortcuts; returns the cleanup. */
export function onSupportShortcut(handler: (detail: SupportShortcutDetail) => void): () => void {
  const listener = (event: Event) => handler((event as CustomEvent<SupportShortcutDetail>).detail)
  window.addEventListener(SUPPORT_SHORTCUT_EVENT, listener)
  return () => window.removeEventListener(SUPPORT_SHORTCUT_EVENT, listener)
}

/**
 * The neighbour of the current conversation in list order. When the current
 * conversation just left the list (resolved or snoozed), its last position
 * decides where to continue.
 */
export function adjacentConversationId(ids: string[], currentId: string | null, direction: 'next' | 'previous', lastIndex?: number): string | null {
  if (ids.length === 0) return null
  let index = currentId ? ids.indexOf(currentId) : -1
  if (index === -1 && lastIndex !== undefined) {
    // The items after it moved up one place.
    const nextIndex = direction === 'next' ? lastIndex : lastIndex - 1
    return ids[Math.min(Math.max(nextIndex, 0), ids.length - 1)]
  }
  if (index === -1) return direction === 'next' ? ids[0] : ids[ids.length - 1]
  index = direction === 'next' ? index + 1 : index - 1
  return index >= 0 && index < ids.length ? ids[index] : null
}
