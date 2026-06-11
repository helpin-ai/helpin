import { formatDistanceToNow } from 'date-fns';

import type { AgentRun } from '@/lib/pmTypes';
import type { CommandBarRunPlan } from '@/components/agents/dock/planSummary';
import { deliveryDotState, groupStepsByWave, runDurationMs } from '@/components/pm/epicDeliveryDag';
import { cn } from '@/lib/utils';
import { StatusDot, type DotKind } from './StatusDot';
import {
  describeStepTarget,
  formatDuration,
  groupStepsByTarget,
  planKindLabel,
  planSummaryText,
  planUpdatedAt,
  runStatusLabel,
  stepDisplayName,
  stepDotState,
  type StepDotState,
} from './utils';

interface EpicDeliveryDagProps {
  plan: CommandBarRunPlan;
  runsById: Record<string, AgentRun>;
  onOpenRun: (runId: string) => void;
}

function waitingDependencyCount(
  plan: CommandBarRunPlan,
  stepIndex: number,
  runsById: Record<string, AgentRun>,
): number {
  const deps = plan.steps[stepIndex]?.depends_on_step_indexes ?? [];
  let waiting = 0;
  for (const dep of deps) {
    const runId = plan.runIdsByStep[dep];
    const run = runId ? runsById[runId] : null;
    if (run?.status !== 'completed') waiting++;
  }
  return waiting;
}

/** Right-aligned status text for a step row: duration when done, live status while active. */
function stepStatusText(
  state: StepDotState,
  run: AgentRun | null,
  waiting: number,
): { text: string; className: string } {
  if (run && state === 'completed') {
    const ms = runDurationMs(run);
    return { text: ms === null ? 'Done' : formatDuration(ms), className: 'text-muted-foreground' };
  }
  if (run && (state === 'running' || state === 'active_step')) {
    const ms = runDurationMs(run);
    return {
      text: ms === null ? 'Running' : `Running · ${formatDuration(ms)}`,
      className: 'text-orange-600 dark:text-orange-400',
    };
  }
  if (run && state === 'awaiting') {
    return { text: runStatusLabel(run), className: 'text-amber-600 dark:text-amber-400' };
  }
  if (state === 'attention') return { text: 'Failed', className: 'text-destructive' };
  if (state === 'cancelled') return { text: 'Cancelled', className: 'text-muted-foreground' };
  if (state === 'blocked') return { text: `waiting on ${waiting}`, className: 'text-muted-foreground' };
  return { text: 'Queued', className: 'text-muted-foreground' };
}

function StepRow({
  plan,
  stepIndex,
  runsById,
  onSelectStep,
}: {
  plan: CommandBarRunPlan;
  stepIndex: number;
  runsById: Record<string, AgentRun>;
  onSelectStep: (stepIndex: number) => void;
}) {
  const step = plan.steps[stepIndex];
  const state = stepDotState(plan, stepIndex, runsById);
  const runId = plan.runIdsByStep[stepIndex];
  const run = runId ? (runsById[runId] ?? null) : null;
  const interactive = !!runId;
  const waiting = state === 'blocked' ? waitingDependencyCount(plan, stepIndex, runsById) : 0;
  const target = describeStepTarget(step);
  const status = stepStatusText(state, run, waiting);
  const isActive = state === 'running' || state === 'active_step';
  const isFailed = state === 'attention';

  return (
    <div
      role={interactive ? 'button' : undefined}
      tabIndex={interactive ? 0 : undefined}
      onClick={interactive ? () => onSelectStep(stepIndex) : undefined}
      onKeyDown={
        interactive
          ? (event) => {
              if (event.key === 'Enter' || event.key === ' ') {
                event.preventDefault();
                onSelectStep(stepIndex);
              }
            }
          : undefined
      }
      className={cn(
        'flex items-center gap-2 rounded-md border border-border/60 bg-card px-2.5 py-1.5',
        interactive && 'cursor-pointer hover:bg-accent',
        isActive && 'border-orange-500/40 bg-orange-500/[0.04]',
        isFailed && 'border-destructive/40',
      )}
      title={run?.error_message ?? undefined}
    >
      <StatusDot state={state} size="md" />
      <span className="min-w-0 flex-1 truncate text-xs">
        <span className="font-medium text-foreground/85">{stepDisplayName(step)}</span>
        {target ? <span className="text-muted-foreground"> · {target}</span> : null}
      </span>
      <span className={cn('shrink-0 text-[11px] tabular-nums', status.className)}>
        {status.text}
      </span>
    </div>
  );
}

