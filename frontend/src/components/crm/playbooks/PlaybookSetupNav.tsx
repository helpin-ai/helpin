import { Tick01Icon } from '@/lib/icons';
import { cn } from '@/lib/utils';
import { playbookSetupSteps, type PlaybookSetupStep } from '@/lib/crmPlaybookSetup';

export function PlaybookSetupNav({ current, complete, onChange }: { current: PlaybookSetupStep; complete: Record<PlaybookSetupStep, boolean>; onChange: (step: PlaybookSetupStep) => void }) {
  return <nav aria-label="Playbook setup steps" className="min-w-0 lg:sticky lg:top-20 lg:self-start">
    <ol className="flex overflow-x-auto pb-3 lg:block lg:overflow-visible lg:pb-0">
      {playbookSetupSteps.map((step, index) => <li key={step.id} className="relative min-w-48 shrink-0 lg:min-w-0 lg:pb-6 lg:last:pb-0">
        {index < playbookSetupSteps.length - 1 && <span aria-hidden="true" className="absolute bottom-0 left-3 top-8 hidden w-px bg-quiet-divider-strong lg:block" />}
        <button type="button" aria-current={current === step.id ? 'step' : undefined} aria-controls={`playbook-step-${step.id}`} aria-label={`${step.label}${complete[step.id] ? ', complete' : ''}`} onClick={() => onChange(step.id)} className="group flex w-full items-start gap-3 rounded-md p-1 text-left focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-quiet-field">
          <span aria-hidden="true" className={cn('flex size-5 shrink-0 items-center justify-center rounded-full text-xs tabular-nums', current === step.id ? 'bg-quiet-text-primary font-semibold text-quiet-surface' : complete[step.id] ? 'text-quiet-positive' : 'text-quiet-text-tertiary')}>
            {complete[step.id] && current !== step.id ? <Tick01Icon className="size-3.5" /> : index + 1}
          </span>
          <span className="min-w-0"><span className={cn('block text-sm group-hover:text-quiet-text-primary', current === step.id ? 'font-semibold text-quiet-text-primary' : 'text-quiet-text-secondary')}>{step.label}</span></span>
        </button>
      </li>)}
    </ol>
  </nav>;
}
