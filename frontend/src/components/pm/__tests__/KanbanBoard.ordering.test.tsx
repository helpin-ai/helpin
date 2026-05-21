import { describe, expect, it } from 'vitest'

import {
  DragPreviewManager,
  PM_BOARD_DRAG_ACTIVATION_DISTANCE,
  PM_BOARD_DRAG_OVER_THROTTLE_MS,
  PM_BOARD_DROP_PLACEHOLDER_FALLBACK_HEIGHT,
  commitDropBeforeClearingPreview,
  getBaseDragSourceColumnId,
  getPMBoardScrollContainerClassName,
  getSameStateBoardDropIndex,
  getStateBoardPreviewInsertIndex,
  getStateColumnTaskGroupsForRender,
  getTaskDropPlaceholderHeight,
  getTaskDropPlaceholderId,
  getTaskDropPlaceholderIndex,
  getTaskDropPlaceholderPreview,
  hasDragPreviewChanged,
  parseTaskDropPlaceholderId,
  resolveBoardDropTarget,
} from '../KanbanBoard.dnd'

describe('board drag responsiveness settings', () => {
  it('keeps card drag activation close to immediate while preserving click tolerance', () => {
    expect(PM_BOARD_DRAG_ACTIVATION_DISTANCE).toBe(2)
  })

  it('updates drag previews at roughly the display frame cadence', () => {
    expect(PM_BOARD_DRAG_OVER_THROTTLE_MS).toBe(16)
  })
})

describe('getPMBoardScrollContainerClassName', () => {
  it.each(['state', 'member'] as const)('reserves scrollbar gutter for %s columns so card widths do not reflow during drag', (variant) => {
    expect(getPMBoardScrollContainerClassName({
      variant,
      isOver: false,
    })).toEqual(expect.stringContaining('[scrollbar-gutter:stable]'))
  })

  it('keeps the list gap stable when a state column is hovered', () => {
    const className = getPMBoardScrollContainerClassName({
      variant: 'state',
      isOver: true,
    })

    expect(className).toContain('gap-2')
    expect(className).not.toContain('gap-4')
    expect(className).toContain('ring-1 ring-inset ring-sky-500/25')
  })
})

