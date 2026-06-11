import type { AgentRun, CommandBarPlanStep, CommandBarPlanSummary } from '@/lib/pmTypes';
import type { CommandBarRunPlan } from '@/components/agents/dock/planSummary';
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
