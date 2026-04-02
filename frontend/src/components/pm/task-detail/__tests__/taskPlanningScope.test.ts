import { describe, expect, it } from 'vitest';
import {
  isEpicSelectableForTaskTeam,
  isSprintSelectableForTaskTeam,
} from '@/components/pm/task-detail/taskPlanningScope';

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
});
