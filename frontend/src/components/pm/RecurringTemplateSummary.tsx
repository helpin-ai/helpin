import { useState, type ReactNode } from 'react';
import { formatDistanceToNow, parseISO } from 'date-fns';
import { AlertTriangle, ChevronLeft, ChevronRight, Clock3, ListTodo, PlayCircle } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import type { RecurringRun, RecurringTemplateStatus } from '@/lib/pmTypes';
import { cn } from '@/lib/utils';

interface RecurringTemplateSummaryProps {
  title?: string;
  status: RecurringTemplateStatus;
  ruleSummary: string;
  nextRunAt?: string;
  generatedCount?: number;
  occurrenceNumber?: number;
  lastError?: string;
  runs?: RecurringRun[];
  actions?: ReactNode;
  compact?: boolean;
  onOpenStory?: (storyId: string) => void;
}

const statusClasses: Record<RecurringTemplateStatus, string> = {
  active: 'border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-900/60 dark:bg-emerald-950/20 dark:text-emerald-200',
  paused: 'border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-900/60 dark:bg-amber-950/20 dark:text-amber-200',
  stopped: 'border-slate-200 bg-slate-50 text-slate-700 dark:border-slate-800 dark:bg-slate-950/20 dark:text-slate-200',
  failed: 'border-red-200 bg-red-50 text-red-700 dark:border-red-900/60 dark:bg-red-950/20 dark:text-red-200',
};

function formatRelative(iso?: string) {
  if (!iso) return 'Not scheduled';
  try {
    return formatDistanceToNow(parseISO(iso), { addSuffix: true });
  } catch {
    return iso;
  }
}

function formatRunLabel(run: RecurringRun) {
  switch (run.trigger_type) {
    case 'schedule':
      return 'Scheduled run';
    case 'completion':
      return 'Completion run';
    case 'generate_now':
      return 'Manual run';
    default:
      return 'Seed run';
  }
}

const PAGE_SIZE = 10;

function RunsSection({ runs, compact, onOpenStory }: { runs: RecurringRun[]; compact: boolean; onOpenStory?: (storyId: string) => void }) {
  const perPage = compact ? 3 : PAGE_SIZE;
  const [page, setPage] = useState(0);
  const totalPages = Math.ceil(runs.length / perPage);
  const visible = runs.slice(page * perPage, (page + 1) * perPage);
  const start = page * perPage + 1;
  const end = Math.min((page + 1) * perPage, runs.length);

  return (
    <div className="mt-4 border-t border-border/50 pt-3">
      <div className="mb-2 flex items-center justify-between">
        <span className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
          {runs.length <= perPage ? `Last ${runs.length} runs` : `Runs ${start}–${end} of ${runs.length}`}
        </span>
        {totalPages > 1 && (
          <div className="flex items-center gap-1">
            <Button type="button" variant="ghost" size="icon" className="h-6 w-6" disabled={page === 0} onClick={() => setPage((p) => p - 1)}>
              <ChevronLeft className="h-3.5 w-3.5" />
            </Button>
            <Button type="button" variant="ghost" size="icon" className="h-6 w-6" disabled={page >= totalPages - 1} onClick={() => setPage((p) => p + 1)}>
              <ChevronRight className="h-3.5 w-3.5" />
            </Button>
          </div>
        )}
      </div>
      <div className="space-y-2">
        {visible.map((run) => (
          <div key={run.id} className="flex items-center gap-2 rounded-md bg-muted/30 px-2 py-1.5 text-xs">
            <PlayCircle className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
            <span className="min-w-0 flex-1 truncate">{formatRunLabel(run)}</span>
            <span className="shrink-0 capitalize text-muted-foreground">{run.status}</span>
            <span className="shrink-0 text-muted-foreground">{formatRelative(run.finished_at ?? run.started_at ?? run.created_at)}</span>
            {run.generated_story_id && onOpenStory ? (
              <button
                type="button"
                className="shrink-0 text-primary hover:underline"
                onClick={() => onOpenStory(run.generated_story_id!)}
              >
                View story
              </button>
            ) : null}
          </div>
        ))}
      </div>
    </div>
  );
}

export function RecurringTemplateSummary({
  title,
  status,
  ruleSummary,
  nextRunAt,
  generatedCount,
  occurrenceNumber,
  lastError,
  runs,
  actions,
  compact = false,
  onOpenStory,
}: RecurringTemplateSummaryProps) {
  return (
    <section className={cn('rounded-lg border border-border/60 bg-background', compact ? 'p-3' : 'p-4')}>
      <div className="flex flex-wrap items-start gap-2">
        <div className="min-w-0 flex-1">
          {title ? <div className="truncate text-sm font-semibold text-foreground">{title}</div> : null}
          <div className="mt-1 flex flex-wrap items-center gap-2">
            <Badge variant="outline" className={cn('capitalize', statusClasses[status])}>
              {status}
            </Badge>
            {occurrenceNumber ? (
              <span className="text-xs text-muted-foreground">Occurrence #{occurrenceNumber}</span>
            ) : null}
            {generatedCount ? (
              <span className="text-xs text-muted-foreground">{generatedCount} generated</span>
            ) : null}
          </div>
        </div>
        {actions ? <div className="flex flex-wrap items-center gap-1">{actions}</div> : null}
      </div>

      <div className="mt-3 grid gap-2 text-sm">
        <div className="flex items-start gap-2 text-muted-foreground">
          <ListTodo className="mt-0.5 h-3.5 w-3.5 shrink-0" />
          <span className="min-w-0 truncate">{ruleSummary}</span>
        </div>

        <div className="flex items-start gap-2 text-muted-foreground">
          <Clock3 className="mt-0.5 h-3.5 w-3.5 shrink-0" />
          <span>{formatRelative(nextRunAt)}</span>
        </div>

        {lastError ? (
          <div className="flex items-start gap-2 rounded-md border border-destructive/20 bg-destructive/5 p-2 text-destructive">
            <AlertTriangle className="mt-0.5 h-3.5 w-3.5 shrink-0" />
            <span className="min-w-0 text-xs">{lastError}</span>
          </div>
        ) : null}
      </div>

      {runs && runs.length > 0 ? (
        <RunsSection runs={runs} compact={compact} onOpenStory={onOpenStory} />
      ) : null}
    </section>
  );
}
