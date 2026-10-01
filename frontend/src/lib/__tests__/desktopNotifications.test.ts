// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { desktopCategoryEnabled, desktopEventAllowed, deliverDesktopNotification, getDesktopPermission, setDesktopEnabled, trackDesktopFocus } from '../desktopNotifications'
import type { NotificationPreferences } from '../notificationTypes'
import type { WSEvent } from '@/hooks/useWebSocket'

const preferences = { mute_workspace: false, do_not_disturb: false, channel_preferences: {} } as NotificationPreferences
const event: WSEvent = { event_id: 'e1', sent_at: new Date().toISOString(), entity: 'notification', action: 'created', entity_id: 'n1', workspace_id: 'w1', actor_id: 'other', data: { recipient_id: 'u1', category: 'mentions', title: 'Sam mentioned you', entity_type: 'task', entity_id: 't1', status: 'unread' } }
const show = vi.fn()
beforeEach(() => {
  localStorage.clear()
  show.mockClear()
  vi.stubGlobal('Notification', class { static permission = 'granted'; onclick = null; close = vi.fn(); constructor(title: string, options: unknown) { show(title, options) } })
  vi.stubGlobal('isSecureContext', true)
  vi.spyOn(document, 'hasFocus').mockReturnValue(false)
  setDesktopEnabled('u1', true)
})

afterEach(() => { vi.restoreAllMocks(); vi.unstubAllGlobals() })

describe('desktop notification policy', () => {
  it('defaults only direct attention categories on', () => {
    for (const category of ['assignments', 'mentions', 'agent_attention', 'support_replies', 'support_mentions']) expect(desktopCategoryEnabled(preferences, category)).toBe(true)
    for (const category of ['comments', 'status_changes', 'subscriptions', 'sprints', 'crm_signals', 'integrations', 'unknown']) expect(desktopCategoryEnabled(preferences, category)).toBe(false)
  })
  it('respects explicit category choices and disabled in-app delivery', () => {
    expect(desktopCategoryEnabled({ ...preferences, channel_preferences: { comments: { desktop: true } } }, 'comments')).toBe(true)
    expect(desktopCategoryEnabled({ ...preferences, channel_preferences: { mentions: { desktop: false } } }, 'mentions')).toBe(false)
    expect(desktopCategoryEnabled({ ...preferences, channel_preferences: { mentions: { in_app: false, desktop: true } } }, 'mentions')).toBe(false)
  })
  it('rejects other recipients, actors, workspaces, state changes, and snoozed alerts', () => {
    expect(desktopEventAllowed(event, 'u1', 'w1', preferences)).toBe(true)
    expect(desktopEventAllowed(event, 'u2', 'w1', preferences)).toBe(false)
    expect(desktopEventAllowed(event, 'u1', 'w2', preferences)).toBe(false)
    expect(desktopEventAllowed({ ...event, actor_id: 'u1' }, 'u1', 'w1', preferences)).toBe(false)
    expect(desktopEventAllowed({ ...event, data: {} }, 'u1', 'w1', preferences)).toBe(false)
    expect(desktopEventAllowed({ ...event, data: { ...event.data, status: 'snoozed' } }, 'u1', 'w1', preferences)).toBe(false)
  })
  it('respects mute and DND including expiration', () => {
    expect(desktopEventAllowed(event, 'u1', 'w1', { ...preferences, mute_workspace: true })).toBe(false)
    expect(desktopEventAllowed(event, 'u1', 'w1', { ...preferences, do_not_disturb: true })).toBe(false)
    expect(desktopEventAllowed(event, 'u1', 'w1', { ...preferences, do_not_disturb: true, dnd_until: '2000-01-01T00:00:00Z' })).toBe(true)
  })
})

