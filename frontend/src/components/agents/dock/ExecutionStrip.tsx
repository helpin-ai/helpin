import { useState } from 'react';
import { formatDistanceToNow } from 'date-fns';
import type { AgentRun } from '@/lib/pmTypes';
import type { CommandBarRunPlan } from '@/stores/commandBarStore';
import { ACTIVE_RUN_STATUSES, getAgentRunDisplayStatus } from '@/components/pm/agentRunConstants';
import {
  ArrowDown01Icon,
  ArrowUpRight01Icon,
  Bookmark01Icon,
  Loading01Icon,
  RotateLeft01Icon,
} from '@/lib/icons';
import { cn } from '@/lib/utils';
import { StatusDot, type DotKind } from './StatusDot';
import { PipelineRail } from './PipelineRail';
import { FanOutRail } from './FanOutRail';
import { TaskPipelineRail } from './TaskPipelineRail';
import { DagRail } from './DagRail';
import { PendingInteractionCard } from './PendingInteractionCard';
import { useAgentRunStream } from './useAgentRunStream';
import type { RunPlanArtifact } from '@/lib/pmTypes';
import {
  classifyPlan,
  classifyRun,
  formatDuration,
  outputSummaryText,
  planKindLabel,
  planSummaryText,
  planUpdatedAt,
  runStatusLabel,
  runUpdatedAt,
  stepDotState,
  targetLabel,
  totalDurationMs,
  type ActivityState,
} from './utils';

export type StripAction = 'rerun' | 'save_as_agent' | 'open' | 'retry' | 'cancel';

interface PlanStripProps {
  kind: 'plan';
  workspaceId: string;
  plan: CommandBarRunPlan;
  runsById: Record<string, AgentRun>;
  busyPlanId?: string | null;
  onAction?: (action: StripAction) => void;
  /** Render the result body inline as a soft card. Pass null to suppress. */
  resultSlot?: React.ReactNode;
}

interface RunStripProps {
  kind: 'run';
  workspaceId: string;
  run: AgentRun;
  busy?: boolean;
  onAction?: (action: StripAction) => void;
  resultSlot?: React.ReactNode;
}

export type ExecutionStripProps = PlanStripProps | RunStripProps;

function activityToDot(state: ActivityState, hasActive = false): DotKind {
  if (state === 'running' && hasActive) return 'active_step';
  return state;
}

/**
 * Pull the most useful one-line description from a live RunPlanArtifact:
 * the in_progress step text, falling back to `note`.
 */
function liveStreamSummary(plan: RunPlanArtifact | null): string | null {
  if (!plan) return null;
  const inProgress = plan.plan?.find((s) => s.status === 'in_progress');
  if (inProgress?.step) return `…${inProgress.step}`;
  if (plan.note) return plan.note;
  return null;
}

export function ExecutionStrip(props: ExecutionStripProps) {
  const [open, setOpen] = useState(false);

  if (props.kind === 'plan') return <PlanStrip {...props} open={open} setOpen={setOpen} />;
  return <RunStrip {...props} open={open} setOpen={setOpen} />;
}

interface InternalProps {
  open: boolean;
  setOpen: (v: boolean) => void;
}

