import { describe, expect, it } from 'vitest'
import { normalizeSprintPlanningWorkspace } from '../useSprints'

describe('normalizeSprintPlanningWorkspace', () => {
  it('converts null planning collections to empty arrays', () => {
    const workspace = normalizeSprintPlanningWorkspace({
      buckets: [{ key: 'active', label: 'Active', sprints: null }],
      backlog_tasks: null,
      backlog_total: 0,
    } as never)

    expect(workspace.buckets[0]?.sprints).toEqual([])
    expect(workspace.backlog_tasks).toEqual([])
  })
})