/**
 * Stage view for `dag` plans: steps grouped into execution waves derived from
 * their dependencies, so steps that run together (fan-out) sit in the same
 * stage and sequencing reads top-to-bottom.
 */
function DagStageView({
  plan,
  runsById,
  onSelectStep,
}: {
  plan: CommandBarRunPlan;
  runsById: Record<string, AgentRun>;
  onSelectStep: (stepIndex: number) => void;
}) {
  const groups = groupStepsByWave(plan);
  return (
    <div className="flex flex-col">
      {groups.map((group, groupIndex) => {
        const parallel = group.stepIndexes.length;
        return (
          <div key={group.wave}>
            {groupIndex > 0 ? <div aria-hidden className="ml-3 h-3 w-px bg-border/70" /> : null}
            <div className="text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
              Stage {groupIndex + 1}
              {parallel > 1 ? ` · ${parallel} in parallel` : ''}
            </div>
            <div className="mt-1 flex flex-col gap-1">
              {group.stepIndexes.map((stepIndex) => (
                <StepRow
                  key={stepIndex}
                  plan={plan}
                  stepIndex={stepIndex}
                  runsById={runsById}
                  onSelectStep={onSelectStep}
                />
              ))}
            </div>
          </div>
        );
      })}
    </div>
  );
}

/** Per-lane (per-task) status text: failures win, then activity, then completion. */
function laneStatusText(
  plan: CommandBarRunPlan,
  stepIndexes: number[],
  runsById: Record<string, AgentRun>,
): { text: string; className: string } {
  const states = stepIndexes.map((i) => stepDotState(plan, i, runsById));
  if (states.some((s) => s === 'attention')) return { text: 'Failed', className: 'text-destructive' };
  if (states.some((s) => s === 'cancelled'))
    return { text: 'Cancelled', className: 'text-muted-foreground' };
  const activeIndex = stepIndexes.find((i) => {
    const s = stepDotState(plan, i, runsById);
    return s === 'running' || s === 'active_step';
  });
  if (activeIndex !== undefined) {
    const run = runsById[plan.runIdsByStep[activeIndex] ?? ''];
    const ms = run ? runDurationMs(run) : null;
    return {
      text: ms === null ? 'Running' : `Running · ${formatDuration(ms)}`,
      className: 'text-orange-600 dark:text-orange-400',
    };
  }
  if (states.some((s) => s === 'awaiting'))
    return { text: 'Awaiting', className: 'text-amber-600 dark:text-amber-400' };
  if (states.every((s) => s === 'completed')) {
    let total = 0;
    for (const i of stepIndexes) {
      const run = runsById[plan.runIdsByStep[i] ?? ''];
      total += run ? (runDurationMs(run) ?? 0) : 0;
    }
    return {
      text: total > 0 ? formatDuration(total) : 'Done',
      className: 'text-muted-foreground',
    };
  }
  if (states.some((s) => s === 'blocked'))
    return { text: 'Waiting', className: 'text-muted-foreground' };
  return { text: 'Queued', className: 'text-muted-foreground' };
}

/**
 * Lane view for `task_pipeline_fan_out` plans: one row per target task (lanes
 * run in parallel), each lane a serial chain of agent steps, with a
 * right-aligned live status for the lane.
 */
