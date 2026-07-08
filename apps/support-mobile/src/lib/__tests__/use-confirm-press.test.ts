import { act, renderHook } from '@testing-library/react'
import { useConfirmPress } from '../use-confirm-press'

beforeEach(() => {
  vi.useFakeTimers()
})

afterEach(() => {
  vi.useRealTimers()
})

test('first trigger arms the confirm state without running the action', () => {
  const action = vi.fn()
  const { result } = renderHook(() => useConfirmPress())

  expect(result.current.armed).toBe(false)
  act(() => result.current.trigger(action))

  expect(result.current.armed).toBe(true)
  expect(action).not.toHaveBeenCalled()
})

test('second trigger while armed runs the action and disarms', () => {
  const action = vi.fn()
  const { result } = renderHook(() => useConfirmPress())

  act(() => result.current.trigger(action))
  act(() => result.current.trigger(action))

  expect(action).toHaveBeenCalledTimes(1)
  expect(result.current.armed).toBe(false)
})

test('disarms itself automatically after the timeout elapses', () => {
  const action = vi.fn()
  const { result } = renderHook(() => useConfirmPress(3000))

  act(() => result.current.trigger(action))
  expect(result.current.armed).toBe(true)

  act(() => {
    vi.advanceTimersByTime(3000)
  })

  expect(result.current.armed).toBe(false)
  expect(action).not.toHaveBeenCalled()
})

test('a trigger after the timeout re-arms instead of running the action', () => {
  const action = vi.fn()
  const { result } = renderHook(() => useConfirmPress(3000))

  act(() => result.current.trigger(action))
  act(() => {
    vi.advanceTimersByTime(3000)
  })

  act(() => result.current.trigger(action))
  expect(result.current.armed).toBe(true)
  expect(action).not.toHaveBeenCalled()
})
