import { act, renderHook } from '@testing-library/react'
import { useSupportPresenceStore } from '@helpin-ai/support-core'
import { useTypingBroadcast } from '../use-typing-broadcast'

type Emit = [string, Record<string, unknown>]

function connect(): Emit[] {
  const emits: Emit[] = []
  useSupportPresenceStore.getState().setWsSend((type, data) => emits.push([type, data]))
  useSupportPresenceStore.getState().setWsConnected(true)
  return emits
}

beforeEach(() => {
  vi.useFakeTimers()
  useSupportPresenceStore.getState().setWsSend(null)
  useSupportPresenceStore.getState().setWsConnected(false)
})

afterEach(() => {
  vi.useRealTimers()
})

test('first keystroke emits start, subsequent (throttled) emit update', () => {
  const emits = connect()
  const { result } = renderHook(() => useTypingBroadcast('c1', true))

  act(() => result.current.notifyTyping('h'))
  expect(emits[0][0]).toBe('support:typing:start')
  expect(emits[0][1]).toEqual({ conversation_id: 'c1', content: 'h' })

  // Within the throttle window: no new emit.
  act(() => result.current.notifyTyping('he'))
  expect(emits).toHaveLength(1)

  // After the throttle window: an update.
  act(() => vi.advanceTimersByTime(300))
  act(() => result.current.notifyTyping('hel'))
  expect(emits[1][0]).toBe('support:typing:update')
})

test('emits stop after 5s of inactivity', () => {
  const emits = connect()
  const { result } = renderHook(() => useTypingBroadcast('c1', true))
  act(() => result.current.notifyTyping('hi'))
  act(() => vi.advanceTimersByTime(5000))
  expect(emits.at(-1)?.[0]).toBe('support:typing:stop')
})

test('never starts when disabled (note mode) but still stops if it was typing', () => {
  const emits = connect()
  const { result, rerender } = renderHook(({ enabled }) => useTypingBroadcast('c1', enabled), {
    initialProps: { enabled: true },
  })
  act(() => result.current.notifyTyping('hi'))
  expect(emits[0][0]).toBe('support:typing:start')

  // Switch to note mode and stop — a stop must still fire.
  rerender({ enabled: false })
  act(() => result.current.stopTyping())
  expect(emits.at(-1)?.[0]).toBe('support:typing:stop')

  // Typing while disabled emits nothing new.
  const count = emits.length
  act(() => result.current.notifyTyping('note text'))
  expect(emits).toHaveLength(count)
})

test('emits stop on unmount if mid-type', () => {
  const emits = connect()
  const { result, unmount } = renderHook(() => useTypingBroadcast('c1', true))
  act(() => result.current.notifyTyping('hi'))
  unmount()
  expect(emits.at(-1)?.[0]).toBe('support:typing:stop')
})

test('does nothing when the socket is disconnected', () => {
  const emits: Emit[] = []
  useSupportPresenceStore.getState().setWsSend((type, data) => emits.push([type, data]))
  useSupportPresenceStore.getState().setWsConnected(false)
  const { result } = renderHook(() => useTypingBroadcast('c1', true))
  act(() => result.current.notifyTyping('hi'))
  expect(emits).toHaveLength(0)
})
