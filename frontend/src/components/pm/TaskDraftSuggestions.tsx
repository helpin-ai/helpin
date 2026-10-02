import { useEffect, useState } from 'react';
import { QuietSection } from '@/components/design-system/quiet';
import { pmTriageService, type TriageReview, type TriageView } from '@/lib/services/pmTriageService';
import { unwrapRequired } from '@/lib/queryUtils';
import { getVisibleTriageSuggestions } from './triageSuggestionRows';
import { TriageSuggestions } from './TriageSuggestions';

export function TaskDraftSuggestions({ workspaceId, workspaceSlug, name, description, teamId, taskType, disabled, onPending, onApply, onOpenTask }: {
  workspaceId: string;
  workspaceSlug: string;
  name: string;
  description: string;
  teamId: string;
  taskType: string;
  disabled: boolean;
  onPending: (pending: boolean) => void;
  onApply: (field: 'task_type' | 'team', value: string) => void;
  onOpenTask: (id: string) => void;
}) {
  const [draftId] = useState(newDraftID);
  const [view, setView] = useState<TriageView | null>(null);
  const [checking, setChecking] = useState(false);
  useEffect(() => {
    if (!name.trim() || !teamId) { setView(null); onPending(false); return; }
    let cancelled = false;
    setView(null); setChecking(true); onPending(true);
    const timer = window.setTimeout(async () => {
      try {
        const result = unwrapRequired(await pmTriageService.analyzeDraft(workspaceId, { draft_id: draftId, name, description, team_id: teamId }), 'Task matching');
        if (!cancelled) setView(result);
      } catch {
        if (!cancelled) setView(null);
      } finally {
        if (!cancelled) { setChecking(false); onPending(false); }
      }
    }, 700);
    return () => { cancelled = true; window.clearTimeout(timer); onPending(false); };
  }, [workspaceId, draftId, name, description, teamId, onPending]);
  if (!name.trim() || !teamId || checking || view?.status !== 'ready' || getVisibleTriageSuggestions(view, taskType, teamId).length === 0) return null;
  const review = (request: TriageReview) => {
    if (request.dismiss) { setView((current) => current ? { ...current, reviewed: { ...current.reviewed, [`${request.action}:${request.value}`]: 'dismissed' } } : current); return; }
    if (request.action === 'match') onOpenTask(request.value);
    else onApply(request.action, request.value);
  };
  return <QuietSection title="Existing work and suggestions">
    <p className="mb-2 text-xs text-quiet-text-tertiary">Based on <a href="#task-title" className="underline underline-offset-4">this draft’s title and description</a>.</p>
    {view?.status === 'ready' ? <TriageSuggestions view={view} currentType={taskType} currentTeam={teamId} disabled={disabled || checking} taskHref={(id) => `/w/${encodeURIComponent(workspaceSlug)}/pm/tasks/${encodeURIComponent(id)}`} onReview={review} /> : null}
  </QuietSection>;
}

function newDraftID(): string {
  if (typeof crypto.randomUUID === 'function') return crypto.randomUUID();
  const bytes = crypto.getRandomValues(new Uint8Array(16));
  bytes[6] = (bytes[6] & 0x0f) | 0x40;
  bytes[8] = (bytes[8] & 0x3f) | 0x80;
  const hex = Array.from(bytes, (byte) => byte.toString(16).padStart(2, '0')).join('');
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`;
}
