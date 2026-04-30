import type { AgentRun, CommandBarPlanStep } from '@/lib/pmTypes';
import type { CommandBarRunPlan } from '@/stores/commandBarStore';
import { ACTIVE_RUN_STATUSES, getAgentRunDisplayStatus } from '@/components/pm/agentRunConstants';

export type ActivityState = 'attention' | 'running' | 'queued' | 'completed';

export type PlanKindLabel =
  | 'One-shot'
  | 'Pipeline'
  | 'Fan-out'
  | 'Task pipeline'
  | 'Orchestrated'
  | 'Agent';

export type StepDotState = ActivityState | 'active_step' | 'blocked';

export function classifyRun(run: AgentRun): ActivityState {
  const displayStatus = getAgentRunDisplayStatus(run);
  if (
    displayStatus === 'awaiting_approval' ||
    displayStatus === 'awaiting_input' ||
    displayStatus === 'awaiting_auth'
  )
    return 'attention';
  if (run.status === 'failed' || run.status === 'cancelled') return 'attention';
  if (ACTIVE_RUN_STATUSES.has(run.status)) return 'running';
  if (run.status === 'completed') return 'completed';
  return 'queued';
}

export function classifyPlan(
  plan: CommandBarRunPlan,
  runsById: Record<string, AgentRun>,
): ActivityState {
  const runs = Object.values(plan.runIdsByStep)
    .map((id) => runsById[id])
    .filter(Boolean);
  if (plan.status === 'failed' || plan.status === 'cancelled') return 'attention';
  if (runs.some((r) => classifyRun(r) === 'attention')) return 'attention';
  if (plan.status === 'completed') return 'completed';
  if (runs.some((r) => ACTIVE_RUN_STATUSES.has(r.status))) return 'running';
  return 'queued';
}

export function runUpdatedAt(run: AgentRun): number {
  return Date.parse(run.updated_at || run.created_at) || 0;
}

export function planUpdatedAt(
  plan: CommandBarRunPlan,
  runsById: Record<string, AgentRun>,
): number {
  let latest = 0;
  for (const id of Object.values(plan.runIdsByStep)) {
    const run = runsById[id];
    if (run) latest = Math.max(latest, runUpdatedAt(run));
  }
  return latest || Date.now();
}

export function targetLabel(run: AgentRun): string {
  const t = run.target_info;
  if (t?.title) return t.title;
  if (t?.task_key) return t.task_key;
  if (run.target_type)
    return `${run.target_type.replaceAll('_', ' ')} ${run.target_id.slice(0, 8)}`;
  return run.target_id.slice(0, 8);
}

export function planKindLabel(
  kind: CommandBarRunPlan['planKind'] | undefined,
  stepCount: number,
): PlanKindLabel {
  if (kind === 'task_pipeline_fan_out') return 'Task pipeline';
  if (kind === 'dag') return 'Orchestrated';
  if (kind === 'fan_out') return 'Fan-out';
  if (kind === 'one_shot_command') return stepCount > 1 ? 'Pipeline' : 'One-shot';
  if (stepCount > 1) return 'Pipeline';
  return 'Agent';
}

/**
 * Shared dot state for an indexed step within a plan. Honors
 * `depends_on_step_indexes`: if any dep step's run isn't completed, the step
 * is considered blocked rather than merely queued.
 */
export function stepDotState(
  plan: CommandBarRunPlan,
  index: number,
  runsById: Record<string, AgentRun>,
): StepDotState {
  const step = plan.steps[index];
  const runId = plan.runIdsByStep[index];
  const run = runId ? runsById[runId] : null;
  const isCurrent = (plan.currentStepIndex ?? 0) === index;

  if (run) {
    if (run.status === 'completed') return 'completed';
    if (run.status === 'failed' || run.status === 'cancelled') return 'attention';
    if (ACTIVE_RUN_STATUSES.has(run.status)) return isCurrent ? 'active_step' : 'running';
  }

  const deps = step?.depends_on_step_indexes;
  if (deps && deps.length > 0) {
    const allDepsDone = deps.every((d) => {
      const depRunId = plan.runIdsByStep[d];
      const depRun = depRunId ? runsById[depRunId] : null;
      return depRun?.status === 'completed';
    });
    if (!allDepsDone) return 'blocked';
  }

  if (isCurrent && plan.status === 'running') return 'active_step';
  return 'queued';
}

