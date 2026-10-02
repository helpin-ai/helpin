// @vitest-environment jsdom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { TooltipProvider } from '@/components/ui/tooltip'
import { NotificationCenter } from '../NotificationCenter'
import { isDesktopEnabled, setDesktopEnabled } from '@/lib/desktopNotifications'

const navigate = vi.hoisted(() => vi.fn())
const session = vi.hoisted(() => ({ user: { id: 'u1' } as { id: string } | null }))
vi.mock('@/stores/authStore', () => ({ useAuthStore: Object.assign((select: (state: unknown) => unknown) => select(session), { getState: () => session }) }))
vi.mock('@/stores/workspaceStore', () => ({ useWorkspaceStore: (select: (state: unknown) => unknown) => select({ currentWorkspace: { id: 'w1', slug: 'acme' } }) }))
vi.mock('@tanstack/react-router', () => ({ useLocation: () => ({}), useNavigate: () => navigate }))
vi.mock('@/hooks/queries', () => ({
  useNotifications: () => ({ data: { pages: [{ data: [] }] } }),
  useUnreadCount: () => ({ data: { count: 0 } }),
  useMarkAsRead: () => ({}), useMarkAsUnread: () => ({}), useArchiveNotification: () => ({}),
  useMarkAllAsRead: () => ({}), useDeleteNotification: () => ({}), useSnoozeNotification: () => ({}),
}))
Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
let root: ReturnType<typeof createRoot>
let container: HTMLDivElement
const requestPermission = vi.fn()
const button = (label: string) => Array.from(document.querySelectorAll('button')).find(b => b.textContent?.trim() === label || b.getAttribute('aria-label') === label)!
const render = () => act(() => root.render(<TooltipProvider><NotificationCenter /></TooltipProvider>))
const click = async (label: string) => { await act(async () => button(label).click()) }
beforeEach(() => {
  localStorage.clear()
  session.user = { id: 'u1' }
  requestPermission.mockReset()
  navigate.mockReset()
  vi.stubGlobal('ResizeObserver', class { observe() {} unobserve() {} disconnect() {} })
  vi.stubGlobal('isSecureContext', true)
  vi.stubGlobal('Notification', class { static permission = 'default'; static requestPermission = requestPermission })
  container = document.createElement('div')
  document.body.append(container)
  root = createRoot(container)
})
afterEach(() => { act(() => root.unmount()); container.remove(); vi.unstubAllGlobals(); vi.restoreAllMocks() })

it('shows an invitation without requesting permission or taking focus; dismissal survives remount', async () => {
  container.tabIndex = 0
  container.focus()
  render()
  expect(document.body.textContent).toContain('Stay updated while working elsewhere')
  expect(document.activeElement).toBe(container)
  expect(requestPermission).not.toHaveBeenCalled()
  await click('Not now')
  expect(document.body.textContent).not.toContain('Stay updated while working elsewhere')
  act(() => root.unmount())
  root = createRoot(container)
  render()
  expect(document.body.textContent).not.toContain('Stay updated while working elsewhere')
  await click('Notifications')
  expect(button('Enable desktop notifications')).toBeDefined()
})

it('opens the inbox when the bell is clicked during the invitation and enables from its footer', async () => {
  requestPermission.mockImplementation(async () => { Object.defineProperty(Notification, 'permission', { value: 'granted', configurable: true }); return 'granted' })
  render()
  await click('Notifications')
  expect(document.body.textContent).toContain("You're all caught up")
  expect(document.body.textContent).not.toContain('Stay updated while working elsewhere')
  await click('Enable desktop notifications')
  expect(requestPermission).toHaveBeenCalledOnce()
  expect(isDesktopEnabled('u1')).toBe(true)
  expect(button('Enable desktop notifications')).toBeUndefined()
})

it('enables directly from the invitation', async () => {
  requestPermission.mockImplementation(async () => { Object.defineProperty(Notification, 'permission', { value: 'granted', configurable: true }); return 'granted' })
  render()
  await click('Enable notifications')
  expect(isDesktopEnabled('u1')).toBe(true)
  expect(document.body.textContent).not.toContain('Stay updated while working elsewhere')
})

it.each(['denied', 'unsupported', 'enabled', 'disabled'])('does not prompt users who are %s', async state => {
  if (state === 'unsupported') vi.stubGlobal('isSecureContext', false)
  if (state === 'denied') Object.defineProperty(Notification, 'permission', { value: 'denied', configurable: true })
  if (state === 'enabled' || state === 'disabled') setDesktopEnabled('u1', state === 'enabled')
  render()
  expect(document.body.textContent).not.toContain('Stay updated while working elsewhere')
  expect(requestPermission).not.toHaveBeenCalled()
  await click('Notifications')
  if (state === 'denied') expect(document.body.textContent).toContain('Allow notifications in site settings')
  if (state === 'disabled') expect(button('Enable desktop notifications')).toBeDefined()
  if (state === 'unsupported') expect(button('Enable desktop notifications')).toBeUndefined()
})

it('does not repeatedly invite users who dismiss the browser permission prompt', async () => {
  requestPermission.mockResolvedValue('default')
  render()
  await click('Enable notifications')
  expect(isDesktopEnabled('u1')).toBe(false)
  expect(document.body.textContent).not.toContain('Stay updated while working elsewhere')
  await click('Notifications')
  expect(button('Enable desktop notifications')).toBeDefined()
})

it('does not save an opt-in if the session changes while browser permission is pending', async () => {
  let resolve!: (value: string) => void
  requestPermission.mockReturnValue(new Promise(r => { resolve = r }))
  render()
  await click('Enable notifications')
  session.user = { id: 'u2' }
  await act(async () => resolve('granted'))
  expect(isDesktopEnabled('u1')).toBe(false)
  expect(isDesktopEnabled('u2')).toBe(false)
})

it('opens notification settings from the dropdown and closes it', async () => {
  setDesktopEnabled('u1', false)
  render()
  await click('Notifications')
  await click('Notification settings')
  expect(navigate).toHaveBeenCalledWith({ to: '/w/$slug/settings/$section', params: { slug: 'acme', section: 'notifications' } })
  expect(document.body.textContent).not.toContain("You're all caught up")
})
