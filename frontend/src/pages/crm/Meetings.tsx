import { useMemo, useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { toast } from 'sonner';

import { PlusSignIcon, Search01Icon, Settings02Icon } from '@/lib/icons';
import {
  QuietEmptyState,
  QuietIconAction,
  QuietListRow,
  QuietPageHeader,
  QuietPageViewport,
  QuietPrimaryAction,
  QuietSection,
  QuietStatusText,
  QuietTextAction,
  QuietUnderlineInput,
} from '@/components/design-system/quiet';
import { QuickTooltip } from '@/components/ui/quick-tooltip';
import { Skeleton } from '@/components/ui/skeleton';
import { CreateMeetingDialog } from '@/components/crm/CreateMeetingDialog';
import { MeetingPlatformLabel } from '@/components/crm/MeetingPlatform';
import { MeetingStatusText } from '@/components/crm/MeetingStatusText';
import { UpcomingCalendarMeetings } from '@/components/crm/UpcomingCalendarMeetings';
import { useCRMMeetings, useCRMMeetingSettings, useUpcomingCalendarMeetings } from '@/hooks/queries/useCRMMeetings';
import { useEmailAccounts } from '@/hooks/queries/useCRM';
import { usePermissions, useWorkspaceAccess } from '@/hooks/queries/useSession';
import { useTitle } from '@/hooks/useTitle';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useAuthStore } from '@/stores/authStore';
import { crmEmailService } from '@/lib/services/crmService';
import { formatMeetingDate } from '@/lib/meetingPresentation';
import type { CRMMeeting } from '@/lib/crmMeetingTypes';

function MeetingRow({ meeting, onOpen }: { meeting: CRMMeeting; onOpen: () => void }) {
  const occurredAt = meeting.actual_start_at ?? meeting.scheduled_start_at ?? meeting.created_at;
  const participantCount = meeting.participants?.length ?? 0;
  const durationMinutes = meeting.duration_seconds ? Math.max(1, Math.round(meeting.duration_seconds / 60)) : null;
  const detail = [
    participantCount ? `${participantCount} participant${participantCount === 1 ? '' : 's'}` : 'Participants pending',
    durationMinutes ? `${durationMinutes} min` : null,
  ].filter(Boolean).join(' · ');
  const active = ['joining', 'waiting', 'recording', 'finalizing', 'processing'].includes(meeting.status);

  return (
    <QuietListRow
      onClick={onOpen}
      actor={<MeetingPlatformLabel platform={meeting.platform} compact presentation="quiet" />}
      meta={formatMeetingDate(occurredAt, 'MMM d, yyyy · p')}
      title={meeting.title}
      detail={detail}
      trailing={<MeetingStatusText status={meeting.status} />}
      state={meeting.status === 'failed' ? 'blocker' : active ? 'current' : 'none'}
      className="py-[13px]"
    />
  );
}
function MeetingRowsLoading() {
  return (
    <div aria-label="Loading meetings">
      {Array.from({ length: 4 }).map((_, index) => (
        <div key={index} className="border-b border-quiet-divider-light py-3">
          <Skeleton className="h-3 w-28 rounded-none" />
          <Skeleton className="mt-2 h-4 w-2/3 rounded-none" />
          <Skeleton className="mt-2 h-3 w-40 rounded-none" />
        </div>
      ))}
    </div>
  );
}

