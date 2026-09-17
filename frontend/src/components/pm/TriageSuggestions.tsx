import { QuietTextAction } from '@/components/design-system/quiet';
import type { TriageReview, TriageView } from '@/lib/services/pmTriageService';

interface Props {
  view: TriageView;
  disabled?: boolean;
  currentType?: string;
  currentTeam?: string;
  taskHref: (id: string) => string;
  onReview: (request: TriageReview) => void;
}

/** Shared review rows for tasks and support feedback. No mutation happens on render. */
export function TriageSuggestions({ view, disabled, currentType, currentTeam, taskHref, onReview }: Props) {
  const assessment = view.assessment;
  if (!assessment || !view.id) return null;
  const rows: { action: TriageReview['action']; value: string; label: string; href?: string; apply: string }[] = [];
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
    rows.push({ action: 'match', value: candidate.id, label: `${relationship}: #${candidate.display_id} ${candidate.name}`, href: taskHref(candidate.id), apply: view.source_kind === 'task_draft' ? 'Use existing task' : view.source_kind === 'task' ? 'Add relationship' : 'Link conversation' });
  }
  const visible = rows.filter((row) => !view.reviewed?.[`${row.action}:${row.value}`]);
  return (
    <div className="min-w-0 text-sm text-quiet-text-secondary">
      {visible.length === 0 ? (
        <p>{assessment.actionable ? 'No new suggestions to review.' : 'There is not enough evidence of concrete product work yet.'}</p>
      ) : (
        <ul className="divide-y divide-quiet-divider-light">
          {visible.map((row) => (
            <li key={`${row.action}:${row.value}`} className="flex min-w-0 flex-wrap items-center justify-between gap-x-5 gap-y-2 py-3">
              <div className="min-w-0 flex-1 basis-52 break-words">
                {row.href ? <a href={row.href} className="text-quiet-accent underline underline-offset-4 focus-visible:outline-auto">{row.label}</a> : row.label}
              </div>
              <div className="flex shrink-0 gap-4">
                <QuietTextAction className="focus-visible:underline" disabled={disabled} onClick={() => onReview({ assessment_id: view.id!, action: row.action, value: row.value, dismiss: false })}>{row.apply}</QuietTextAction>
                <QuietTextAction className="focus-visible:underline" aria-label={`Dismiss ${row.label}`} disabled={disabled} onClick={() => onReview({ assessment_id: view.id!, action: row.action, value: row.value, dismiss: true })}>Dismiss</QuietTextAction>
              </div>
            </li>
          ))}
        </ul>
      )}
      <p className="mt-2 text-xs text-quiet-text-tertiary">Compared with {assessment.candidates_checked} accessible {assessment.candidates_checked === 1 ? 'task' : 'tasks'}. Other matches may exist.</p>
    </div>
  );
}
