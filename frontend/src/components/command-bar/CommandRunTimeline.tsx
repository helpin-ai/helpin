import { useMemo } from 'react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { ACTIVE_RUN_STATUSES, getAgentRunDisplayStatus } from '@/components/pm/agentRunConstants';
import { ArrowUpRight01Icon } from '@/lib/icons';
import { cn } from '@/lib/utils';
import type { AgentRun, CommandBarPageContext, CommandBarPlanStep } from '@/lib/pmTypes';
import type { CommandBarRunPlan } from '@/stores/commandBarStore';

export type RunAction = 'approve' | 'cancel';

interface CommandPlanTimelineProps {
  plan: CommandBarRunPlan;
  runsById: Record<string, AgentRun>;
  busyPlanId: string | null;
  busyRunId: string | null;
  onOpenRun: (runId: string) => void;
  onRunAction: (run: AgentRun, action: RunAction) => void;
  onCancelPlan: (planId: string) => void;
  onRetryPlan: (planId: string, stepIndex: number) => void;
  onPromoteRun: (run: AgentRun) => void;
}

interface StandaloneRunTimelineProps {
  run: AgentRun;
  busyRunId: string | null;
  onOpenRun: (runId: string) => void;
  onRunAction: (run: AgentRun, action: RunAction) => void;
}

type NodeState = 'queued' | 'running' | 'completed' | 'failed' | 'cancelled' | 'approval' | 'paused';

const AGENT_ACCENTS: Record<string, { text: string; dot: string; pulse: string }> = {
  code_builder: {
    text: 'text-orange-800 dark:text-orange-300',
    dot: 'bg-orange-600',
    pulse: 'border-orange-500',
  },
  review_agent: {
    text: 'text-teal-800 dark:text-teal-300',
    dot: 'bg-teal-600',
    pulse: 'border-teal-500',
  },
  task_planner: {
    text: 'text-violet-800 dark:text-violet-300',
    dot: 'bg-violet-600',
    pulse: 'border-violet-500',
  },
  researcher: {
    text: 'text-sky-800 dark:text-sky-300',
    dot: 'bg-sky-600',
    pulse: 'border-sky-500',
  },
  crm_operator: {
    text: 'text-emerald-800 dark:text-emerald-300',
    dot: 'bg-emerald-600',
    pulse: 'border-emerald-500',
  },
  support_agent: {
    text: 'text-amber-800 dark:text-amber-300',
    dot: 'bg-amber-600',
    pulse: 'border-amber-500',
  },
};

const DEFAULT_ACCENT = {
  text: 'text-foreground',
  dot: 'bg-slate-500 dark:bg-slate-400',
  pulse: 'border-slate-400',
};

export function CommandPlanTimeline({
  plan,
  runsById,
  busyPlanId,
  busyRunId,
  onOpenRun,
  onRunAction,
  onCancelPlan,
  onRetryPlan,
  onPromoteRun,
}: CommandPlanTimelineProps) {
  const steps = plan.steps;
  const isFanOut = plan.planKind === 'fan_out' || steps.some((step) => step.plan_kind === 'fan_out');
  const isOneShot = plan.planKind === 'one_shot_command' || steps.some((step) => step.plan_kind === 'one_shot_command');
  const stepRuns = useMemo(
    () => steps.map((_, index) => {
      const runId = plan.runIdsByStep[index];
      return runId ? runsById[runId] : undefined;
    }),
    [plan.runIdsByStep, runsById, steps],
  );
  const aggregate = summarizePlan(plan, stepRuns);
  const title = planTitle(plan, stepRuns);
  const subtitle = isFanOut
    ? `${steps.length} targets`
    : isOneShot
    ? 'one-shot'
    : `${steps.length} ${steps.length === 1 ? 'step' : 'steps'}`;
  const retryLabel = isFanOut ? 'Retry failed' : 'Retry';

  return (
    <article className="group/plan border-b border-border/60 px-3 py-3 last:border-b-0 hover:bg-muted/30">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="flex min-w-0 items-center gap-2">
            <h3 className="truncate text-[13px] font-medium leading-5 text-foreground">{title}</h3>
            {isFanOut ? <Badge variant="outline" className="h-5 px-1.5 text-[10px]">fan-out</Badge> : null}
            {isOneShot ? <Badge variant="outline" className="h-5 px-1.5 text-[10px]">one-shot</Badge> : null}
          </div>
          <p className="mt-0.5 truncate text-[11px] leading-4 text-muted-foreground">
            {subtitle}
            {aggregate ? ` · ${aggregate}` : ''}
          </p>
        </div>
        <div className="flex shrink-0 items-center gap-1">
          {plan.status === 'running' ? (
            <Button type="button" variant="ghost" size="sm" className="h-7 px-2 text-[11px]" disabled={busyPlanId === plan.id} onClick={() => onCancelPlan(plan.id)}>
              Cancel
            </Button>
          ) : null}
          {plan.status === 'failed' || plan.status === 'cancelled' ? (
            <Button type="button" variant="ghost" size="sm" className="h-7 px-2 text-[11px]" disabled={busyPlanId === plan.id} onClick={() => onRetryPlan(plan.id, plan.currentStepIndex ?? 0)}>
              {retryLabel}
            </Button>
          ) : null}
        </div>
      </div>

      <div className={cn('relative mt-2 pl-5', steps.length > 1 ? 'before:absolute before:left-[5px] before:top-3 before:bottom-3 before:w-px before:bg-border' : '')}>
        {steps.map((step, index) => {
          const run = stepRuns[index];
          const previousRun = index > 0 ? stepRuns[index - 1] : undefined;
          const waitingLabel = isFanOut ? 'queued' : previousRun && ACTIVE_RUN_STATUSES.has(previousRun.status) ? `waits ${index}` : 'queued';
          return (
            <CommandRunNode
              key={`${plan.id}-${index}`}
              step={step}
              run={run}
              index={index}
              total={steps.length}
              branch={isFanOut}
              statusText={run ? runStatusText(run) : waitingLabel}
              busy={!!run && busyRunId === run.id}
              onOpenRun={onOpenRun}
              onRunAction={onRunAction}
              onPromoteRun={onPromoteRun}
            />
          );
        })}
      </div>
    </article>
  );
}

