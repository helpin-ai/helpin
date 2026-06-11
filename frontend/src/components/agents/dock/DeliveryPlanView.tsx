import { formatDistanceToNow } from 'date-fns';
import type { ReactNode } from 'react';

import type { AgentRun } from '@/lib/pmTypes';
import type { CommandBarRunPlan } from '@/components/agents/dock/planSummary';
import { AgentAvatar } from '@/components/agents/AgentAvatar';
import {
  deliveryDotState,
  deliveryProgress,
  deliveryVerdict,
  groupStepsByWave,
  planHasFailedSteps,
  runDurationMs,
} from '@/components/pm/epicDeliveryDag';
import { cn } from '@/lib/utils';
import { FINALIZE_STEP_TYPE, SETUP_STEP_TYPE, buildTaskNodes, layerTasks } from './planLayers';
import { StatusDot } from './StatusDot';
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
  totalDurationMs,
  type StepDotState,
} from './utils';

interface DeliveryPlanViewProps {
  plan: CommandBarRunPlan;
  runsById: Record<string, AgentRun>;
  onOpenRun: (runId: string) => void;
  /** Rendered at the right edge of the header row (e.g. Resume / Retry buttons). */
  headerActions?: ReactNode;
  /** Skip the status header row — for hosts that render their own (e.g. the dock strip). */
  hideHeader?: boolean;
}

function isAgentStep(plan: CommandBarRunPlan, stepIndex: number): boolean {
  return !plan.steps[stepIndex]?.step_type;
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
  hasFailure: boolean,
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
  if (state === 'cancelled') return { text: 'Cancelled', className: 'text-destructive' };
  if (state === 'blocked') {
    // Once something upstream failed, "waiting" would imply progress will
    // resume on its own — it won't until someone retries.
    return hasFailure
      ? { text: 'Blocked', className: 'text-muted-foreground' }
      : { text: `waiting on ${waiting}`, className: 'text-muted-foreground' };
  }
  return { text: 'Queued', className: 'text-muted-foreground' };
}

