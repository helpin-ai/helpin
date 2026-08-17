import { useState } from 'react';
import { format } from 'date-fns';
import { useNavigate } from '@tanstack/react-router';
import { toast } from 'sonner';
import { Calendar01Icon, Loading01Icon, PlusSignIcon, Settings02Icon } from '@/lib/icons';
import { MeetingPlatformIcon } from '@/components/crm/MeetingPlatform';
import { MeetingStatusBadge } from '@/components/crm/MeetingStatusBadge';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { Switch } from '@/components/ui/switch';
import { useUpdateCalendarMeetingCapture, useUpdateCalendarMeetingSeriesCapture } from '@/hooks/queries/useCRMMeetings';
import { detectMeetingPlatform } from '@/lib/meetingPresentation';
import { groupUpcomingMeetings, type UpcomingMeetingGroup } from '@/components/crm/upcomingCalendarMeetingGroups';
import type { CRMCalendarMeetingCandidate } from '@/lib/crmMeetingTypes';


export function UpcomingCalendarMeetings({
  workspaceId,
  workspaceSlug,
  candidates,
  canEdit,
  canManageSettings,
  loading,
  calendarConnected,
  calendarLoading,
  connectingCalendar,
  meetingNotesEnabled,
  searching,
  onConnectCalendar,
  onConfigureSettings,
  onAddMeeting,
}: {
  workspaceId: string;
  workspaceSlug: string;
  candidates: CRMCalendarMeetingCandidate[];
  canEdit: boolean;
  canManageSettings: boolean;
  loading: boolean;
  calendarConnected: boolean;
  calendarLoading: boolean;
  connectingCalendar: boolean;
  meetingNotesEnabled: boolean;
  searching: boolean;
  onConnectCalendar: () => void;
  onConfigureSettings: () => void;
  onAddMeeting: () => void;
}) {
  const navigate = useNavigate();
  const updateCapture = useUpdateCalendarMeetingCapture(workspaceId);
  const updateSeriesCapture = useUpdateCalendarMeetingSeriesCapture(workspaceId);
  const [expandedSeries, setExpandedSeries] = useState<Set<string>>(() => new Set());
  const groups = groupUpcomingMeetings(candidates);

  if (loading || calendarLoading) {
    return <div className="space-y-2"><Skeleton className="h-16 w-full" /><Skeleton className="h-16 w-full" /></div>;
  }
  if (!candidates.length && searching) return null;

  const setCapture = async (candidate: CRMCalendarMeetingCandidate, enabled: boolean) => {
    try {
      await updateCapture.mutateAsync({ calendarEventId: candidate.event.id, payload: { enabled } });
      toast.success(enabled ? 'Automatic joining scheduled' : 'Automatic joining removed');
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Unable to update calendar meeting');
    }
  };

  const setSeriesCapture = async (group: UpcomingMeetingGroup, enabled: boolean) => {
    const first = group.occurrences[0];
    const seriesExternalId = first?.event.recurring_series_id;
    if (!first || !seriesExternalId) return;
    try {
      await updateSeriesCapture.mutateAsync({
        email_account_id: first.event.email_account_id,
        series_external_id: seriesExternalId,
        enabled,
      });
      toast.success(enabled ? 'Helpin will join this recurring series' : 'Automatic joining removed for this series');
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Unable to update recurring meeting');
    }
  };

  const toggleSeriesExpanded = (key: string) => {
    setExpandedSeries((current) => {
      const next = new Set(current);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  };

  return (
    <section className="mb-5">
      <div className="mb-2 flex items-center justify-between gap-3">
        <div>
          <h2 className="text-sm font-semibold">Upcoming</h2>
          <p className="text-xs text-muted-foreground">Choose which meetings Helpin should automatically join.</p>
        </div>
      </div>
      <div className="overflow-hidden rounded-xl border border-border bg-card shadow-none">
        {!candidates.length ? (
          <div className="flex min-h-56 items-center justify-center px-6 py-8 text-center">
            <div className="max-w-md">
              <div className="mx-auto flex h-11 w-11 items-center justify-center rounded-lg border bg-muted/30">
                <Calendar01Icon className="h-5 w-5 text-muted-foreground" />
              </div>
              {!calendarConnected ? (
                <>
                  <h3 className="mt-4 text-sm font-semibold">Connect Google Calendar</h3>
                  <p className="mt-1.5 text-sm leading-6 text-muted-foreground">
                    See upcoming calls here and choose which ones Helpin should automatically join.
                  </p>
                  <div className="mt-4 flex flex-wrap justify-center gap-2">
                    <Button size="sm" onClick={onConnectCalendar} disabled={connectingCalendar}>
                      {connectingCalendar ? <Loading01Icon className="mr-1.5 h-3.5 w-3.5 animate-spin" /> : <Calendar01Icon className="mr-1.5 h-3.5 w-3.5" />}
                      Connect Google Calendar
                    </Button>
                    {canEdit && <Button size="sm" variant="outline" onClick={onAddMeeting}><PlusSignIcon className="mr-1.5 h-3.5 w-3.5" />Add meeting</Button>}
                  </div>
                </>
              ) : !meetingNotesEnabled ? (
                <>
                  <h3 className="mt-4 text-sm font-semibold">Turn on meeting notes</h3>
                  <p className="mt-1.5 text-sm leading-6 text-muted-foreground">
                    Enable the Helpin notetaker before selecting meetings for automatic joining.
                  </p>
                  {canManageSettings ? (
                    <Button className="mt-4" size="sm" onClick={onConfigureSettings}>
                      <Settings02Icon className="mr-1.5 h-3.5 w-3.5" />Enable meeting notes
                    </Button>
                  ) : (
                    <p className="mt-3 text-xs text-muted-foreground">Ask a workspace admin to enable meeting notes.</p>
                  )}
                </>
              ) : (
                <>
                  <h3 className="mt-4 text-sm font-semibold">No upcoming meetings</h3>
                  <p className="mt-1.5 text-sm leading-6 text-muted-foreground">
                    Calls with a Google Meet, Zoom, Teams, or Webex link will appear here after calendar sync.
                  </p>
                  {canEdit && <Button className="mt-4" size="sm" variant="outline" onClick={onAddMeeting}><PlusSignIcon className="mr-1.5 h-3.5 w-3.5" />Add meeting manually</Button>}
                </>
              )}
            </div>
          </div>
        ) : groups.map((group) => {
          const candidate = group.occurrences[0];
          const { event, meeting } = candidate;
          const platform = meeting?.platform ?? detectMeetingPlatform(event.meeting_url ?? '');
          const externalAttendees = event.attendees.filter((attendee) => !attendee.self);
          const captureConfigured = group.recurring
            ? (candidate.series_auto_join ?? group.occurrences.every((occurrence) => occurrence.effective_auto_join))
            : candidate.effective_auto_join;
          const captureEditable = !meeting || ['scheduled', 'failed'].includes(meeting.status);
          const expanded = group.recurring && expandedSeries.has(group.key);
          return (
            <div key={group.key} className="border-b last:border-b-0">
              <div className="grid gap-3 px-4 py-3 md:grid-cols-[minmax(0,1fr)_180px_200px] md:items-center">
                <button
                  type="button"
                  className="flex min-w-0 items-center gap-3 text-left"
                  onClick={() => {
                    if (group.recurring) {
                      toggleSeriesExpanded(group.key);
                    } else if (meeting) {
                      void navigate({ to: '/w/$slug/crm/meetings/$meetingId', params: { slug: workspaceSlug, meetingId: meeting.id } });
                    }
                  }}
                >
                  {platform ? <MeetingPlatformIcon platform={platform} /> : <span className="h-8 w-8 shrink-0 rounded-lg border bg-muted/30" />}
                  <span className="min-w-0">
                    <span className="block truncate text-sm font-medium">{event.title}</span>
                    <span className="mt-0.5 block truncate text-xs text-muted-foreground">
                      {group.recurring
                        ? `Recurring · Next ${format(new Date(event.start_time), 'EEE, MMM d · p')} · ${group.occurrences.length} upcoming`
                        : format(new Date(event.start_time), 'EEE, MMM d · p')}
                      {externalAttendees.length > 0 ? ` · ${externalAttendees.length} attendee${externalAttendees.length === 1 ? '' : 's'}` : ''}
                    </span>
                  </span>
                </button>

                <div className="text-xs text-muted-foreground">
                  {candidate.eligible
                    ? (group.recurring ? 'Applies to future occurrences' : 'Ready for automatic joining')
                    : candidate.ineligibility_reason}
                </div>

                <div className="flex items-center justify-between gap-2 md:justify-end">
                  {group.recurring && (
                    <Button variant="ghost" size="sm" className="h-7 px-2 text-xs" onClick={() => toggleSeriesExpanded(group.key)}>
                      {expanded ? 'Hide dates' : 'Show dates'}
                    </Button>
                  )}
                  {!group.recurring && meeting && meeting.status !== 'scheduled'
                    ? <MeetingStatusBadge status={meeting.status} />
                    : <span className="text-xs font-medium">{group.recurring ? 'Auto-join series' : 'Auto-join'}</span>}
                  {(group.recurring || captureEditable) ? (
                    <Switch
                      aria-label={group.recurring ? `Automatically join recurring series ${event.title}` : `Automatically join ${event.title}`}
                      checked={captureConfigured}
                      disabled={!canEdit || !candidate.eligible || updateCapture.isPending || updateSeriesCapture.isPending}
                      onCheckedChange={(enabled) => void (group.recurring ? setSeriesCapture(group, enabled) : setCapture(candidate, enabled))}
                    />
                  ) : meeting ? (
                    <Button size="sm" variant="outline" className="h-7 text-xs" onClick={() => void navigate({ to: '/w/$slug/crm/meetings/$meetingId', params: { slug: workspaceSlug, meetingId: meeting.id } })}>
                      Open
                    </Button>
                  ) : null}
                </div>
              </div>

              {expanded && (
                <div className="border-t bg-muted/15 px-4 py-1">
                  {group.occurrences.map((occurrence) => {
                    const occurrenceEditable = !occurrence.meeting || ['scheduled', 'failed'].includes(occurrence.meeting.status);
                    return (
                      <div key={occurrence.event.id} className="flex items-center gap-3 border-b py-2.5 pl-11 last:border-b-0">
                        <div className="min-w-0 flex-1">
                          <p className="text-xs font-medium">{format(new Date(occurrence.event.start_time), 'EEE, MMM d · p')}</p>
                          <p className="mt-0.5 text-xs text-muted-foreground">
                            {occurrence.meeting && occurrence.meeting.status !== 'scheduled'
                              ? 'Meeting capture has started'
                              : occurrence.effective_auto_join ? 'Helpin will join' : 'Helpin will skip'}
                          </p>
                        </div>
                        {occurrence.meeting && occurrence.meeting.status !== 'scheduled' ? (
                          <MeetingStatusBadge status={occurrence.meeting.status} />
                        ) : (
                          <Switch
                            aria-label={`Automatically join ${event.title} on ${format(new Date(occurrence.event.start_time), 'MMM d')}`}
                            checked={occurrence.effective_auto_join}
                            disabled={!canEdit || !occurrence.eligible || !occurrenceEditable || updateCapture.isPending || updateSeriesCapture.isPending}
                            onCheckedChange={(enabled) => void setCapture(occurrence, enabled)}
                          />
                        )}
                      </div>
                    );
                  })}
                </div>
              )}
            </div>
          );
        })}
      </div>
    </section>
  );
}
