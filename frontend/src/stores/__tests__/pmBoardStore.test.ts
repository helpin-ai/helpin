import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('@/lib/services/pmTaskService', () => ({
  pmTaskService: {
    listBoard: vi.fn(),
    listBoardColumn: vi.fn(),
    listBoardByMember: vi.fn(),
    listBoardMemberColumn: vi.fn(),
    create: vi.fn(),
    move: vi.fn(),
    reorder: vi.fn(),
    update: vi.fn(),
  },
}))

vi.mock('@/lib/services/pmWorkflowService', () => ({
  pmWorkflowService: {
    list: vi.fn(),
    resolveTeamWorkflow: vi.fn(),
  },
}))

vi.mock('@/lib/services/pmViewService', () => ({
  pmViewService: {
    list: vi.fn(),
    create: vi.fn(),
    update: vi.fn(),
    remove: vi.fn(),
  },
}))

import { pmTaskService } from '@/lib/services/pmTaskService'
import { usePMBoardStore } from '../pmBoardStore'

const mockedTaskService = vi.mocked(pmTaskService)

const makeStory = (overrides: Record<string, unknown>) => ({
  id: 'task-default',
  name: 'Task',
  workflow_state_id: 'state-todo',
  position: 0,
  updated_at: '2026-03-22T00:00:00Z',
  ...overrides,
})

const makeStateColumn = (overrides: Record<string, unknown>) => ({
  state: {
    id: 'state-todo',
    name: 'To Do',
    state_type: 'unstarted',
    position: 0,
    ...((overrides.state as Record<string, unknown> | undefined) ?? {}),
  },
  tasks: [],
  task_groups: [],
  task_count: 0,
  point_total: 0,
  has_more: false,
  ...overrides,
})

describe('usePMBoardStore.moveMemberTask', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockedTaskService.reorder.mockResolvedValue({
      data: null,
      error: null,
      status: 200,
    } as never)
    usePMBoardStore.setState({
      workflow: null,
      teamId: null,
      error: null,
      memberColumns: [
        {
          member: { id: 'member-1', display_name: 'Ada', email: 'ada@example.com', role: 'member', status: 'active' },
          tasks: [
            {
              id: 'task-1',
              name: 'Task 1',
              owner_member_ids: ['member-1'],
              workflow_state_id: 'state-todo',
              position: 0,
              updated_at: '2026-03-22T00:00:00Z',
            },
            {
              id: 'task-2',
              name: 'Task 2',
              owner_member_ids: ['member-1'],
              workflow_state_id: 'state-todo',
              position: 1,
              updated_at: '2026-03-22T00:00:01Z',
            },
          ],
          task_count: 2,
          point_total: 0,
          has_more: false,
        },
        {
          member: { id: 'member-2', display_name: 'Grace', email: 'grace@example.com', role: 'member', status: 'active' },
          tasks: [],
          task_count: 0,
          point_total: 0,
          has_more: false,
        },
      ] as never,
    })
  })

  afterEach(() => {
    usePMBoardStore.setState({
      error: null,
      memberColumns: [],
    })
  })

  it('does nothing for same-member drag attempts', async () => {
    const before = structuredClone(usePMBoardStore.getState().memberColumns)

    await usePMBoardStore.getState().moveMemberTask({
      workspaceId: 'ws-1',
      taskId: 'task-1',
      fromMemberId: 'member-1',
      toMemberId: 'member-1',
      toIndex: 1,
    })

    expect(mockedTaskService.update).not.toHaveBeenCalled()
    expect(mockedTaskService.reorder).not.toHaveBeenCalled()
    expect(usePMBoardStore.getState().memberColumns).toEqual(before)
  })

  it('persists cross-member moves as reassignment only', async () => {
    mockedTaskService.update.mockResolvedValue({
      data: { task: { id: 'task-1', owner_member_ids: ['member-2'] } },
      error: null,
      status: 200,
    } as never)

    await usePMBoardStore.getState().moveMemberTask({
      workspaceId: 'ws-1',
      taskId: 'task-1',
      fromMemberId: 'member-1',
      toMemberId: 'member-2',
      toIndex: 0,
    })

    expect(mockedTaskService.update).toHaveBeenCalledWith('ws-1', 'task-1', {
      owner_member_ids: ['member-2'],
    })
    expect(mockedTaskService.reorder).not.toHaveBeenCalled()
    expect(usePMBoardStore.getState().memberColumns[0]?.tasks.map((task) => task.id)).toEqual(['task-2'])
    expect(usePMBoardStore.getState().memberColumns[1]?.tasks.map((task) => task.id)).toEqual(['task-1'])
    expect(usePMBoardStore.getState().memberColumns[1]?.tasks[0]?.owner_member_ids).toEqual(['member-2'])
  })
})