function StepRow({
  plan,
  stepIndex,
  runsById,
  hasFailure,
  onSelectStep,
}: {
  plan: CommandBarRunPlan;
  stepIndex: number;
  runsById: Record<string, AgentRun>;
  hasFailure: boolean;
  onSelectStep: (stepIndex: number) => void;
}) {
  const step = plan.steps[stepIndex];
  const state = stepDotState(plan, stepIndex, runsById);
  const runId = plan.runIdsByStep[stepIndex];
  const run = runId ? (runsById[runId] ?? null) : null;
  const interactive = !!runId;
  const waiting = state === 'blocked' ? waitingDependencyCount(plan, stepIndex, runsById) : 0;
  const target = describeStepTarget(step);
  const status = stepStatusText(state, run, waiting, hasFailure);
  const isActive = state === 'running' || state === 'active_step';
  const isFailed = state === 'attention' || state === 'cancelled';

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
        'group/step flex items-center gap-2 rounded-md border border-border/60 bg-card px-2.5 py-1.5',
        interactive && 'cursor-pointer hover:bg-accent',
        isActive && 'border-orange-500/40 bg-orange-500/[0.04]',
        isFailed && 'border-destructive/40',
      )}
      title={run?.error_message ?? (interactive ? 'View run' : undefined)}
    >
      <StatusDot state={state} size="md" />
      {isAgentStep(plan, stepIndex) ? (
        <AgentAvatar
          name={step.agent_name}
          className="h-4 w-4 shrink-0 rounded-none border-0 bg-transparent shadow-none"
          genericBare
        />
      ) : null}
      <span className="min-w-0 flex-1 truncate text-sm">
        <span
          className={cn(
            'font-medium text-foreground/85',
            interactive && 'group-hover/step:underline',
          )}
        >
          {stepDisplayName(step)}
        </span>
        {target ? <span className="text-muted-foreground"> · {target}</span> : null}
      </span>
      <span className={cn('shrink-0 text-xs tabular-nums', status.className)}>
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
  const hasFailure = planHasFailedSteps(plan, runsById);
  return (
    <div className="flex flex-col">
      {groups.map((group, groupIndex) => {
        const parallel = group.stepIndexes.length;
        return (
          <div key={group.wave}>
            {groupIndex > 0 ? <div aria-hidden className="ml-3 h-3 w-px bg-border/70" /> : null}
            <div className="text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
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
                  hasFailure={hasFailure}
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
  hasFailure: boolean,
): { text: string; className: string } {
  const states = stepIndexes.map((i) => stepDotState(plan, i, runsById));
  if (states.some((s) => s === 'attention')) return { text: 'Failed', className: 'text-destructive' };
  if (states.some((s) => s === 'cancelled'))
    return { text: 'Cancelled', className: 'text-destructive' };
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
    return hasFailure
      ? { text: 'Blocked', className: 'text-muted-foreground' }
      : { text: 'Waiting', className: 'text-muted-foreground' };
  return { text: 'Queued', className: 'text-muted-foreground' };
}

/** First error message among a lane's failed/cancelled runs, if any. */
function laneErrorMessage(
  plan: CommandBarRunPlan,
  stepIndexes: number[],
  runsById: Record<string, AgentRun>,
): string | null {
  for (const i of stepIndexes) {
    const run = runsById[plan.runIdsByStep[i] ?? ''];
    if (run && (run.status === 'failed' || run.status === 'cancelled') && run.error_message) {
      return run.error_message;
    }
  }
  return null;
}

function StepChip({
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
  const interactive = !!plan.runIdsByStep[stepIndex];
  const run = runsById[plan.runIdsByStep[stepIndex] ?? ''];
  return (
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
        'group/chip flex items-center gap-1 rounded px-1 py-0.5',
        interactive && 'cursor-pointer hover:bg-accent',
      )}
      title={run?.error_message ?? (interactive ? 'View run' : stepDisplayName(step))}
    >
      <StatusDot state={state} />
      {isAgentStep(plan, stepIndex) ? (
        <AgentAvatar
          name={step.agent_name}
          className="h-3.5 w-3.5 shrink-0 rounded-none border-0 bg-transparent shadow-none"
          genericBare
        />
      ) : null}
      <span
        className={cn(
          'truncate text-xs font-medium text-muted-foreground',
          interactive && 'group-hover/chip:text-foreground group-hover/chip:underline',
        )}
      >
        {stepDisplayName(step)}
      </span>
    </span>
  );
}

function stepChain(
  plan: CommandBarRunPlan,
  stepIndexes: number[],
  runsById: Record<string, AgentRun>,
  onSelectStep: (stepIndex: number) => void,
) {
  return stepIndexes.map((stepIndex, i) => {
    const state = stepDotState(plan, stepIndex, runsById);
    return (
      <div key={`${plan.steps[stepIndex].agent_id}-${stepIndex}`} className="flex items-center gap-1">
        <StepChip plan={plan} stepIndex={stepIndex} runsById={runsById} onSelectStep={onSelectStep} />
        {i < stepIndexes.length - 1 ? (
          <span
            aria-hidden
            className={cn('mx-0.5 h-px w-3 bg-border/70', state === 'completed' && 'bg-emerald-500/40')}
          />
        ) : null}
      </div>
    );
  });
}

const LANE_TITLE_WIDTH = 'w-36 shrink-0 sm:w-56 lg:w-72';

/** Slim, chrome-less row for the pipeline's scaffolding steps (epic branch / final PR). */
function BookendRow({
  label,
  plan,
  stepIndexes,
  runsById,
  hasFailure,
  onSelectStep,
}: {
  label: string;
  plan: CommandBarRunPlan;
  stepIndexes: number[];
  runsById: Record<string, AgentRun>;
  hasFailure: boolean;
  onSelectStep: (stepIndex: number) => void;
}) {
  const status = laneStatusText(plan, stepIndexes, runsById, hasFailure);
  return (
    <div className="flex items-center gap-3 px-2.5 py-1">
      <span
        className={cn(LANE_TITLE_WIDTH, 'text-[11px] font-medium uppercase tracking-wider text-muted-foreground')}
      >
        {label}
      </span>
      <div className="flex min-w-0 flex-1 items-center gap-1">
        {stepChain(plan, stepIndexes, runsById, onSelectStep)}
      </div>
      <span className={cn('shrink-0 text-xs tabular-nums', status.className)}>
        {status.text}
      </span>
    </div>
  );
}

/**
 * Lane view for `task_pipeline_fan_out` plans: the epic-scaffolding steps
 * (branch setup, final PR) render as slim bookend rows — they run before and
 * after everything else, not alongside it — with one card per task lane in
 * between. Lanes run in parallel; each is a serial chain of agent steps with
 * a right-aligned live status.
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
  const hasFailure = planHasFailedSteps(plan, runsById);
  const setup: number[] = [];
  const finalize: number[] = [];
  const laneGroups = groupStepsByTarget(plan)
    .map((group) => ({
      ...group,
      stepIndexes: group.stepIndexes.filter((i) => {
        const stepType = plan.steps[i]?.step_type;
        if (stepType === SETUP_STEP_TYPE) {
          setup.push(i);
          return false;
        }
        if (stepType === FINALIZE_STEP_TYPE) {
          finalize.push(i);
          return false;
        }
        return true;
      }),
    }))
    .filter((group) => group.stepIndexes.length > 0);

  if (laneGroups.length === 0 && setup.length === 0 && finalize.length === 0) return null;

  // Same staging the plan preview shows: lanes whose tasks block each other
  // land in successive stages instead of one flat "everything in parallel"
  // list. buildTaskNodes ignores scaffolding, so no epic-node cycle here.
  const lanesByKey = new Map(laneGroups.map((group) => [group.key, group]));
  const layers = layerTasks(buildTaskNodes({ steps: plan.steps, run_count: plan.steps.length }));
  const stagedLanes = layers
    .map((layer) =>
      layer.tasks
        .map((task) => lanesByKey.get(task.key))
        .filter((group): group is NonNullable<typeof group> => !!group),
    )
    .filter((lanes) => lanes.length > 0);
  const placedKeys = new Set(stagedLanes.flat().map((group) => group.key));
  const leftoverLanes = laneGroups.filter((group) => !placedKeys.has(group.key));
  const showStages = stagedLanes.length > 1;

  const renderLane = (group: (typeof laneGroups)[number]) => {
    const status = laneStatusText(plan, group.stepIndexes, runsById, hasFailure);
    const laneFailed = status.text === 'Failed' || status.text === 'Cancelled';
    const laneActive = status.text.startsWith('Running');
    const error = laneFailed ? laneErrorMessage(plan, group.stepIndexes, runsById) : null;
    return (
      <div
        key={group.key}
        className={cn(
          'rounded-md border border-border/60 bg-card px-2.5 py-1.5',
          laneActive && 'border-orange-500/40 bg-orange-500/[0.04]',
          laneFailed && 'border-destructive/40',
        )}
      >
        <div className="flex items-center gap-3">
          <span
            className={cn(LANE_TITLE_WIDTH, 'truncate text-sm font-medium text-foreground/85')}
            title={group.title}
          >
            {group.title}
          </span>
          <div className="flex min-w-0 flex-1 items-center gap-1">
            {stepChain(plan, group.stepIndexes, runsById, onSelectStep)}
          </div>
          <span className={cn('shrink-0 text-xs tabular-nums', status.className)}>
            {status.text}
          </span>
        </div>
        {error ? (
          <p className="mt-1 truncate text-xs text-destructive" title={error}>
            {error}
          </p>
        ) : null}
      </div>
    );
  };

  return (
    <div className="flex flex-col gap-1">
      {setup.length > 0 ? (
        <BookendRow
          label="Setup"
          plan={plan}
          stepIndexes={setup}
          runsById={runsById}
          hasFailure={hasFailure}
          onSelectStep={onSelectStep}
        />
      ) : null}
      {showStages ? (
        <>
          {stagedLanes.map((lanes, stageIndex) => (
            <div key={stageIndex}>
              {stageIndex > 0 ? <div aria-hidden className="ml-3 h-3 w-px bg-border/70" /> : null}
              <div className="text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
                Stage {stageIndex + 1}
                {lanes.length > 1 ? ` · ${lanes.length} in parallel` : ''}
              </div>
              <div className="mt-1 flex flex-col gap-1">{lanes.map(renderLane)}</div>
            </div>
          ))}
          {leftoverLanes.map(renderLane)}
        </>
      ) : (
        laneGroups.map(renderLane)
      )}
      {finalize.length > 0 ? (
        <BookendRow
          label="Finalize"
          plan={plan}
          stepIndexes={finalize}
          runsById={runsById}
          hasFailure={hasFailure}
          onSelectStep={onSelectStep}
        />
      ) : null}
    </div>
  );
}

/**
 * Slim segmented progress bar: done (green) → active (ember) → failed (red) →
 * rest (muted). Also used standalone as the compact glance for delivery plans
 * in dock lists, where the full lane/stage view would be too tall.
 */
export function DeliveryProgressBar({
  plan,
  runsById,
}: {
  plan: CommandBarRunPlan;
  runsById: Record<string, AgentRun>;
}) {
  const { done, failed, active, rest, total } = deliveryProgress(plan, runsById);
  if (total === 0) return null;
  const pct = (n: number) => `${(n / total) * 100}%`;
  return (
    <div aria-hidden className="mt-2 flex h-1 w-full overflow-hidden rounded-full bg-muted/60">
      {done > 0 ? <span className="bg-emerald-500" style={{ width: pct(done) }} /> : null}
      {active > 0 ? <span className="bg-orange-500" style={{ width: pct(active) }} /> : null}
      {failed > 0 ? <span className="bg-destructive" style={{ width: pct(failed) }} /> : null}
      {rest > 0 ? <span style={{ width: pct(rest) }} /> : null}
    </div>
  );
}

/**
 * Read-only, presentational view of a command-bar *delivery* plan (a `dag` or
 * `task_pipeline_fan_out`) for the epic page. DAG plans render as dependency
 * stages (steps that run together grouped per stage); task pipelines render
 * one lane per task between Setup/Finalize bookends. Clicking a step opens
 * that step's run in the coding-session drawer.
 */
export function DeliveryPlanView({
  plan,
  runsById,
  onOpenRun,
  headerActions,
  hideHeader = false,
}: DeliveryPlanViewProps) {
  if (plan.planKind !== 'dag' && plan.planKind !== 'task_pipeline_fan_out') return null;

  const dot = deliveryDotState(plan, runsById);
  const label = planKindLabel(plan.planKind, plan.steps.length);
  const summary = planSummaryText(plan, runsById);
  const ts = planUpdatedAt(plan, runsById);
  // End-to-end span is only meaningful when the delivery ran to completion —
  // for failed/cancelled plans it includes however long the plan sat broken.
  const totalMs = plan.status === 'completed' ? totalDurationMs(plan, runsById) : null;
  const verdict = deliveryVerdict(plan, runsById);

  const handleSelectStep = (stepIndex: number) => {
    const runId = plan.runIdsByStep[stepIndex];
    if (runId) onOpenRun(runId);
  };

  return (
    <div>
      {hideHeader ? null : (
        <div className="flex items-center gap-2">
          <StatusDot state={dot} />
          <span className="text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
            {label}
          </span>
          <span className="text-muted-foreground/60">·</span>
          <span className="min-w-0 flex-1 truncate text-xs text-foreground/80">{summary}</span>
          <span className="shrink-0 text-xs text-muted-foreground">
            {totalMs !== null ? `ran ${formatDuration(totalMs)} · ` : ''}
            {formatDistanceToNow(ts, { addSuffix: true })}
          </span>
          {headerActions}
        </div>
      )}

      {verdict ? (
        <p
          className={cn(
            'mt-1.5 truncate text-xs',
            verdict.tone === 'attention' ? 'text-destructive' : 'text-amber-600 dark:text-amber-400',
          )}
          title={verdict.text}
        >
          {verdict.text}
        </p>
      ) : null}

      <DeliveryProgressBar plan={plan} runsById={runsById} />

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