export function StandaloneRunTimeline({ run, busyRunId, onOpenRun, onRunAction }: StandaloneRunTimelineProps) {
  const pseudoStep: CommandBarPlanStep = {
    agent_id: run.agent_id,
    agent_name: agentNameForRun(run),
    target: {
      entity_type: commandBarEntityTypeForRun(run),
      entity_id: run.target_id,
      display_title: targetLabel(run),
    },
    instructions: targetLabel(run),
  };

  if (run.status === 'running') {
    return <RunningRunHero run={run} onOpenRun={onOpenRun} onRunAction={onRunAction} busy={busyRunId === run.id} />;
  }

  return (
    <article className="border-b border-border/60 px-3 py-3 last:border-b-0 hover:bg-muted/30">
      <div className="relative pl-5">
        <CommandRunNode
          step={pseudoStep}
          run={run}
          index={0}
          total={1}
          branch={false}
          statusText={runStatusText(run)}
          busy={busyRunId === run.id}
          onOpenRun={onOpenRun}
          onRunAction={onRunAction}
        />
      </div>
    </article>
  );
}

function RunningRunHero({
  run,
  onOpenRun,
  onRunAction,
  busy,
}: {
  run: AgentRun;
  onOpenRun: (runId: string) => void;
  onRunAction: (run: AgentRun, action: RunAction) => void;
  busy: boolean;
}) {
  const accent = AGENT_ACCENTS[normalizeAgentName(agentNameForRun(run))] ?? DEFAULT_ACCENT;
  const agentName = agentNameForRun(run);
  const role = run.runtime_kind === 'native_sdk' ? 'default role' : (run.runtime_kind || '').replaceAll('_', ' ') || 'default role';
  const targetTag = targetTypeLabel(run.target_type);
  const activity = (run.execution_stage?.trim()) || 'working';
  const elapsed = runDuration(run) ?? 'running';

  return (
    <article className="border-b border-border/60 px-3 py-3 last:border-b-0 hover:bg-muted/20">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="flex min-w-0 items-baseline gap-1.5 text-[13px] leading-5">
            <span className={cn('font-medium', accent.text)}>{agentName}</span>
            <span className="text-muted-foreground">·</span>
            <span className="font-medium text-foreground">{role}</span>
            <span className="text-[11px] font-normal text-muted-foreground">{targetTag}</span>
          </div>
        </div>
        <div className="flex shrink-0 items-center gap-1.5 text-[11px] tabular-nums text-muted-foreground">
          <span className={cn('h-1.5 w-1.5 rounded-full', accent.dot)} />
          <span>{elapsed}</span>
        </div>
      </div>

      <div className="mt-2.5 flex items-center gap-2">
        <span className={cn('h-2 w-2 shrink-0 rounded-full', accent.dot)} />
        <div className="min-w-0 flex-1 truncate text-[12.5px] leading-4">
          <span className={cn('mr-1 font-medium', accent.text)}>{agentName}</span>
          <span className="text-foreground/80">{activity}</span>
        </div>
      </div>
      <div className="relative mt-2 h-px w-full overflow-hidden bg-border">
        <span className={cn('absolute left-0 top-0 h-px w-2/5 motion-safe:animate-pulse', accent.dot)} />
      </div>

      <div className="mt-2 flex items-center justify-between text-[11px] leading-4 text-muted-foreground">
        <span className="truncate">streaming</span>
        <div className="flex shrink-0 items-center gap-2">
          {ACTIVE_RUN_STATUSES.has(run.status) ? (
            <button
              type="button"
              disabled={busy}
              onClick={() => onRunAction(run, 'cancel')}
              className="text-muted-foreground hover:text-foreground disabled:opacity-50"
            >
              Cancel
            </button>
          ) : null}
          <button
            type="button"
            onClick={() => onOpenRun(run.id)}
            className="inline-flex items-center gap-0.5 text-foreground hover:text-foreground/80"
          >
            Open <ArrowUpRight01Icon className="h-3 w-3" />
          </button>
        </div>
      </div>
    </article>
  );
}