export interface TaskPipelineGroup {
  key: string;
  title: string;
  stepIndexes: number[];
}

/**
 * Group steps by their target entity for task_pipeline_fan_out plans. Each
 * group represents one task's serial pipeline (e.g. Forge → Lens for HLP-123).
 */
export function groupStepsByTarget(plan: CommandBarRunPlan): TaskPipelineGroup[] {
  const groups = new Map<string, TaskPipelineGroup>();
  plan.steps.forEach((step, index) => {
    const key = step.target?.entity_id ?? `step-${index}`;
    const title = step.target?.display_title ?? key;
    const existing = groups.get(key);
    if (existing) existing.stepIndexes.push(index);
    else groups.set(key, { key, title, stepIndexes: [index] });
  });
  return Array.from(groups.values());
}

export function describeStepTarget(step: CommandBarPlanStep): string {
  return step.target?.display_title ?? step.target?.entity_id ?? '';
}

/**
 * Goal extraction for one-shot brief instructions of the form:
 *   "One-shot execution brief\nGoal:\n{goal}\nPlan:\n..."
 * Falls back to the raw instruction text when no Goal: section exists.
 */
export function describeStep(instructions: string | undefined | null): string {
  const text = (instructions ?? '').trim();
  if (!text) return '';
  if (!text.toLowerCase().includes('goal:')) return text;
  const lines = text.split('\n');
  const goalIdx = lines.findIndex((l) => l.trim().toLowerCase() === 'goal:');
  if (goalIdx < 0) return text;
  const out: string[] = [];
  for (let i = goalIdx + 1; i < lines.length; i++) {
    const line = lines[i].trim();
    if (!line) continue;
    const lower = line.toLowerCase();
    if (
      lower === 'plan:' ||
      lower === 'constraints:' ||
      lower.startsWith('target:') ||
      lower.startsWith('user request:')
    )
      break;
    out.push(line.replace(/^[-*]\s+/, '').replace(/^\d+\.\s+/, ''));
  }
  return out.join(' ').trim() || text;
}

export function outputSummaryText(run: AgentRun): string {
  const summary = run.output_summary;
  if (!summary || Object.keys(summary).length === 0) return '';
  const preferred = summary.summary ?? summary.message ?? summary.result;
  if (typeof preferred === 'string' && preferred.trim().length > 0) return preferred;
  return '';
}

export function runStatusLabel(run: AgentRun): string {
  const display = getAgentRunDisplayStatus(run);
  if (display === 'awaiting_approval') return 'Awaiting approval';
  if (display === 'awaiting_input') return 'Awaiting input';
  if (display === 'awaiting_auth') return 'Awaiting sign-in';
  return run.status.charAt(0).toUpperCase() + run.status.slice(1);
}

