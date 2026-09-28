// @vitest-environment jsdom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { PORTAL_POLL_INTERVAL_MS, usePortalLiveUpdates } from '../usePortalLiveUpdates'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

class FakeSocket {
  static OPEN = 1
  static instances: FakeSocket[] = []
  readyState = 0
  onopen: (() => void) | null = null
  onmessage: ((event: { data: string }) => void) | null = null
  onclose: (() => void) | null = null
  constructor(public url: string) { FakeSocket.instances.push(this) }
  open() { this.readyState = FakeSocket.OPEN; this.onopen?.() }
  send(data: object) { this.onmessage?.({ data: JSON.stringify(data) }) }
  close() { this.readyState = 3; this.onclose?.() }
}

let root: Root
const onChange = vi.fn()
const onTyping = vi.fn()

function Harness() {
  usePortalLiveUpdates('acme', true, { onChange, onTyping })
  return null
}

beforeEach(async () => {
  vi.useFakeTimers()
  FakeSocket.instances = []
  onChange.mockClear()
  onTyping.mockClear()
  vi.stubGlobal('WebSocket', FakeSocket)
  root = createRoot(document.createElement('div'))
  await act(async () => root.render(<Harness />))
})

afterEach(() => {
  act(() => root.unmount())
  vi.unstubAllGlobals()
  vi.useRealTimers()
})

describe('usePortalLiveUpdates', () => {
  it('connects to the portal socket and relays change and typing signals', () => {
    const socket = FakeSocket.instances[0]
    expect(socket.url).toMatch(/\/public\/portal\/acme\/ws$/)
    expect(socket.url).toMatch(/^wss?:/)
    act(() => socket.open())
    act(() => socket.send({ type: 'request:changed', reference: 'req_a' }))
    act(() => socket.send({ type: 'request:typing', reference: 'req_a', typing: true }))
    act(() => socket.send({ type: 'unknown', reference: 'req_a' }))
    expect(onChange).toHaveBeenCalledWith('req_a')
    expect(onTyping).toHaveBeenCalledWith('req_a', true)
    expect(onChange).toHaveBeenCalledTimes(1)
  })

  it('reconnects with backoff and refreshes once back online', () => {
    const first = FakeSocket.instances[0]
    act(() => first.open())
    act(() => first.close())
    act(() => { vi.advanceTimersByTime(1000) })
    expect(FakeSocket.instances).toHaveLength(2)
    act(() => FakeSocket.instances[1].open())
    expect(onChange).toHaveBeenCalledWith(null)
  })

  it('polls while the socket is down and not while it is connected', () => {
    act(() => { vi.advanceTimersByTime(PORTAL_POLL_INTERVAL_MS) })
    expect(onChange).toHaveBeenCalledWith(null)
    onChange.mockClear()
    const socket = FakeSocket.instances[FakeSocket.instances.length - 1]
    act(() => socket.open())
    act(() => { vi.advanceTimersByTime(PORTAL_POLL_INTERVAL_MS) })
    expect(onChange).not.toHaveBeenCalled()
  })

  it('stops reconnecting after unmount', () => {
    act(() => root.unmount())
    const count = FakeSocket.instances.length
    act(() => { vi.advanceTimersByTime(60_000) })
    expect(FakeSocket.instances).toHaveLength(count)
    root = createRoot(document.createElement('div'))
  })
})
