import type { SupportRealtimeEvent } from '@helpin-ai/support-core'
import { isTauriDesktop } from './desktopHost'
import { revealDesktopWindow } from './desktopWindow'

/** Cooldown per conversation to avoid notification storms (ms). */
const COOLDOWN_MS = 5_000
/** Max tracked event IDs for dedupe (ring buffer). */
const MAX_SEEN = 200

const recentlySeen = new Set<string>()
const seenOrder: string[] = []
const lastNotifyAt = new Map<string, number>()

let permissionGranted = false
let nextNotificationId = 1

// ── Permission ──────────────────────────────────────────────

export async function ensureNotificationPermission(): Promise<boolean> {
  if (!isTauriDesktop()) return false

  try {
    const {
      isPermissionGranted,
      requestPermission,
    } = await import('@tauri-apps/plugin-notification')

    permissionGranted = await isPermissionGranted()
    if (!permissionGranted) {
      const result = await requestPermission()
      permissionGranted = result === 'granted'
    }
    return permissionGranted
  } catch {
    return false
  }
}

// ── Send ────────────────────────────────────────────────────

interface NotifyPayload {
  id?: number
  title: string
  body: string
  /** Passed through to click handler for routing. */
  extra?: {
    workspaceSlug: string
    conversationId: string
  }
}

async function sendNativeNotification(payload: NotifyPayload) {
  if (!isTauriDesktop() || !permissionGranted) return

  try {
    const { sendNotification } = await import('@tauri-apps/plugin-notification')
    sendNotification({
      id: payload.id,
      title: payload.title,
      body: payload.body,
      extra: payload.extra,
      autoCancel: true,
    })
  } catch {
    // Notification send failed — silently ignore.
  }
}

// ── Click handling ──────────────────────────────────────────

type NavigateFn = (to: string) => void
let registeredNavigate: NavigateFn | null = null

export function registerNotificationClickHandler(navigate: NavigateFn) {
  registeredNavigate = navigate
}

async function handleNotificationClick(extra: Record<string, unknown> | undefined) {
  if (!registeredNavigate) return

  const workspaceSlug =
    typeof extra?.workspaceSlug === 'string' ? extra.workspaceSlug : null
  const conversationId =
    typeof extra?.conversationId === 'string' ? extra.conversationId : null

  if (!workspaceSlug || !conversationId) {
    return
  }

  await revealDesktopWindow()
  registeredNavigate(`/w/${workspaceSlug}/support/${conversationId}`)
}

export async function setupNotificationClickListener() {
  if (!isTauriDesktop()) return

  try {
    const { onAction } = await import('@tauri-apps/plugin-notification')
    await onAction((notification) => {
      void handleNotificationClick(notification.extra)
    })
  } catch {
    // Plugin not available — ignore.
  }
}

// ── Dedupe helpers ──────────────────────────────────────────

function dedupeKey(event: SupportRealtimeEvent): string {
  if (event.event_id) return event.event_id
  return `${event.entity}:${event.entity_id}:${event.sent_at ?? ''}`
}

function isDuplicate(event: SupportRealtimeEvent): boolean {
  const key = dedupeKey(event)
  if (recentlySeen.has(key)) return true

  recentlySeen.add(key)
  seenOrder.push(key)
  if (seenOrder.length > MAX_SEEN) {
    const oldest = seenOrder.shift()!
    recentlySeen.delete(oldest)
  }
  return false
}

function isOnCooldown(conversationId: string): boolean {
  const last = lastNotifyAt.get(conversationId)
  if (!last) return false
  return Date.now() - last < COOLDOWN_MS
}

function recordNotify(conversationId: string) {
  lastNotifyAt.set(conversationId, Date.now())
}

// ── Window focus tracking ───────────────────────────────────

let windowFocused = typeof document !== 'undefined' && document.hasFocus()

if (typeof window !== 'undefined') {
  window.addEventListener('focus', () => { windowFocused = true })
  window.addEventListener('blur', () => { windowFocused = false })
}

// ── Main entry point ────────────────────────────────────────

export interface SupportNotificationContext {
  /** Current user's ID — suppress notifications for own actions. */
  currentUserId: string
  /** Currently selected conversation ID — suppress if focused on it. */
  selectedConversationId: string | null
  /** Current workspace slug — for routing in click handler. */
  workspaceSlug: string
}

/**
 * Evaluate a realtime event and show a native notification if appropriate.
 * Called from the desktop's `onEvent` callback.
 */
export function handleSupportRealtimeEvent(
  event: SupportRealtimeEvent,
  ctx: SupportNotificationContext,
) {
  // Only notify for new messages.
  if (event.entity !== 'support_conversation_message' || event.action !== 'created') return

  // Don't notify for own messages.
  if (event.actor_id === ctx.currentUserId) return

  // Dedupe: skip if we've already processed this event.
  if (isDuplicate(event)) return

  const conversationId = event.parent_id ?? event.entity_id
  if (!conversationId) return

  // Don't notify if the conversation is already focused.
  if (windowFocused && ctx.selectedConversationId === conversationId) return

  // Cooldown: avoid notification storms from rapid messages.
  if (isOnCooldown(conversationId)) return
  recordNotify(conversationId)

  // Build notification content.
  const senderName = typeof event.data?.sender_name === 'string'
    ? event.data.sender_name
    : 'Customer'
  const title = event.actor_id?.startsWith('widget:')
    ? senderName
    : 'New support message'
  const notificationId = nextNotificationId++
  const preview = typeof event.data?.content === 'string'
    ? event.data.content
    : 'New message'

  sendNativeNotification({
    id: notificationId,
    title,
    body: preview.length > 120 ? `${preview.slice(0, 117)}...` : preview,
    extra: {
      workspaceSlug: ctx.workspaceSlug,
      conversationId,
    },
  })
}
