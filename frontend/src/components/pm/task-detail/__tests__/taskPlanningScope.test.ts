import { describe, expect, it } from 'vitest';
import {
  getVisibleSprintsForTaskScope,
  isEpicSelectableForTaskTeam,
  isSprintSelectableForTaskTeam,
} from '@/components/pm/task-detail/taskPlanningScope';
import type { SprintWithStats } from '@/lib/pmTypes';

const sprints: SprintWithStats[] = [
  {
    sprint: {
      id: 'sprint-team-a',
      workspace_id: 'ws-1',
      name: 'Team A Sprint',
      goal: '',
      status: 'planned',
      start_date: '2026-04-01T00:00:00Z',
      end_date: '2026-04-15T00:00:00Z',
      team_id: 'team-a',
      archived: false,
      created_at: '2026-04-01T00:00:00Z',
      updated_at: '2026-04-01T00:00:00Z',
    },
    labels: [],
    stats: { task_count: 0, done_task_count: 0, total_points: 0, done_points: 0 },
  },
  {
    sprint: {
      id: 'sprint-team-b',
      workspace_id: 'ws-1',
      name: 'Team B Sprint',
      goal: '',
      status: 'planned',
      start_date: '2026-04-01T00:00:00Z',
      end_date: '2026-04-15T00:00:00Z',
      team_id: 'team-b',
      archived: false,
      created_at: '2026-04-01T00:00:00Z',
      updated_at: '2026-04-01T00:00:00Z',
    },
    labels: [],
    stats: { task_count: 0, done_task_count: 0, total_points: 0, done_points: 0 },
  },
  {
    sprint: {
      id: 'sprint-workspace',
      workspace_id: 'ws-1',
      name: 'Workspace Sprint',
      goal: '',
      status: 'planned',
      start_date: '2026-04-01T00:00:00Z',
      end_date: '2026-04-15T00:00:00Z',
      team_id: undefined,
      archived: false,
      created_at: '2026-04-01T00:00:00Z',
      updated_at: '2026-04-01T00:00:00Z',
    },
    labels: [],
    stats: { task_count: 0, done_task_count: 0, total_points: 0, done_points: 0 },
  },
];

describe('task planning scope', () => {
  it('allows shared epics for any task team', () => {
    expect(isEpicSelectableForTaskTeam(null, 'team-a')).toBe(true);
    expect(isEpicSelectableForTaskTeam(undefined, '')).toBe(true);
  });

  it('allows only same-team epics when the epic is team-scoped', () => {
    expect(isEpicSelectableForTaskTeam('team-a', 'team-a')).toBe(true);
    expect(isEpicSelectableForTaskTeam('team-b', 'team-a')).toBe(false);
    expect(isEpicSelectableForTaskTeam('team-b', '')).toBe(false);
  });

  it('allows only same-team sprints and blocks shared sprints for team tasks', () => {
    expect(isSprintSelectableForTaskTeam('team-a', 'team-a')).toBe(true);
    expect(isSprintSelectableForTaskTeam('team-b', 'team-a')).toBe(false);
    expect(isSprintSelectableForTaskTeam(null, 'team-a')).toBe(false);
  });

  it('allows shared sprints only when the task has no team', () => {
    expect(isSprintSelectableForTaskTeam(null, '')).toBe(true);
    expect(isSprintSelectableForTaskTeam(undefined, null)).toBe(true);
  });

  it('keeps all sprints visible when the task list is not team-scoped', () => {
    expect(getVisibleSprintsForTaskScope(sprints, { taskTeamId: 'team-a' }).map((entry) => entry.sprint.id)).toEqual([
      'sprint-team-a',
      'sprint-team-b',
      'sprint-workspace',
    ]);
  });

  it('filters sprint options by task team and falls back to the scoped list team', () => {
    expect(
      getVisibleSprintsForTaskScope(sprints, {
        taskTeamId: 'team-a',
        listTeamId: 'team-b',
      }).map((entry) => entry.sprint.id),
    ).toEqual(['sprint-team-a']);

    expect(
      getVisibleSprintsForTaskScope(sprints, {
        listTeamId: null,
      }).map((entry) => entry.sprint.id),
    ).toEqual(['sprint-workspace']);
  });
});
