// @vitest-environment jsdom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { useDebounce } from '../useDebounce'

;(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

function Harness({ value }: { value: string }) {
  const debounced = useDebounce(value, 200)
  return <span data-testid="out">{debounced}</span>
}

describe('useDebounce', () => {
  beforeEach(() => { vi.useFakeTimers() })
  afterEach(() => { vi.useRealTimers() })

  it('returns the initial value immediately and trailing value after the delay', async () => {
    const container = document.createElement('div')
    const root = createRoot(container)
    await act(async () => { root.render(<Harness value="a" />) })
    expect(container.textContent).toBe('a')

    await act(async () => { root.render(<Harness value="ab" />) })
    expect(container.textContent).toBe('a')

    await act(async () => { root.render(<Harness value="abc" />) })
    await act(async () => { vi.advanceTimersByTime(199) })
    expect(container.textContent).toBe('a')

    await act(async () => { vi.advanceTimersByTime(1) })
    expect(container.textContent).toBe('abc')

    await act(async () => { root.unmount() })
  })
})
