import type { AgentRun } from '@/lib/pmTypes';
import type { CommandBarRunPlan } from '@/stores/commandBarStore';
import { cn } from '@/lib/utils';
import { StatusDot } from './StatusDot';
import { stepDotState } from './utils';

interface DagRailProps {
  plan: CommandBarRunPlan;
  runsById: Record<string, AgentRun>;
  maxRows?: number;
  /** When set, each step row becomes clickable and reports its step index. */
  onSelectStep?: (stepIndex: number) => void;
}

function waitingDependencyCount(
  plan: CommandBarRunPlan,
  stepIndex: number,
  runsById: Record<string, AgentRun>,
): number {
  const deps = plan.steps[stepIndex]?.depends_on_step_indexes ?? [];
  let waiting = 0;
  for (const dep of deps) {
    const runId = plan.runIdsByStep[dep];
    const run = runId ? runsById[runId] : null;
    if (run?.status !== 'completed') waiting++;
  }
  return waiting;
}

export function DagRail({ plan, runsById, maxRows = 8, onSelectStep }: DagRailProps) {
  const visibleIndexes = plan.steps.map((_, i) => i).slice(0, maxRows);
  const hidden = plan.steps.length - visibleIndexes.length;

  return (
    <div className="flex flex-col gap-1 pt-1.5">
      {visibleIndexes.map((stepIndex) => {
        const step = plan.steps[stepIndex];
        const state = stepDotState(plan, stepIndex, runsById);
        const waiting = state === 'blocked' ? waitingDependencyCount(plan, stepIndex, runsById) : 0;
        const target = step.target?.display_title ?? step.target?.entity_id ?? '';
        const interactive = !!onSelectStep && !!plan.runIdsByStep[stepIndex];
        return (
          <div
            key={`${step.agent_id}-${stepIndex}`}
            role={interactive ? 'button' : undefined}
            tabIndex={interactive ? 0 : undefined}
            onClick={interactive ? () => onSelectStep?.(stepIndex) : undefined}
            onKeyDown={
              interactive
                ? (event) => {
                    if (event.key === 'Enter' || event.key === ' ') {
                      event.preventDefault();
                      onSelectStep?.(stepIndex);
                    }
                  }
                : undefined
            }
            className={cn(
              'flex items-center gap-2',
              interactive && 'cursor-pointer rounded px-1 -mx-1 hover:bg-accent',
            )}
          >
            <StatusDot state={state} />
            <span
              className="min-w-0 flex-1 truncate text-[10px] font-medium text-muted-foreground"
              title={target ? `${step.agent_name} · ${target}` : step.agent_name}
            >
              <span className="uppercase tracking-wider">{step.agent_name}</span>
              {target ? <span className="font-normal normal-case tracking-normal"> · {target}</span> : null}
            </span>
            {waiting > 0 ? (
              <span className="shrink-0 text-[10px] text-muted-foreground">
                waiting on {waiting}
              </span>
            ) : null}
          </div>
        );
      })}
      {hidden > 0 ? (
        <span className={cn('text-[10px] font-medium text-muted-foreground')}>
          +{hidden} more step{hidden === 1 ? '' : 's'}
        </span>
      ) : null}
    </div>
  );
}
