import { describe, expect, it } from 'vitest'
import { getSprintProgress } from '../sprintProgress'

describe('getSprintProgress', () => {
  it('uses closeout totals so rolled-over tasks remain in a completed sprint denominator', () => {
    expect(getSprintProgress(
      { task_count: 148, done_task_count: 148, total_points: 218, done_points: 218 },
      {
        committed_count: 180,
        completed_count: 148,
        rolled_over_count: 32,
        committed_points: 268,
        completed_points: 218,
      } as never,
    )).toEqual({
      completedTasks: 148,
      committedTasks: 180,
      rolledOverTasks: 32,
      completedPoints: 218,
      committedPoints: 268,
      percentage: 82,
      historical: true,
    })
  })

  it('uses live stats for active sprints without a closeout', () => {
    expect(getSprintProgress({ task_count: 235, done_task_count: 108, total_points: 247, done_points: 106 })).toEqual({
      completedTasks: 108,
      committedTasks: 235,
      rolledOverTasks: 0,
      completedPoints: 106,
      committedPoints: 247,
      percentage: 46,
      historical: false,
    })
  })
})
