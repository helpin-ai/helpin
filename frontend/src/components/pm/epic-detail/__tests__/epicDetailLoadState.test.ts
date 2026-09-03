import { describe, expect, it } from 'vitest'

import type { EpicWithStats, Task } from '@/lib/pmTypes'
import { getEpicTaskDisplayCount, shouldShowEpicLoading } from '../epicDetailLoadState'

const epic = {
  epic: { id: 'epic-1', workspace_id: 'ws-1', name: 'Fast epic' },
  stats: { task_count: 37 },
} as EpicWithStats

describe('epic detail progressive loading', () => {
  it('stops blocking the page as soon as epic data is available', () => {
    expect(shouldShowEpicLoading(null, true)).toBe(true)
    expect(shouldShowEpicLoading(epic, true)).toBe(false)
    expect(shouldShowEpicLoading(epic, false)).toBe(false)
  })

  it('uses the epic aggregate count until the task collection arrives', () => {
    expect(getEpicTaskDisplayCount(epic, [], true)).toBe(37)
    expect(getEpicTaskDisplayCount(epic, [{ id: 'task-1' }, { id: 'task-2' }] as Task[], false)).toBe(2)
  })
})
