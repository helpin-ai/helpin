import { describe, expect, it } from 'vitest';

import { buildSprintOptionGroups } from '@/lib/pmSprintOptions';
import type { SprintWithStats } from '@/lib/pmTypes';
import type { WorkspaceTeam } from '@/lib/types';

const teams: WorkspaceTeam[] = [
  {
    id: 'team-eng',
    workspace_id: 'ws-1',
    name: 'Engineering',
    handle: 'eng',
    description: '',
    manager_id: '',
    team_type: 'product',
    created_at: '2026-03-27T00:00:00Z',
    updated_at: '2026-03-27T00:00:00Z',
    default_story_type: 'feature',
  },
  {
    id: 'team-mkt',
    workspace_id: 'ws-1',
    name: 'Marketing',
    handle: 'mkt',
    description: '',
    manager_id: '',
    team_type: 'general',
    created_at: '2026-03-27T00:00:00Z',
    updated_at: '2026-03-27T00:00:00Z',
    default_story_type: 'feature',
  },
];

const sprints: SprintWithStats[] = [
  {
    sprint: {
      id: 'sprint-eng',
      workspace_id: 'ws-1',
      name: 'Engineering Sprint',
      goal: '',
      status: 'started',
      start_date: '2026-03-27T00:00:00Z',
      end_date: '2026-04-03T00:00:00Z',
      team_id: 'team-eng',
      archived: false,
      created_at: '2026-03-27T00:00:00Z',
      updated_at: '2026-03-27T00:00:00Z',
    },
    labels: [],
    stats: { story_count: 0, done_story_count: 0, total_points: 0, done_points: 0 },
  },
  {
    sprint: {
      id: 'sprint-mkt',
      workspace_id: 'ws-1',
      name: 'Marketing Sprint',
      goal: '',
      status: 'started',
      start_date: '2026-03-27T00:00:00Z',
      end_date: '2026-04-03T00:00:00Z',
      team_id: 'team-mkt',
      archived: false,
      created_at: '2026-03-27T00:00:00Z',
      updated_at: '2026-03-27T00:00:00Z',
    },
    labels: [],
    stats: { story_count: 0, done_story_count: 0, total_points: 0, done_points: 0 },
  },
  {
    sprint: {
      id: 'sprint-ws',
      workspace_id: 'ws-1',
      name: 'Workspace Sprint',
      goal: '',
      status: 'started',
      start_date: '2026-03-27T00:00:00Z',
      end_date: '2026-04-03T00:00:00Z',
      team_id: undefined,
      archived: false,
      created_at: '2026-03-27T00:00:00Z',
      updated_at: '2026-03-27T00:00:00Z',
    },
    labels: [],
    stats: { story_count: 0, done_story_count: 0, total_points: 0, done_points: 0 },
  },
];

describe('pmSprintOptions', () => {
  it('returns only the selected team sprints when a team is selected', () => {
    expect(buildSprintOptionGroups(sprints, teams, 'team-eng')).toEqual([
      {
        key: 'team-eng',
        label: 'Engineering',
        options: [{ value: 'sprint-eng', label: 'Engineering Sprint' }],
      },
    ]);
  });

  it('groups sprints by team when no team is selected', () => {
    expect(buildSprintOptionGroups(sprints, teams, null)).toEqual([
      {
        key: 'team-eng',
        label: 'Engineering',
        options: [{ value: 'sprint-eng', label: 'Engineering Sprint' }],
      },
      {
        key: 'team-mkt',
        label: 'Marketing',
        options: [{ value: 'sprint-mkt', label: 'Marketing Sprint' }],
      },
      {
        key: '__workspace__',
        label: 'Workspace',
        options: [{ value: 'sprint-ws', label: 'Workspace Sprint' }],
      },
    ]);
  });
});
