import { describe, expect, it } from 'vitest'

import { DragPreviewManager, commitDropBeforeClearingPreview, getSameStateBoardDropIndex, getStableCrossColumnPreviewIndex, getStateBoardPreviewInsertIndex, getStoredCrossColumnDropTarget } from '../KanbanBoard.dnd'

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

describe('getStoredCrossColumnDropTarget', () => {
  it('returns the stored between-card target when it matches the source column', () => {
    expect(getStoredCrossColumnDropTarget({
      previewTarget: {
        fromColumnId: 'state-todo',
        toColumnId: 'state-doing',
        toIndex: 2,
      },
      fromColumnId: 'state-todo',
      validColumnIds: ['state-todo', 'state-doing', 'state-done'],
    })).toEqual({
      toColumnId: 'state-doing',
      toIndex: 2,
    })
  })

  it('ignores stale targets from another source column', () => {
    expect(getStoredCrossColumnDropTarget({
      previewTarget: {
        fromColumnId: 'state-backlog',
        toColumnId: 'state-doing',
        toIndex: 1,
      },
      fromColumnId: 'state-todo',
      validColumnIds: ['state-todo', 'state-doing'],
    })).toBeNull()
  })
})

describe('DragPreviewManager', () => {
  it('clears stored cross-column targets when preview overrides are cleared', () => {
    const manager = new DragPreviewManager()

    manager.updatePreview(
      'state-todo',
      'state-doing',
      [{ id: 'story-1' } as never],
      [{ id: 'story-2' } as never, { id: 'story-1' } as never],
      1,
    )

    expect(manager.getDropTarget()).toEqual({
      fromColumnId: 'state-todo',
      toColumnId: 'state-doing',
      toIndex: 1,
    })

    manager.clearColumnOverrides()

    expect(manager.getDropTarget()).toBeNull()
    expect(manager.getColumnStories('state-todo')).toBeNull()
    expect(manager.getColumnStories('state-doing')).toBeNull()
  })
})

describe('getStableCrossColumnPreviewIndex', () => {
  it('keeps the last between-card target when the hover falls back to the column container', () => {
    expect(getStableCrossColumnPreviewIndex({
      previewTarget: {
        fromColumnId: 'state-todo',
        toColumnId: 'state-doing',
        toIndex: 2,
      },
      fromColumnId: 'state-todo',
      toColumnId: 'state-doing',
      overId: 'state-doing',
      containerId: 'state-doing',
      computedIndex: 5,
      columnLength: 5,
    })).toBe(2)
  })

  it('uses the computed append index when there is no matching stored target', () => {
    expect(getStableCrossColumnPreviewIndex({
      previewTarget: {
        fromColumnId: 'state-backlog',
        toColumnId: 'state-doing',
        toIndex: 1,
      },
      fromColumnId: 'state-todo',
      toColumnId: 'state-doing',
      overId: 'state-doing',
      containerId: 'state-doing',
      computedIndex: 5,
      columnLength: 5,
    })).toBe(5)
  })
})
