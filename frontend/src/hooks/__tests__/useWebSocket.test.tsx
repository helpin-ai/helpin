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
  static created = 0
  readyState = 0
  onopen: (() => void) | null = null
  onclose: (() => void) | null = null
  onmessage: ((event: { data: string }) => void) | null = null
  onerror: (() => void) | null = null
  send = vi.fn()
  close = vi.fn()
  constructor() { MockWebSocket.last = this; MockWebSocket.created++ }
}

let root: Root
function Probe() { useWebSocket({ workspaceId: 'ws-1', onEvent: () => {} }); return null }
beforeEach(() => {
  MockWebSocket.created = 0
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

it('reconnects with growing backoff after the connection drops', () => {
  expect(MockWebSocket.created).toBe(1)
  act(() => { MockWebSocket.last.readyState = 3; MockWebSocket.last.onclose?.() })
  act(() => { vi.advanceTimersByTime(999) })
  expect(MockWebSocket.created).toBe(1)
  act(() => { vi.advanceTimersByTime(1) })
  expect(MockWebSocket.created).toBe(2)
  // Still failing: the next wait doubles.
  act(() => { MockWebSocket.last.onclose?.() })
  act(() => { vi.advanceTimersByTime(1999) })
  expect(MockWebSocket.created).toBe(2)
  act(() => { vi.advanceTimersByTime(1) })
  expect(MockWebSocket.created).toBe(3)
  // A successful connection resets the wait.
  act(() => { MockWebSocket.last.readyState = MockWebSocket.OPEN; MockWebSocket.last.onopen?.(); MockWebSocket.last.onclose?.() })
  act(() => { vi.advanceTimersByTime(1000) })
  expect(MockWebSocket.created).toBe(4)
})

it('stops reconnecting once unmounted', () => {
  const socket = MockWebSocket.last
  act(() => root.unmount())
  act(() => { socket.onclose?.(); vi.advanceTimersByTime(60_000) })
  expect(MockWebSocket.created).toBe(1)
})

it('delivers events to the latest handler without reconnecting', () => {
  const first = vi.fn()
  const second = vi.fn()
  function HandlerProbe({ onEvent }: { onEvent: (event: unknown) => void }) { useWebSocket({ workspaceId: 'ws-1', onEvent }); return null }
  const handlerRoot = createRoot(document.createElement('div'))
  act(() => handlerRoot.render(<HandlerProbe onEvent={first} />))
  const socket = MockWebSocket.last
  const created = MockWebSocket.created
  act(() => handlerRoot.render(<HandlerProbe onEvent={second} />))
  expect(MockWebSocket.created).toBe(created)
  act(() => socket.onmessage?.({ data: JSON.stringify({ action: 'created', entity: 'support_conversation', entity_id: 'conv-1', workspace_id: 'ws-1' }) }))
  expect(second).toHaveBeenCalledTimes(1)
  expect(first).not.toHaveBeenCalled()
  act(() => handlerRoot.unmount())
})
