import { Badge } from '@/components/ui/badge';
import type { CRMMeetingStatus } from '@/lib/crmMeetingTypes';
import { getMeetingStatusLabel } from '@/lib/meetingPresentation';
import { cn } from '@/lib/utils';

const statusClasses: Record<CRMMeetingStatus, string> = {
  scheduled: 'bg-muted text-muted-foreground',
  joining: 'bg-blue-500/10 text-blue-600 dark:text-blue-400',
  waiting: 'bg-amber-500/10 text-amber-600 dark:text-amber-400',
  recording: 'bg-red-500/10 text-red-600 dark:text-red-400',
  finalizing: 'bg-violet-500/10 text-violet-600 dark:text-violet-400',
  processing: 'bg-violet-500/10 text-violet-600 dark:text-violet-400',
  ready: 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400',
  failed: 'bg-destructive/10 text-destructive',
  cancelled: 'bg-muted text-muted-foreground',
};

const activeStatuses = new Set<CRMMeetingStatus>(['joining', 'waiting', 'recording', 'finalizing', 'processing']);

export function MeetingStatusBadge({ status, className }: { status: CRMMeetingStatus; className?: string }) {
  return (
    <Badge variant="secondary" className={cn('gap-1.5 whitespace-nowrap font-medium', statusClasses[status], className)}>
      <span
        className={cn(
          'h-1.5 w-1.5 rounded-full bg-current',
          activeStatuses.has(status) && 'motion-safe:animate-pulse',
        )}
      />
      {getMeetingStatusLabel(status)}
    </Badge>
  );
}
