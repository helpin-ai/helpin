import { describe, expect, it } from 'vitest'

import { commitDropBeforeClearingPreview, getSameStateBoardDropIndex, getStateBoardPreviewInsertIndex } from '../KanbanBoard.dnd'

describe('getStateBoardPreviewInsertIndex', () => {
  it('pins done-column drag previews to the top instead of a hovered slot', () => {
    expect(getStateBoardPreviewInsertIndex({
      toStateType: 'done',
      overId: 'story-2',
      toStateId: 'state-done',
      overIdx: 1,
      columnLength: 3,
      pointerBelowMid: true,
    })).toBe(0)
  })

  it('keeps active-column previews using hovered-slot semantics', () => {
    expect(getStateBoardPreviewInsertIndex({
      toStateType: 'started',
      overId: 'story-2',
      toStateId: 'state-started',
      overIdx: 1,
      columnLength: 3,
      pointerBelowMid: true,
    })).toBe(2)
  })

  it('uses the column tail when hovering the column dropzone itself', () => {
    expect(getStateBoardPreviewInsertIndex({
      toStateType: 'started',
      overId: 'state-started',
      toStateId: 'state-started',
      overIdx: -1,
      columnLength: 3,
      pointerBelowMid: false,
    })).toBe(3)
  })
})

describe('getSameStateBoardDropIndex', () => {
  it('uses arrayMove semantics when dragging downward over another card', () => {
    expect(getSameStateBoardDropIndex({
      fromIndex: 1,
      overId: 'story-4',
      stateId: 'state-started',
      overIndex: 3,
      columnLength: 5,
      pointerBelowMid: false,
    })).toBe(3)
  })

  it('uses arrayMove semantics when dragging upward over another card', () => {
    expect(getSameStateBoardDropIndex({
      fromIndex: 4,
      overId: 'story-2',
      stateId: 'state-started',
      overIndex: 1,
      columnLength: 5,
      pointerBelowMid: true,
    })).toBe(1)
  })

  it('uses the column tail when hovering the column dropzone itself', () => {
    expect(getSameStateBoardDropIndex({
      fromIndex: 1,
      overId: 'state-started',
      stateId: 'state-started',
      overIndex: 4,
      columnLength: 5,
      pointerBelowMid: false,
    })).toBe(4)
  })
})

describe('commitDropBeforeClearingPreview', () => {
  it('starts the optimistic commit before clearing the drag preview', async () => {
    const calls: string[] = []

    const result = await commitDropBeforeClearingPreview({
      commit: async () => {
        calls.push('commit-start')
        await Promise.resolve()
        calls.push('commit-end')
        return 42
      },
      clearPreview: () => {
        calls.push('clear-preview')
      },
    })

    expect(result).toBe(42)
    expect(calls).toEqual(['commit-start', 'clear-preview', 'commit-end'])
  })
})
