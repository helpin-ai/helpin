import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('@/lib/services/pmStoryService', () => ({
  pmStoryService: {
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

import { pmStoryService } from '@/lib/services/pmStoryService'
import { usePMBoardStore } from '../pmBoardStore'

const mockedStoryService = vi.mocked(pmStoryService)

describe('usePMBoardStore.moveMemberStory', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockedStoryService.reorder.mockResolvedValue({
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

    await usePMBoardStore.getState().moveMemberStory({
      workspaceId: 'ws-1',
      storyId: 'story-1',
      fromMemberId: 'member-1',
      toMemberId: 'member-1',
      toIndex: 1,
    })

    expect(mockedStoryService.update).not.toHaveBeenCalled()
    expect(mockedStoryService.reorder).not.toHaveBeenCalled()
    expect(usePMBoardStore.getState().memberColumns).toEqual(before)
  })

  it('persists cross-member moves as reassignment only', async () => {
    mockedStoryService.update.mockResolvedValue({
      data: { story: { id: 'story-1', owner_member_id: 'member-2' } },
      error: null,
      status: 200,
    } as never)

    await usePMBoardStore.getState().moveMemberStory({
      workspaceId: 'ws-1',
      storyId: 'story-1',
      fromMemberId: 'member-1',
      toMemberId: 'member-2',
      toIndex: 0,
    })

    expect(mockedStoryService.update).toHaveBeenCalledWith('ws-1', 'story-1', {
      owner_member_id: 'member-2',
    })
    expect(mockedStoryService.reorder).not.toHaveBeenCalled()
    expect(usePMBoardStore.getState().memberColumns[0]?.stories.map((story) => story.id)).toEqual(['story-2'])
    expect(usePMBoardStore.getState().memberColumns[1]?.stories.map((story) => story.id)).toEqual(['story-1'])
  })
})
