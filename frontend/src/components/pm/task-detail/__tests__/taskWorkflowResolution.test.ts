import { describe, expect, it } from 'vitest';
import type { WorkflowWithStates } from '@/lib/pmTypes';
import {
  resolveTaskTeamWorkflow,
  resolveTaskWorkflowStates,
} from '../taskWorkflowResolution';

function workflow(id: string, stateId: string, teamId?: string): WorkflowWithStates {
  return {
    workflow: {
      id,
      workspace_id: 'ws-1',
      name: id,
      description: '',
      team_id: teamId,
      auto_assign_owner: false,
      created_at: '2026-01-01T00:00:00Z',
      updated_at: '2026-01-01T00:00:00Z',
    },
    states: [
      {
        id: stateId,
        workflow_id: id,
        name: `${id} status`,
        state_type: 'unstarted',
        position: 1,
        is_default: true,
        created_at: '2026-01-01T00:00:00Z',
        updated_at: '2026-01-01T00:00:00Z',
      },
    ],
  };
}

describe('task workflow resolution', () => {
  it('uses the task workflow states instead of stale fallback states', () => {
    const defaultWorkflow = workflow('default-workflow', 'default-state');
    const teamWorkflow = workflow('team-workflow', 'team-state', 'team-a');

    expect(resolveTaskWorkflowStates([defaultWorkflow, teamWorkflow], defaultWorkflow.states, 'team-workflow')).toEqual(teamWorkflow.states);
  });

  it('uses the team workflow when changing a task to a team with custom states', () => {
    const defaultWorkflow = workflow('default-workflow', 'default-state');
    const teamWorkflow = workflow('team-workflow', 'team-state', 'team-a');

    expect(resolveTaskTeamWorkflow([defaultWorkflow, teamWorkflow], 'team-a')?.workflow.id).toBe('team-workflow');
  });
});