describe('browser delivery', () => {
  it('deduplicates local-hub events without suppressing later updates to the same notification', async () => {
    const options = { userId: 'u1', workspaceId: 'w1', preferences, isCurrent: () => true, onClick: vi.fn() }
    const localEvent = { ...event, event_id: undefined, sent_at: undefined, data: { ...event.data, delivery_id: 'delivery-1' } }
    await deliverDesktopNotification(localEvent, options)
    await deliverDesktopNotification(localEvent, options)
    await deliverDesktopNotification({ ...localEvent, data: { ...localEvent.data, delivery_id: 'delivery-2' } }, options)
    expect(show).toHaveBeenCalledTimes(2)
  })
  it('shows while unfocused, deduplicates events, and groups by notification', async () => {
    const options = { userId: 'u1', workspaceId: 'w1', preferences, isCurrent: () => true, onClick: vi.fn() }
    await deliverDesktopNotification(event, options)
    await deliverDesktopNotification(event, options)
    expect(show).toHaveBeenCalledTimes(1)
    expect(show.mock.calls[0][1].tag).toContain('n1')
  })
  it('suppresses focused-app delivery and stops when disabled or signed out', async () => {
    const options = { userId: 'u1', workspaceId: 'w1', preferences, isCurrent: () => true, onClick: vi.fn() }
    vi.spyOn(document, 'hasFocus').mockReturnValue(true)
    await deliverDesktopNotification(event, options)
    expect(show).not.toHaveBeenCalled()
    vi.spyOn(document, 'hasFocus').mockReturnValue(false)
    setDesktopEnabled('u1', false)
    await deliverDesktopNotification(event, options)
    setDesktopEnabled('u1', true)
    await deliverDesktopNotification(event, { ...options, isCurrent: () => false })
    expect(show).not.toHaveBeenCalled()
  })
  it('fails safely when permission is denied or the browser is unsupported', async () => {
    vi.stubGlobal('Notification', undefined)
    expect(getDesktopPermission()).toBe('unsupported')
    await deliverDesktopNotification(event, { userId: 'u1', workspaceId: 'w1', preferences, isCurrent: () => true, onClick: vi.fn() })
    expect(show).not.toHaveBeenCalled()
  })
})


describe('multiple Helpin tabs', () => {
  it('suppresses background delivery when another Helpin tab is focused', async () => {
    vi.spyOn(document, 'visibilityState', 'get').mockReturnValue('visible')
    vi.spyOn(document, 'hasFocus').mockReturnValue(true)
    const stop = trackDesktopFocus('u1')
    vi.spyOn(document, 'hasFocus').mockReturnValue(false)
    await deliverDesktopNotification(event, { userId: 'u1', workspaceId: 'w1', preferences, isCurrent: () => true, onClick: vi.fn() })
    expect(show).not.toHaveBeenCalled()
    stop()
  })
  it('releases focus immediately when a focused page closes', async () => {
    vi.spyOn(document, 'visibilityState', 'get').mockReturnValue('visible')
    vi.spyOn(document, 'hasFocus').mockReturnValue(true)
    const stop = trackDesktopFocus('u1')
    window.dispatchEvent(new Event('pagehide'))
    vi.spyOn(document, 'hasFocus').mockReturnValue(false)
    await deliverDesktopNotification(event, { userId: 'u1', workspaceId: 'w1', preferences, isCurrent: () => true, onClick: vi.fn() })
    stop()
    expect(show).toHaveBeenCalledOnce()
  })
  it('serializes simultaneous event delivery using browser locks', async () => {
    let queue = Promise.resolve()
    const request = vi.fn((_key, callback) => { queue = queue.then(callback); return queue })
    vi.stubGlobal('navigator', { locks: { request } })
    const options = { userId: 'u1', workspaceId: 'w1', preferences, isCurrent: () => true, onClick: vi.fn() }
    await Promise.all([deliverDesktopNotification(event, options), deliverDesktopNotification(event, options)])
    expect(request).toHaveBeenCalledTimes(2)
    expect(show).toHaveBeenCalledOnce()
  })
})
