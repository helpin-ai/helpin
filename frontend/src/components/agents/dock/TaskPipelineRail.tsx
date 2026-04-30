import type { AgentRun } from '@/lib/pmTypes';
import type { CommandBarRunPlan } from '@/stores/commandBarStore';
import { cn } from '@/lib/utils';
import { StatusDot } from './StatusDot';
import { groupStepsByTarget, stepDotState } from './utils';

interface TaskPipelineRailProps {
  plan: CommandBarRunPlan;
  runsById: Record<string, AgentRun>;
  /** When set, render at most this many task rows and append a "+N more" row. */
  maxRows?: number;
}

/**
 * Renders a fan-out of serial pipelines: one row per target task, each row a
 * mini chain of agent steps (e.g. Forge → Lens). Steps blocked on unfinished
 * dependencies render with a dashed dot from `stepDotState`.
 */
export function TaskPipelineRail({ plan, runsById, maxRows = 6 }: TaskPipelineRailProps) {
  const groups = groupStepsByTarget(plan);
  if (groups.length === 0) return null;
  const visible = groups.slice(0, maxRows);
  const hidden = groups.length - visible.length;

  return (
    <div className="flex flex-col gap-1 pt-1.5">
      {visible.map((group) => (
        <div key={group.key} className="flex items-center gap-2">
          <span
            className="w-24 shrink-0 truncate text-[10px] font-medium text-muted-foreground"
            title={group.title}
          >
            {group.title}
          </span>
          <div className="flex flex-1 items-center gap-1">
            {group.stepIndexes.map((stepIndex, i) => {
              const step = plan.steps[stepIndex];
              const state = stepDotState(plan, stepIndex, runsById);
              return (
                <div key={`${step.agent_id}-${stepIndex}`} className="flex items-center gap-1">
                  <StatusDot state={state} />
                  <span
                    className="truncate text-[10px] font-medium uppercase tracking-wider text-muted-foreground"
                    title={step.agent_name}
                  >
                    {step.agent_name}
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
        </div>
      ))}
      {hidden > 0 ? (
        <span className="text-[10px] font-medium text-muted-foreground">
          +{hidden} more task{hidden === 1 ? '' : 's'}
        </span>
      ) : null}
    </div>
  );
}