function agentNameForRun(run: AgentRun): string {
  const fromInput = (run.input as Record<string, unknown> | undefined)?.['agent_name'];
  if (typeof fromInput === 'string' && fromInput.trim()) return fromInput.trim();
  const fromTitle = run.target_info?.title;
  if (fromTitle) return 'Agent';
  return 'Agent';
}

function CommandRunNode({
  step,
  run,
  index,
  total,
  branch,
  statusText,
  busy,
  onOpenRun,
  onRunAction,
  onPromoteRun,
}: {
  step: CommandBarPlanStep;
  run?: AgentRun;
  index: number;
  total: number;
  branch: boolean;
  statusText: string;
  busy: boolean;
  onOpenRun: (runId: string) => void;
  onRunAction: (run: AgentRun, action: RunAction) => void;
  onPromoteRun?: (run: AgentRun) => void;
}) {
  const state = run ? nodeStateForRun(run) : 'queued';
  const accent = agentAccent(step);
  const isOneShot = step.plan_kind === 'one_shot_command';
  const awaitingApproval = run && getAgentRunDisplayStatus(run) === 'awaiting_approval';
  const canPromote = !!run && run.status === 'completed' && isOneShot && !!onPromoteRun;
  const description = stepDescription(step, run);
  const target = run ? targetLabel(run) : step.target.display_title || step.target.entity_id;

  return (
    <div className={cn('relative py-1.5', branch ? 'ml-4 pl-3 before:absolute before:left-[-14px] before:top-[17px] before:h-px before:w-4 before:bg-border' : '')}>
      <TimelineDot state={state} accent={accent} branch={branch} />
      <div className="min-w-0">
        <div className="flex items-start gap-2">
          <div className="min-w-0 flex-1">
            <div className="flex min-w-0 items-center gap-1.5">
              {branch ? <span className="shrink-0 rounded bg-muted px-1.5 py-0.5 text-[10px] font-medium uppercase text-muted-foreground">{targetKindLabel(step.target.entity_type, index)}</span> : null}
              <span className={cn('shrink-0 whitespace-nowrap text-[12.5px] font-medium leading-5', accent.text)}>{step.agent_name || 'Agent'}</span>
              <span className="min-w-0 truncate text-[12.5px] leading-5 text-muted-foreground">{description}</span>
            </div>
            <div className="mt-0.5 flex min-w-0 items-center gap-1.5 text-[11px] leading-4 text-muted-foreground">
              <span className="truncate">{targetTypeLabel(step.target.entity_type)} · {target}</span>
              {isOneShot && step.allowed_tools?.length ? <span className="shrink-0">· {step.allowed_tools.length} tools</span> : null}
              {!branch && total > 1 ? <span className="shrink-0">· step {index + 1}</span> : null}
            </div>
          </div>
          <span className={cn('shrink-0 pt-0.5 text-[11px] leading-5 tabular-nums', statusTone(state))}>{statusText}</span>
        </div>

        {awaitingApproval ? (
          <div className="mt-2 flex items-center justify-between gap-2 rounded-md border border-amber-500/25 bg-amber-500/10 px-2 py-1.5">
            <div className="flex items-center gap-2 text-[11px] text-amber-800 dark:text-amber-300">
              <span className="h-2 w-2 rotate-45 rounded-[1px] border border-amber-500 bg-background" />
              Approval needed
            </div>
            <Button type="button" variant="outline" size="sm" className="h-6 px-2 text-[11px]" disabled={busy} onClick={() => onRunAction(run, 'approve')}>
              Approve
            </Button>
          </div>
        ) : null}

        {run ? (
          <div className="mt-1.5 flex items-center gap-1 opacity-100 sm:opacity-0 sm:transition-opacity sm:group-hover/plan:opacity-100">
            {ACTIVE_RUN_STATUSES.has(run.status) ? (
              <Button type="button" variant="ghost" size="sm" className="h-6 px-2 text-[11px]" disabled={busy} onClick={() => onRunAction(run, 'cancel')}>
                Cancel
              </Button>
            ) : null}
            {canPromote ? (
              <Button type="button" variant="ghost" size="sm" className="h-6 px-2 text-[11px]" onClick={() => onPromoteRun?.(run)}>
                Save as agent
              </Button>
            ) : null}
            <Button type="button" variant="ghost" size="sm" className="h-6 px-2 text-[11px]" onClick={() => onOpenRun(run.id)}>
              Open
            </Button>
          </div>
        ) : null}
      </div>
    </div>
  );
}

