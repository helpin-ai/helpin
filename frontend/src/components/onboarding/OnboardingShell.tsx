import { useEffect, useRef, type ComponentProps, type ReactNode } from 'react';
import { PublicPageShell } from '@/components/layout/PublicPageShell';
import type { WorkspaceOnboardingStep } from '@/lib/workspaceOnboardingMode';
import { cn } from '@/lib/utils';

const stepLabels: Record<WorkspaceOnboardingStep, string> = {
  workspace: 'Workspace',
  ai: 'AI',
  context: 'Company',
  teams: 'Teams',
  invite: 'Invite',
  finish: 'Done',
};

type OnboardingShellProps = {
  /** Visible steps, in order; the current one is highlighted. */
  steps: WorkspaceOnboardingStep[];
  current: WorkspaceOnboardingStep | undefined;
  title: ReactNode;
  description?: ReactNode;
  headerAction?: ReactNode;
  children: ReactNode;
};

/**
 * The signup page shell with a step indicator and heading, so onboarding reads
 * as a continuation of creating an account. Focus moves to the heading when the
 * step changes, so keyboard and screen reader users start at the new step.
 */
export function OnboardingShell({ steps, current, title, description, headerAction, children }: OnboardingShellProps) {
  const headingRef = useRef<HTMLHeadingElement>(null);
  const previousStep = useRef<WorkspaceOnboardingStep | undefined>(undefined);
  const currentIndex = current ? steps.indexOf(current) : -1;

  useEffect(() => {
    if (!current) return;
    // Loading states in between don't count; the first step keeps its own autofocus.
    if (previousStep.current && previousStep.current !== current) {
      headingRef.current?.focus();
    }
    previousStep.current = current;
  }, [current]);

  return (
    <PublicPageShell headerAction={headerAction} contentWidth="wide" brandPanel={false}>
      <div className="space-y-7">
        {currentIndex >= 0 && (
          // One continuous bar, no visible count: the AI step is only known after
          // the workspace exists, so a "Step N of M" label would change mid-flow.
          <div
            role="progressbar"
            aria-label="Onboarding progress"
            aria-valuemin={0}
            aria-valuemax={steps.length}
            aria-valuenow={currentIndex + 1}
            aria-valuetext={`${stepLabels[steps[currentIndex]]}, step ${currentIndex + 1} of ${steps.length}`}
            className="h-0.5 w-full overflow-hidden rounded-full bg-border"
          >
            <div
              className="h-full rounded-full bg-foreground transition-[width] duration-300 ease-out motion-reduce:transition-none"
              style={{ width: `${((currentIndex + 1) / steps.length) * 100}%` }}
            />
          </div>
        )}
        <header className="space-y-2.5">
          <h1
            ref={headingRef}
            tabIndex={-1}
            className="text-[28px] font-semibold leading-tight tracking-[-0.035em] outline-none sm:text-[30px]"
          >
            {title}
          </h1>
          {description ? <p className="text-sm leading-relaxed text-muted-foreground">{description}</p> : null}
        </header>
        {children}
      </div>
    </PublicPageShell>
  );
}

/** Footer row for a step: secondary actions on the left, the primary action on the right. */
export function OnboardingActions({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <div className={cn('flex flex-col-reverse gap-3 pt-2 sm:flex-row sm:items-center sm:justify-end', className)}>
      {children}
    </div>
  );
}

/** Quiet text action used for Skip, Back and similar choices in the auth shell. */
export function OnboardingTextButton({ className, type = 'button', ...props }: ComponentProps<'button'>) {
  return (
    <button
      type={type}
      className={cn(
        'min-h-11 rounded-sm px-1 text-sm text-muted-foreground underline-offset-4 transition-colors hover:text-foreground hover:underline disabled:pointer-events-none disabled:opacity-50',
        className,
      )}
      {...props}
    />
  );
}
