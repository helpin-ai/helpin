import type { SprintWithStats } from '@/lib/pmTypes';

export function isEpicSelectableForTaskTeam(
  epicTeamId?: string | null,
  taskTeamId?: string | null,
) {
  if (!epicTeamId) {
    return true;
  }
  if (!taskTeamId) {
    return false;
  }
  return epicTeamId === taskTeamId;
}

export function isSprintSelectableForTaskTeam(
  sprintTeamId?: string | null,
  taskTeamId?: string | null,
) {
  if (!sprintTeamId) {
    return !taskTeamId;
  }
  if (!taskTeamId) {
    return false;
  }
  return sprintTeamId === taskTeamId;
}

export function getVisibleSprintsForTaskScope(
  sprints: SprintWithStats[],
  {
    taskTeamId,
    listTeamId,
  }: {
    taskTeamId?: string | null;
    listTeamId?: string | null;
  },
) {
  if (listTeamId === undefined) {
    return sprints;
  }

  const effectiveTeamId = taskTeamId ?? listTeamId ?? null;

  return sprints.filter((entry) =>
    isSprintSelectableForTaskTeam(entry.sprint.team_id ?? null, effectiveTeamId),
  );
}