function TimelineDot({ state, accent, branch }: { state: NodeState; accent: typeof DEFAULT_ACCENT; branch: boolean }) {
  const base = branch ? 'left-[-3px]' : 'left-[-19px]';
  if (state === 'approval') {
    return <span className={cn('absolute top-[13px] h-2.5 w-2.5 rotate-45 rounded-[2px] border border-amber-500 bg-background', base)} />;
  }
  if (state === 'queued') {
    return <span className={cn('absolute top-[13px] h-2.5 w-2.5 rounded-full border-[1.5px] border-border bg-background', base)} />;
  }
  if (state === 'running') {
    return (
      <span className={cn('absolute top-[13px] h-2.5 w-2.5 rounded-full', accent.dot, base)}>
        <span className={cn('absolute inset-[-5px] rounded-full border opacity-60 motion-safe:animate-ping', accent.pulse)} />
      </span>
    );
  }
  if (state === 'completed') {
    return <span className={cn('absolute top-[13px] h-2.5 w-2.5 rounded-full', accent.dot, base)} />;
  }
  if (state === 'failed') {
    return <span className={cn('absolute top-[13px] h-2.5 w-2.5 rounded-full bg-red-500', base)} />;
  }
  if (state === 'cancelled') {
    return <span className={cn('absolute top-[13px] h-2.5 w-2.5 rounded-full border border-muted-foreground/50 bg-muted', base)} />;
  }
  return <span className={cn('absolute top-[13px] h-2.5 w-2.5 rounded-full border border-violet-500 bg-violet-500/20', base)} />;
}

function nodeStateForRun(run: AgentRun): NodeState {
  const display = getAgentRunDisplayStatus(run);
  if (display === 'awaiting_approval') return 'approval';
  if (run.status === 'running') return 'running';
  if (run.status === 'queued') return 'queued';
  if (run.status === 'completed') return 'completed';
  if (run.status === 'failed') return 'failed';
  if (run.status === 'cancelled') return 'cancelled';
  return 'paused';
}

function statusTone(state: NodeState) {
  switch (state) {
    case 'running':
      return 'font-medium text-orange-700 dark:text-orange-300';
    case 'completed':
      return 'text-emerald-700 dark:text-emerald-300';
    case 'failed':
      return 'font-medium text-red-700 dark:text-red-300';
    case 'approval':
      return 'font-medium text-amber-700 dark:text-amber-300';
    default:
      return 'text-muted-foreground';
  }
}

function agentAccent(step: CommandBarPlanStep) {
  return AGENT_ACCENTS[step.agent_key ?? ''] ?? AGENT_ACCENTS[normalizeAgentName(step.agent_name)] ?? DEFAULT_ACCENT;
}

function normalizeAgentName(name: string) {
  const lower = name.toLowerCase();
  if (lower.includes('forge') || lower.includes('code')) return 'code_builder';
  if (lower.includes('lens') || lower.includes('review')) return 'review_agent';
  if (lower.includes('atlas') || lower.includes('planner') || lower.includes('task')) return 'task_planner';
  if (lower.includes('command')) return 'researcher';
  if (lower.includes('crm')) return 'crm_operator';
  if (lower.includes('support')) return 'support_agent';
  return '';
}

