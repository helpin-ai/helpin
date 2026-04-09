import { Calendar01Icon, Clock02Icon, MapPinIcon } from '@/lib/icons';
import { format } from 'date-fns';
import { useContactCalendar, useDealCalendar } from '@/hooks/queries/useCRM';
import type { CRMCalendarEvent } from '@/lib/crmTypes';

interface CalendarEventsProps {
  workspaceId: string;
  contactId?: string;
  dealId?: string;
}

export function CalendarEvents({ workspaceId, contactId, dealId }: CalendarEventsProps) {
  const contactQuery = useContactCalendar(workspaceId, contactId ?? '');
  const dealQuery = useDealCalendar(workspaceId, dealId ?? '');
  const query = contactId ? contactQuery : dealQuery;
  const events = [...((query.data?.data ?? []) as CRMCalendarEvent[])].sort(
    (left, right) => new Date(left.start_time).getTime() - new Date(right.start_time).getTime(),
  );

  if (query.isLoading) {
    return (
      <div className="space-y-3">
        {Array.from({ length: 3 }).map((_, index) => (
          <div key={index} className="animate-pulse rounded-lg border border-border/60 px-4 py-4">
            <div className="h-4 w-40 rounded bg-muted" />
            <div className="mt-3 h-3 w-60 rounded bg-muted" />
            <div className="mt-2 h-3 w-36 rounded bg-muted" />
          </div>
        ))}
      </div>
    );
  }

  if (events.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center rounded-lg border border-dashed border-border/60 px-6 py-12 text-center">
        <div className="rounded-full bg-muted p-3">
          <Calendar01Icon className="h-7 w-7 text-muted-foreground" />
        </div>
        <p className="mt-4 text-base font-medium text-foreground">No meetings tracked</p>
        <p className="mt-2 max-w-sm text-sm text-muted-foreground">
          Calendar activity will appear here once events are linked to this contact.
        </p>
      </div>
    );
  }

  return (
    <div className="space-y-3">
      {events.map((event) => (
        <div key={event.id} className="rounded-lg border border-border/60 bg-card px-4 py-4">
          <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
            <div className="min-w-0">
              <p className="text-sm font-medium text-foreground">{event.title}</p>
              {event.description ? (
                <p className="mt-1 line-clamp-2 text-sm text-muted-foreground">{event.description}</p>
              ) : null}
            </div>
            <span className="shrink-0 text-xs font-medium text-muted-foreground">
              {format(new Date(event.start_time), 'MMM d')}
            </span>
          </div>

          <div className="mt-4 flex flex-wrap items-center gap-4 text-xs text-muted-foreground">
            <span className="inline-flex items-center gap-1.5">
              <Clock02Icon className="h-3.5 w-3.5" />
              {format(new Date(event.start_time), 'MMM d, yyyy h:mm a')} to{' '}
              {format(new Date(event.end_time), 'h:mm a')}
            </span>
            {event.location ? (
              <span className="inline-flex items-center gap-1.5">
                <MapPinIcon className="h-3.5 w-3.5" />
                {event.location}
              </span>
            ) : null}
          </div>
        </div>
      ))}
    </div>
  );
}
