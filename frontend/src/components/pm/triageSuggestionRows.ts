import type { TriageReview, TriageView } from '@/lib/services/pmTriageService';

export function getVisibleTriageSuggestions(view: TriageView, currentType?: string, currentTeam?: string) {
  const assessment = view.assessment;
  if (!assessment || !view.id) return [];
  const rows: { action: TriageReview['action']; value: string; label: string; apply: string }[] = [];
  const type = assessment.task_type?.id;
  const team = assessment.team?.id;
  if (view.source_kind !== 'support_conversation' && type && type !== currentType) {
    rows.push({ action: 'task_type', value: type, label: `Type: ${type}`, apply: 'Apply type' });
  }
  if (view.source_kind !== 'support_conversation' && team && team !== currentTeam) {
    const option = view.teams.find((item) => item.id === team);
    if (option) rows.push({ action: 'team', value: team, label: `Team: ${option.name}`, apply: 'Change team' });
  }
  for (const match of assessment.matches) {
    const candidate = view.candidates.find((item) => item.id === match.task_id);
    if (!candidate) continue;
    const relationship = match.relationship === 'duplicates' ? 'Possible duplicate' : 'Related work';
    rows.push({ action: 'match', value: candidate.id, label: `${relationship}: #${candidate.display_id} ${candidate.name}`, apply: view.source_kind === 'task_draft' ? 'Use existing task' : view.source_kind === 'task' ? 'Add relationship' : 'Link conversation' });
  }
  return rows.filter((row) => !view.reviewed?.[`${row.action}:${row.value}`]);
}
