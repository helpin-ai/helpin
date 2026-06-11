import type { AgentRun, CommandBarPlanStep, CommandBarPlanSummary } from '@/lib/pmTypes';
import type { CommandBarRunPlan } from '@/components/agents/dock/planSummary';
import type { DotKind } from '@/components/agents/dock/StatusDot';
import {
  classifyPlan,
  describeStepTarget,
  stepDisplayName,
  stepDotState,
} from '@/components/agents/dock/utils';
import { ACTIVE_RUN_STATUSES } from '@/components/pm/agentRunConstants';

/**
 * Command-bar plan kinds that represent a multi-step epic *delivery* (run all
 * tasks: branch → build → review → merge → PR). Single one-shot commands and
 * simple fan-outs are excluded — those stay in the dock, not the epic page.
 */
export const DELIVERY_PLAN_KINDS = ['dag', 'task_pipeline_fan_out'] as const;

export function isDeliveryPlanKind(kind: CommandBarPlanSummary['plan_kind'] | undefined): boolean {
  return kind === 'dag' || kind === 'task_pipeline_fan_out';
}

/**
 * Chooses the plan to feature on the epic page: among delivery-kind plans,
 * prefer the most recent one that is still `running`, otherwise the most recent
 * overall. Returns null when no delivery plan exists.
 */
export function pickLatestDeliveryPlan(plans: CommandBarPlanSummary[]): CommandBarPlanSummary | null {
  const delivery = plans.filter((plan) => isDeliveryPlanKind(plan.plan_kind));
  if (delivery.length === 0) return null;
  const byNewest = (a: CommandBarPlanSummary, b: CommandBarPlanSummary) =>
    (b.created_at ?? '').localeCompare(a.created_at ?? '');
  const running = delivery.filter((plan) => plan.status === 'running').sort(byNewest);
  if (running.length > 0) return running[0];
  return [...delivery].sort(byNewest)[0];
}

/** Indexes a plan summary's hydrated runs by id for rail lookups. */
export function buildRunsById(plan: CommandBarPlanSummary): Record<string, AgentRun> {
  const map: Record<string, AgentRun> = {};
  for (const run of plan.runs ?? []) {
    map[run.id] = run;
  }
  return map;
}

/**
 * All run ids referenced by the plan's steps — used to decide whether an
 * incoming `agent_run-updated` event belongs to this plan (DAG child runs
 * target tasks, not the epic, so parent_id correlation is unreliable).
 */
export function planRunIdSet(plan: CommandBarRunPlan): Set<string> {
  return new Set(Object.values(plan.runIdsByStep ?? {}));
}

/**
 * Status-dot state for a delivery surface (panel header, tasks-view toggle,
 * sidebar chip). Running plans pulse like an active step. Unlike the dock's
 * neutral treatment of cancellation, a cancelled delivery reads as attention:
 * the delivery stopped before finishing and is retryable, and the lane
 * summaries already count cancelled lanes as failed.
 */
export function deliveryDotState(
  plan: CommandBarRunPlan,
  runsById: Record<string, AgentRun>,
): DotKind {
  const state = classifyPlan(plan, runsById);
  if (state === 'running') return 'active_step';
  if (state === 'cancelled') return 'attention';
  return state;
}

/**
 * Assigns each step an execution wave derived from `depends_on_step_indexes`:
 * wave 0 = no dependencies, wave N = 1 + the deepest wave among its deps.
 * Steps in the same wave run together (fan-out); cycles and out-of-range deps
 * are tolerated by ignoring the offending edge.
 */
export function computeStepWaves(steps: CommandBarPlanStep[]): number[] {
  const waves = new Array<number>(steps.length).fill(-1);
  const visiting = new Set<number>();
  const visit = (index: number): number => {
    if (waves[index] >= 0) return waves[index];
    if (visiting.has(index)) return 0;
    visiting.add(index);
    let wave = 0;
    for (const dep of steps[index]?.depends_on_step_indexes ?? []) {
      if (dep === index || dep < 0 || dep >= steps.length) continue;
      wave = Math.max(wave, visit(dep) + 1);
    }
    visiting.delete(index);
    waves[index] = wave;
    return wave;
  };
  for (let i = 0; i < steps.length; i++) visit(i);
  return waves;
}

export interface StepWaveGroup {
  wave: number;
  stepIndexes: number[];
}

/** Groups a plan's steps by execution wave, in wave order. */
export function groupStepsByWave(plan: CommandBarRunPlan): StepWaveGroup[] {
  const waves = computeStepWaves(plan.steps);
  const groups = new Map<number, StepWaveGroup>();
  waves.forEach((wave, stepIndex) => {
    const existing = groups.get(wave);
    if (existing) existing.stepIndexes.push(stepIndex);
    else groups.set(wave, { wave, stepIndexes: [stepIndex] });
  });
  return Array.from(groups.values()).sort((a, b) => a.wave - b.wave);
}

