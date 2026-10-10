// @vitest-environment jsdom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { DesktopNotificationControl } from '../DesktopNotificationControl'
import { isDesktopEnabled, setDesktopEnabled } from '@/lib/desktopNotifications'
import { TooltipProvider } from '@/components/ui/tooltip'

vi.mock('@/stores/authStore', () => ({ useAuthStore: Object.assign((selector: (state: unknown) => unknown) => selector({ user: { id: 'u1' } }), { getState: () => ({ user: { id: 'u1' } }) }) }))
Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
let container: HTMLDivElement
let root: ReturnType<typeof createRoot>
const requestPermission = vi.fn()
const show = vi.fn()
const notifications: MockNotification[] = []
class MockNotification {
  static permission = 'default'
  static requestPermission = requestPermission
  onshow: (() => void) | null = null
  onerror: (() => void) | null = null
  onclick: (() => void) | null = null
  close = vi.fn()
  constructor(...args: unknown[]) { show(...args); notifications.push(this) }
}
beforeEach(() => {
  localStorage.clear()
  vi.stubGlobal('isSecureContext', true)
  MockNotification.permission = 'default'
  vi.stubGlobal('Notification', MockNotification)
  requestPermission.mockReset()
  show.mockReset()
  notifications.length = 0
  container = document.createElement('div')
  document.body.append(container)
  root = createRoot(container)
})
afterEach(() => { act(() => root.unmount()); container.remove(); vi.useRealTimers(); vi.unstubAllGlobals() })

function renderEnabled() {
  MockNotification.permission = 'granted'
  setDesktopEnabled('u1', true)
  renderControl()
}

function renderControl() {
  act(() => root.render(<TooltipProvider><DesktopNotificationControl /></TooltipProvider>))
}

function testButton() {
  return container.querySelector<HTMLButtonElement>('button[aria-label="Send test notification"]')
    ?? Array.from(container.querySelectorAll('button')).find(button => button.textContent === 'Send test notification')!
}

describe('existing settings desktop control', () => {
  it('asks permission only after enable and exposes a working test action', async () => {
    requestPermission.mockImplementation(async () => { Object.defineProperty(Notification, 'permission', { value: 'granted', configurable: true }); return 'granted' })
    renderControl()
    expect(requestPermission).not.toHaveBeenCalled()
    expect(isDesktopEnabled('u1')).toBe(false)
    await act(async () => (container.querySelector('[role="switch"]') as HTMLElement).click())
    expect(requestPermission).toHaveBeenCalledOnce()
    expect(isDesktopEnabled('u1')).toBe(true)
    act(() => Array.from(container.querySelectorAll('button')).find(button => button.textContent === 'Send test notification')!.click())
    expect(show).toHaveBeenCalledOnce()
    await act(async () => (container.querySelector('[role="switch"]') as HTMLElement).click())
    expect(isDesktopEnabled('u1')).toBe(false)
  })
  it('shows recovery instructions when permission is blocked', () => {
    Object.defineProperty(Notification, 'permission', { value: 'denied' })
    renderControl()
    expect(container.textContent).toContain('Allow notifications in site settings')
    expect((container.querySelector('[role="switch"]') as HTMLButtonElement).disabled).toBe(true)
    expect(requestPermission).not.toHaveBeenCalled()
  })

  it('reports asynchronous browser delivery errors and allows retrying', () => {
    renderEnabled()
    act(() => testButton().click())
    act(() => notifications[0].onerror?.())
    expect(container.querySelector('[role="alert"]')?.textContent).toContain('Couldn’t display')
    expect(testButton().disabled).toBe(false)
    act(() => testButton().click())
    expect(show).toHaveBeenCalledTimes(2)
  })

  it('waits for browser confirmation and prevents duplicate clicks while sending', () => {
    renderEnabled()
    act(() => testButton().click())
    expect(testButton().disabled).toBe(true)
    expect(container.querySelector('[role="status"]')).toBeNull()
    act(() => testButton().click())
    expect(show).toHaveBeenCalledOnce()
    act(() => notifications[0].onshow?.())
    expect(testButton().disabled).toBe(false)
    expect(container.querySelector('[role="status"]')?.textContent).toContain('Test sent')
  })

  it('shows troubleshooting when the browser never reports a result', () => {
    vi.useFakeTimers()
    renderEnabled()
    act(() => testButton().click())
    act(() => vi.advanceTimersByTime(8_000))
    expect(testButton().disabled).toBe(false)
    expect(container.querySelector('[role="status"]')?.textContent).toContain('couldn’t confirm')
    expect(container.textContent).toContain('system notification settings')
  })

  it('uses a fresh tag on each test so repeated tests are not silently replaced', () => {
    renderEnabled()
    act(() => testButton().click())
    act(() => notifications[0].onshow?.())
    act(() => testButton().click())
    expect(show.mock.calls[0][1].tag).not.toBe(show.mock.calls[1][1].tag)
    expect(notifications[0].close).toHaveBeenCalledOnce()
  })

  it('rechecks permission on test rather than silently using stale settings', () => {
    renderEnabled()
    MockNotification.permission = 'denied'
    act(() => testButton().click())
    expect(show).not.toHaveBeenCalled()
    expect(container.querySelector('[role="alert"]')?.textContent).toContain('Allow notifications')
  })

  it('closes the test and detaches feedback handlers when the control unmounts', () => {
    vi.useFakeTimers()
    renderEnabled()
    act(() => testButton().click())
    act(() => root.render(null))
    expect(notifications[0].close).toHaveBeenCalledOnce()
    expect(notifications[0].onerror).toBeNull()
    expect(notifications[0].onshow).toBeNull()
  })
})
