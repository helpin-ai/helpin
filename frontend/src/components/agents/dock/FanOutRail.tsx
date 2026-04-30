import type { AgentRun } from '@/lib/pmTypes';
import type { CommandBarRunPlan } from '@/stores/commandBarStore';
import { StatusDot } from './StatusDot';
import { classifyRun } from './utils';

interface FanOutRailProps {
  plan: CommandBarRunPlan;
  runsById: Record<string, AgentRun>;
  /** When set, hide the row beyond this many entries and append a "+N" affordance. */
  max?: number;
}

export function FanOutRail({ plan, runsById, max = 12 }: FanOutRailProps) {
  if (plan.planKind !== 'fan_out') return null;
  const ordered = plan.steps
    .map((_, i) => plan.runIdsByStep[i])
    .map((id) => (id ? runsById[id] : null));
  const visible = ordered.slice(0, max);
  const hidden = ordered.length - visible.length;

  return (
    <div className="flex flex-wrap items-center gap-1 pt-1.5">
      {visible.map((run, i) => (
        <StatusDot key={i} state={run ? classifyRun(run) : 'queued'} />
      ))}
      {hidden > 0 ? (
        <span className="text-[10px] font-medium text-muted-foreground">+{hidden}</span>
      ) : null}
    </div>
  );
}
