// @vitest-environment jsdom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { useWebSocket } from '../useWebSocket'

vi.mock('@/lib/api', () => ({ API_BASE: 'https://api.example.test/api' }))
vi.mock('@helpin-ai/support-core', () => ({ buildWorkspaceWebSocketUrl: () => 'wss://api.example.test/api/ws' }))
vi.mock('@/stores/supportPresenceStore', () => ({ useSupportPresenceStore: { getState: () => ({ setOnlineVisitors: vi.fn() }) } }))

class MockWebSocket {
  static OPEN = 1
  static last: MockWebSocket
  readyState = 0
  onopen: (() => void) | null = null
  onclose: (() => void) | null = null
  onmessage: ((event: { data: string }) => void) | null = null
  onerror: (() => void) | null = null
  send = vi.fn()
  close = vi.fn()
  constructor() { MockWebSocket.last = this }
}

let root: Root
function Probe() { useWebSocket({ workspaceId: 'ws-1', onEvent: () => {} }); return null }
beforeEach(() => {
  vi.useFakeTimers()
  vi.stubGlobal('IS_REACT_ACT_ENVIRONMENT', true)
  vi.stubGlobal('WebSocket', MockWebSocket)
  vi.spyOn(document, 'visibilityState', 'get').mockReturnValue('visible')
  root = createRoot(document.createElement('div'))
  act(() => root.render(<Probe />))
})
afterEach(() => {
  act(() => root.unmount())
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
  vi.useRealTimers()
})

it('refreshes on return to the tab, coalesces focus/visibility events, and cleans up', () => {
  const socket = MockWebSocket.last
  act(() => { socket.readyState = MockWebSocket.OPEN; socket.onopen?.() })
  act(() => {
    window.dispatchEvent(new Event('focus'))
    document.dispatchEvent(new Event('visibilitychange'))
  })
  expect(socket.send).toHaveBeenCalledTimes(1)
  expect(JSON.parse(socket.send.mock.calls[0][0])).toEqual({ type: 'support:ping', data: { active: true } })
  vi.mocked(Object.getOwnPropertyDescriptor(document, 'visibilityState')!.get!).mockReturnValue('hidden')
  act(() => { vi.advanceTimersByTime(1000); document.dispatchEvent(new Event('visibilitychange')) })
  expect(socket.send).toHaveBeenCalledTimes(1)
  vi.mocked(Object.getOwnPropertyDescriptor(document, 'visibilityState')!.get!).mockReturnValue('visible')
  act(() => document.dispatchEvent(new Event('visibilitychange')))
  expect(socket.send).toHaveBeenCalledTimes(2)
  act(() => root.unmount())
  act(() => { vi.advanceTimersByTime(1000); window.dispatchEvent(new Event('focus')) })
  expect(socket.send).toHaveBeenCalledTimes(2)
})

it('does not send a refresh while disconnected', () => {
  const socket = MockWebSocket.last
  act(() => window.dispatchEvent(new Event('focus')))
  expect(socket.send).not.toHaveBeenCalled()
  act(() => { socket.readyState = MockWebSocket.OPEN; socket.onopen?.(); socket.readyState = 3; socket.onclose?.() })
  act(() => window.dispatchEvent(new Event('focus')))
  expect(socket.send).not.toHaveBeenCalled()
})
