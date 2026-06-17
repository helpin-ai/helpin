import { useMemo } from 'react';
import type { AgentRun } from '@/lib/pmTypes';
import type { CommandBarRunPlan } from '@/stores/commandBarStore';
import { Cancel01Icon, Loading01Icon, PlayIcon, RotateLeft01Icon } from '@/lib/icons';
import { cn, timeAgo } from '@/lib/utils';
import { StatusDot, type DotKind } from './StatusDot';
import { PipelineRail } from './PipelineRail';
import { FanOutRail } from './FanOutRail';
import { DeliveryProgressBar } from './DeliveryPlanView';
import {
  classifyPlan,
  classifyRun,
  listGroupKey,
  LIST_GROUP_LABEL,
  planKindLabel,
  planSummaryText,
  planUpdatedAt,
  runContextLine,
  runDisplayTitle,
  runStatusLabel,
  runUpdatedAt,
  type ActivityState,
  type ListGroupKey,
} from './utils';

type ListItem =
  | { id: string; kind: 'plan'; plan: CommandBarRunPlan; ts: number }
  | { id: string; kind: 'run'; run: AgentRun; ts: number };

interface RunListViewProps {
  plans: CommandBarRunPlan[];
  standaloneRuns: AgentRun[];
  runsById: Record<string, AgentRun>;
  filter: string;
  busyPlanId?: string | null;
  busyRunId?: string | null;
  onSelect: (item: ListItem) => void;
  onResumePlan: (plan: CommandBarRunPlan) => void;
  onCancelPlan: (plan: CommandBarRunPlan) => void;
  onRetryPlan: (plan: CommandBarRunPlan) => void;
  onRetryRun: (run: AgentRun) => void;
}

export function RunListView({
  plans,
  standaloneRuns,
  runsById,
  filter,
  busyPlanId,
  busyRunId,
  onSelect,
  onResumePlan,
  onCancelPlan,
  onRetryPlan,
  onRetryRun,
}: RunListViewProps) {
  const items = useMemo<ListItem[]>(() => {
    const planItems: ListItem[] = plans.map((plan) => ({
      id: `plan-${plan.id}`,
      kind: 'plan',
      plan,
      ts: planUpdatedAt(plan, runsById),
    }));
    const runItems: ListItem[] = standaloneRuns.map((run) => ({
      id: `run-${run.id}`,
      kind: 'run',
      run,
      ts: runUpdatedAt(run),
    }));
    return [...planItems, ...runItems].sort((a, b) => b.ts - a.ts);
  }, [plans, standaloneRuns, runsById]);

  const filtered = useMemo(() => {
    const q = filter.trim().toLowerCase();
    if (!q) return items;
    return items.filter((it) => titleFor(it, runsById).toLowerCase().includes(q));
  }, [items, filter, runsById]);

  const grouped = useMemo(() => {
    const now = new Date();
    const buckets: Record<ListGroupKey, ListItem[]> = {
      active: [],
      today: [],
      yesterday: [],
      earlier_this_week: [],
      older: [],
    };
    for (const it of filtered) {
      const state = it.kind === 'plan' ? classifyPlan(it.plan, runsById) : classifyRun(it.run);
      if (state === 'running') {
        buckets.active.push(it);
      } else {
        const key = listGroupKey(now, new Date(it.ts));
        buckets[key].push(it);
      }
    }
    return buckets;
  }, [filtered, runsById]);

  const groups: ListGroupKey[] = ['active', 'today', 'yesterday', 'earlier_this_week', 'older'];

  if (filtered.length === 0) {
    return (
      <div className="flex flex-col items-center gap-1 py-12 text-center">
        <p className="text-sm font-medium text-foreground">
          {filter.trim() ? 'No runs match' : 'No runs yet'}
        </p>
        <p className="text-xs text-muted-foreground">
          {filter.trim() ? 'Press Enter to start a new run.' : 'Ask the dock to start one.'}
        </p>
      </div>
    );
  }

  return (
    <div className="divide-y divide-border/40">
      {groups.map((g) => {
        const rows = grouped[g];
        if (rows.length === 0) return null;
        return (
          <div key={g} className="py-1">
            <div className="px-3 pt-2 pb-1 text-[11px] font-medium text-muted-foreground">
              {LIST_GROUP_LABEL[g]}
            </div>
            {rows.map((it) => (
              <ListRow
                key={it.id}
                item={it}
                runsById={runsById}
                busyPlanId={busyPlanId}
                busyRunId={busyRunId}
                onSelect={onSelect}
                onResumePlan={onResumePlan}
                onCancelPlan={onCancelPlan}
                onRetryPlan={onRetryPlan}
                onRetryRun={onRetryRun}
              />
            ))}
          </div>
        );
      })}
    </div>
  );
}