function stepDescription(step: CommandBarPlanStep, run?: AgentRun) {
  const source = step.instructions || run?.execution_stage || step.target.display_title || step.agent_name;
  const trimmed = source.trim();
  if (!trimmed) return '';
  return trimmed.length > 96 ? `${trimmed.slice(0, 93)}...` : trimmed;
}

function planTitle(plan: CommandBarRunPlan, runs: Array<AgentRun | undefined>) {
  if (plan.planKind === 'fan_out') {
    const agent = plan.steps[0]?.agent_name ?? 'Agent';
    const target = targetTypeLabel(plan.steps[0]?.target.entity_type ?? 'target');
    return `${agent} across ${plan.steps.length} ${target.toLowerCase()}${plan.steps.length === 1 ? '' : 's'}`;
  }
  if (plan.steps.length > 1) {
    return plan.steps.map((step) => step.agent_name).filter(Boolean).join(' -> ') || plan.prompt || 'Command plan';
  }
  if (plan.planKind === 'one_shot_command') return 'Command Agent';
  return runs[0] ? targetLabel(runs[0]) : plan.prompt || plan.steps[0]?.agent_name || 'Command plan';
}

function summarizePlan(plan: CommandBarRunPlan, runs: Array<AgentRun | undefined>) {
  if (plan.planKind === 'fan_out') {
    const counts = countRunStates(runs);
    return [
      counts.running ? `${counts.running} running` : '',
      counts.completed ? `${counts.completed} done` : '',
      counts.failed ? `${counts.failed} failed` : '',
      counts.queued ? `${counts.queued} queued` : '',
    ].filter(Boolean).join(' · ');
  }
  if (plan.status === 'failed') return 'failed';
  if (plan.status === 'cancelled') return 'cancelled';
  if (plan.status === 'completed') return 'completed';
  const active = runs.find((run) => run && ACTIVE_RUN_STATUSES.has(run.status));
  if (active) return runStatusText(active);
  return 'queued';
}

function countRunStates(runs: Array<AgentRun | undefined>) {
  return runs.reduce(
    (acc, run) => {
      if (!run) acc.queued += 1;
      else if (run.status === 'completed') acc.completed += 1;
      else if (run.status === 'failed' || run.status === 'cancelled') acc.failed += 1;
      else if (ACTIVE_RUN_STATUSES.has(run.status)) acc.running += 1;
      else acc.queued += 1;
      return acc;
    },
    { queued: 0, running: 0, completed: 0, failed: 0 },
  );
}

function runStatusText(run: AgentRun) {
  const display = getAgentRunDisplayStatus(run);
  if (display === 'awaiting_approval') return 'approval';
  if (display === 'awaiting_input') return 'input';
  if (display === 'awaiting_auth') return 'sign-in';
  if (run.status === 'completed') return runDuration(run) ?? 'done';
  if (run.status === 'running') return runDuration(run) ?? 'running';
  return run.status;
}

function runDuration(run: AgentRun) {
  const start = run.started_at ? Date.parse(run.started_at) : run.created_at ? Date.parse(run.created_at) : NaN;
  if (Number.isNaN(start)) return null;
  const end = run.completed_at ? Date.parse(run.completed_at) : Date.now();
  const seconds = Math.max(0, Math.floor((end - start) / 1000));
  if (seconds < 60) return `${seconds}s`;
  const minutes = Math.floor(seconds / 60);
  const rem = seconds % 60;
  if (minutes < 60) return `${minutes}m ${String(rem).padStart(2, '0')}s`;
  return `${Math.floor(minutes / 60)}h ${minutes % 60}m`;
}

function targetLabel(run: AgentRun) {
  return run.target_info?.title || run.target_info?.task_key || `${targetTypeLabel(run.target_type)} ${run.target_id.slice(0, 8)}`;
}

function targetTypeLabel(type: string) {
  return type.replaceAll('_', ' ');
}

function targetKindLabel(type: string, index: number) {
  const shortType = type === 'crm_contact' ? 'contact' : type === 'crm_deal' ? 'deal' : type.replace('_', '');
  return shortType.slice(0, 3) || String(index + 1);
}

function commandBarEntityTypeForRun(run: AgentRun): CommandBarPageContext['entity_type'] {
  switch (run.target_type) {
    case 'task':
    case 'epic':
    case 'document':
    case 'crm_deal':
    case 'workspace':
      return run.target_type;
    default:
      return 'workspace';
  }
}
