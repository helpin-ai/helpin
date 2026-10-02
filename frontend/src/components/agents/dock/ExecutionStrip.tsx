import { useNotificationView } from '@/hooks/useNotificationView';
import { useId, useState } from 'react';
import { formatDistanceToNow } from 'date-fns';
import type { AgentRun } from '@/lib/pmTypes';
import type { CommandBarRunPlan } from './planSummary';
import { ACTIVE_RUN_STATUSES, getAgentRunDisplayStatus } from '@/components/pm/agentRunConstants';
import {
  ArrowReloadHorizontalIcon,
  ArrowUpRight01Icon,
  Bookmark01Icon,
  Cancel01Icon,
  Loading01Icon,
  PlayIcon,
  RotateLeft01Icon,
} from '@/lib/icons';
import { cn } from '@/lib/utils';
import { DisclosureChevron } from '@/components/agents/transcript/DisclosureChevron';
import { displayAgentName } from '@/lib/agentTerminology';
import { StatusDot, type DotKind } from './StatusDot';
import { PipelineRail } from './PipelineRail';
import { FanOutRail } from './FanOutRail';
import { DeliveryPlanView, DeliveryProgressBar } from './DeliveryPlanView';
import { PendingInteractionCard } from './PendingInteractionCard';
import { DockInteractionPrompt } from './DockInteractionLayer';
import { DockTranscript } from './DockTranscript';
import { dockTranscriptHasContent } from './dockTranscriptContent';
import { useAgentRunStream } from './useAgentRunStream';
import type { RunPlanArtifact } from '@/lib/pmTypes';
import {
  classifyPlan,
  classifyRun,
  describeStep,
  formatDuration,
  outputSummaryText,
  planKindLabel,
  planSummaryText,
  planUpdatedAt,
  runDisplayTitle,
  runStatusLabel,
  runUpdatedAt,
  stepDotState,
  targetLabel,
  totalDurationMs,
  type ActivityState,
} from './utils';

export type StripAction = 'rerun' | 'save_as_agent' | 'open' | 'retry' | 'resume' | 'cancel';

interface PlanStripProps {
  active?: boolean;
  kind: 'plan';
  workspaceId: string;
  plan: CommandBarRunPlan;
  runsById: Record<string, AgentRun>;
  busyPlanId?: string | null;
  onAction?: (action: StripAction) => void;
  /** Open a specific step's run (delivery plans only — step click-through). */
  onOpenRun?: (runId: string) => void;
  /** Render the result body inline as a soft card. Pass null to suppress. */
  resultSlot?: React.ReactNode;
  /** Start expanded — used for single-output (one-shot) plans. */
  defaultOpen?: boolean;
}

interface RunStripProps {
  active?: boolean;
  kind: 'run';
  workspaceId: string;
  run: AgentRun;
  busy?: boolean;
  onAction?: (action: StripAction) => void;
  resultSlot?: React.ReactNode;
  /**
   * When true, collapse the run to a one-line header — hiding the inline
   * pending-interaction card, result slot, and action chips until the user
   * expands it. Used to reduce noise when several paused runs queue up.
   */
  compact?: boolean;
  /**
   * Initial open state. When `compact` is true, the first paused run in the
   * timeline starts expanded so the user lands on something actionable, while
   * subsequent paused runs start collapsed.
   */
  defaultOpen?: boolean;
}

export type ExecutionStripProps = PlanStripProps | RunStripProps;

function activityToDot(state: ActivityState, hasActive = false): DotKind {
  if (state === 'running' && hasActive) return 'active_step';
  return state;
}

function activityStatusLabel(state: ActivityState): string {
  switch (state) {
    case 'running': return 'Running';
    case 'awaiting': return 'Waiting';
    case 'attention': return 'Failed';
    case 'cancelled': return 'Cancelled';
    case 'completed': return 'Completed';
    case 'queued': return 'Queued';
  }
}

