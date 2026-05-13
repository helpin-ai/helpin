import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { createDebouncedBoardFetchScheduler } from '../pmBoardFetchScheduler'

describe('createDebouncedBoardFetchScheduler', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('runs once with the latest scheduled payload after the debounce window', () => {
    const run = vi.fn()
    const scheduler = createDebouncedBoardFetchScheduler(250, run)

    scheduler.schedule({ filters: { priority: 'high' } })
    scheduler.schedule({ filters: { priority: 'low' } })

    vi.advanceTimersByTime(249)
    expect(run).not.toHaveBeenCalled()

    vi.advanceTimersByTime(1)
    expect(run).toHaveBeenCalledTimes(1)
    expect(run).toHaveBeenCalledWith({ filters: { priority: 'low' } })
  })

  it('cancels the pending fetch when asked', () => {
    const run = vi.fn()
    const scheduler = createDebouncedBoardFetchScheduler(250, run)

    scheduler.schedule({ filters: { owner_member_ids: 'member-1' } })
    scheduler.cancel()
    vi.advanceTimersByTime(250)

    expect(run).not.toHaveBeenCalled()
  })
})
