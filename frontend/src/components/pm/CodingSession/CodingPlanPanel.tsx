import {
  CancelCircleIcon,
  CheckListIcon,
  CheckmarkCircle02Icon,
  CircleIcon,
  InformationCircleIcon,
  Loading01Icon,
  MinusSignIcon,
} from '@/lib/icons';

import { Badge } from '@/components/ui/badge';
import { cn } from '@/lib/utils';
import type { AgentRunStatus, RunPlanArtifact, RunPlanStep } from '@/lib/pmTypes';

interface CodingPlanPanelProps {
  plan: RunPlanArtifact | null;
  runStatus?: AgentRunStatus;
  title?: string;
}

const TERMINAL_STATUSES: readonly AgentRunStatus[] = ['completed', 'failed', 'cancelled'];

function isTerminalStatus(status?: AgentRunStatus): boolean {
  return !!status && (TERMINAL_STATUSES as readonly string[]).includes(status);
}

export function CodingPlanPanel({ plan, runStatus, title = 'Agent plan' }: CodingPlanPanelProps) {
  const steps = plan?.plan ?? [];
  const hasPlan = steps.length > 0;
  const completedCount = steps.filter((s) => s.status === 'completed').length;
  const hasInProgress = steps.some((s) => s.status === 'in_progress');
  const hasPending = steps.some((s) => s.status === 'pending');
  const allDone = hasPlan && completedCount === steps.length;

  const terminal = isTerminalStatus(runStatus);
  const unfinalized = terminal && hasPlan && (hasInProgress || hasPending) && !allDone;

  const badgeLabel = (() => {
    if (!hasPlan) return 'Waiting';
    if (terminal && runStatus === 'cancelled') return 'Run cancelled';
    if (terminal && runStatus === 'failed') return 'Run failed';
    if (unfinalized) return 'Plan not finalized';
    return `${completedCount}/${steps.length} steps`;
  })();

  return (
    <details
      className="rounded-xl border border-border bg-card shadow-sm"
      open
    >
      <summary className="flex cursor-pointer list-none items-center justify-between gap-3 px-4 py-3">
        <div className="flex items-center gap-2 text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
          <CheckListIcon className="h-3.5 w-3.5" />
          {title}
        </div>
        <Badge variant="outline" className="text-[10px]">
          {badgeLabel}
        </Badge>
      </summary>

      <div className="border-t border-border px-4 pb-3 pt-3">
        {plan?.note ? (
          <p className="mb-3 text-xs italic text-muted-foreground">{plan.note}</p>
        ) : null}
        {hasPlan ? (
          <ol className="space-y-1.5">
            {steps.map((step, index) => (
              <PlanStepRow key={index} step={step} runStatus={runStatus} />
            ))}
          </ol>
        ) : (
          <p className="text-sm text-muted-foreground">
            {terminal
              ? 'Agent ended the run without publishing a plan.'
              : 'Waiting for the agent to publish its first plan update.'}
          </p>
        )}

        {unfinalized ? (
          <div className="mt-3 flex items-start gap-2 rounded-md border border-border/60 bg-muted/40 px-2.5 py-2 text-[11px] text-muted-foreground">
            <InformationCircleIcon className="mt-0.5 h-3.5 w-3.5 shrink-0" />
            <span>
              {runStatus === 'completed'
                ? "Agent finished without publishing a final plan update. Remaining steps may or may not have been completed."
                : 'Run ended before the agent could finish its plan.'}
            </span>
          </div>
        ) : null}
      </div>
    </details>
  );
}

function PlanStepRow({ step, runStatus }: { step: RunPlanStep; runStatus?: AgentRunStatus }) {
  const terminal = isTerminalStatus(runStatus);

  if (step.status === 'completed') {
    return (
      <li className="flex items-start gap-2.5 text-sm">
        <CheckmarkCircle02Icon className="mt-0.5 h-4 w-4 shrink-0 text-emerald-500" />
        <span className="text-muted-foreground line-through">{step.step}</span>
      </li>
    );
  }

  if (step.status === 'in_progress') {
    if (terminal) {
      if (runStatus === 'completed') {
        return (
          <li className="flex items-start gap-2.5 text-sm">
            <MinusSignIcon className="mt-0.5 h-4 w-4 shrink-0 rounded-full border border-muted-foreground/40 p-0.5 text-muted-foreground" />
            <span className="text-muted-foreground">{step.step}</span>
          </li>
        );
      }
      return (
        <li className="flex items-start gap-2.5 text-sm">
          <CancelCircleIcon className="mt-0.5 h-4 w-4 shrink-0 text-muted-foreground" />
          <span className="text-muted-foreground line-through">{step.step}</span>
        </li>
      );
    }
    return (
      <li className={cn(
        'flex items-start gap-2.5 rounded-lg px-2.5 py-1.5 text-sm',
        'bg-amber-50 dark:bg-amber-950/30',
      )}>
        <Loading01Icon className="mt-0.5 h-4 w-4 shrink-0 animate-spin text-amber-600 dark:text-amber-400" />
        <span className="font-medium text-foreground">{step.step}</span>
      </li>
    );
  }

  // pending
  return (
    <li className="flex items-start gap-2.5 text-sm">
      {terminal && runStatus !== 'completed' ? (
        <CancelCircleIcon className="mt-0.5 h-4 w-4 shrink-0 text-muted-foreground/50" />
      ) : (
        <CircleIcon className="mt-0.5 h-4 w-4 shrink-0 text-muted-foreground/40" />
      )}
      <span className={cn('text-muted-foreground', terminal && runStatus !== 'completed' && 'line-through')}>
        {step.step}
      </span>
    </li>
  );
}
