import type { CRMMeetingStatus, CRMMeetingSummaryStatus } from '@/lib/crmMeetingTypes';
import { cn } from '@/lib/utils';

type ProcessingStepState = 'complete' | 'active' | 'pending';

function ProcessingStep({ sentence, state }: { sentence: string; state: ProcessingStepState }) {
  return (
    <li className="relative min-h-11 pl-5">
      <span
        aria-hidden="true"
        className={cn(
          'absolute left-0 top-[7px] z-10 size-[7px] -translate-x-[3px] rounded-full',
          state === 'complete' && 'bg-quiet-empty-glyph',
          state === 'active' && 'bg-quiet-text-primary motion-safe:animate-pulse',
          state === 'pending' && 'bg-quiet-divider-strong',
        )}
      />
      <p className={cn(
        'text-sm leading-5',
        state === 'active' ? 'font-medium text-quiet-text-primary' : 'text-quiet-text-tertiary',
      )}>
        {sentence}
      </p>
    </li>
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
  const title = captureActive ? 'Helpin is capturing this meeting' : finalizing ? 'The transcript is being prepared' : 'Meeting notes are being created';
  const description = captureActive
    ? 'The notetaker is building a speaker-attributed record while the call is in progress.'
    : finalizing
      ? 'The call has ended and Helpin is securely collecting the final transcript.'
      : 'Helpin is reading the transcript for decisions, risks, and follow-up work.';

  return (
    <div className="border-y border-quiet-divider-strong py-5">
      <p className="text-[14px] font-medium text-quiet-text-primary">{title}</p>
      <p className="mt-1 max-w-[680px] text-sm leading-[1.6] text-quiet-text-tertiary">{description}</p>

      <ol className="relative mt-5 ml-1 border-l border-quiet-divider-strong">
        <ProcessingStep
          sentence={captureActive ? 'The meeting is being captured now.' : 'The meeting capture is complete.'}
          state={captureActive ? 'active' : 'complete'}
        />
        <ProcessingStep
          sentence={captureActive ? 'The final transcript will be assembled after the call.' : finalizing ? 'The final transcript is being assembled now.' : 'The transcript is ready for analysis.'}
          state={captureActive ? 'pending' : finalizing ? 'active' : 'complete'}
        />
        <ProcessingStep
          sentence={processing ? 'Decisions and next steps are being extracted now.' : 'Meeting notes will be created from the transcript.'}
          state={processing ? 'active' : 'pending'}
        />
      </ol>
    </div>
  );
}
