import { formatDistanceToNow } from 'date-fns';

import type { AgentRun } from '@/lib/pmTypes';
import type { CommandBarRunPlan } from '@/components/agents/dock/planSummary';
import { DagRail } from './DagRail';
import { TaskPipelineRail } from './TaskPipelineRail';
import { StatusDot, type DotKind } from './StatusDot';
import { classifyPlan, planKindLabel, planSummaryText, planUpdatedAt } from './utils';

interface EpicDeliveryDagProps {
  plan: CommandBarRunPlan;
  runsById: Record<string, AgentRun>;
  onOpenRun: (runId: string) => void;
}

/**
 * Read-only, presentational view of a command-bar *delivery* plan (a `dag` or
 * `task_pipeline_fan_out`) for the epic page. Mirrors the dock's rail selection
 * but without the dock's streaming/interaction coupling. Clicking a step opens
 * that step's run in the coding-session drawer.
 */
export function EpicDeliveryDag({ plan, runsById, onOpenRun }: EpicDeliveryDagProps) {
  if (plan.planKind !== 'dag' && plan.planKind !== 'task_pipeline_fan_out') return null;

  const state = classifyPlan(plan, runsById);
  const dot: DotKind = state === 'running' ? 'active_step' : state;
  const label = planKindLabel(plan.planKind, plan.steps.length);
  const summary = planSummaryText(plan, runsById);
  const ts = planUpdatedAt(plan, runsById);

  const handleSelectStep = (stepIndex: number) => {
    const runId = plan.runIdsByStep[stepIndex];
    if (runId) onOpenRun(runId);
  };

  return (
    <div className="rounded-md border border-border/60 bg-muted/30 px-3 py-2.5">
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

      <div className="mt-1">
        {plan.planKind === 'task_pipeline_fan_out' ? (
          <TaskPipelineRail plan={plan} runsById={runsById} onSelectStep={handleSelectStep} />
        ) : (
          <DagRail plan={plan} runsById={runsById} onSelectStep={handleSelectStep} />
        )}
      </div>
    </div>
  );
}
