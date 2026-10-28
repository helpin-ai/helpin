import type { ReactNode } from 'react';
import { QuietIconAction, QuietPrimaryAction, QuietTextAction } from '@/components/design-system/quiet';
import { DialogDescription, DialogTitle } from '@/components/ui/dialog';
import { Cancel01Icon, CheckmarkCircle02Icon, ClipboardIcon, Loading01Icon } from '@/lib/icons';
import { cn } from '@/lib/utils';

interface Props {
  phase: 'loading' | 'matching' | 'matches' | 'draft' | 'failed' | 'blocked';
  error?: string;
  checkedExistingTasks?: boolean;
  reviewing?: boolean;
  onCancel: () => void;
  onRetry?: () => void;
  children?: ReactNode;
}

/** Preparation and review share the task dialog; no editable fields appear yet. */
export function SupportTaskPreparation({
  phase, error, checkedExistingTasks, reviewing,
  onCancel, onRetry, children,
}: Props) {
  const draftFailed = phase === 'draft' && !!error;
  const working = !error && (phase === 'loading' || phase === 'matching' || phase === 'draft');
  const title = draftFailed ? 'Couldn’t prepare the task'
    : phase === 'failed' ? 'Couldn’t check existing tasks'
      : phase === 'blocked' ? 'Can’t prepare this task'
        : phase === 'matches' ? 'This may already be tracked' : 'Preparing your task';
  const description = draftFailed ? 'Try again to generate a draft from this conversation.'
    : phase === 'failed' ? 'Try again, or continue with a new task.'
      : phase === 'blocked' ? error
        : phase === 'matches' ? 'Link this conversation to existing work, or create a new task.'
          : 'We’ll fill in the details from this conversation.';
  const steps = [
    {
      label: phase === 'draft' ? checkedExistingTasks ? 'Existing tasks checked' : 'Existing-task check skipped' : 'Checking for existing tasks',
      detail: 'Looking for work that already covers this issue.',
      active: phase === 'matching',
      complete: phase === 'draft' && checkedExistingTasks,
    },
    {
      label: 'Preparing task details',
      detail: 'Writing the title, description, type and priority.',
      active: phase === 'draft',
      complete: false,
    },
  ];
  return (
    <div className="flex max-h-[85dvh] min-h-0 min-w-0 flex-col">
      <div className="flex items-center justify-between px-6 pt-5 text-xs text-quiet-text-tertiary">
        <span className="flex items-center gap-2"><ClipboardIcon className="size-4" />Task from conversation</span>
        <QuietIconAction aria-label="Close" disabled={reviewing} onClick={onCancel}><Cancel01Icon className="size-4" /></QuietIconAction>
      </div>
      <div className="min-h-0 min-w-0 overflow-y-auto px-6 pt-5 pb-6 sm:px-8">
        <DialogTitle className="text-xl leading-snug font-semibold tracking-[-0.018em]">{title}</DialogTitle>
        <DialogDescription role={draftFailed || phase === 'failed' || phase === 'blocked' ? 'alert' : undefined}
          className="mt-2 leading-relaxed text-quiet-text-tertiary">{description}</DialogDescription>
        {working && (
          <>
            <p role="status" className={phase === 'loading' ? 'mt-6 flex items-center gap-2 text-xs text-quiet-text-tertiary' : 'sr-only'}>
              {phase === 'loading' && <Loading01Icon className="size-3.5 motion-safe:animate-spin" />}
              {phase === 'draft' ? 'Preparing task details' : phase === 'matching' ? 'Checking for existing tasks' : 'Getting started…'}
            </p>
            <ol className="mt-7" aria-label="Task preparation progress">
              {steps.map((step, index) => (
                <li key={index} aria-current={step.active ? 'step' : undefined}
                  className={cn('relative flex gap-3 pb-6 text-sm last:pb-0', step.active ? 'font-medium text-quiet-text-primary' : 'text-quiet-text-tertiary')}>
                  {index === 0 && <span aria-hidden="true" className="absolute top-6 bottom-1 left-[9px] w-px bg-quiet-divider-strong" />}
                  <span aria-hidden="true" className="flex size-5 shrink-0 items-center justify-center">
                    {step.complete ? <CheckmarkCircle02Icon className="size-[18px] text-quiet-positive" />
                      : step.active ? <Loading01Icon className="size-[18px] text-quiet-accent motion-safe:animate-spin" />
                        : <span className="size-2 rounded-full border border-quiet-field" />}
                  </span>
                  <div>{step.label}{step.active && <p className="mt-1 text-xs leading-relaxed font-normal text-quiet-text-tertiary">{step.detail}</p>}</div>
                </li>
              ))}
            </ol>
          </>
        )}
        {children}
      </div>
      <div className={cn('flex flex-wrap items-center justify-end gap-5 border-t px-6 py-4', working ? 'border-quiet-accent/25' : 'border-quiet-divider-strong')}>
        {working && <span className="mr-auto text-xs text-quiet-text-tertiary">Opens automatically when ready</span>}
        <QuietTextAction disabled={reviewing} onClick={onCancel}>Cancel</QuietTextAction>
        {draftFailed && onRetry && <QuietPrimaryAction onClick={onRetry}>Try again</QuietPrimaryAction>}
      </div>
    </div>
  );
}