export function planSummaryText(
  plan: CommandBarRunPlan,
  runsById: Record<string, AgentRun>,
): string {
  const runs = Object.values(plan.runIdsByStep)
    .map((id) => runsById[id])
    .filter(Boolean);
  const total = plan.steps.length;
  const done = runs.filter((r) => r.status === 'completed').length;
  const active = runs.filter((r) => ACTIVE_RUN_STATUSES.has(r.status)).length;
  const failed = runs.filter((r) => r.status === 'failed' || r.status === 'cancelled').length;

  if (plan.planKind === 'task_pipeline_fan_out') {
    const groups = groupStepsByTarget(plan);
    const taskCount = groups.length;
    let tasksDone = 0;
    let tasksActive = 0;
    let tasksFailed = 0;
    let tasksWaiting = 0;
    for (const g of groups) {
      const stepStates = g.stepIndexes.map((i) => stepDotState(plan, i, runsById));
      if (stepStates.some((s) => s === 'attention')) tasksFailed++;
      else if (stepStates.every((s) => s === 'completed')) tasksDone++;
      else if (stepStates.some((s) => s === 'running' || s === 'active_step'))
        tasksActive++;
      else tasksWaiting++;
    }
    const parts: string[] = [`${taskCount} task${taskCount === 1 ? '' : 's'}`];
    if (tasksActive) parts.push(`${tasksActive} running`);
    if (tasksWaiting) parts.push(`${tasksWaiting} waiting`);
    if (tasksDone) parts.push(`${tasksDone} done`);
    if (tasksFailed) parts.push(`${tasksFailed} failed`);
    return parts.join(' · ');
  }
  if (plan.planKind === 'dag') {
    let waiting = 0;
    let queued = 0;
    let running = 0;
    let completed = 0;
    let failedCount = 0;
    for (let i = 0; i < plan.steps.length; i++) {
      const state = stepDotState(plan, i, runsById);
      if (state === 'blocked') waiting++;
      else if (state === 'queued') queued++;
      else if (state === 'running' || state === 'active_step') running++;
      else if (state === 'completed') completed++;
      else if (state === 'attention') failedCount++;
    }
    const parts: string[] = [`${total} steps`];
    if (running) parts.push(`${running} running`);
    if (waiting) parts.push(`${waiting} waiting`);
    if (queued) parts.push(`${queued} queued`);
    if (completed) parts.push(`${completed} done`);
    if (failedCount) parts.push(`${failedCount} failed`);
    return parts.join(' · ');
  }
  if (plan.planKind === 'fan_out') {
    const parts: string[] = [`${total} agents`];
    if (done) parts.push(`${done} done`);
    if (active) parts.push(`${active} running`);
    if (failed) parts.push(`${failed} failed`);
    return parts.join(' — ');
  }
  if (total > 1) {
    const idx = Math.min((plan.currentStepIndex ?? done) + 1, total);
    if (plan.status === 'completed') return `${total} steps`;
    return `step ${idx} of ${total}`;
  }
  // single step → summary from its run if any
  const only = runs[0];
  if (only) return outputSummaryText(only) || runStatusLabel(only);
  return '';
}

export function totalDurationMs(
  plan: CommandBarRunPlan | null,
  runsById: Record<string, AgentRun>,
  fallbackRun?: AgentRun,
): number | null {
  const runs = plan
    ? Object.values(plan.runIdsByStep)
        .map((id) => runsById[id])
        .filter(Boolean)
    : fallbackRun
      ? [fallbackRun]
      : [];
  if (runs.length === 0) return null;
  let earliest = Number.POSITIVE_INFINITY;
  let latest = 0;
  for (const r of runs) {
    const start = Date.parse(r.started_at || r.created_at);
    const end = Date.parse(r.completed_at || r.updated_at || r.created_at);
    if (Number.isFinite(start)) earliest = Math.min(earliest, start);
    if (Number.isFinite(end)) latest = Math.max(latest, end);
  }
  if (!Number.isFinite(earliest) || latest === 0) return null;
  return Math.max(0, latest - earliest);
}

export function formatDuration(ms: number): string {
  const seconds = Math.round(ms / 1000);
  if (seconds < 60) return `${seconds}s`;
  const m = Math.floor(seconds / 60);
  const s = seconds % 60;
  if (m < 60) return s ? `${m}m ${s}s` : `${m}m`;
  const h = Math.floor(m / 60);
  const rm = m % 60;
  return rm ? `${h}h ${rm}m` : `${h}h`;
}

export type ListGroupKey = 'active' | 'today' | 'yesterday' | 'earlier_this_week' | 'older';

export function listGroupKey(now: Date, when: Date): Exclude<ListGroupKey, 'active'> {
  const startOfDay = (d: Date) => {
    const c = new Date(d);
    c.setHours(0, 0, 0, 0);
    return c;
  };
  const today = startOfDay(now);
  const whenDay = startOfDay(when);
  const diffDays = Math.round((today.getTime() - whenDay.getTime()) / 86_400_000);
  if (diffDays <= 0) return 'today';
  if (diffDays === 1) return 'yesterday';
  if (diffDays <= 6) return 'earlier_this_week';
  return 'older';
}

export const LIST_GROUP_LABEL: Record<ListGroupKey, string> = {
  active: 'Active now',
  today: 'Today',
  yesterday: 'Yesterday',
  earlier_this_week: 'Earlier this week',
  older: 'Older',
};

export function shortId(id: string): string {
  return id.slice(0, 8);
}