function titleFor(it: ListItem, runsById: Record<string, AgentRun>): string {
  if (it.kind === 'plan') {
    if (it.plan.prompt) return it.plan.prompt;
    const firstId = it.plan.runIdsByStep[0];
    const first = firstId ? runsById[firstId] : null;
    return first ? runDisplayTitle(first) || 'Command run' : 'Command run';
  }
  return runDisplayTitle(it.run) || 'Agent run';
}

function stateTextClass(state: ActivityState): string {
  switch (state) {
    case 'attention':
      return 'text-destructive';
    case 'awaiting':
      return 'text-amber-700 dark:text-amber-300';
    case 'cancelled':
      return 'text-muted-foreground';
    case 'running':
      return 'text-orange-700 dark:text-orange-300';
    case 'completed':
      return 'text-emerald-700 dark:text-emerald-300';
    case 'queued':
    default:
      return 'text-muted-foreground';
  }
}

function ListRow({
  item,
  runsById,
  busyPlanId,
  busyRunId,
  onSelect,
  onResumePlan,
  onCancelPlan,
  onRetryPlan,
  onRetryRun,
}: {
  item: ListItem;
  runsById: Record<string, AgentRun>;
  busyPlanId?: string | null;
  busyRunId?: string | null;
  onSelect: (item: ListItem) => void;
  onResumePlan: (plan: CommandBarRunPlan) => void;
  onCancelPlan: (plan: CommandBarRunPlan) => void;
  onRetryPlan: (plan: CommandBarRunPlan) => void;
  onRetryRun: (run: AgentRun) => void;
}) {
  const state =
    item.kind === 'plan' ? classifyPlan(item.plan, runsById) : classifyRun(item.run);
  const dot: DotKind = state === 'running' ? 'active_step' : state;
  const title = titleFor(item, runsById);
  const ts = new Date(item.ts);

  let metaLabel: string;
  let subline: string;
  let rail: React.ReactNode = null;
  let busy = false;
  let onRetry: (() => void) | null = null;
  let onResume: (() => void) | null = null;
  let onCancel: (() => void) | null = null;
  let errorMessage: string | null = null;

  if (item.kind === 'plan') {
    const { plan } = item;
    metaLabel = planKindLabel(plan.planKind, plan.steps.length);
    subline = planSummaryText(plan, runsById);
    rail =
      plan.planKind === 'task_pipeline_fan_out' || plan.planKind === 'dag' ? (
        <DeliveryProgressBar plan={plan} runsById={runsById} />
      ) : plan.planKind === 'fan_out' ? (
        <FanOutRail plan={plan} runsById={runsById} max={6} />
      ) : plan.steps.length > 1 ? (
        <PipelineRail plan={plan} runsById={runsById} />
      ) : null;
    busy = busyPlanId === plan.id;
    if (state === 'running') {
      onResume = () => onResumePlan(plan);
      onCancel = () => onCancelPlan(plan);
    }
    if (state === 'attention' || state === 'cancelled') {
      onRetry = () => onRetryPlan(plan);
      const failedRun = Object.values(plan.runIdsByStep)
        .map((id) => runsById[id])
        .find((r) => r && (r.status === 'failed' || r.status === 'cancelled'));
      errorMessage = failedRun?.error_message ?? null;
    }
  } else {
    const { run } = item;
    metaLabel = runContextLine(run);
    subline = runStatusLabel(run);
    busy = busyRunId === run.id;
    if (state === 'attention' || state === 'cancelled') {
      onRetry = () => onRetryRun(run);
      errorMessage = run.error_message ?? null;
    }
  }

  // For attention/cancelled rows, the error message replaces the subline so
  // the user sees *why* the run needs attention without expanding it. When
  // there's no error_message we keep the status label as a soft fallback.
  const showError = (state === 'attention' || state === 'cancelled') && !!errorMessage;
  const sublineText = showError ? errorMessage! : subline;
  const sublineTextClass = showError
    ? 'text-destructive'
    : stateTextClass(state);

  return (
    <div
      className={cn(
        'group flex w-full items-start gap-2 px-3 py-2 transition',
        'hover:bg-muted/40 cursor-pointer',
      )}
      role="button"
      tabIndex={0}
      onClick={() => onSelect(item)}
      onKeyDown={(e) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault();
          onSelect(item);
        }
      }}
    >
      <div className="mt-1.5">
        <StatusDot state={dot} />
      </div>
      <div className="min-w-0 flex-1">
        <div className="flex items-baseline justify-between gap-2">
          <span className="truncate text-sm font-medium text-foreground">{title}</span>
          <span className="shrink-0 text-[11px] text-muted-foreground">{timeAgo(ts)}</span>
        </div>
        <div className="mt-0.5 flex items-center justify-between gap-2">
          <span
            className={cn('min-w-0 flex-1 truncate text-[11px]', sublineTextClass)}
          >
            {sublineText}
            {metaLabel ? (
              <>
                <span className="text-muted-foreground/60"> · </span>
                <span className="text-muted-foreground">{metaLabel}</span>
              </>
            ) : null}
          </span>
          {onResume || onCancel ? (
            <div className="flex shrink-0 items-center gap-1">
              {onResume ? (
                <button
                  type="button"
                  onClick={(e) => {
                    e.stopPropagation();
                    onResume?.();
                  }}
                  disabled={busy}
                  className="inline-flex shrink-0 items-center gap-1 rounded border border-border/70 bg-background/80 px-1.5 py-0.5 text-[11px] font-medium text-foreground transition hover:border-foreground/30 hover:bg-muted/60 disabled:cursor-not-allowed disabled:opacity-60"
                >
                  {busy ? (
                    <Loading01Icon className="h-3 w-3 animate-spin" />
                  ) : (
                    <PlayIcon className="h-3 w-3" />
                  )}
                  {busy ? 'Resuming…' : 'Resume'}
                </button>
              ) : null}
              {onCancel ? (
                <button
                  type="button"
                  onClick={(e) => {
                    e.stopPropagation();
                    onCancel?.();
                  }}
                  disabled={busy}
                  className="inline-flex h-[22px] w-[22px] shrink-0 items-center justify-center rounded border border-border/70 bg-background/80 text-muted-foreground transition hover:border-destructive/40 hover:bg-destructive/10 hover:text-destructive disabled:cursor-not-allowed disabled:opacity-60"
                  aria-label="Cancel plan"
                >
                  <Cancel01Icon className="h-3 w-3" />
                </button>
              ) : null}
            </div>
          ) : onRetry ? (
            <button
              type="button"
              onClick={(e) => {
                e.stopPropagation();
                onRetry?.();
              }}
              disabled={busy}
              className="inline-flex shrink-0 items-center gap-1 rounded border border-border/70 bg-background/80 px-1.5 py-0.5 text-[11px] font-medium text-foreground transition hover:border-foreground/30 hover:bg-muted/60 disabled:cursor-not-allowed disabled:opacity-60"
            >
              {busy ? (
                <Loading01Icon className="h-3 w-3 animate-spin" />
              ) : (
                <RotateLeft01Icon className="h-3 w-3" />
              )}
              {busy ? 'Retrying…' : 'Retry'}
            </button>
          ) : null}
        </div>
        {rail}
      </div>
    </div>
  );
}
