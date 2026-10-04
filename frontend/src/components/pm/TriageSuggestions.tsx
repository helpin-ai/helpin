import { getVisibleTriageSuggestions } from './triageSuggestionRows';
import { QuietTextAction } from '@/components/design-system/quiet';
import type { TriageReview, TriageView } from '@/lib/services/pmTriageService';

interface Props {
  view: TriageView;
  disabled?: boolean;
  showComparisonSummary?: boolean;
  currentType?: string;
  currentTeam?: string;
  taskHref: (id: string) => string;
  onReview: (request: TriageReview) => void;
}

/** Shared review rows for tasks and support feedback. No mutation happens on render. */
export function TriageSuggestions({ view, disabled, currentType, currentTeam, taskHref, onReview, showComparisonSummary = true }: Props) {
  const assessment = view.assessment;
  if (!assessment || !view.id) return null;
  const visible = getVisibleTriageSuggestions(view, currentType, currentTeam);
  return (
    <div className="min-w-0 text-sm text-quiet-text-secondary">
      {visible.length === 0 ? (
        <p>No suggestions to review.</p>
      ) : (
        <ul className="divide-y divide-quiet-divider-light">
          {visible.map((row) => (
            <li key={`${row.action}:${row.value}`} className="flex min-w-0 flex-wrap items-center justify-between gap-x-5 gap-y-2 py-3">
              <div className="min-w-0 flex-1 basis-52 break-words">
                {row.action === 'match' ? <a href={taskHref(row.value)} className="text-quiet-accent underline underline-offset-4 focus-visible:outline-auto">{row.label}</a> : row.label}
              </div>
              <div className="flex shrink-0 gap-4">
                <QuietTextAction className="focus-visible:underline" disabled={disabled} onClick={() => onReview({ assessment_id: view.id!, action: row.action, value: row.value, dismiss: false })}>{row.apply}</QuietTextAction>
                <QuietTextAction className="focus-visible:underline" aria-label={`Dismiss ${row.label}`} disabled={disabled} onClick={() => onReview({ assessment_id: view.id!, action: row.action, value: row.value, dismiss: true })}>Dismiss</QuietTextAction>
              </div>
            </li>
          ))}
        </ul>
      )}
      {showComparisonSummary && <p className="mt-2 text-xs text-quiet-text-tertiary">Compared with {assessment.candidates_checked} accessible {assessment.candidates_checked === 1 ? 'task' : 'tasks'}. Other matches may exist.</p>}
    </div>
  );
}