/**
 * A plan is stalled when the backend still considers it running but no step
 * run is actively executing and nothing has failed — typically after a worker
 * restart. Stalled plans can be revived via the resume endpoint.
 */
export function isPlanStalled(plan: CommandBarRunPlan, runsById: Record<string, AgentRun>): boolean {
  if (plan.status !== 'running') return false;
  const runs = Object.values(plan.runIdsByStep ?? {})
    .map((id) => runsById[id])
    .filter(Boolean);
  if (runs.some((run) => ACTIVE_RUN_STATUSES.has(run.status))) return false;
  if (runs.some((run) => run.status === 'failed')) return false;
  return true;
}

/** True when any step run failed or was cancelled — the delivery cannot finish on its own. */
export function planHasFailedSteps(
  plan: CommandBarRunPlan,
  runsById: Record<string, AgentRun>,
): boolean {
  return plan.steps.some((_, index) => {
    const state = stepDotState(plan, index, runsById);
    return state === 'attention' || state === 'cancelled';
  });
}

export interface DeliveryVerdict {
  tone: 'attention' | 'awaiting';
  text: string;
}

/**
 * One-line story of what the delivery needs, derived from step states. Null
 * when the delivery is healthy (running or finished clean) — the summary
 * counts cover that. Rendered under the panel header so a stalled or failed
 * delivery says what happened and what unblocks it, instead of making the
 * reader execute the dependency graph in their head.
 */
export function deliveryVerdict(
  plan: CommandBarRunPlan,
  runsById: Record<string, AgentRun>,
): DeliveryVerdict | null {
  const states = plan.steps.map((_, index) => stepDotState(plan, index, runsById));
  const failed = states.flatMap((state, i) => (state === 'attention' || state === 'cancelled' ? [i] : []));
  const blocked = states.flatMap((state, i) => (state === 'blocked' ? [i] : []));
  const awaiting = states.flatMap((state, i) => (state === 'awaiting' ? [i] : []));

  if (failed.length > 0) {
    const step = plan.steps[failed[0]];
    const verb = states[failed[0]] === 'cancelled' ? 'was cancelled' : 'failed';
    const target = describeStepTarget(step);
    const subject = target ? `${stepDisplayName(step)} on ${target}` : stepDisplayName(step);
    let text =
      failed.length === 1 ? `${subject} ${verb}` : `${subject} ${verb} (+${failed.length - 1} more)`;
    if (blocked.length > 0) {
      const names = blocked.slice(0, 2).map((i) => stepDisplayName(plan.steps[i]));
      const extra = blocked.length - names.length;
      text += ` — ${names.join(', ')}${extra > 0 ? ` +${extra} more` : ''} blocked downstream`;
    }
    if (plan.status !== 'running') text += '. Retry to continue.';
    return { tone: 'attention', text };
  }
  if (awaiting.length > 0) {
    const n = awaiting.length;
    return {
      tone: 'awaiting',
      text: `Waiting on you — ${n} step${n === 1 ? '' : 's'} need${n === 1 ? 's' : ''} approval or input.`,
    };
  }
  if (isPlanStalled(plan, runsById)) {
    return {
      tone: 'awaiting',
      text: 'Stalled — nothing is currently running. Resume to continue.',
    };
  }
  return null;
}

/**
 * Step-state counts for the delivery progress bar: done (green), failed
 * (red, includes cancelled), active (ember), rest (muted: queued, blocked,
 * awaiting).
 */
export function deliveryProgress(
  plan: CommandBarRunPlan,
  runsById: Record<string, AgentRun>,
): { done: number; failed: number; active: number; rest: number; total: number } {
  let done = 0;
  let failed = 0;
  let active = 0;
  for (let i = 0; i < plan.steps.length; i++) {
    const state = stepDotState(plan, i, runsById);
    if (state === 'completed') done++;
    else if (state === 'attention' || state === 'cancelled') failed++;
    else if (state === 'running' || state === 'active_step') active++;
  }
  const total = plan.steps.length;
  return { done, failed, active, rest: total - done - failed - active, total };
}

/** Wall-clock duration of a single run, or null when it hasn't started. */
export function runDurationMs(run: AgentRun): number | null {
  const start = Date.parse(run.started_at || run.created_at);
  if (!Number.isFinite(start)) return null;
  const end = ACTIVE_RUN_STATUSES.has(run.status)
    ? Date.now()
    : Date.parse(run.completed_at || run.updated_at || run.created_at);
  if (!Number.isFinite(end)) return null;
  return Math.max(0, end - start);
}
