import { CheckCircle2, Circle, ListTodo, Loader2 } from 'lucide-react';

import { Badge } from '@/components/ui/badge';
import { cn } from '@/lib/utils';
import type { RunPlanArtifact, RunPlanStep } from '@/lib/pmTypes';

export function CodingPlanPanel({ plan }: { plan: RunPlanArtifact | null }) {
  const steps = plan?.plan ?? [];
  const hasPlan = steps.length > 0;
  const completedCount = steps.filter((s) => s.status === 'completed').length;
  const hasInProgress = steps.some((s) => s.status === 'in_progress');
  const allDone = hasPlan && completedCount === steps.length;

  return (
    <details
      className="rounded-xl border border-border bg-card shadow-sm"
      open={!hasPlan || hasInProgress || !allDone}
    >
      <summary className="flex cursor-pointer list-none items-center justify-between gap-3 px-4 py-3">
        <div className="flex items-center gap-2 text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
          <ListTodo className="h-3.5 w-3.5" />
          Agent plan
        </div>
        <Badge variant="outline" className="text-[10px]">
          {hasPlan ? `${completedCount}/${steps.length} steps` : 'Waiting'}
        </Badge>
      </summary>

      <div className="border-t border-border px-4 pb-3 pt-3">
        {plan?.note ? (
          <p className="mb-3 text-xs italic text-muted-foreground">{plan.note}</p>
        ) : null}
        {hasPlan ? (
          <ol className="space-y-1.5">
            {steps.map((step, index) => (
              <PlanStepRow key={index} step={step} />
            ))}
          </ol>
        ) : (
          <p className="text-sm text-muted-foreground">
            Waiting for the agent to publish its first plan update.
          </p>
        )}
      </div>
    </details>
  );
}

function PlanStepRow({ step }: { step: RunPlanStep }) {
  if (step.status === 'completed') {
    return (
      <li className="flex items-start gap-2.5 text-sm">
        <CheckCircle2 className="mt-0.5 h-4 w-4 shrink-0 text-emerald-500" />
        <span className="text-muted-foreground line-through">{step.step}</span>
      </li>
    );
  }
  if (step.status === 'in_progress') {
    return (
      <li className={cn(
        'flex items-start gap-2.5 rounded-lg px-2.5 py-1.5 text-sm',
        'bg-amber-50 dark:bg-amber-950/30',
      )}>
        <Loader2 className="mt-0.5 h-4 w-4 shrink-0 animate-spin text-amber-600 dark:text-amber-400" />
        <span className="font-medium text-foreground">{step.step}</span>
      </li>
    );
  }
  return (
    <li className="flex items-start gap-2.5 text-sm">
      <Circle className="mt-0.5 h-4 w-4 shrink-0 text-muted-foreground/40" />
      <span className="text-muted-foreground">{step.step}</span>
    </li>
  );
}
