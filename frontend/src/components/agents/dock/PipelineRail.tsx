import type { AgentRun } from '@/lib/pmTypes';
import type { CommandBarRunPlan } from '@/stores/commandBarStore';
import { cn } from '@/lib/utils';
import { StatusDot } from './StatusDot';
import { stepDotState } from './utils';

interface PipelineRailProps {
  plan: CommandBarRunPlan;
  runsById: Record<string, AgentRun>;
}

export function PipelineRail({ plan, runsById }: PipelineRailProps) {
  if (plan.steps.length <= 1) return null;
  return (
    <div className="flex items-center gap-1 pt-1.5">
      {plan.steps.map((step, i) => {
        const state = stepDotState(plan, i, runsById);
        return (
          <div key={`${step.agent_id}-${i}`} className="flex flex-1 items-center gap-1">
            <StatusDot state={state} />
            <span
              className="truncate text-[10px] font-medium uppercase tracking-wider text-muted-foreground"
              title={step.agent_name}
            >
              {step.agent_name}
            </span>
            {i < plan.steps.length - 1 ? (
              <span
                aria-hidden
                className={cn(
                  'mx-0.5 h-px flex-1 bg-border/70',
                  state === 'completed' && 'bg-emerald-500/40',
                )}
              />
            ) : null}
          </div>
        );
      })}
    </div>
  );
}
