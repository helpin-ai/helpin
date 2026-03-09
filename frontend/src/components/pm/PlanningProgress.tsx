import { Check } from 'lucide-react';

import { cn } from '@/lib/utils';
import { type PlanningStep, type StepStatus, STEPS, STEP_LABELS, getStepStatus } from './planningStepUtils';

interface Props {
  currentStep: PlanningStep;
}

export function PlanningProgress({ currentStep }: Props) {
  return (
    <div className="flex items-center gap-0">
      {STEPS.map((step, index) => {
        const status = getStepStatus(step, currentStep);
        return (
          <div key={step} className="flex items-center">
            <StepDot step={step} status={status} />
            {index < STEPS.length - 1 && (
              <div
                className={cn(
                  'h-px w-6 sm:w-10',
                  status === 'completed' ? 'bg-emerald-500' : 'bg-border',
                )}
              />
            )}
          </div>
        );
      })}
    </div>
  );
}

function StepDot({ step, status }: { step: PlanningStep; status: StepStatus }) {
  return (
    <div className="flex flex-col items-center gap-1">
      <div
        className={cn(
          'flex h-6 w-6 items-center justify-center rounded-full text-[10px] font-medium transition-colors',
          status === 'completed' && 'bg-emerald-500 text-white',
          status === 'current' && 'bg-primary text-primary-foreground ring-2 ring-primary/20',
          status === 'upcoming' && 'bg-muted text-muted-foreground',
        )}
      >
        {status === 'completed' ? (
          <Check className="h-3 w-3" />
        ) : (
          STEPS.indexOf(step) + 1
        )}
      </div>
      <span
        className={cn(
          'max-w-[72px] text-center text-[10px] leading-tight',
          status === 'completed' && 'text-emerald-600',
          status === 'current' && 'font-medium text-foreground',
          status === 'upcoming' && 'text-muted-foreground',
        )}
      >
        {STEP_LABELS[step]}
      </span>
    </div>
  );
}