function PlanStrip({
  workspaceId,
  plan,
  runsById,
  busyPlanId,
  onAction,
  resultSlot,
  open,
  setOpen,
}: PlanStripProps & InternalProps) {
  const state = classifyPlan(plan, runsById);
  const label = planKindLabel(plan.planKind, plan.steps.length);
  const baseSummary = planSummaryText(plan, runsById);
  const ts = planUpdatedAt(plan, runsById);
  const duration = totalDurationMs(plan, runsById);
  const dot: DotKind = activityToDot(state, state === 'running');

  // Live stream the currently-active sub-run, if any.
  const activeRunId = (() => {
    const idx = plan.currentStepIndex ?? 0;
    const id = plan.runIdsByStep[idx];
    const run = id ? runsById[id] : null;
    if (run && ACTIVE_RUN_STATUSES.has(run.status)) return run.id;
    // fallback: any active run in the plan
    for (const rid of Object.values(plan.runIdsByStep)) {
      const r = runsById[rid];
      if (r && ACTIVE_RUN_STATUSES.has(r.status)) return r.id;
    }
    return null;
  })();
  const stream = useAgentRunStream(workspaceId, activeRunId, !!activeRunId);
  const liveSummary = liveStreamSummary(stream.currentPlan);
  const summary = state === 'running' && liveSummary ? `${baseSummary} · ${liveSummary}` : baseSummary;
  const isFanOut = plan.planKind === 'fan_out';
  const isTaskPipeline = plan.planKind === 'task_pipeline_fan_out';
  const isDAG = plan.planKind === 'dag';
  const showRail = plan.steps.length > 1 || isFanOut || isTaskPipeline || isDAG;

  const renderRail = () => {
    if (isTaskPipeline) return <TaskPipelineRail plan={plan} runsById={runsById} />;
    if (isDAG) return <DagRail plan={plan} runsById={runsById} />;
    if (isFanOut) return <FanOutRail plan={plan} runsById={runsById} />;
    return <PipelineRail plan={plan} runsById={runsById} />;
  };
  const completed = plan.status === 'completed' || state === 'completed';
  const busy = busyPlanId === plan.id;

  return (
    <div className="space-y-2">
      <button
        type="button"
        onClick={() => setOpen(!open)}
        className="group flex w-full items-center gap-2 rounded-md px-1 py-1 text-left transition hover:bg-muted/40"
      >
        <StatusDot state={dot} />
        <span className="text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
          {label}
        </span>
        <span className="text-muted-foreground/60">·</span>
        <span className="min-w-0 flex-1 truncate text-xs text-foreground/80">{summary}</span>
        <span className="shrink-0 text-[11px] text-muted-foreground">
          {duration != null ? formatDuration(duration) : formatDistanceToNow(ts, { addSuffix: true })}
        </span>
        <ArrowDown01Icon
          className={cn(
            'h-3 w-3 shrink-0 text-muted-foreground transition-transform',
            open && 'rotate-180',
          )}
        />
      </button>

      {showRail && !open ? renderRail() : null}

      {open ? (
        <div className="space-y-2 pl-4">
          {plan.prompt ? (
            <p className="text-[11px] italic text-muted-foreground">"{plan.prompt}"</p>
          ) : null}
          {showRail ? renderRail() : null}
          <div className="space-y-1.5">
            {plan.steps.map((step, i) => {
              const runId = plan.runIdsByStep[i];
              const run = runId ? runsById[runId] : null;
              const dot: DotKind = stepDotState(plan, i, runsById);
              const targetTitle = isTaskPipeline || isDAG ? step.target?.display_title : null;
              return (
                <div
                  key={`${step.agent_id}-${i}`}
                  className="flex items-start gap-2 text-xs leading-snug"
                >
                  <StatusDot state={dot} className="mt-1" />
                  <div className="min-w-0 flex-1">
                    <div className="flex items-baseline gap-2">
                      {targetTitle ? (
                        <span className="truncate text-[11px] text-muted-foreground">
                          {targetTitle}:
                        </span>
                      ) : null}
                      <span className="truncate font-medium text-foreground">{step.agent_name}</span>
                      {run ? (
                        <span className="shrink-0 text-[11px] text-muted-foreground">
                          {runStatusLabel(run)}
                        </span>
                      ) : dot === 'blocked' ? (
                        <span className="shrink-0 text-[11px] text-muted-foreground">Waiting</span>
                      ) : null}
                    </div>
                    {run && outputSummaryText(run) ? (
                      <p className="line-clamp-2 text-[11px] text-muted-foreground">
                        {outputSummaryText(run)}
                      </p>
                    ) : null}
                  </div>
                </div>
              );
            })}
          </div>
        </div>
      ) : null}

      {stream.pendingInteraction && activeRunId ? (
        <PendingInteractionCard
          workspaceId={workspaceId}
          runId={activeRunId}
          interaction={stream.pendingInteraction}
        />
      ) : null}

      {resultSlot}

      {onAction && completed ? (
        <ChipRow>
          <ActionChip icon={RotateLeft01Icon} label="Re-run" onClick={() => onAction('rerun')} />
          <ActionChip
            icon={Bookmark01Icon}
            label="Save as agent"
            onClick={() => onAction('save_as_agent')}
          />
          <ActionChip
            icon={ArrowUpRight01Icon}
            label="Open"
            onClick={() => onAction('open')}
          />
        </ChipRow>
      ) : null}

      {onAction && state === 'attention' ? (
        <ChipRow>
          <ActionChip
            icon={busy ? Loading01Icon : RotateLeft01Icon}
            label={busy ? 'Retrying…' : 'Retry'}
            onClick={() => onAction('retry')}
            disabled={busy}
          />
          <ActionChip
            icon={ArrowUpRight01Icon}
            label="Open"
            onClick={() => onAction('open')}
          />
        </ChipRow>
      ) : null}
    </div>
  );
}