/** Tailwind classes for the compact-mode right-side status pill, by state. */
function pillClasses(state: ActivityState): string {
  switch (state) {
    case 'awaiting':
      return 'bg-amber-500/10 text-amber-700 dark:text-amber-300';
    case 'attention':
      return 'bg-destructive/10 text-destructive dark:text-red-300';
    case 'cancelled':
      return 'bg-muted text-muted-foreground';
    case 'completed':
      return 'bg-emerald-500/10 text-emerald-700 dark:text-emerald-300';
    case 'running':
      return 'bg-orange-500/10 text-orange-700 dark:text-orange-300';
    case 'queued':
    default:
      return 'bg-muted text-muted-foreground';
  }
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

/** Keep the task's opening phrase in the row; the full brief lives in the disclosure. */
function compactTaskSummary(text: string): string {
  const firstLine = text.trim().split(/\n/).find(line => line.trim()) ?? '';
  const task = firstLine.replace(/^[#>*\s-]+/, '').replace(/^(?:task|goal|objective):\s*/i, '')
    .split(/\s+\(|[.!?](?:\s|$)/, 1)[0].replace(/\s+/g, ' ').trim();
  if (task.length <= 110) return task;
  const prefix = task.slice(0, 107);
  const boundary = prefix.lastIndexOf(' ');
  return `${prefix.slice(0, boundary > 70 ? boundary : prefix.length)}…`;
}

function planRuns(plan: CommandBarRunPlan, runsById: Record<string, AgentRun>): AgentRun[] {
  return Object.values(plan.runIdsByStep)
    .map((id) => runsById[id])
    .filter(Boolean);
}

function planHasReadyUnstartedStep(plan: CommandBarRunPlan, runsById: Record<string, AgentRun>): boolean {
  return plan.steps.some((step, index) => {
    if (plan.runIdsByStep[index]) return false;
    const deps = step.depends_on_step_indexes ?? [];
    return deps.every((depIndex) => {
      const depRunId = plan.runIdsByStep[depIndex];
      const depRun = depRunId ? runsById[depRunId] : null;
      return depRun?.status === 'completed';
    });
  });
}

export function ExecutionStrip(props: ExecutionStripProps) {
  const initialOpen = props.defaultOpen ?? false;
  const [open, setOpen] = useState(initialOpen);

  if (props.kind === 'plan') return <PlanStrip {...props} open={open} setOpen={setOpen} />;
  return <RunStrip {...props} open={open} setOpen={setOpen} />;
}


interface InternalProps {
  open: boolean;
  setOpen: (v: boolean) => void;
}

function PlanStrip({
  active = true,
  workspaceId,
  plan,
  runsById,
  busyPlanId,
  onAction,
  onOpenRun,
  resultSlot,
  open,
  setOpen,
}: PlanStripProps & InternalProps) {
  const detailsId = useId();
  const state = classifyPlan(plan, runsById);
  const runs = planRuns(plan, runsById);
  const failedToStart = plan.status === 'failed' && runs.length === 0;
  const isSingleStep = plan.steps.length === 1;
  const singleStep = isSingleStep ? plan.steps[0] : null;
  const agentName = singleStep ? displayAgentName(singleStep.agent_name) : '';
  const label = agentName || planKindLabel(plan.planKind, plan.steps.length);
  const baseSummary = planSummaryText(plan, runsById);
  const taskSummary = singleStep ? describeStep(singleStep.instructions) || plan.prompt || baseSummary : '';
  const summary = isSingleStep
    ? [taskSummary, failedToStart ? 'Failed to start' : activityStatusLabel(state)].filter(Boolean).join(' · ')
    : baseSummary;
  const createdAt = Date.parse(plan.createdAt ?? '');
  const ts = Number.isFinite(createdAt) ? createdAt : planUpdatedAt(plan, runsById);
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
  useNotificationView(workspaceId, 'run', activeRunId, active);
  const stream = useAgentRunStream(workspaceId, activeRunId, !!activeRunId);
  // Single-step plans (one-shot / known-agent) render the run's full output
  // inline, the same way standalone runs do. Stream the lone run while active,
  // and once after it finishes whenever the row is open.
  const singleRunId = isSingleStep ? Object.values(plan.runIdsByStep)[0] ?? null : null;
  const singleRun = singleRunId ? runsById[singleRunId] : null;
  const singleError = singleRun?.error_message || plan.errorMessage || null;
  const singleResult = singleRun ? outputSummaryText(singleRun) : '';
  const singleActive = singleRun ? ACTIVE_RUN_STATUSES.has(singleRun.status) : false;
  // While the lone run is active the header already streams it (activeRunId), so
  // reuse that state. Only open a second stream once it's terminal and the row
  // is expanded, to fetch the finished transcript without polling.
  const needTerminalTranscript = !!singleRunId && singleRunId !== activeRunId && open;
  const terminalTranscript = useAgentRunStream(workspaceId, singleRunId, needTerminalTranscript, 0);
  const transcriptState = singleRunId
    ? singleRunId === activeRunId
      ? stream.streamState
      : terminalTranscript.streamState
    : null;
  const hasTranscript = dockTranscriptHasContent(transcriptState, singleActive);
  const liveSummary = liveStreamSummary(stream.currentPlan);
  const taskLabel = compactTaskSummary(
    state === 'running' && liveSummary ? liveSummary.replace(/^…/, '') : taskSummary,
  ) || 'Delegated task';
  const taskInstructions = singleStep?.instructions?.trim() || plan.prompt?.trim() || '';
  const expandedSummary = !isSingleStep && state === 'running' && liveSummary ? `${summary} · ${liveSummary}` : summary;
  const isFanOut = plan.planKind === 'fan_out';
  const isTaskPipeline = plan.planKind === 'task_pipeline_fan_out';
  const isDAG = plan.planKind === 'dag';
  const showRail = plan.steps.length > 1 || isFanOut || isTaskPipeline || isDAG;
  const firstRunId = Object.values(plan.runIdsByStep)[0];
  // Output is inline once the transcript is present, so the sheet shortcut is
  // only offered when there's no inline output to read (e.g. coding runs).
  const showHeaderOpen = !!onAction && !!(activeRunId || firstRunId) && !hasTranscript;

  const isDelivery = isDAG || isTaskPipeline;
  const renderRail = () => {
    // Delivery plans (dag / task pipeline) share the rich DeliveryPlanView when
    // expanded; collapsed they show the slim segmented progress bar instead of
    // a multi-row rail.
    if (isDelivery) return <DeliveryProgressBar plan={plan} runsById={runsById} />;
    if (isFanOut) return <FanOutRail plan={plan} runsById={runsById} />;
    return <PipelineRail plan={plan} runsById={runsById} />;
  };
  const completed = plan.status === 'completed' || state === 'completed';
  const busy = busyPlanId === plan.id;
  const pendingInteraction = activeRunId ? stream.pendingInteraction : null;
  const hasActiveRun = runs.some((run) => ACTIVE_RUN_STATUSES.has(run.status));
  const canContinuePlan =
    plan.status === 'running' &&
    !hasActiveRun &&
    planHasReadyUnstartedStep(plan, runsById);
  const canCancelPlan = plan.status === 'running' && (hasActiveRun || canContinuePlan);

  return (
    <div className="space-y-2">
      <div className="group flex w-full items-center gap-1 rounded-md px-1 py-1 transition hover:bg-muted/40">
        <button
          type="button"
          onClick={() => setOpen(!open)}
          className="flex min-w-0 flex-1 items-center gap-2 rounded text-left focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
          aria-expanded={open}
          aria-controls={open ? detailsId : undefined}
        >
          <StatusDot state={dot} />
          {isSingleStep ? (
            <span className="flex min-w-0 flex-1 flex-col gap-1 sm:flex-row sm:items-baseline sm:gap-2">
              <span className="flex min-w-0 items-baseline gap-2 sm:max-w-[45%]">
                <span className="min-w-0 truncate text-xs font-medium text-foreground" title={label}>{label}</span>
                <span className={cn('shrink-0 text-xs', state === 'attention' ? 'text-destructive' : 'text-muted-foreground')}>
                  {failedToStart ? 'Failed to start' : activityStatusLabel(state)}
                </span>
              </span>
              <span className="hidden text-muted-foreground/60 sm:inline" aria-hidden="true">·</span>
              <span className="min-w-0 truncate text-xs text-muted-foreground sm:flex-1" title={taskLabel}>{taskLabel}</span>
            </span>
          ) : (
            <>
              <span className="text-[11px] font-medium uppercase tracking-wider text-muted-foreground">{label}</span>
              <span className="text-muted-foreground/60">·</span>
              <span className="min-w-0 flex-1 truncate text-xs text-foreground/80">{expandedSummary}</span>
            </>
          )}
          {ts !== null ? (
            <time
              dateTime={new Date(ts).toISOString()}
              title={new Date(ts).toLocaleString()}
              className="hidden shrink-0 text-[11px] text-muted-foreground sm:inline"
            >
              {formatDistanceToNow(ts, { addSuffix: true })}
            </time>
          ) : null}
          <DisclosureChevron open={open} className="h-3 w-3" />
        </button>
        {showHeaderOpen ? (
          <button
            type="button"
            aria-label="Open agent run"
            onClick={() => onAction?.('open')}
            className="inline-flex h-6 w-6 shrink-0 items-center justify-center rounded text-muted-foreground transition hover:bg-background/80 hover:text-foreground"
          >
            <ArrowUpRight01Icon className="h-3.5 w-3.5" />
          </button>
        ) : null}
      </div>

      {showRail && !open ? renderRail() : null}

      {open ? (
        <div id={detailsId} className="space-y-3 border-l border-border/60 pl-4">
          {isSingleStep && taskInstructions && taskInstructions !== taskLabel ? (
            <div className="space-y-1">
              <div className="text-xs font-medium text-muted-foreground">Task instructions</div>
              <p className="whitespace-pre-wrap break-words text-xs leading-relaxed text-foreground/80">
                {taskInstructions}
              </p>
            </div>
          ) : null}
          {!isSingleStep && plan.prompt ? (
            <p className="text-[11px] italic text-muted-foreground">"{plan.prompt}"</p>
          ) : null}
          {isDelivery ? (
            <DeliveryPlanView
              plan={plan}
              runsById={runsById}
              hideHeader
              compact
              onOpenRun={(runId) => onOpenRun?.(runId)}
            />
          ) : (
            <>
              {showRail ? renderRail() : null}
              {isSingleStep ? (
                hasTranscript ? (
                  <DockTranscript stream={transcriptState} active={singleActive} showUserMessages={false} />
                ) : singleError ? (
                  <p className="rounded bg-destructive/10 px-2 py-1 text-xs text-destructive">{singleError}</p>
                ) : singleResult ? (
                  <p className="text-xs text-foreground/80">{singleResult}</p>
                ) : null
              ) : (
                <div className="space-y-1.5">
                  {plan.steps.map((step, i) => {
                    const runId = plan.runIdsByStep[i];
                    const run = runId ? runsById[runId] : null;
                    const dot: DotKind = stepDotState(plan, i, runsById);
                    return (
                      <div
                        key={`${step.agent_id}-${i}`}
                        className="flex items-start gap-2 text-xs leading-snug"
                      >
                        <StatusDot state={dot} className="mt-1" />
                        <div className="min-w-0 flex-1">
                          <div className="flex items-baseline gap-2">
                            <span className="truncate font-medium text-foreground">{displayAgentName(step.agent_name)}</span>
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
              )}
            </>
          )}
        </div>
      ) : null}

      {pendingInteraction && activeRunId ? (
        <DockInteractionPrompt id={`${activeRunId}:${pendingInteraction.interaction_id}`}>
          <PendingInteractionCard
            workspaceId={workspaceId}
            runId={activeRunId}
            interaction={pendingInteraction}
            onResolved={() => {
              if (!pendingInteraction) return;
              stream.clearPendingInteraction(pendingInteraction.interaction_id);
              void stream.refetch();
            }}
          />
        </DockInteractionPrompt>
      ) : null}

      {hasTranscript ? null : resultSlot}

      {onAction && completed ? (
        <ChipRow>
          <ActionChip icon={RotateLeft01Icon} label="Re-run" onClick={() => onAction('rerun')} />
          <ActionChip
            icon={Bookmark01Icon}
            label="Save as agent"
            onClick={() => onAction('save_as_agent')}
          />
          {hasTranscript ? null : (
            <ActionChip
              icon={ArrowUpRight01Icon}
              label="Open"
              onClick={() => onAction('open')}
            />
          )}
        </ChipRow>
      ) : null}

      {onAction && (state === 'attention' || state === 'cancelled') ? (
        <ChipRow>
          <ActionChip
            icon={busy ? Loading01Icon : ArrowReloadHorizontalIcon}
            label={busy ? 'Retrying…' : 'Retry'}
            onClick={() => onAction('retry')}
            disabled={busy}
            retry
          />
          {hasTranscript ? null : (
            <ActionChip
              icon={ArrowUpRight01Icon}
              label="Open"
              onClick={() => onAction('open')}
            />
          )}
        </ChipRow>
      ) : null}

      {onAction && (canContinuePlan || canCancelPlan) ? (
        <ChipRow align="end">
          {canContinuePlan ? (
            <ActionChip
              icon={busy ? Loading01Icon : PlayIcon}
              label={busy ? 'Continuing…' : 'Continue'}
              onClick={() => onAction('resume')}
              disabled={busy}
            />
          ) : null}
          {hasTranscript ? null : (
            <ActionChip
              icon={ArrowUpRight01Icon}
              label="Open"
              onClick={() => onAction('open')}
              accent
            />
          )}
          {canCancelPlan ? (
            <ActionChip
              icon={Cancel01Icon}
              label="Cancel"
              onClick={() => onAction('cancel')}
              disabled={busy}
              subtle
            />
          ) : null}
        </ChipRow>
      ) : null}
    </div>
  );
}

function RunStrip({ active = true, workspaceId, run, busy, onAction, resultSlot, open, setOpen, compact }: RunStripProps & InternalProps) {
  const state = classifyRun(run);
  const dot: DotKind = activityToDot(state, state === 'running');
  const isActive = ACTIVE_RUN_STATUSES.has(run.status);
  const expanded = open;
  useNotificationView(workspaceId, 'run', run.id, active && expanded);
  // Stream while the run is active, and also once after it finishes whenever the
  // row is expanded, so the agent's full output renders inline in the dock
  // instead of only behind the session sheet. Don't poll terminal runs.
  const stream = useAgentRunStream(workspaceId, run.id, isActive || expanded, isActive ? 5_000 : 0);
  const hasTranscript = dockTranscriptHasContent(stream.streamState, isActive);
  const liveSummary = liveStreamSummary(stream.currentPlan);
  const outputSummary = outputSummaryText(run);
  // Prefer the pending interaction's title ("Approve pricing reply to
  // customer") over the generic status label so the header line tells the
  // user *what* needs review, not just that something does. Status is already
  // conveyed by the dot + the right-side pill.
  const pendingInteraction = isActive ? stream.pendingInteraction : null;
  const interactionTitle = pendingInteraction?.title?.trim() || null;
  const baseSummary = outputSummary || interactionTitle || runDisplayTitle(run) || runStatusLabel(run);
  const summary = isActive && liveSummary ? liveSummary : baseSummary;
  const ts = runUpdatedAt(run);
  const duration = totalDurationMs(null, {}, run);
  const completed = run.status === 'completed';
  const display = getAgentRunDisplayStatus(run);
  const awaitingApproval = display === 'awaiting_approval';
  const canCancel = ACTIVE_RUN_STATUSES.has(run.status);
  // Hide the sheet shortcut once output is inline — the transcript is the result.
  const showHeaderOpen = !!onAction && !hasTranscript;

  const body = (
    <div className="space-y-2">
      {/* Primary output: the agent's full transcript, rendered inline so a
          one-shot run's result is readable in the bar. Falls back to the live
          plan / terse summary only until transcript segments arrive. */}
      {expanded && hasTranscript ? (
        <div className="space-y-2 pl-4">
          {open && targetLabel(run) ? (
            <p className="text-[11px] text-muted-foreground">{targetLabel(run)}</p>
          ) : null}
          <DockTranscript stream={stream.streamState} active={isActive} showUserMessages={false} />
          {run.error_message ? (
            <p className="rounded bg-destructive/10 px-2 py-1 text-xs text-destructive">{run.error_message}</p>
          ) : null}
        </div>
      ) : expanded ? (
        <div className="space-y-2 pl-4 text-xs leading-snug">
          {targetLabel(run) ? (
            <p className="text-[11px] text-muted-foreground">{targetLabel(run)}</p>
          ) : null}
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
          ) : outputSummary ? (
            <p className="text-foreground/80">{outputSummary}</p>
          ) : null}
          {run.error_message ? (
            <p className="rounded bg-destructive/10 px-2 py-1 text-destructive">{run.error_message}</p>
          ) : null}
        </div>
      ) : null}

      {expanded && pendingInteraction ? (
        <DockInteractionPrompt id={`${run.id}:${pendingInteraction.interaction_id}`}>
          <PendingInteractionCard
            workspaceId={workspaceId}
            runId={run.id}
            interaction={pendingInteraction}
            onResolved={() => {
              if (!pendingInteraction) return;
              stream.clearPendingInteraction(pendingInteraction.interaction_id);
              void stream.refetch();
            }}
          />
        </DockInteractionPrompt>
      ) : null}

      {expanded && !hasTranscript ? resultSlot : null}

      {expanded && onAction ? (
        <ChipRow
          // When the inline approval card already provides the primary action,
          // demote Cancel/Open to a right-aligned meta row so the user's eye
          // stays on the actual decision (Approve / Request changes).
          align={pendingInteraction || canCancel ? 'end' : 'start'}
        >
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
          {(state === 'attention' || state === 'cancelled') && !awaitingApproval ? (
            <ActionChip
              icon={busy ? Loading01Icon : ArrowReloadHorizontalIcon}
              label={busy ? 'Retrying…' : 'Retry'}
              onClick={() => onAction('retry')}
              disabled={busy}
              retry
            />
          ) : null}
          {awaitingApproval && !pendingInteraction ? (
            <ActionChip
              icon={busy ? Loading01Icon : ArrowUpRight01Icon}
              label={busy ? 'Approving…' : 'Approve'}
              onClick={() => onAction('rerun')}
              disabled={busy}
              accent
            />
          ) : null}
          {/* Output is inline once the transcript is present, so the prominent
              "Open the session sheet" chip is only offered as a fallback. */}
          <ActionChip icon={ArrowUpRight01Icon} label="Open" onClick={() => onAction('open')} accent />
          {canCancel ? (
            <ActionChip
              icon={busy ? Loading01Icon : Cancel01Icon}
              label={busy ? 'Cancelling…' : 'Cancel'}
              onClick={() => onAction('cancel')}
              disabled={busy}
              subtle
            />
          ) : null}
        </ChipRow>
      ) : null}
    </div>
  );

  return (
    <div>
      <div className="group flex w-full items-center gap-1 rounded-md px-1 py-1 transition hover:bg-muted/40">
        <button
          type="button"
          onClick={() => setOpen(!open)}
          className="flex min-w-0 flex-1 items-center gap-2 text-left"
          aria-expanded={open}
        >
          <StatusDot state={dot} />
          <span className="text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
            Agent
          </span>
          <span className="text-muted-foreground/60">·</span>
          <span className="min-w-0 flex-1 truncate text-xs text-foreground/80">{summary}</span>
          {compact && !open ? (
            <span
              className={cn(
                'shrink-0 rounded-full px-1.5 py-0.5 text-[10px] font-medium',
                pillClasses(state),
              )}
            >
              {runStatusLabel(run)}
            </span>
          ) : null}
          <span className="shrink-0 text-[11px] text-muted-foreground">
            {duration != null ? formatDuration(duration) : formatDistanceToNow(ts, { addSuffix: true })}
          </span>
          <DisclosureChevron open={open} className="h-3 w-3" />
        </button>
        {showHeaderOpen ? (
          <button
            type="button"
            aria-label="Open agent run"
            onClick={() => onAction?.('open')}
            className="inline-flex h-6 w-6 shrink-0 items-center justify-center rounded text-muted-foreground transition hover:bg-background/80 hover:text-foreground"
          >
            <ArrowUpRight01Icon className="h-3.5 w-3.5" />
          </button>
        ) : null}
      </div>

      {/* Grid-rows trick: animates intrinsic content height between 0fr and
          1fr without measuring. Wrapper stays mounted through the transition
          so the pending-interaction card animates instead of popping. */}
      <div
        className={cn(
          'grid transition-[grid-template-rows,margin-top,opacity] duration-200 ease-out',
          expanded ? 'mt-2 grid-rows-[1fr] opacity-100' : 'grid-rows-[0fr] opacity-0',
        )}
        aria-hidden={!expanded}
      >
        <div className="overflow-hidden">{body}</div>
      </div>
    </div>
  );
}

function ChipRow({
  children,
  align = 'start',
}: {
  children: React.ReactNode;
  align?: 'start' | 'end';
}) {
  return (
    <div
      className={cn(
        'flex flex-wrap items-center gap-1.5 pt-0.5',
        align === 'end' && 'justify-end',
      )}
    >
      {children}
    </div>
  );
}

function ActionChip({
  icon: Icon,
  label,
  onClick,
  disabled,
  accent,
  subtle,
  danger,
  retry,
}: {
  icon: React.ComponentType<{ className?: string }>;
  label: string;
  onClick: () => void;
  disabled?: boolean;
  accent?: boolean;
  /** Borderless, transparent variant for low-emphasis meta actions. */
  subtle?: boolean;
  danger?: boolean;
  retry?: boolean;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      className={cn(
        'inline-flex items-center gap-1 rounded px-2 py-0.5 text-[11px] font-medium transition',
        accent
          ? 'border border-primary bg-primary text-primary-foreground hover:bg-primary/90'
          : danger
            ? 'border border-destructive/30 bg-transparent text-destructive hover:bg-destructive/10 hover:text-destructive'
            : retry
              ? 'border border-orange-500/30 bg-transparent text-orange-700 hover:bg-orange-500/10 dark:text-orange-300'
              : subtle
                ? 'text-muted-foreground hover:bg-muted/60 hover:text-foreground'
                : 'border border-border/70 bg-background/80 text-foreground hover:border-foreground/30 hover:bg-muted/60',
        disabled && 'cursor-not-allowed opacity-60',
      )}
    >
      <Icon className={cn('h-3 w-3', label.endsWith('…') ? 'animate-spin' : null)} />
      {label}
    </button>
  );
}
