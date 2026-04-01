import { describe, expect, it } from 'vitest';
import {
  isEpicSelectableForStoryTeam,
  isSprintSelectableForStoryTeam,
} from '@/components/pm/story-detail/storyPlanningScope';

describe('story planning scope', () => {
  it('allows shared epics for any story team', () => {
    expect(isEpicSelectableForStoryTeam(null, 'team-a')).toBe(true);
    expect(isEpicSelectableForStoryTeam(undefined, '')).toBe(true);
  });

  it('allows only same-team epics when the epic is team-scoped', () => {
    expect(isEpicSelectableForStoryTeam('team-a', 'team-a')).toBe(true);
    expect(isEpicSelectableForStoryTeam('team-b', 'team-a')).toBe(false);
    expect(isEpicSelectableForStoryTeam('team-b', '')).toBe(false);
  });

  it('allows only same-team sprints and blocks shared sprints for team stories', () => {
    expect(isSprintSelectableForStoryTeam('team-a', 'team-a')).toBe(true);
    expect(isSprintSelectableForStoryTeam('team-b', 'team-a')).toBe(false);
    expect(isSprintSelectableForStoryTeam(null, 'team-a')).toBe(false);
  });

  it('allows shared sprints only when the story has no team', () => {
    expect(isSprintSelectableForStoryTeam(null, '')).toBe(true);
    expect(isSprintSelectableForStoryTeam(undefined, null)).toBe(true);
  });
});
