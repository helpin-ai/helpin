export function isEpicSelectableForStoryTeam(
  epicTeamId?: string | null,
  storyTeamId?: string | null,
) {
  if (!epicTeamId) {
    return true;
  }
  if (!storyTeamId) {
    return false;
  }
  return epicTeamId === storyTeamId;
}

export function isSprintSelectableForStoryTeam(
  sprintTeamId?: string | null,
  storyTeamId?: string | null,
) {
  if (!sprintTeamId) {
    return !storyTeamId;
  }
  if (!storyTeamId) {
    return false;
  }
  return sprintTeamId === storyTeamId;
}