describe('usePMBoardStore.patchTask owner filters', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    usePMBoardStore.setState({
      workflow: null,
      teamId: null,
      filters: { owner_member_ids: 'member-2,member-3' },
      columns: [
        makeStateColumn({
          state: { id: 'state-todo' },
          tasks: [
            makeStory({
              id: 'task-1',
              owner_member_ids: ['member-1'],
              workflow_state_id: 'state-todo',
            }),
          ],
          task_count: 1,
        }),
      ] as never,
    })
  })

  afterEach(() => {
    usePMBoardStore.setState({
      filters: {},
      columns: [],
      teamId: null,
    })
  })

  it('keeps tasks whose owners overlap the selected owner ids', () => {
    const reconciled = usePMBoardStore.getState().patchTask('updated', 'task-1', makeStory({
      id: 'task-1',
      owner_member_ids: ['member-1', 'member-3'],
      workflow_state_id: 'state-todo',
    }) as never)

    expect(reconciled).toBe(true)
    expect(usePMBoardStore.getState().columns[0]?.tasks.map((task) => task.id)).toEqual(['task-1'])
  })
})

describe('usePMBoardStore.moveTask', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockedTaskService.reorder.mockResolvedValue({
      data: null,
      error: null,
      status: 200,
    } as never)
    mockedTaskService.move.mockResolvedValue({
      data: {
        task: makeStory({
          id: 'task-1',
          workflow_state_id: 'state-done',
          position: 3,
          completed: true,
          completed_at: '2026-03-24T10:00:00Z',
          moved_at: '2026-03-24T10:00:00Z',
          updated_at: '2026-03-24T10:00:00Z',
        }),
      },
      error: null,
      status: 200,
    } as never)
  })

  afterEach(() => {
    usePMBoardStore.setState({
      error: null,
      columns: [],
      workflow: null,
      teamId: null,
    })
  })

  it('reindexes loaded siblings after same-column reorder', async () => {
    usePMBoardStore.setState({
      workflow: null,
      teamId: null,
      error: null,
      columns: [
        makeStateColumn({
          state: { id: 'state-todo', name: 'To Do', state_type: 'unstarted', position: 0 },
          task_count: 3,
          tasks: [
            makeStory({ id: 'task-1', workflow_state_id: 'state-todo', position: 0, updated_at: '2026-03-24T10:00:00Z' }),
            makeStory({ id: 'task-2', workflow_state_id: 'state-todo', position: 1, updated_at: '2026-03-24T10:01:00Z' }),
            makeStory({ id: 'task-3', workflow_state_id: 'state-todo', position: 2, updated_at: '2026-03-24T10:02:00Z' }),
          ],
        }),
      ] as never,
    })

    await usePMBoardStore.getState().moveTask({
      workspaceId: 'ws-1',
      taskId: 'task-1',
      fromStateId: 'state-todo',
      toStateId: 'state-todo',
      toIndex: 2,
    })

    expect(mockedTaskService.reorder).toHaveBeenCalledWith(
      'ws-1',
      'task-1',
      expect.objectContaining({
        position: 2,
        debug_trace_id: expect.any(String),
      }),
    )
    const tasks = usePMBoardStore.getState().columns[0]?.tasks ?? []
    expect(tasks.map((task) => task.id)).toEqual(['task-2', 'task-3', 'task-1'])
    expect(tasks.map((task) => task.position)).toEqual([0, 1, 2])
  })

  it('passes top position and shifts loaded done tasks when moving into done', async () => {
    usePMBoardStore.setState({
      workflow: null,
      teamId: null,
      error: null,
      columns: [
        makeStateColumn({
          state: { id: 'state-todo', name: 'To Do', state_type: 'started', position: 0 },
          task_count: 1,
          tasks: [
            makeStory({ id: 'task-1', workflow_state_id: 'state-todo', position: 0, updated_at: '2026-03-24T10:00:00Z' }),
          ],
        }),
        makeStateColumn({
          state: { id: 'state-done', name: 'Done', state_type: 'done', position: 1 },
          task_count: 1,
          task_groups: [
            {
              key: 'today',
              label: 'Today',
              tasks: [
                makeStory({
                  id: 'task-done-old',
                  workflow_state_id: 'state-done',
                  position: 0,
                  completed: true,
                  completed_at: '2026-03-24T09:00:00Z',
                  moved_at: '2026-03-24T09:00:00Z',
                  updated_at: '2026-03-24T09:00:00Z',
                }),
              ],
            },
          ],
          tasks: [
            makeStory({
              id: 'task-done-old',
              workflow_state_id: 'state-done',
              position: 0,
              completed: true,
              completed_at: '2026-03-24T09:00:00Z',
              moved_at: '2026-03-24T09:00:00Z',
              updated_at: '2026-03-24T09:00:00Z',
            }),
          ],
        }),
      ] as never,
    })

    await usePMBoardStore.getState().moveTask({
      workspaceId: 'ws-1',
      taskId: 'task-1',
      fromStateId: 'state-todo',
      toStateId: 'state-done',
      toIndex: 0,
    })

    expect(mockedTaskService.move).toHaveBeenCalledWith(
      'ws-1',
      'task-1',
      expect.objectContaining({
        state_id: 'state-done',
        position: 0,
        debug_trace_id: expect.any(String),
      }),
    )

    const doneColumn = usePMBoardStore.getState().columns.find((column) => column.state.id === 'state-done')
    expect(doneColumn?.tasks.map((task) => task.id)).toEqual(['task-1', 'task-done-old'])
    expect(doneColumn?.tasks.map((task) => task.position)).toEqual([0, 1])
    expect(doneColumn?.task_groups).toEqual([])
  })

  it('preserves board-enriched labels and associations when a move response only returns the base task', async () => {
    mockedTaskService.move.mockResolvedValue({
      data: {
        task: makeStory({
          id: 'task-1',
          workflow_state_id: 'state-review',
          position: 0,
          updated_at: '2026-03-24T10:00:00Z',
        }),
      },
      error: null,
      status: 200,
    } as never)
    const label = { id: 'label-1', workspace_id: 'ws-1', name: 'Bug', color: '#ef4444', created_at: '', updated_at: '' }
    const contact = { id: 'contact-1', object_type: 'contact', display_name: 'Ada Lovelace' }

    usePMBoardStore.setState({
      workflow: null,
      teamId: null,
      error: null,
      columns: [
        makeStateColumn({
          state: { id: 'state-todo', name: 'To Do', state_type: 'started', position: 0 },
          task_count: 1,
          tasks: [
            makeStory({
              id: 'task-1',
              workflow_state_id: 'state-todo',
              position: 0,
              updated_at: '2026-03-24T09:00:00Z',
              labels: [label],
              contacts: [contact],
              sprint_id: 'sprint-1',
              sprint_name: 'Sprint 1',
              team_id: 'team-1',
              team_name: 'Platform',
              latest_run_id: 'run-1',
              latest_run_agent_id: 'agent-1',
              latest_run_status: 'paused',
              latest_run_pause_reason: 'human_approval',
              latest_run_at: '2026-03-24T08:00:00Z',
              blocked: true,
              blocked_by_count: 1,
              blocked_by_tasks: [{
                id: 'task-blocker',
                display_id: 99,
                task_key: 'HLP-99',
                name: 'Blocking task',
                workflow_state_id: 'state-todo',
                completed: false,
              }],
            }),
          ],
        }),
        makeStateColumn({
          state: { id: 'state-review', name: 'Review', state_type: 'started', position: 1 },
          task_count: 0,
          tasks: [],
        }),
      ] as never,
    })

    await usePMBoardStore.getState().moveTask({
      workspaceId: 'ws-1',
      taskId: 'task-1',
      fromStateId: 'state-todo',
      toStateId: 'state-review',
      toIndex: 0,
    })

    const movedTask = usePMBoardStore.getState().columns.find((column) => column.state.id === 'state-review')?.tasks[0]
    expect(movedTask?.labels).toEqual([label])
    expect(movedTask?.contacts).toEqual([contact])
    expect(movedTask?.sprint_name).toBe('Sprint 1')
    expect(movedTask?.team_name).toBe('Platform')
    expect(movedTask?.latest_run_id).toBe('run-1')
    expect(movedTask?.latest_run_agent_id).toBe('agent-1')
    expect(movedTask?.latest_run_status).toBe('paused')
    expect(movedTask?.latest_run_pause_reason).toBe('human_approval')
    expect(movedTask?.latest_run_at).toBe('2026-03-24T08:00:00Z')
    expect(movedTask?.blocked_by_count).toBe(1)
    expect(movedTask?.blocked_by_tasks?.[0]?.task_key).toBe('HLP-99')
  })

  it('rolls back failed cross-state moves without mutating the restored task', async () => {
    mockedTaskService.move.mockResolvedValue({
      data: null,
      error: 'Move failed',
      status: 400,
    } as never)
    usePMBoardStore.setState({
      workflow: null,
      teamId: null,
      error: null,
      columns: [
        makeStateColumn({
          state: { id: 'state-todo', name: 'To Do', state_type: 'started', position: 0 },
          task_count: 1,
          tasks: [
            makeStory({
              id: 'task-1',
              workflow_state_id: 'state-todo',
              position: 0,
              completed: false,
              completed_at: undefined,
              moved_at: undefined,
              updated_at: '2026-03-24T10:00:00Z',
            }),
          ],
        }),
        makeStateColumn({
          state: { id: 'state-done', name: 'Done', state_type: 'done', position: 1 },
          task_count: 0,
          tasks: [],
        }),
      ] as never,
    })

    await usePMBoardStore.getState().moveTask({
      workspaceId: 'ws-1',
      taskId: 'task-1',
      fromStateId: 'state-todo',
      toStateId: 'state-done',
      toIndex: 0,
    })

    const [todoColumn, doneColumn] = usePMBoardStore.getState().columns
    const restoredTask = todoColumn?.tasks[0]
    expect(todoColumn?.tasks.map((task) => task.id)).toEqual(['task-1'])
    expect(doneColumn?.tasks).toEqual([])
    expect(restoredTask?.workflow_state_id).toBe('state-todo')
    expect(restoredTask?.completed).toBe(false)
    expect(restoredTask?.completed_at).toBeUndefined()
    expect(usePMBoardStore.getState().error).toBe('Move failed')
  })
})
