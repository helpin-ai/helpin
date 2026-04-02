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