describe('getStateBoardPreviewInsertIndex', () => {
  it('pins done-column drag previews to the top instead of a hovered slot', () => {
    expect(getStateBoardPreviewInsertIndex({
      toStateType: 'done',
      overId: 'task-2',
      toStateId: 'state-done',
      overIdx: 1,
      columnLength: 3,
      pointerBelowMid: true,
    })).toBe(0)
  })

  it('keeps active-column previews using hovered-slot semantics', () => {
    expect(getStateBoardPreviewInsertIndex({
      toStateType: 'started',
      overId: 'task-2',
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
      overId: 'task-4',
      stateId: 'state-started',
      overIndex: 3,
      columnLength: 5,
      pointerBelowMid: false,
    })).toBe(2)
  })

  it('uses arrayMove semantics when dragging upward over another card', () => {
    expect(getSameStateBoardDropIndex({
      fromIndex: 4,
      overId: 'task-2',
      stateId: 'state-started',
      overIndex: 1,
      columnLength: 5,
      pointerBelowMid: true,
    })).toBe(2)
  })

  it('can target the gap before the next card after removing the active card', () => {
    expect(getSameStateBoardDropIndex({
      fromIndex: 0,
      overId: 'task-2',
      stateId: 'state-started',
      overIndex: 1,
      columnLength: 4,
      pointerBelowMid: false,
    })).toBe(0)
  })

  it('can target a middle gap while dragging downward without skipping a card', () => {
    expect(getSameStateBoardDropIndex({
      fromIndex: 0,
      overId: 'task-3',
      stateId: 'state-started',
      overIndex: 2,
      columnLength: 4,
      pointerBelowMid: false,
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

describe('resolveBoardDropTarget', () => {
  const sameColumn = {
    id: 'state-started',
    stateType: 'started',
    tasks: [
      { id: 'task-1' },
      { id: 'task-2' },
      { id: 'task-3' },
      { id: 'task-4' },
    ],
  }

  it('keeps the same-column top placeholder target when dnd-kit reports the column container', () => {
    expect(resolveBoardDropTarget({
      activeId: 'task-2',
      fromColumnId: 'state-started',
      overId: 'state-started',
      pointerBelowMid: false,
      previewTarget: {
        fromColumnId: 'state-started',
        toColumnId: 'state-started',
        toIndex: 0,
      },
      columns: [sameColumn],
    })).toEqual({
      toColumnId: 'state-started',
      toIndex: 0,
    })
  })

  it.each([0, 1, 2, 3])('keeps same-column placeholder gap %i when the pointer is over the visual placeholder', (toIndex) => {
    expect(resolveBoardDropTarget({
      activeId: 'task-2',
      fromColumnId: 'state-started',
      overId: 'state-started',
      pointerBelowMid: false,
      previewTarget: {
        fromColumnId: 'state-started',
        toColumnId: 'state-started',
        toIndex,
      },
      columns: [sameColumn],
    })).toEqual({
      toColumnId: 'state-started',
      toIndex,
    })
  })

  it('can move the second card to the first position when hovering the first card', () => {
    expect(resolveBoardDropTarget({
      activeId: 'task-2',
      fromColumnId: 'state-started',
      overId: 'task-1',
      pointerBelowMid: false,
      previewTarget: {
        fromColumnId: 'state-started',
        toColumnId: 'state-started',
        toIndex: 1,
      },
      columns: [sameColumn],
    })).toEqual({
      toColumnId: 'state-started',
      toIndex: 0,
    })
  })

  it('uses the real hovered card over a stale same-column placeholder target', () => {
    expect(resolveBoardDropTarget({
      activeId: 'task-2',
      fromColumnId: 'state-started',
      overId: 'task-1',
      pointerBelowMid: false,
      previewTarget: {
        fromColumnId: 'state-started',
        toColumnId: 'state-started',
        toIndex: 3,
      },
      columns: [sameColumn],
    })).toEqual({
      toColumnId: 'state-started',
      toIndex: 0,
    })
  })

  it('uses an explicit same-column placeholder droppable over a stale stored target', () => {
    expect(resolveBoardDropTarget({
      activeId: 'task-2',
      fromColumnId: 'state-started',
      overId: getTaskDropPlaceholderId('state-started', 0),
      pointerBelowMid: false,
      previewTarget: {
        fromColumnId: 'state-started',
        toColumnId: 'state-started',
        toIndex: 3,
      },
      columns: [sameColumn],
    })).toEqual({
      toColumnId: 'state-started',
      toIndex: 0,
    })
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

describe('DragPreviewManager', () => {
  it('uses the measured task card height and resets to the fallback when cleared', () => {
    const manager = new DragPreviewManager()

    expect(manager.getDropPlaceholderHeight()).toBe(PM_BOARD_DROP_PLACEHOLDER_FALLBACK_HEIGHT)

    manager.setDropPlaceholderRect({ height: 100, width: 300 })
    expect(manager.getDropPlaceholderHeight()).toBe(100)

    manager.clear()
    expect(manager.getDropPlaceholderHeight()).toBe(PM_BOARD_DROP_PLACEHOLDER_FALLBACK_HEIGHT)
  })

  it('clears stored cross-column targets when preview overrides are cleared', () => {
    const manager = new DragPreviewManager()

    manager.updatePreview(
      'state-todo',
      'state-doing',
      [{ id: 'task-1' } as never],
      [{ id: 'task-2' } as never, { id: 'task-1' } as never],
      1,
    )

    expect(manager.getDropTarget()).toEqual({
      fromColumnId: 'state-todo',
      toColumnId: 'state-doing',
      toIndex: 1,
    })

    manager.clearColumnOverrides()

    expect(manager.getDropTarget()).toBeNull()
    expect(manager.getColumnTasks('state-todo')).toBeNull()
    expect(manager.getColumnTasks('state-doing')).toBeNull()
  })

  it('clears the previous target column when cross-column preview moves to another target', () => {
    const manager = new DragPreviewManager()
    let previousTargetNotifications = 0
    manager.subscribeColumn('state-review', () => {
      previousTargetNotifications++
    })

    manager.updatePreview(
      'state-done',
      'state-review',
      [{ id: 'done-old' } as never],
      [{ id: 'moving' } as never, { id: 'review-old' } as never],
      0,
    )
    manager.updatePreview(
      'state-done',
      'state-progress',
      [{ id: 'done-old' } as never],
      [{ id: 'moving' } as never, { id: 'progress-old' } as never],
      0,
    )

    expect(manager.getColumnTasks('state-review')).toBeNull()
    expect(manager.getColumnTasks('state-progress')?.map((task) => task.id)).toEqual(['moving', 'progress-old'])
    expect(previousTargetNotifications).toBe(2)
  })
})

describe('getStateColumnTaskGroupsForRender', () => {
  it('disables done-column groups while a drag preview override is active', () => {
    expect(getStateColumnTaskGroupsForRender({
      stateType: 'done',
      hasPreviewOverride: true,
      taskGroups: [
        { key: 'today', label: 'Today', tasks: [{ id: 'task-1' } as never] },
      ],
    })).toEqual([])
  })

  it('keeps done-column groups when rendering persisted board data', () => {
    const taskGroups = [
      { key: 'today', label: 'Today', tasks: [{ id: 'task-1' } as never] },
    ]

    expect(getStateColumnTaskGroupsForRender({
      stateType: 'done',
      hasPreviewOverride: false,
      taskGroups,
    })).toBe(taskGroups)
  })
})

describe('getBaseDragSourceColumnId', () => {
  it('keeps the source anchored to base board data even when preview has moved the task', () => {
    expect(getBaseDragSourceColumnId({
      activeId: 'task-1',
      columns: [
        { id: 'state-todo', tasks: [{ id: 'task-1' }] },
        { id: 'state-done', tasks: [{ id: 'task-2' }, { id: 'task-1' }] },
      ],
    })).toBe('state-todo')
  })
})

describe('hasDragPreviewChanged', () => {
  it('detects a changed target order even when the source order is unchanged', () => {
    expect(hasDragPreviewChanged({
      currentFrom: [{ id: 'task-2' }] as never,
      nextFrom: [{ id: 'task-2' }] as never,
      currentTo: [{ id: 'task-1' }, { id: 'task-3' }] as never,
      nextTo: [{ id: 'task-3' }, { id: 'task-1' }] as never,
    })).toBe(true)
  })

  it('ignores unchanged previews', () => {
    expect(hasDragPreviewChanged({
      currentFrom: [{ id: 'task-2' }] as never,
      nextFrom: [{ id: 'task-2' }] as never,
      currentTo: [{ id: 'task-1' }, { id: 'task-3' }] as never,
      nextTo: [{ id: 'task-1' }, { id: 'task-3' }] as never,
    })).toBe(false)
  })

  it('detects placeholder index changes even when task lists stay unchanged', () => {
    expect(hasDragPreviewChanged({
      currentFrom: [{ id: 'task-2' }] as never,
      nextFrom: [{ id: 'task-2' }] as never,
      currentTo: [{ id: 'task-3' }] as never,
      nextTo: [{ id: 'task-3' }] as never,
      currentDropTarget: {
        fromColumnId: 'state-todo',
        toColumnId: 'state-doing',
        toIndex: 0,
      },
      nextDropTarget: {
        fromColumnId: 'state-todo',
        toColumnId: 'state-doing',
        toIndex: 1,
      },
    })).toBe(true)
  })
})

describe('getTaskDropPlaceholderPreview', () => {
  it('removes the active task from both preview columns without inserting a duplicate target card', () => {
    const task = { id: 'task-1' } as never
    const fromTask = { id: 'task-2' } as never
    const targetTask = { id: 'task-3' } as never

    expect(getTaskDropPlaceholderPreview({
      activeId: 'task-1',
      fromColumnId: 'state-todo',
      toColumnId: 'state-doing',
      fromTasks: [task, fromTask],
      toTasks: [targetTask],
      toIndex: 1,
    })).toEqual({
      fromTasks: [fromTask],
      toTasks: [targetTask],
      dropTarget: {
        fromColumnId: 'state-todo',
        toColumnId: 'state-doing',
        toIndex: 1,
      },
    })
  })

  it('uses one active-task-free task list for same-column placeholder previews', () => {
    const task = { id: 'task-1' } as never
    const taskBefore = { id: 'task-0' } as never
    const taskAfter = { id: 'task-2' } as never

    expect(getTaskDropPlaceholderPreview({
      activeId: 'task-1',
      fromColumnId: 'state-todo',
      toColumnId: 'state-todo',
      fromTasks: [taskBefore, task, taskAfter],
      toTasks: [taskBefore, task, taskAfter],
      toIndex: 2,
    })).toEqual({
      fromTasks: [taskBefore, taskAfter],
      toTasks: [taskBefore, taskAfter],
      dropTarget: {
        fromColumnId: 'state-todo',
        toColumnId: 'state-todo',
        toIndex: 2,
      },
    })
  })
})

describe('getTaskDropPlaceholderIndex', () => {
  it('returns a clamped placeholder index for the target column', () => {
    expect(getTaskDropPlaceholderIndex({
      dropTarget: {
        fromColumnId: 'state-todo',
        toColumnId: 'state-doing',
        toIndex: 8,
      },
      columnId: 'state-doing',
      taskCount: 3,
    })).toBe(3)
  })

  it('returns null for non-target columns', () => {
    expect(getTaskDropPlaceholderIndex({
      dropTarget: {
        fromColumnId: 'state-todo',
        toColumnId: 'state-doing',
        toIndex: 1,
      },
      columnId: 'state-review',
      taskCount: 3,
    })).toBeNull()
  })
})

describe('task drop placeholder ids', () => {
  it('round trips placeholder droppable ids without losing column ids that contain separators', () => {
    const id = getTaskDropPlaceholderId('member:alpha', 2)

    expect(parseTaskDropPlaceholderId(id)).toEqual({
      columnId: 'member:alpha',
      index: 2,
    })
  })

  it('returns null for non-placeholder ids', () => {
    expect(parseTaskDropPlaceholderId('task-1')).toBeNull()
  })
})

describe('getTaskDropPlaceholderHeight', () => {
  it('uses the dragged card layout height for the drop slot', () => {
    expect(getTaskDropPlaceholderHeight({
      height: 100,
      width: 300,
    })).toBe(100)
  })

  it('does not grow the drop slot when a card is wider', () => {
    expect(getTaskDropPlaceholderHeight({
      height: 100,
      width: 600,
    })).toBe(100)
  })

  it('falls back when there is no measured dragged rect', () => {
    expect(getTaskDropPlaceholderHeight(null)).toBe(PM_BOARD_DROP_PLACEHOLDER_FALLBACK_HEIGHT)
  })
})
