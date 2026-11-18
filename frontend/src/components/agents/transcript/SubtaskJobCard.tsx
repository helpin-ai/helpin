import { useQuery } from '@tanstack/react-query';
import { Progress } from '@/components/ui/progress';
import { agentService } from '@/lib/services/agentService';
import { unwrap } from '@/lib/queryUtils';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { cn } from '@/lib/utils';
import type { TranscriptSegment } from './segments';
import { isSubtaskJobFinished, subtaskJobIdsFromSegments, subtaskJobProgress } from './subtaskJob';

const STATE_LABELS: Record<string, string> = {
  queued: 'Queued',
  running: 'Running',
  waiting: 'Waiting',
  paused: 'Paused',
  completed: 'Done',
  failed: 'Failed',
  cancelled: 'Cancelled',
};

function compactTokens(tokens: number): string {
  if (tokens >= 1_000_000) return `${(tokens / 1_000_000).toFixed(1)}M tokens`;
  if (tokens >= 1_000) return `${Math.round(tokens / 1_000)}k tokens`;
  return `${tokens} tokens`;
}

/** Live progress of the sub-tasks one spawn_subtasks call started. */
export function SubtaskJobCard({ jobId }: { jobId: string }) {
  const workspaceId = useWorkspaceStore((state) => state.currentWorkspace?.id);
  const { data: job, isError } = useQuery({
    queryKey: ['subtask-job', workspaceId, jobId],
    queryFn: async () => unwrap(await agentService.getSubtaskJob(workspaceId!, jobId)),
    enabled: !!workspaceId,
    // Poll only while workers are still running.
    refetchInterval: (query) => (query.state.data && isSubtaskJobFinished(query.state.data) ? false : 3000),
  });

  if (isError) {
    return <div className="px-1 py-1 text-xs text-muted-foreground">Sub-task progress is unavailable.</div>;
  }
  if (!job) {
    return <div className="px-1 py-1 text-xs text-muted-foreground">Loading sub-tasks…</div>;
  }

  const progress = subtaskJobProgress(job);
  const failed = job.state === 'failed';
  const tokens = job.usage.input_tokens + job.usage.output_tokens;

  return (
    <section className="space-y-1.5 border-y border-border/70 px-1 py-2" data-subtask-job={job.id} aria-label="Sub-tasks">
      <div className="text-[11px] font-medium uppercase tracking-wide text-muted-foreground">Sub-tasks</div>
      <div className="flex items-baseline justify-between gap-3">
        <div className="min-w-0 truncate text-xs font-medium text-foreground" title={job.objective}>
          {job.objective}
        </div>
        <div className={cn('shrink-0 text-[11px] text-muted-foreground', failed && 'text-destructive')}>
          {STATE_LABELS[job.state] ?? job.state}
        </div>
      </div>
      <Progress
        value={progress.percent}
        className="h-1"
        indicatorClassName={failed ? 'bg-destructive' : undefined}
        aria-label={`${progress.settled} of ${progress.total} sub-tasks finished`}
      />
      <div className="flex flex-wrap items-center gap-x-2 text-[11px] text-muted-foreground">
        <span>
          {progress.settled} of {progress.total} finished
        </span>
        {progress.detail.map((part) => (
          <span key={part}>· {part}</span>
        ))}
        {tokens > 0 ? <span>· {compactTokens(tokens)}</span> : null}
      </div>
      {job.error ? <div className="text-[11px] text-destructive">{job.error}</div> : null}
    </section>
  );
}

/**
 * Progress cards for every sub-task job a group of work started. Rendered
 * beside the group, not inside it, so progress stays visible while the group's
 * tool rows are collapsed.
 */
export function SubtaskJobCards({ segments }: { segments: TranscriptSegment[] }) {
  const jobIds = subtaskJobIdsFromSegments(segments);
  if (jobIds.length === 0) return null;
  return (
    <div className="space-y-2 pt-2">
      {jobIds.map((jobId) => (
        <SubtaskJobCard key={jobId} jobId={jobId} />
      ))}
    </div>
  );
}
