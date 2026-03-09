import { Calendar, MapPin, Clock } from 'lucide-react';
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
  const events = (query.data?.data ?? []) as CRMCalendarEvent[];

  if (events.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center py-8 text-muted-foreground">
        <Calendar className="mb-2 h-8 w-8 opacity-40" />
        <p className="text-sm">No calendar events linked</p>
        <p className="mt-1 text-xs text-muted-foreground/70">Calendar integration coming soon</p>
      </div>
    );
  }

  return (
    <div className="space-y-3">
      {events.map((event) => (
        <div key={event.id} className="rounded-md border p-3">
          <p className="text-sm font-medium">{event.title}</p>
          <div className="mt-1 flex flex-wrap items-center gap-3 text-xs text-muted-foreground">
            <span className="flex items-center gap-1">
              <Clock className="h-3 w-3" />
              {format(new Date(event.start_time), 'MMM d, yyyy h:mm a')} - {format(new Date(event.end_time), 'h:mm a')}
            </span>
            {event.location && (
              <span className="flex items-center gap-1">
                <MapPin className="h-3 w-3" />
                {event.location}
              </span>
            )}
          </div>
          {event.description && (
            <p className="mt-1 line-clamp-2 text-xs text-muted-foreground">{event.description}</p>
          )}
        </div>
      ))}
    </div>
  );
}
