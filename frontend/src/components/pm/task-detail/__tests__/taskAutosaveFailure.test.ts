import { describe, expect, it } from 'vitest'
import { getTaskPatchSignature, isBlockedTaskPatch } from '@/components/pm/task-detail/taskAutosaveFailure'

describe('task autosave failure helpers', () => {
  it('creates stable signatures independent of key order', () => {
    expect(getTaskPatchSignature({ estimate: 3, priority: 'high' } as never))
      .toBe(getTaskPatchSignature({ priority: 'high', estimate: 3 } as never))
  })

  it('blocks only the failed patch signature', () => {
    const failed = getTaskPatchSignature({ estimate: 3 } as never)

    expect(isBlockedTaskPatch({ estimate: 3 } as never, failed)).toBe(true)
    expect(isBlockedTaskPatch({ estimate: 5 } as never, failed)).toBe(false)
  })
})
