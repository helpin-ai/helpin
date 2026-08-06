export function getLinkTasksDisabledReason(canEdit: boolean, teamId: string): string | null {
  if (!canEdit) return 'You do not have permission to edit this epic.'
  if (!teamId) return 'Assign this epic to a team before linking tasks.'
  return null
}
