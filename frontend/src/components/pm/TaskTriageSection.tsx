import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { QuietSection, QuietTextAction } from '@/components/design-system/quiet';
import { pmTriageService, type TriageReview } from '@/lib/services/pmTriageService';
import { pmTaskService } from '@/lib/services/pmTaskService';
import { unwrapRequired } from '@/lib/queryUtils';
import { queryKeys } from '@/lib/queryKeys';
import type { TaskDetail } from '@/lib/pmTypes';
import { useAuthStore } from '@/stores/authStore';
import { QuickTooltip } from '@/components/ui/quick-tooltip';
import { InformationCircleIcon } from '@/lib/icons';
import { TriageSuggestions } from './TriageSuggestions';

export function TaskTriageSection({ workspaceId, workspaceSlug, detail, disabled, onTaskUpdated }: {
  workspaceId: string;
  workspaceSlug: string;
  detail: TaskDetail;
  disabled: boolean;
  onTaskUpdated: (detail: TaskDetail) => void;
}) {
  const actorId = useAuthStore((state) => state.user?.id);
  const queryClient = useQueryClient();
  const queryKey = ['pm-triage', workspaceId, actorId, detail.task.id, detail.task.updated_at] as const;
  const query = useQuery({
    queryKey,
    queryFn: async () => unwrapRequired(await pmTriageService.analyze(workspaceId, 'task', detail.task.id), 'Task suggestions'),
    enabled: Boolean(actorId) && !disabled,
    retry: false,
    staleTime: 60_000,
    refetchOnWindowFocus: false,
  });
  const review = useMutation({
    mutationFn: async (request: TriageReview) => unwrapRequired(await pmTriageService.review(workspaceId, 'task', detail.task.id, request), 'Suggestion review'),
    onSuccess: async (result, request) => {
      queryClient.setQueryData(queryKey, query.data ? { ...query.data, reviewed: { ...query.data.reviewed, [result.key]: result.status } } : undefined);
      if (!request.dismiss) {
        const updated = unwrapRequired(await pmTaskService.get(workspaceId, detail.task.id), 'Updated task');
        onTaskUpdated(updated);
        await queryClient.invalidateQueries({ queryKey: queryKeys.pm.tasks(workspaceId) });
        await queryClient.invalidateQueries({ queryKey: queryKeys.pm.board(workspaceId) });
      }
    },
  });
  const view = query.data;
  if (view?.status === 'disabled' || view?.status === 'shadow') return null;
  const taskHref = (id: string) => `/w/${encodeURIComponent(workspaceSlug)}/pm/tasks/${encodeURIComponent(id)}`;
  const assessment = view?.assessment;
  const suggestionHelp = [
    'Based on this task’s saved title and description.',
    assessment ? `Compared with ${assessment.candidates_checked} accessible ${assessment.candidates_checked === 1 ? 'task' : 'tasks'}. Other matches may exist.` : null,
    assessment && !assessment.actionable ? 'There is not enough evidence of concrete product work yet.' : null,
  ].filter(Boolean).join(' ');
  const messages = {
    input_limit: 'This task has more context than suggestions can assess. You can triage it manually.',
    daily_limit: 'Suggestions have reached today’s limit. You can continue manually.',
    pending: 'Suggestions are being prepared. Refresh shortly.',
    failed: 'Suggestions are temporarily unavailable. You can continue manually.',
  };
  return (
    <QuietSection title={
      <span className="inline-flex items-center gap-1.5">
        Task suggestions
        <QuickTooltip label={suggestionHelp}>
          <button type="button" aria-label="About task suggestions" className="inline-flex rounded-sm text-quiet-text-tertiary hover:text-foreground focus-visible:outline-auto">
            <InformationCircleIcon className="size-3.5" />
          </button>
        </QuickTooltip>
      </span>
    } className="px-0 sm:px-0 lg:px-0" action={
      <QuietTextAction className="focus-visible:underline" disabled={disabled || query.isFetching || review.isPending} onClick={() => { review.reset(); void query.refetch(); }}>Refresh</QuietTextAction>
    }>
      {disabled ? <p className="text-sm text-quiet-text-secondary">Save your changes to review suggestions.</p> : null}
      {query.isFetching ? <p role="status" className="text-sm text-quiet-text-secondary">Checking task suggestions…</p> : null}
      {query.error || review.error ? <p role="alert" className="text-sm text-destructive">{(review.error || query.error)?.message}</p> : null}
      {view && view.status in messages ? <p role="status" className="text-sm text-quiet-text-secondary">{messages[view.status as keyof typeof messages]}</p> : null}
      {view?.status === 'ready' ? <TriageSuggestions showComparisonSummary={false} view={view} disabled={disabled || query.isFetching || review.isPending} currentType={detail.task.task_type} currentTeam={detail.task.team_id ?? undefined} taskHref={taskHref} onReview={(request) => review.mutate(request)} /> : null}
      {review.isPending ? <p role="status" className="mt-2 text-xs text-quiet-text-tertiary">Saving review…</p> : null}
    </QuietSection>
  );
}
