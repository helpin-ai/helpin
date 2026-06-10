import type { AgentRun, CommandBarPlanSummary } from '@/lib/pmTypes';
import type { CommandBarRunPlan } from '@/components/agents/dock/planSummary';

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
