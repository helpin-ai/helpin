import { describe, expect, it } from 'vitest';

import {
  buildRoadmapQuarterSegments,
  getRoadmapEpicRange,
  getScheduledRoadmapEpics,
  roadmapEpicMatchesSearch,
} from '@/components/pm/roadmapUtils';
import type { RoadmapEpic } from '@/lib/pmTypes';

function roadmapEpic({
  name = 'Launch onboarding',
  teamId = 'team-1',
  start,
  target,
  objective = 'Increase activation',
}: {
  name?: string;
  teamId?: string;
  start?: string;
  target?: string;
  objective?: string;
}): RoadmapEpic {
  return {
    epic: {
      id: name,
      name,
      team_id: teamId,
      planned_start_date: start,
      deadline: target,
    },
    objectives: objective ? [{ id: 'objective-1', name: objective }] : [],
    labels: [],
    stats: {},
  } as RoadmapEpic;
}

describe('roadmap planning helpers', () => {
  it('schedules only epics with a valid chronological date range', () => {
    const scheduled = roadmapEpic({ start: '2026-09-01', target: '2026-09-30' });
    const partial = roadmapEpic({ name: 'Partial', start: '2026-09-01' });
    const reversed = roadmapEpic({ name: 'Reversed', start: '2026-10-01', target: '2026-09-01' });
    const invalid = roadmapEpic({ name: 'Invalid', start: 'not-a-date', target: '2026-09-01' });

    expect(getRoadmapEpicRange(scheduled)).toMatchObject({
      start: new Date(2026, 8, 1),
      target: new Date(2026, 8, 30),
    });
    expect(getRoadmapEpicRange(partial)).toBeNull();
    expect(getRoadmapEpicRange(reversed)).toBeNull();
    expect(getRoadmapEpicRange(invalid)).toBeNull();
    expect(getScheduledRoadmapEpics([scheduled, partial, reversed, invalid])).toEqual([scheduled]);
  });

  it('matches roadmap search across epic, objective, and team names', () => {
    const entry = roadmapEpic({});
    const teamNames = new Map([['team-1', 'Growth platform']]);

    expect(roadmapEpicMatchesSearch(entry, 'onboard', teamNames)).toBe(true);
    expect(roadmapEpicMatchesSearch(entry, 'ACTIVATION', teamNames)).toBe(true);
    expect(roadmapEpicMatchesSearch(entry, 'growth', teamNames)).toBe(true);
    expect(roadmapEpicMatchesSearch(entry, 'billing', teamNames)).toBe(false);
  });

  it('groups consecutive months under real quarter headers', () => {
    const months = [
      new Date(2026, 6, 1),
      new Date(2026, 7, 1),
      new Date(2026, 8, 1),
      new Date(2026, 9, 1),
      new Date(2026, 10, 1),
      new Date(2026, 11, 1),
      new Date(2027, 0, 1),
    ];

    expect(buildRoadmapQuarterSegments(months).map((segment) => ({
      label: segment.label,
      monthCount: segment.months.length,
    }))).toEqual([
      { label: 'Q3 2026', monthCount: 3 },
      { label: 'Q4 2026', monthCount: 3 },
      { label: 'Q1 2027', monthCount: 1 },
    ]);
  });
});
