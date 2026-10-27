/** toggleHelpSpaceId adds or removes one space from a selection. */
export function toggleHelpSpaceId(current: string[], spaceId: string, enabled: boolean) {
  if (enabled) return current.includes(spaceId) ? current : [...current, spaceId];
  return current.filter((id) => id !== spaceId);
}
