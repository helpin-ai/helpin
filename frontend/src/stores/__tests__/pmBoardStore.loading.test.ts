import { beforeEach, describe, expect, it, vi } from 'vitest'

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

import type { WorkflowWithStates } from '@/lib/pmTypes'
import { pmTaskService } from '@/lib/services/pmTaskService'
import { pmWorkflowService } from '@/lib/services/pmWorkflowService'
import { usePMBoardStore } from '../pmBoardStore'

const mockedTaskService = vi.mocked(pmTaskService)
const mockedWorkflowService = vi.mocked(pmWorkflowService)

const workflow = (id: string, teamId?: string): WorkflowWithStates => ({
  workflow: {
    id,
    workspace_id: 'ws-1',
    name: id,
    team_id: teamId,
    auto_assign_owner: false,
    created_at: '2026-09-03T00:00:00Z',
    updated_at: '2026-09-03T00:00:00Z',
  },
  states: [],
})

const ok = <T,>(data: T) => ({ data, error: null, status: 200 })

describe('usePMBoardStore workflow discovery', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockedTaskService.listBoard.mockResolvedValue(ok([]) as never)
    usePMBoardStore.setState({
      workspaceId: null,
      workflows: [],
      workflow: null,
      columns: [],
      loading: false,
      error: null,
      teamId: null,
      filters: {},
    })
  })

  it('loadBoard reuses a listed workflow for the selected team', async () => {
    const matching = workflow('workflow-team-a', 'team-a')
    mockedWorkflowService.list.mockResolvedValue(ok([
      workflow('workflow-default'),
      matching,
    ]) as never)
    usePMBoardStore.setState({ teamId: 'team-a' })

    await usePMBoardStore.getState().loadBoard('ws-1')

    expect(mockedWorkflowService.resolveTeamWorkflow).not.toHaveBeenCalled()
    expect(mockedTaskService.listBoard).toHaveBeenCalledWith('ws-1', matching.workflow.id, { team_id: 'team-a' }, 25)
    expect(usePMBoardStore.getState().workflow).toEqual(matching)
  })

  it('loadBoard resolves a team workflow only when the list has no match', async () => {
    const resolved = workflow('workflow-resolved', 'team-a')
    mockedWorkflowService.list.mockResolvedValue(ok([workflow('workflow-default')]) as never)
    mockedWorkflowService.resolveTeamWorkflow.mockResolvedValue(ok(resolved) as never)
    usePMBoardStore.setState({ teamId: 'team-a' })

    await usePMBoardStore.getState().loadBoard('ws-1')

    expect(mockedWorkflowService.resolveTeamWorkflow).toHaveBeenCalledWith('ws-1', 'team-a')
    expect(mockedTaskService.listBoard).toHaveBeenCalledWith('ws-1', resolved.workflow.id, { team_id: 'team-a' }, 25)
  })

  it('setTeamFilter reuses an already loaded workflow for the selected team', async () => {
    const matching = workflow('workflow-team-b', 'team-b')
    usePMBoardStore.setState({
      workspaceId: 'ws-1',
      workflows: [workflow('workflow-default'), matching],
    })

    await usePMBoardStore.getState().setTeamFilter('team-b')

    expect(mockedWorkflowService.resolveTeamWorkflow).not.toHaveBeenCalled()
    expect(mockedTaskService.listBoard).toHaveBeenCalledWith('ws-1', matching.workflow.id, { team_id: 'team-b' }, 25)
    expect(usePMBoardStore.getState().workflow).toEqual(matching)
  })

  it('setTeamFilter resolves a workflow when the loaded list has no team match', async () => {
    const resolved = workflow('workflow-team-b-resolved', 'team-b')
    mockedWorkflowService.resolveTeamWorkflow.mockResolvedValue(ok(resolved) as never)
    usePMBoardStore.setState({
      workspaceId: 'ws-1',
      workflows: [workflow('workflow-default')],
    })

    await usePMBoardStore.getState().setTeamFilter('team-b')

    expect(mockedWorkflowService.resolveTeamWorkflow).toHaveBeenCalledWith('ws-1', 'team-b')
    expect(mockedTaskService.listBoard).toHaveBeenCalledWith('ws-1', resolved.workflow.id, { team_id: 'team-b' }, 25)
  })
})
