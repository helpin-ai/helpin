import { useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { toast } from 'sonner';

import { Calendar01Icon, Loading01Icon, PlusSignIcon } from '@/lib/icons';
import {
  QuietEmptyState,
  QuietListRow,
  QuietPrimaryAction,
  QuietSection,
  QuietStatusText,
  QuietTextAction,
} from '@/components/design-system/quiet';
import { MeetingPlatformLabel } from '@/components/crm/MeetingPlatform';
import { MeetingStatusText } from '@/components/crm/MeetingStatusText';
import { ServerSetupNotice } from '@/components/crm/ServerSetupNotice';
import { Skeleton } from '@/components/ui/skeleton';
import { Switch } from '@/components/ui/switch';
import { useUpdateCalendarMeetingCapture, useUpdateCalendarMeetingSeriesCapture } from '@/hooks/queries/useCRMMeetings';
import { detectMeetingPlatform, formatMeetingDate } from '@/lib/meetingPresentation';
import { groupUpcomingMeetings, type UpcomingMeetingGroup } from '@/components/crm/upcomingCalendarMeetingGroups';
import type { CRMCalendarMeetingCandidate } from '@/lib/crmMeetingTypes';

const GOOGLE_CONNECT_UNAVAILABLE_ID = 'google-calendar-connect-unavailable';

function UpcomingRowsLoading() {
  return (
    <QuietSection title="Upcoming" className="px-0 sm:px-0 lg:px-0">
      <div aria-label="Loading upcoming meetings" className="border-t border-quiet-divider-light">
        {Array.from({ length: 2 }).map((_, index) => (
          <div key={index} className="border-b border-quiet-divider-light py-3">
            <Skeleton className="h-3 w-32 rounded-none" />
            <Skeleton className="mt-2 h-4 w-1/2 rounded-none" />
            <Skeleton className="mt-2 h-3 w-52 rounded-none" />
          </div>
        ))}
      </div>
    </QuietSection>
  );
}

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
  calendarConnectUnavailable = false,
  isServerAdmin = false,
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
  /** The server has no usable Google OAuth client, so connecting would fail. */
  calendarConnectUnavailable?: boolean;
  isServerAdmin?: boolean;
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

  if (loading || calendarLoading) return <UpcomingRowsLoading />;
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
    <QuietSection
      title="Upcoming"
      count={candidates.length || undefined}
      className="px-0 sm:px-0 lg:px-0"
    >
      <p className="mb-3 max-w-[680px] text-sm leading-[1.6] text-quiet-text-tertiary">
        Choose which calendar calls Helpin should automatically join.
      </p>

      {!candidates.length ? (
        <QuietEmptyState
          className="border-b-0"
          title={!calendarConnected
            ? 'Connect Google Calendar'
            : !meetingNotesEnabled
              ? 'Turn on meeting notes'
              : 'No upcoming meetings'}
          description={!calendarConnected
            ? 'Bring upcoming calls into this agenda and choose which ones Helpin should join.'
            : !meetingNotesEnabled
              ? 'The notetaker must be enabled before meetings can be selected for automatic joining.'
              : 'Calls with a Google Meet, Zoom, Teams, or Webex link will appear after calendar sync.'}
          action={!calendarConnected ? (
            <div className="flex flex-wrap items-center gap-4">
              {calendarConnectUnavailable ? (
                <ServerSetupNotice
                  id={GOOGLE_CONNECT_UNAVAILABLE_ID}
                  title="Connecting Google Calendar isn’t set up on this server."
                  slug={workspaceSlug}
                  isServerAdmin={isServerAdmin}
                  className="basis-full"
                />
              ) : null}
              <QuietPrimaryAction
                onClick={onConnectCalendar}
                disabled={connectingCalendar || calendarConnectUnavailable}
                aria-describedby={calendarConnectUnavailable ? GOOGLE_CONNECT_UNAVAILABLE_ID : undefined}
              >
                {connectingCalendar ? <Loading01Icon className="h-3.5 w-3.5 animate-spin" /> : <Calendar01Icon className="h-3.5 w-3.5" />}
                Connect Google Calendar
              </QuietPrimaryAction>
              {canEdit ? (
                <QuietTextAction className="border-b border-quiet-field pb-0.5" onClick={onAddMeeting}>
                  Add meeting manually
                </QuietTextAction>
              ) : null}
            </div>
          ) : !meetingNotesEnabled ? (
            canManageSettings ? (
              <QuietTextAction className="border-b border-quiet-field pb-0.5" onClick={onConfigureSettings}>
                Enable meeting notes
              </QuietTextAction>
            ) : (
              <QuietStatusText tone="blocker" className="text-quiet-accent">
                Ask a workspace admin to enable meeting notes
              </QuietStatusText>
            )
          ) : canEdit ? (
            <QuietTextAction className="border-b border-quiet-field pb-0.5" onClick={onAddMeeting}>
              <PlusSignIcon className="h-3.5 w-3.5" />
              Add meeting manually
            </QuietTextAction>
          ) : undefined}
        >
          {calendarConnected && meetingNotesEnabled ? (
            <div className="max-w-[620px] border-t border-quiet-divider-light text-[12.5px] text-quiet-text-tertiary">
              <p className="border-b border-quiet-divider-light py-2.5">Watching synchronized calendar events for a supported meeting link.</p>
              <p className="py-2.5">Recurring series appear once, with each upcoming occurrence available underneath.</p>
            </div>
          ) : null}
        </QuietEmptyState>
      ) : (
        <div className="border-t border-quiet-divider-light">
          {groups.map((group) => {
            const candidate = group.occurrences[0];
            const { event, meeting } = candidate;
            const platform = meeting?.platform ?? detectMeetingPlatform(event.meeting_url ?? '');
            const externalAttendees = event.attendees.filter((attendee) => !attendee.self);
            const captureConfigured = group.recurring
              ? (candidate.series_auto_join ?? group.occurrences.every((occurrence) => occurrence.effective_auto_join))
              : candidate.effective_auto_join;
            const captureEditable = !meeting || ['scheduled', 'failed'].includes(meeting.status);
            const expanded = group.recurring && expandedSeries.has(group.key);
            const openMeeting = () => meeting
              ? navigate({
                  to: '/w/$slug/crm/meetings/$meetingId',
                  params: { slug: workspaceSlug, meetingId: meeting.id },
                })
              : undefined;

            return (
              <div key={group.key}>
                <QuietListRow
                  actor={platform
                    ? <MeetingPlatformLabel platform={platform} compact presentation="quiet" />
                    : 'Calendar event'}
                  meta={group.recurring
                    ? `Next ${formatMeetingDate(event.start_time, 'EEE, MMM d · p')}`
                    : formatMeetingDate(event.start_time, 'EEE, MMM d · p')}
                  title={group.recurring || meeting ? (
                    <button
                      type="button"
                      className="text-left underline-offset-4 hover:underline focus-visible:outline-none focus-visible:underline"
                      onClick={() => group.recurring ? toggleSeriesExpanded(group.key) : void openMeeting()}
                    >
                      {event.title}
                    </button>
                  ) : event.title}
                  detail={[
                    externalAttendees.length > 0
                      ? `${externalAttendees.length} external attendee${externalAttendees.length === 1 ? '' : 's'}`
                      : 'No external attendees',
                    candidate.eligible
                      ? (group.recurring ? `${group.occurrences.length} upcoming occurrences` : 'Ready for automatic joining')
                      : candidate.ineligibility_reason,
                  ].filter(Boolean).join(' · ')}
                  provenance={group.recurring ? 'Series setting applies to future occurrences unless a date is changed below.' : undefined}
                  state={!candidate.eligible ? 'blocker' : captureConfigured ? 'positive' : 'none'}
                  className="grid-cols-1 px-0 sm:grid-cols-[minmax(0,1fr)_auto] sm:px-0"
                  trailing={(
                    <span className="mt-2 flex flex-wrap items-center gap-3 justify-self-start sm:mt-0 sm:justify-self-end">
                      {group.recurring ? (
                        <QuietTextAction onClick={() => toggleSeriesExpanded(group.key)}>
                          {expanded ? 'Hide dates' : 'Show dates'}
                        </QuietTextAction>
                      ) : null}
                      {!group.recurring && meeting && meeting.status !== 'scheduled' ? (
                        <MeetingStatusText status={meeting.status} />
                      ) : (
                        <span className="text-[12.5px] font-medium text-quiet-text-secondary">
                          {group.recurring ? 'Auto-join series' : 'Auto-join'}
                        </span>
                      )}
                      {group.recurring || captureEditable ? (
                        <Switch
                          aria-label={group.recurring ? `Automatically join recurring series ${event.title}` : `Automatically join ${event.title}`}
                          checked={captureConfigured}
                          disabled={!canEdit || !candidate.eligible || updateCapture.isPending || updateSeriesCapture.isPending}
                          onCheckedChange={(enabled) => void (group.recurring ? setSeriesCapture(group, enabled) : setCapture(candidate, enabled))}
                        />
                      ) : meeting ? (
                        <QuietTextAction className="border-b border-quiet-field pb-0.5" onClick={() => void openMeeting()}>
                          Open meeting
                        </QuietTextAction>
                      ) : null}
                    </span>
                  )}
                />

                {expanded ? (
                  <div className="border-b border-quiet-divider-light pl-5 sm:pl-8">
                    {group.occurrences.map((occurrence) => {
                      const occurrenceEditable = !occurrence.meeting || ['scheduled', 'failed'].includes(occurrence.meeting.status);
                      return (
                        <QuietListRow
                          key={occurrence.event.id}
                          actor={formatMeetingDate(occurrence.event.start_time, 'EEE, MMM d')}
                          meta={formatMeetingDate(occurrence.event.start_time, 'p')}
                          title={occurrence.meeting && occurrence.meeting.status !== 'scheduled'
                            ? 'Meeting capture has started'
                            : occurrence.effective_auto_join
                              ? 'Helpin will join this occurrence'
                              : 'Helpin will skip this occurrence'}
                          detail={!occurrence.eligible ? occurrence.ineligibility_reason : undefined}
                          state={!occurrence.eligible ? 'blocker' : occurrence.effective_auto_join ? 'positive' : 'none'}
                          className="grid-cols-1 px-0 sm:grid-cols-[minmax(0,1fr)_auto] sm:px-0"
                          trailing={occurrence.meeting && occurrence.meeting.status !== 'scheduled' ? (
                            <MeetingStatusText status={occurrence.meeting.status} />
                          ) : (
                            <Switch
                              aria-label={`Automatically join ${event.title} on ${formatMeetingDate(occurrence.event.start_time, 'MMM d', 'the scheduled date')}`}
                              checked={occurrence.effective_auto_join}
                              disabled={!canEdit || !occurrence.eligible || !occurrenceEditable || updateCapture.isPending || updateSeriesCapture.isPending}
                              onCheckedChange={(enabled) => void setCapture(occurrence, enabled)}
                            />
                          )}
                        />
                      );
                    })}
                  </div>
                ) : null}
              </div>
            );
          })}
        </div>
      )}
    </QuietSection>
  );
}