function RunStrip({ workspaceId, run, busy, onAction, resultSlot, open, setOpen }: RunStripProps & InternalProps) {
  const state = classifyRun(run);
  const dot: DotKind = activityToDot(state, state === 'running');
  const isActive = ACTIVE_RUN_STATUSES.has(run.status);
  const stream = useAgentRunStream(workspaceId, run.id, isActive);
  const liveSummary = liveStreamSummary(stream.currentPlan);
  const baseSummary = outputSummaryText(run) || runStatusLabel(run);
  const summary = isActive && liveSummary ? liveSummary : baseSummary;
  const ts = runUpdatedAt(run);
  const duration = totalDurationMs(null, {}, run);
  const completed = run.status === 'completed';
  const display = getAgentRunDisplayStatus(run);
  const awaitingApproval = display === 'awaiting_approval';
  const canCancel = ACTIVE_RUN_STATUSES.has(run.status);

  return (
    <div className="space-y-2">
      <button
        type="button"
        onClick={() => setOpen(!open)}
        className="group flex w-full items-center gap-2 rounded-md px-1 py-1 text-left transition hover:bg-muted/40"
      >
        <StatusDot state={dot} />
        <span className="text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
          Agent
        </span>
        <span className="text-muted-foreground/60">·</span>
        <span className="min-w-0 flex-1 truncate text-xs text-foreground/80">{summary}</span>
        <span className="shrink-0 text-[11px] text-muted-foreground">
          {duration != null ? formatDuration(duration) : formatDistanceToNow(ts, { addSuffix: true })}
        </span>
        <ArrowDown01Icon
          className={cn(
            'h-3 w-3 shrink-0 text-muted-foreground transition-transform',
            open && 'rotate-180',
          )}
        />
      </button>

      {open ? (
        <div className="space-y-2 pl-4 text-xs leading-snug">
          <p className="text-[11px] text-muted-foreground">{targetLabel(run)}</p>
          {stream.currentPlan?.plan?.length ? (
            <div className="space-y-1">
              {stream.currentPlan.plan.map((step, i) => (
                <div key={i} className="flex items-start gap-2">
                  <StatusDot
                    state={
                      step.status === 'completed'
                        ? 'completed'
                        : step.status === 'in_progress'
                          ? 'active_step'
                          : 'queued'
                    }
                    className="mt-1"
                  />
                  <span className="text-foreground/80">{step.step}</span>
                </div>
              ))}
              {stream.currentPlan.note ? (
                <p className="pl-4 text-[11px] italic text-muted-foreground">{stream.currentPlan.note}</p>
              ) : null}
            </div>
          ) : (
            <p className="text-foreground/80">{baseSummary}</p>
          )}
          {run.error_message ? (
            <p className="rounded bg-destructive/10 px-2 py-1 text-destructive">{run.error_message}</p>
          ) : null}
        </div>
      ) : null}

      {stream.pendingInteraction ? (
        <PendingInteractionCard
          workspaceId={workspaceId}
          runId={run.id}
          interaction={stream.pendingInteraction}
        />
      ) : null}

      {resultSlot}

      {onAction ? (
        <ChipRow>
          {completed ? (
            <>
              <ActionChip icon={RotateLeft01Icon} label="Re-run" onClick={() => onAction('rerun')} />
              <ActionChip
                icon={Bookmark01Icon}
                label="Save as agent"
                onClick={() => onAction('save_as_agent')}
              />
            </>
          ) : null}
          {state === 'attention' && !awaitingApproval ? (
            <ActionChip
              icon={busy ? Loading01Icon : RotateLeft01Icon}
              label={busy ? 'Retrying…' : 'Retry'}
              onClick={() => onAction('retry')}
              disabled={busy}
            />
          ) : null}
          {awaitingApproval ? (
            <ActionChip
              icon={busy ? Loading01Icon : ArrowUpRight01Icon}
              label={busy ? 'Approving…' : 'Approve'}
              onClick={() => onAction('rerun')}
              disabled={busy}
              accent
            />
          ) : null}
          {canCancel ? (
            <ActionChip
              icon={Loading01Icon}
              label="Cancel"
              onClick={() => onAction('cancel')}
              disabled={busy}
            />
          ) : null}
          <ActionChip icon={ArrowUpRight01Icon} label="Open" onClick={() => onAction('open')} />
        </ChipRow>
      ) : null}
    </div>
  );
}

function ChipRow({ children }: { children: React.ReactNode }) {
  return <div className="flex flex-wrap items-center gap-1.5 pt-0.5">{children}</div>;
}

function ActionChip({
  icon: Icon,
  label,
  onClick,
  disabled,
  accent,
}: {
  icon: React.ComponentType<{ className?: string }>;
  label: string;
  onClick: () => void;
  disabled?: boolean;
  accent?: boolean;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      className={cn(
        'inline-flex items-center gap-1 rounded border px-2 py-0.5 text-[11px] font-medium transition',
        accent
          ? 'border-orange-500/40 bg-orange-500/10 text-orange-700 hover:bg-orange-500/15 dark:text-orange-300'
          : 'border-border/70 bg-background/80 text-foreground hover:border-foreground/30 hover:bg-muted/60',
        disabled && 'cursor-not-allowed opacity-60',
      )}
    >
      <Icon className={cn('h-3 w-3', label === 'Retrying…' || label === 'Approving…' ? 'animate-spin' : null)} />
      {label}
    </button>
  );
}
