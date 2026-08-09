import type { SprintWithStats } from '@/lib/pmTypes';
import { isSprintOpenForPlanning } from '@/lib/pmSprintOptions';

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
    currentSprintId,
  }: {
    taskTeamId?: string | null;
    listTeamId?: string | null;
    currentSprintId?: string | null;
  },
) {
  const selectableSprints = sprints.filter((entry) =>
    isSprintOpenForPlanning(entry.sprint.status) || entry.sprint.id === currentSprintId,
  );
  if (listTeamId === undefined) {
    return selectableSprints;
  }

  const effectiveTeamId = taskTeamId ?? listTeamId ?? null;

  return selectableSprints.filter((entry) =>
    isSprintSelectableForTaskTeam(entry.sprint.team_id ?? null, effectiveTeamId),
  );
}
