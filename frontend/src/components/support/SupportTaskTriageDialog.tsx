import { useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter } from '@/components/ui/dialog';
import { QuietPrimaryAction, QuietTextAction, QuietUnderlineInput, QuietSection } from '@/components/design-system/quiet';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { TiptapEditor } from '@/components/ui/tiptap-editor';
import { TriageSuggestions } from '@/components/pm/TriageSuggestions';
import { pmTriageService, type SupportTaskDraft, type TriageReview, type TriageView } from '@/lib/services/pmTriageService';
import { unwrapRequired } from '@/lib/queryUtils';
import { queryKeys } from '@/lib/queryKeys';

export function SupportTaskTriageDialog({ workspaceId, workspaceSlug, conversationId, initialView, initialError, teams, defaultTeamId, isCreating, onOpenChange, onCreate, onLinked }: {
  workspaceId: string;
  workspaceSlug: string;
  conversationId: string;
  initialView: TriageView | null;
  initialError?: string;
  teams: { id: string; name: string }[];
  defaultTeamId?: string;
  isCreating: boolean;
  onOpenChange: (open: boolean) => void;
  onCreate: (teamId: string, draft: SupportTaskDraft) => Promise<void>;
  onLinked: () => void;
}) {
  const queryClient = useQueryClient();
  const [view, setView] = useState(initialView);
  const [error, setError] = useState(initialError ?? '');
  const [busy, setBusy] = useState(false);
  const [draft, setDraft] = useState<SupportTaskDraft | null>(null);
  const availableTeams = view?.teams.length ? view.teams : teams;
  const suggestedTeam = view?.assessment?.team?.id;
  const [teamId, setTeamId] = useState(() => [defaultTeamId, suggestedTeam, availableTeams[0]?.id].find((id) => availableTeams.some((team) => team.id === id)) ?? '');
  const disabled = busy || isCreating;
  const taskHref = (id: string) => `/w/${encodeURIComponent(workspaceSlug)}/pm/tasks/${encodeURIComponent(id)}`;
  const run = async (action: () => Promise<void>) => {
    setBusy(true); setError('');
    try { await action(); } catch (cause) { setError(cause instanceof Error ? cause.message : 'Unable to finish this action. Please try again.'); }
    finally { setBusy(false); }
  };
  const review = (request: TriageReview) => void run(async () => {
    const result = unwrapRequired(await pmTriageService.review(workspaceId, 'support_conversation', conversationId, request), 'Suggestion review');
    setView((current) => current ? { ...current, reviewed: { ...current.reviewed, [result.key]: result.status } } : current);
    if (!request.dismiss) {
      await queryClient.invalidateQueries({ queryKey: queryKeys.support.conversation(workspaceId, conversationId) });
      await queryClient.invalidateQueries({ queryKey: queryKeys.support.conversationAssociations(workspaceId, conversationId) });
      onLinked();
    }
  });
  return (
    <Dialog open onOpenChange={(open) => { if (!disabled) onOpenChange(open); }}>
      <DialogContent className="max-h-[90dvh] overflow-y-auto sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>{draft ? 'Review new task' : 'Find existing product work'}</DialogTitle>
          <DialogDescription>Based on <a className="underline underline-offset-4" href={`/w/${encodeURIComponent(workspaceSlug)}/support/${encodeURIComponent(conversationId)}`}>this conversation’s public messages</a>.</DialogDescription>
        </DialogHeader>
        {error ? <p role="alert" className="text-sm text-destructive">{error}</p> : null}
        {busy ? <p role="status" className="text-sm text-quiet-text-secondary">Preparing changes…</p> : null}
        {!draft ? (
          <QuietSection className="px-0 py-0 pb-4 sm:px-0 lg:px-0">
            {view?.status === 'ready' ? <TriageSuggestions view={view} disabled={disabled} taskHref={taskHref} onReview={review} /> : <p className="text-sm text-quiet-text-secondary">Matching is unavailable right now. You can still review a new task draft.</p>}
            <QuietTextAction className="mt-3 focus-visible:underline" disabled={disabled} onClick={() => void run(async () => setView(unwrapRequired(await pmTriageService.analyze(workspaceId, 'support_conversation', conversationId), 'Task suggestions')))}>Refresh matches</QuietTextAction>
          </QuietSection>
        ) : (
          <div className="space-y-4">
            <div><label htmlFor="support-task-title" className="text-sm text-quiet-text-secondary">Title</label><QuietUnderlineInput id="support-task-title" value={draft.name} disabled={disabled} onChange={(event) => setDraft({ ...draft, name: event.target.value })} /></div>
            <div><span id="support-task-description-label" className="text-sm text-quiet-text-secondary">Description</span><div role="group" aria-labelledby="support-task-description-label" inert={disabled}><TiptapEditor content={draft.description} onChange={(description) => setDraft({ ...draft, description })} variant="divider" /></div></div>
            <div className="grid gap-4 sm:grid-cols-3">
              <div><label htmlFor="support-task-team" className="text-sm text-quiet-text-secondary">Team</label><Select value={teamId} onValueChange={setTeamId} disabled={disabled}><SelectTrigger id="support-task-team" variant="underline" className="w-full px-0.5"><SelectValue placeholder="Select team" /></SelectTrigger><SelectContent>{availableTeams.map((team) => <SelectItem key={team.id} value={team.id}>{team.name}</SelectItem>)}</SelectContent></Select></div>
              <div><label htmlFor="support-task-type" className="text-sm text-quiet-text-secondary">Type</label><Select value={draft.task_type} onValueChange={(task_type) => setDraft({ ...draft, task_type: task_type as SupportTaskDraft['task_type'] })} disabled={disabled}><SelectTrigger id="support-task-type" variant="underline" className="w-full px-0.5"><SelectValue /></SelectTrigger><SelectContent>{['bug', 'feature', 'chore'].map((type) => <SelectItem key={type} value={type}>{type}</SelectItem>)}</SelectContent></Select></div>
              <div><label htmlFor="support-task-priority" className="text-sm text-quiet-text-secondary">Priority</label><Select value={draft.priority} onValueChange={(priority) => setDraft({ ...draft, priority: priority as SupportTaskDraft['priority'] })} disabled={disabled}><SelectTrigger id="support-task-priority" variant="underline" className="w-full px-0.5"><SelectValue /></SelectTrigger><SelectContent>{['none', 'low', 'medium', 'high', 'urgent'].map((priority) => <SelectItem key={priority} value={priority}>{priority}</SelectItem>)}</SelectContent></Select></div>
            </div>
          </div>
        )}
        <DialogFooter>
          <QuietTextAction disabled={disabled} onClick={() => draft ? setDraft(null) : onOpenChange(false)}>{draft ? 'Back to matches' : 'Cancel'}</QuietTextAction>
          {draft ? <QuietPrimaryAction disabled={disabled || !teamId || !draft.name.trim() || !draft.description.trim()} onClick={() => void run(() => onCreate(teamId, draft))}>{isCreating ? 'Creating…' : 'Create task'}</QuietPrimaryAction> : <QuietPrimaryAction disabled={disabled} onClick={() => void run(async () => setDraft(unwrapRequired(await pmTriageService.draft(workspaceId, conversationId), 'Task draft')))}>Draft a new task</QuietPrimaryAction>}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
