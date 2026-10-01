// @vitest-environment jsdom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { DesktopNotificationControl } from '../DesktopNotificationControl'
import { isDesktopEnabled } from '@/lib/desktopNotifications'

vi.mock('@/stores/authStore', () => ({ useAuthStore: Object.assign((selector: (state: unknown) => unknown) => selector({ user: { id: 'u1' } }), { getState: () => ({ user: { id: 'u1' } }) }) }))
Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
let container: HTMLDivElement
let root: ReturnType<typeof createRoot>
const requestPermission = vi.fn()
const show = vi.fn()
beforeEach(() => {
  localStorage.clear()
  vi.stubGlobal('isSecureContext', true)
  vi.stubGlobal('Notification', class { static permission = 'default'; static requestPermission = requestPermission; constructor(...args: unknown[]) { show(...args) } })
  requestPermission.mockReset()
  show.mockClear()
  container = document.createElement('div')
  document.body.append(container)
  root = createRoot(container)
})
afterEach(() => { act(() => root.unmount()); container.remove(); vi.unstubAllGlobals() })

describe('existing settings desktop control', () => {
  it('asks permission only after enable and exposes a working test action', async () => {
    requestPermission.mockImplementation(async () => { Object.defineProperty(Notification, 'permission', { value: 'granted', configurable: true }); return 'granted' })
    act(() => root.render(<DesktopNotificationControl />))
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
    act(() => root.render(<DesktopNotificationControl />))
    expect(container.textContent).toContain('Allow notifications in site settings')
    expect((container.querySelector('[role="switch"]') as HTMLButtonElement).disabled).toBe(true)
    expect(requestPermission).not.toHaveBeenCalled()
  })
})