export function MeetingsPage() {
  useTitle('Meetings');
  const workspace = useWorkspaceStore((state) => state.currentWorkspace);
  const user = useAuthStore((state) => state.user);
  const workspaceId = workspace?.id ?? '';
  const workspaceSlug = workspace?.slug ?? '';
  const navigate = useNavigate();
  const [search, setSearch] = useState('');
  const { data: access } = useWorkspaceAccess(workspaceId);
  const permissions = usePermissions(access);
  const canEdit = permissions.has('crm.edit');
  const canAdmin = permissions.has('crm.admin');
  const [showCreate, setShowCreate] = useState(false);
  const [connectingCalendar, setConnectingCalendar] = useState(false);
  const filters = useMemo(() => ({ search: search.trim() || undefined, per_page: 50 }), [search]);
  const meetingsQuery = useCRMMeetings(workspaceId, filters);
  const upcomingQuery = useUpcomingCalendarMeetings(workspaceId);
  const settingsQuery = useCRMMeetingSettings(workspaceId);
  const accountsQuery = useEmailAccounts(workspaceId, user?.id ? { member_id: user.id } : undefined);
  const normalizedSearch = search.trim().toLowerCase();
  const upcomingCandidates = (upcomingQuery.data?.data ?? []).filter(
    (candidate) => !normalizedSearch || candidate.event.title.toLowerCase().includes(normalizedSearch),
  );
  const historyMeetings = (meetingsQuery.data?.data ?? []).filter((meeting) => {
    if (!meeting.calendar_event_id || meeting.status !== 'scheduled' || !meeting.scheduled_start_at) return true;
    return new Date(meeting.scheduled_start_at).getTime() <= Date.now();
  });
  const hasUpcoming = upcomingCandidates.length > 0;
  const calendarConnected = (accountsQuery.data ?? []).some(
    (account) => account.member_id === user?.id
      && account.provider === 'gmail'
      && account.status === 'connected'
      && account.is_active,
  );
  const meetingNotesEnabled = settingsQuery.data?.settings.enabled ?? true;
  const upcomingUnavailable = upcomingQuery.isError || settingsQuery.isError || accountsQuery.isError;
  const loadingSearch = meetingsQuery.isLoading || upcomingQuery.isLoading || settingsQuery.isLoading || accountsQuery.isLoading;
  const noSearchResults = Boolean(normalizedSearch)
    && !loadingSearch
    && !upcomingUnavailable
    && !meetingsQuery.isError
    && !hasUpcoming
    && historyMeetings.length === 0;

  const connectGoogleCalendar = async () => {
    setConnectingCalendar(true);
    try {
      const { data: oauth, error } = await crmEmailService.initiateOAuth(workspaceId, 'gmail');
      if (error || !oauth) {
        toast.error(error || 'Unable to connect Google Calendar');
        return;
      }
      window.location.href = oauth.redirect_url;
    } catch {
      toast.error('Unable to connect Google Calendar');
    } finally {
      setConnectingCalendar(false);
    }
  };

  const openMeetingSettings = () => navigate({
    to: '/w/$slug/settings/crm-meetings',
    params: { slug: workspaceSlug },
  });

  return (
    <>
      <div className="flex h-full min-h-0 flex-col">
        <QuietPageHeader
          variant="shell"
          title="Meetings"
          description="Choose the calls Helpin should join, then review the recording, transcript, decisions, and follow-up work in one place."
          actions={(
            <>
              {canAdmin ? (
                <QuickTooltip label="Meeting settings">
                  <QuietIconAction aria-label="Meeting settings" onClick={openMeetingSettings}>
                    <Settings02Icon className="h-[15px] w-[15px]" />
                  </QuietIconAction>
                </QuickTooltip>
              ) : null}
              {canEdit ? (
                <QuietPrimaryAction className="gap-1.5" onClick={() => setShowCreate(true)}>
                  <PlusSignIcon className="h-4 w-4" />
                  Add meeting
                </QuietPrimaryAction>
              ) : null}
            </>
          )}
        />

        <QuietPageViewport className="min-h-0 flex-1">
        <div className="mb-6 flex items-center justify-between gap-4 border-b border-quiet-divider-strong pb-4">
          <div className="relative w-full max-w-xs">
            <Search01Icon className="pointer-events-none absolute left-0.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-quiet-muted" />
            <QuietUnderlineInput
              aria-label="Search meetings"
              value={search}
              onChange={(event) => setSearch(event.target.value)}
              placeholder="Search meetings"
              className="w-full pl-6"
            />
          </div>
        </div>

        {settingsQuery.data?.settings.enabled === false && hasUpcoming && !meetingsQuery.isLoading ? (
          <QuietSection
            title="Meeting capture"
            className="mb-6 border-t px-0 sm:px-0 lg:px-0"
            action={canAdmin ? (
              <QuietTextAction className="border-b border-quiet-field pb-0.5" onClick={openMeetingSettings}>
                Configure meeting notes
              </QuietTextAction>
            ) : undefined}
          >
            <QuietStatusText tone="blocker" className="text-quiet-accent">Meeting notes are off</QuietStatusText>
            <p className="mt-1 max-w-[680px] text-sm leading-[1.6] text-quiet-text-tertiary">
              Helpin cannot join selected calendar calls until a workspace admin enables meeting notes.
            </p>
          </QuietSection>
        ) : null}

        {noSearchResults ? (
          <QuietEmptyState
            title="No meetings match this search"
            description="Try a shorter meeting title or clear the search to see upcoming and recorded meetings."
            action={(
              <QuietTextAction className="border-b border-quiet-field pb-0.5" onClick={() => setSearch('')}>
                Clear search
              </QuietTextAction>
            )}
          />
        ) : (
          <>
            {upcomingUnavailable ? (
              <QuietSection title="Upcoming" className="px-0 sm:px-0 lg:px-0">
                <QuietEmptyState
                  className="border-b-0"
                  title="Upcoming meetings could not be loaded"
                  description="Calendar and capture settings are temporarily unavailable. Your existing meeting choices have not changed."
                  action={(
                    <QuietTextAction
                      className="border-b border-quiet-field pb-0.5"
                      onClick={() => void Promise.all([
                        upcomingQuery.refetch(),
                        settingsQuery.refetch(),
                        accountsQuery.refetch(),
                      ])}
                    >
                      Try again
                    </QuietTextAction>
                  )}
                />
              </QuietSection>
            ) : (
              <UpcomingCalendarMeetings
                workspaceId={workspaceId}
                workspaceSlug={workspaceSlug}
                candidates={upcomingCandidates}
                canEdit={canEdit}
                canManageSettings={canAdmin}
                loading={upcomingQuery.isLoading}
                calendarConnected={calendarConnected}
                calendarLoading={accountsQuery.isLoading || settingsQuery.isLoading}
                connectingCalendar={connectingCalendar}
                meetingNotesEnabled={meetingNotesEnabled}
                searching={Boolean(normalizedSearch)}
                onConnectCalendar={() => void connectGoogleCalendar()}
                onConfigureSettings={openMeetingSettings}
                onAddMeeting={() => setShowCreate(true)}
              />
            )}

            <QuietSection
              title="Meeting history"
              count={historyMeetings.length || undefined}
              className="px-0 sm:px-0 lg:px-0"
            >
              {meetingsQuery.isLoading ? (
                <MeetingRowsLoading />
              ) : meetingsQuery.isError ? (
                <QuietEmptyState
                  className="border-b-0"
                  title="Meeting history could not be loaded"
                  description="Recorded meetings are temporarily unavailable. No meeting data has been changed."
                  action={(
                    <QuietTextAction className="border-b border-quiet-field pb-0.5" onClick={() => void meetingsQuery.refetch()}>
                      Try again
                    </QuietTextAction>
                  )}
                />
              ) : historyMeetings.length ? (
                <div className="border-t border-quiet-divider-light">
                  {historyMeetings.map((meeting) => (
                    <MeetingRow
                      key={meeting.id}
                      meeting={meeting}
                      onOpen={() => navigate({
                        to: '/w/$slug/crm/meetings/$meetingId',
                        params: { slug: workspaceSlug, meetingId: meeting.id },
                      })}
                    />
                  ))}
                </div>
              ) : (
                <QuietEmptyState
                  className="border-b-0"
                  title="Meeting notes will collect here"
                  description="After Helpin joins a call, its recording, transcript, decisions, and next steps will appear in this history."
                  action={canEdit ? (
                    <QuietTextAction className="border-b border-quiet-field pb-0.5" onClick={() => setShowCreate(true)}>
                      Add a meeting manually
                    </QuietTextAction>
                  ) : undefined}
                />
              )}
            </QuietSection>
          </>
        )}
        </QuietPageViewport>
      </div>

      {canEdit ? (
        <CreateMeetingDialog
          open={showCreate}
          onOpenChange={setShowCreate}
          workspaceId={workspaceId}
          workspaceSlug={workspaceSlug}
        />
      ) : null}
    </>
  );
}
