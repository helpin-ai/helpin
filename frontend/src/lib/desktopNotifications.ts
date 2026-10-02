import type { WSEvent } from '@/hooks/useWebSocket'
import type { NotificationPreferences } from './notificationTypes'

export const DESKTOP_DEFAULT_CATEGORIES = new Set(['assignments', 'mentions', 'agent_attention', 'support_replies', 'support_mentions'])
export type DesktopPermission = NotificationPermission | 'unsupported'
const PREFIX = 'helpin:desktop:'
export const DESKTOP_SETTINGS_EVENT = 'helpin:desktop-settings'
const settingKey = (userId: string) => `${PREFIX}${userId}:enabled`

export function isDesktopPreferenceStorageKey(key: string): boolean {
  return /^helpin:desktop:[^:]+:enabled$/.test(key)
}

export function getDesktopPermission(): DesktopPermission {
  return typeof Notification === 'undefined' || !window.isSecureContext ? 'unsupported' : Notification.permission
}

export function getDesktopChoice(userId: string): boolean | null {
  if (!userId) return false
  try {
    const value = localStorage.getItem(settingKey(userId))
    return value === null ? null : value === 'true'
  } catch { return false }
}

export function isDesktopEnabled(userId: string): boolean {
  return getDesktopChoice(userId) === true
}

export function setDesktopEnabled(userId: string, enabled: boolean) {
  if (!userId) return
  localStorage.setItem(settingKey(userId), String(enabled))
  window.dispatchEvent(new Event(DESKTOP_SETTINGS_EVENT))
}

export function desktopCategoryEnabled(preferences: NotificationPreferences, category: string): boolean {
  const choice = preferences.channel_preferences?.[category]
  return choice?.in_app !== false && (choice?.desktop ?? DESKTOP_DEFAULT_CATEGORIES.has(category))
}

export function desktopEventAllowed(event: WSEvent, userId: string, workspaceId: string, preferences: NotificationPreferences): boolean {
  const data = event.data
  if (event.entity !== 'notification' || !['created', 'updated'].includes(event.action)
    || event.workspace_id !== workspaceId || data?.recipient_id !== userId || event.actor_id === userId
    || data?.status !== 'unread' || typeof data.title !== 'string' || !data.title.trim()) return false
  const paused = preferences.dnd_until ? Date.parse(preferences.dnd_until) > Date.now() : preferences.do_not_disturb
  return !paused && !preferences.mute_workspace && typeof data.category === 'string' && desktopCategoryEnabled(preferences, data.category)
}

// Each focused tab publishes a short lease. A closed/crashed tab cannot suppress alerts indefinitely.
export function trackDesktopFocus(userId: string): () => void {
  const key = `${PREFIX}${userId}:focus`
  const tabId = crypto.randomUUID()
  const release = () => {
    try {
      if (JSON.parse(localStorage.getItem(key) || 'null')?.tabId === tabId) localStorage.removeItem(key)
    } catch { /* Storage is optional. */ }
  }
  const update = () => {
    try {
      if (document.visibilityState === 'visible' && document.hasFocus()) {
        localStorage.setItem(key, JSON.stringify({ tabId, at: Date.now() }))
      } else if (JSON.parse(localStorage.getItem(key) || 'null')?.tabId === tabId) {
        localStorage.removeItem(key)
      }
    } catch { /* Delivery remains conservative if storage is unavailable. */ }
  }
  update()
  const timer = setInterval(update, 15_000)
  window.addEventListener('focus', update)
  window.addEventListener('blur', update)
  document.addEventListener('visibilitychange', update)
  window.addEventListener('pagehide', release)
  window.addEventListener('pageshow', update)
  return () => {
    clearInterval(timer)
    window.removeEventListener('focus', update)
    window.removeEventListener('blur', update)
    document.removeEventListener('visibilitychange', update)
    window.removeEventListener('pagehide', release)
    window.removeEventListener('pageshow', update)
    release()
  }
}

export function anyTabFocused(userId: string) {
  if (document.visibilityState === 'visible' && document.hasFocus()) return true
  const lease = JSON.parse(localStorage.getItem(`${PREFIX}${userId}:focus`) || 'null')
  return lease && Date.now() - lease.at < 45_000
}

// Count Unicode code points so truncation never leaves half a surrogate pair.
function desktopPreview(text: string, limit: number): string {
  const characters = Array.from(text.replace(/\s+/g, ' ').trim())
  return characters.length > limit ? characters.slice(0, limit - 1).join('').trimEnd() + '…' : characters.join('')
}

interface DeliveryOptions {
  userId: string
  workspaceId: string
  preferences: NotificationPreferences
  isCurrent: () => boolean
  onClick: () => void
}

export async function deliverDesktopNotification(event: WSEvent, options: DeliveryOptions): Promise<void> {
  const { userId, workspaceId, preferences } = options
  if (!desktopEventAllowed(event, userId, workspaceId, preferences)) return
  // Stable event identity prevents duplicate delivery from multiple tabs or a replayed frame.
  const eventKey = typeof event.data?.delivery_id === 'string' ? event.data.delivery_id : event.event_id
    || (event.sent_at ? `${event.entity_id}:${event.sent_at}` : '')
  if (!eventKey) return
  const deliver = () => {
    if (!options.isCurrent() || !isDesktopEnabled(userId) || getDesktopPermission() !== 'granted') return
    const key = `${PREFIX}${userId}:delivered`
    const now = Date.now()
    const entries: [string, number][] = JSON.parse(localStorage.getItem(key) || '[]')
    const recent = entries.filter(([, at]) => now - at < 300_000).slice(-199)
    if (recent.some(([id]) => id === eventKey)) return
    // Consume foreground events too, so a second tab cannot show them later.
    localStorage.setItem(key, JSON.stringify([...recent, [eventKey, now]]))
    if (anyTabFocused(userId)) return
    const notification = new Notification(desktopPreview(event.data!.title as string, 120), {
      body: typeof event.data?.body === 'string' ? desktopPreview(event.data.body, 240) : undefined,
      tag: `${PREFIX}${userId}:${workspaceId}:${event.entity_id}`,
    })
    notification.onclick = () => {
      notification.close()
      if (!options.isCurrent()) return
      window.focus()
      options.onClick()
    }
  }
  try {
    // Locks serialize the shared-storage claim across tabs; the OS tag also collapses updates.
    if (navigator.locks) await navigator.locks.request(`${PREFIX}${userId}:delivery`, deliver)
    else deliver()
  } catch (error) {
    console.warn('Desktop notification could not be displayed', error)
  }
}
