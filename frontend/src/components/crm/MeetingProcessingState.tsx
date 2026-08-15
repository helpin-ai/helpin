import { Loading01Icon, SparklesIcon, Tick01Icon } from '@/lib/icons';
import type { CRMMeetingStatus, CRMMeetingSummaryStatus } from '@/lib/crmMeetingTypes';
import { cn } from '@/lib/utils';

function ProcessingStep({ label, state }: { label: string; state: 'complete' | 'active' | 'pending' }) {
  return (
    <div className={cn('flex items-center gap-2.5 text-sm', state === 'pending' && 'text-muted-foreground/70')}>
      <span className={cn(
        'flex h-5 w-5 shrink-0 items-center justify-center rounded-full border',
        state === 'complete' && 'border-emerald-500/30 bg-emerald-500/10 text-emerald-600',
        state === 'active' && 'border-violet-500/30 bg-violet-500/10 text-violet-600',
      )}>
        {state === 'complete' ? <Tick01Icon className="h-3 w-3" /> : state === 'active' ? <span className="h-1.5 w-1.5 rounded-full bg-current motion-safe:animate-pulse" /> : <span className="h-1.5 w-1.5 rounded-full bg-muted-foreground/30" />}
      </span>
      <span className={cn(state === 'active' && 'font-medium text-foreground')}>{label}</span>
    </div>
  );
}

export function MeetingProcessingState({
  status,
  summaryStatus,
}: {
  status: CRMMeetingStatus;
  summaryStatus: CRMMeetingSummaryStatus;
}) {
  const captureActive = ['joining', 'waiting', 'recording'].includes(status);
  const finalizing = status === 'finalizing';
  const processing = status === 'processing' || summaryStatus === 'processing';
  const title = captureActive ? 'Capturing your meeting' : finalizing ? 'Preparing the transcript' : 'Creating meeting notes';
  const description = captureActive
    ? 'Helpin is listening and building the speaker-attributed transcript.'
    : finalizing
      ? 'The meeting ended. Helpin is securely collecting the final transcript.'
      : 'Helpin is analyzing the transcript for decisions, risks, and follow-up work.';

  return (
    <div className="relative overflow-hidden rounded-lg border border-violet-500/20 bg-gradient-to-br from-violet-500/[0.07] via-background to-blue-500/[0.05] p-5">
      <div className="pointer-events-none absolute inset-x-0 top-0 h-px bg-gradient-to-r from-transparent via-violet-500/60 to-transparent motion-safe:animate-pulse" />
      <div className="flex items-start gap-3">
        <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg border border-violet-500/20 bg-background text-violet-600 shadow-sm">
          {captureActive ? <Loading01Icon className="h-4 w-4 motion-safe:animate-spin" /> : <SparklesIcon className="h-4 w-4 motion-safe:animate-pulse" />}
        </span>
        <div className="min-w-0">
          <p className="text-sm font-semibold">{title}</p>
          <p className="mt-1 text-xs leading-5 text-muted-foreground">{description}</p>
        </div>
      </div>

      <div className="mt-5 grid gap-3 sm:grid-cols-3">
        <ProcessingStep label="Capture meeting" state={captureActive ? 'active' : 'complete'} />
        <ProcessingStep label="Build transcript" state={captureActive ? 'pending' : finalizing ? 'active' : 'complete'} />
        <ProcessingStep label="Create meeting notes" state={processing ? 'active' : 'pending'} />
      </div>

      <div className="mt-5 space-y-2" aria-hidden="true">
        <div className="h-2.5 w-5/6 rounded-full bg-muted/80 motion-safe:animate-pulse" />
        <div className="h-2.5 w-2/3 rounded-full bg-muted/60 motion-safe:animate-pulse [animation-delay:150ms]" />
      </div>
    </div>
  );
}