function TaskPipelineLanes({
  plan,
  runsById,
  onSelectStep,
}: {
  plan: CommandBarRunPlan;
  runsById: Record<string, AgentRun>;
  onSelectStep: (stepIndex: number) => void;
}) {
  const groups = groupStepsByTarget(plan);
  if (groups.length === 0) return null;

  return (
    <div className="flex flex-col gap-1">
      {groups.map((group) => {
        const status = laneStatusText(plan, group.stepIndexes, runsById);
        const laneFailed = status.text === 'Failed';
        const laneActive = status.text.startsWith('Running');
        return (
          <div
            key={group.key}
            className={cn(
              'flex items-center gap-3 rounded-md border border-border/60 bg-card px-2.5 py-1.5',
              laneActive && 'border-orange-500/40 bg-orange-500/[0.04]',
              laneFailed && 'border-destructive/40',
            )}
          >
            <span
              className="w-36 shrink-0 truncate text-xs font-medium text-foreground/85 sm:w-56 lg:w-72"
              title={group.title}
            >
              {group.title}
            </span>
            <div className="flex min-w-0 flex-1 items-center gap-1">
              {group.stepIndexes.map((stepIndex, i) => {
                const step = plan.steps[stepIndex];
                const state = stepDotState(plan, stepIndex, runsById);
                const interactive = !!plan.runIdsByStep[stepIndex];
                const run = runsById[plan.runIdsByStep[stepIndex] ?? ''];
                return (
                  <div key={`${step.agent_id}-${stepIndex}`} className="flex items-center gap-1">
                    <span
                      role={interactive ? 'button' : undefined}
                      tabIndex={interactive ? 0 : undefined}
                      onClick={interactive ? () => onSelectStep(stepIndex) : undefined}
                      onKeyDown={
                        interactive
                          ? (event) => {
                              if (event.key === 'Enter' || event.key === ' ') {
                                event.preventDefault();
                                onSelectStep(stepIndex);
                              }
                            }
                          : undefined
                      }
                      className={cn(
                        'flex items-center gap-1 rounded px-1 py-0.5',
                        interactive && 'cursor-pointer hover:bg-accent',
                      )}
                      title={run?.error_message ?? stepDisplayName(step)}
                    >
                      <StatusDot state={state} />
                      <span className="truncate text-[11px] font-medium text-muted-foreground">
                        {stepDisplayName(step)}
                      </span>
                    </span>
                    {i < group.stepIndexes.length - 1 ? (
                      <span
                        aria-hidden
                        className={cn(
                          'mx-0.5 h-px w-3 bg-border/70',
                          state === 'completed' && 'bg-emerald-500/40',
                        )}
                      />
                    ) : null}
                  </div>
                );
              })}
            </div>
            <span className={cn('shrink-0 text-[11px] tabular-nums', status.className)}>
              {status.text}
            </span>
          </div>
        );
      })}
    </div>
  );
}

/**
 * Read-only, presentational view of a command-bar *delivery* plan (a `dag` or
 * `task_pipeline_fan_out`) for the epic page. DAG plans render as dependency
 * stages (steps that run together grouped per stage); task pipelines render
 * one lane per task. Clicking a step opens that step's run in the
 * coding-session drawer.
 */
export function EpicDeliveryDag({ plan, runsById, onOpenRun }: EpicDeliveryDagProps) {
  if (plan.planKind !== 'dag' && plan.planKind !== 'task_pipeline_fan_out') return null;

  const dot: DotKind = deliveryDotState(plan, runsById);
  const label = planKindLabel(plan.planKind, plan.steps.length);
  const summary = planSummaryText(plan, runsById);
  const ts = planUpdatedAt(plan, runsById);

  const handleSelectStep = (stepIndex: number) => {
    const runId = plan.runIdsByStep[stepIndex];
    if (runId) onOpenRun(runId);
  };

  return (
    <div>
      <div className="flex items-center gap-2">
        <StatusDot state={dot} />
        <span className="text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
          {label}
        </span>
        <span className="text-muted-foreground/60">·</span>
        <span className="min-w-0 flex-1 truncate text-xs text-foreground/80">{summary}</span>
        <span className="shrink-0 text-[11px] text-muted-foreground">
          {formatDistanceToNow(ts, { addSuffix: true })}
        </span>
      </div>

      <div className="mt-2.5">
        {plan.planKind === 'task_pipeline_fan_out' ? (
          <TaskPipelineLanes plan={plan} runsById={runsById} onSelectStep={handleSelectStep} />
        ) : (
          <DagStageView plan={plan} runsById={runsById} onSelectStep={handleSelectStep} />
        )}
      </div>
    </div>
  );
}
