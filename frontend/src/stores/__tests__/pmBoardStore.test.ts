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
  id: 'story-default',
  name: 'Story',
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
  stories: [],
  story_groups: [],
  story_count: 0,
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
          stories: [
            {
              id: 'story-1',
              name: 'Story 1',
              owner_member_id: 'member-1',
              workflow_state_id: 'state-todo',
              position: 0,
              updated_at: '2026-03-22T00:00:00Z',
            },
            {
              id: 'story-2',
              name: 'Story 2',
              owner_member_id: 'member-1',
              workflow_state_id: 'state-todo',
              position: 1,
              updated_at: '2026-03-22T00:00:01Z',
            },
          ],
          story_count: 2,
          point_total: 0,
          has_more: false,
        },
        {
          member: { id: 'member-2', display_name: 'Grace', email: 'grace@example.com', role: 'member', status: 'active' },
          stories: [],
          story_count: 0,
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
      storyId: 'story-1',
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
      data: { story: { id: 'story-1', owner_member_id: 'member-2' } },
      error: null,
      status: 200,
    } as never)

    await usePMBoardStore.getState().moveMemberTask({
      workspaceId: 'ws-1',
      storyId: 'story-1',
      fromMemberId: 'member-1',
      toMemberId: 'member-2',
      toIndex: 0,
    })

    expect(mockedTaskService.update).toHaveBeenCalledWith('ws-1', 'story-1', {
      owner_member_id: 'member-2',
    })
    expect(mockedTaskService.reorder).not.toHaveBeenCalled()
    expect(usePMBoardStore.getState().memberColumns[0]?.stories.map((story) => story.id)).toEqual(['story-2'])
    expect(usePMBoardStore.getState().memberColumns[1]?.stories.map((story) => story.id)).toEqual(['story-1'])
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
        story: makeStory({
          id: 'story-1',
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
          story_count: 3,
          stories: [
            makeStory({ id: 'story-1', workflow_state_id: 'state-todo', position: 0, updated_at: '2026-03-24T10:00:00Z' }),
            makeStory({ id: 'story-2', workflow_state_id: 'state-todo', position: 1, updated_at: '2026-03-24T10:01:00Z' }),
            makeStory({ id: 'story-3', workflow_state_id: 'state-todo', position: 2, updated_at: '2026-03-24T10:02:00Z' }),
          ],
        }),
      ] as never,
    })

    await usePMBoardStore.getState().moveTask({
      workspaceId: 'ws-1',
      storyId: 'story-1',
      fromStateId: 'state-todo',
      toStateId: 'state-todo',
      toIndex: 2,
    })

    expect(mockedTaskService.reorder).toHaveBeenCalledWith(
      'ws-1',
      'story-1',
      expect.objectContaining({
        position: 2,
        debug_trace_id: expect.any(String),
      }),
    )
    const stories = usePMBoardStore.getState().columns[0]?.stories ?? []
    expect(stories.map((story) => story.id)).toEqual(['story-2', 'story-3', 'story-1'])
    expect(stories.map((story) => story.position)).toEqual([0, 1, 2])
  })

  it('omits manual position when moving into done', async () => {
    usePMBoardStore.setState({
      workflow: null,
      teamId: null,
      error: null,
      columns: [
        makeStateColumn({
          state: { id: 'state-todo', name: 'To Do', state_type: 'started', position: 0 },
          story_count: 1,
          stories: [
            makeStory({ id: 'story-1', workflow_state_id: 'state-todo', position: 0, updated_at: '2026-03-24T10:00:00Z' }),
          ],
        }),
        makeStateColumn({
          state: { id: 'state-done', name: 'Done', state_type: 'done', position: 1 },
          story_count: 1,
          stories: [
            makeStory({
              id: 'story-done-old',
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
      storyId: 'story-1',
      fromStateId: 'state-todo',
      toStateId: 'state-done',
      toIndex: 1,
    })

    expect(mockedTaskService.move).toHaveBeenCalledWith(
      'ws-1',
      'story-1',
      expect.objectContaining({
        state_id: 'state-done',
        debug_trace_id: expect.any(String),
      }),
    )
  })
})
