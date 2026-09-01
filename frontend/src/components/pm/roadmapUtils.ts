import { isValid, parseISO } from 'date-fns';

import type { RoadmapEpic } from '@/lib/pmTypes';

export function getRoadmapEpicRange(epic: RoadmapEpic) {
  if (!epic.epic.planned_start_date || !epic.epic.deadline) return null;
  const start = parseISO(epic.epic.planned_start_date);
  const target = parseISO(epic.epic.deadline);
  if (!isValid(start) || !isValid(target) || target < start) return null;
  return { start, target };
}

export function getScheduledRoadmapEpics(epics: RoadmapEpic[]) {
  return epics.filter((epic) => getRoadmapEpicRange(epic) !== null);
}

export interface RoadmapQuarterSegment {
  key: string;
  label: string;
  months: Date[];
}

export function buildRoadmapQuarterSegments(months: Date[]): RoadmapQuarterSegment[] {
  const segments: RoadmapQuarterSegment[] = [];
  for (const month of months) {
    const quarter = Math.floor(month.getMonth() / 3) + 1;
    const year = month.getFullYear();
    const key = `${year}-q${quarter}`;
    const current = segments[segments.length - 1];
    if (current?.key === key) {
      current.months.push(month);
    } else {
      segments.push({ key, label: `Q${quarter} ${year}`, months: [month] });
    }
  }
  return segments;
}

export function roadmapEpicMatchesSearch(
  entry: RoadmapEpic,
  search: string,
  teamNameMap: Map<string, string>,
) {
  const query = search.trim().toLocaleLowerCase();
  if (!query) return true;
  const teamName = entry.epic.team_id ? teamNameMap.get(entry.epic.team_id) : '';
  return [entry.epic.name, teamName, ...entry.objectives.map((objective) => objective.name)]
    .some((value) => value?.toLocaleLowerCase().includes(query));
}
