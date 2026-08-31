import { QuietStatusText } from '@/components/design-system/quiet';
import type { CRMMeetingStatus } from '@/lib/crmMeetingTypes';
import { getMeetingStatusLabel } from '@/lib/meetingPresentation';
import { cn } from '@/lib/utils';

const activeStatuses = new Set<CRMMeetingStatus>([
  'joining',
  'waiting',
  'recording',
  'finalizing',
  'processing',
]);

function meetingStatusTone(status: CRMMeetingStatus) {
  if (status === 'ready') return 'positive' as const;
  if (status === 'failed') return 'blocker' as const;
  if (activeStatuses.has(status)) return 'current' as const;
  return 'neutral' as const;
}

export function MeetingStatusText({ status, className }: { status: CRMMeetingStatus; className?: string }) {
  return (
    <QuietStatusText
      tone={meetingStatusTone(status)}
      pulse={activeStatuses.has(status)}
      className={cn(
        status === 'failed' && 'text-quiet-accent',
        status === 'ready' && 'text-quiet-positive',
        activeStatuses.has(status) && 'text-quiet-text-primary',
        className,
      )}
    >
      {getMeetingStatusLabel(status)}
    </QuietStatusText>
  );
}
