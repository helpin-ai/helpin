import { useMemo, useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { format } from 'date-fns';
import { Camera01Icon, PlusSignIcon, Search01Icon, Settings02Icon } from '@/lib/icons';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { QuickTooltip } from '@/components/ui/quick-tooltip';
import { Skeleton } from '@/components/ui/skeleton';
import { CreateMeetingDialog } from '@/components/crm/CreateMeetingDialog';
import { MeetingPlatformIcon, MeetingPlatformLabel } from '@/components/crm/MeetingPlatform';
import { MeetingStatusBadge } from '@/components/crm/MeetingStatusBadge';
import { useCRMMeetings, useCRMMeetingSettings } from '@/hooks/queries/useCRMMeetings';
import { usePermissions, useWorkspaceAccess } from '@/hooks/queries/useSession';
import { useTitle } from '@/hooks/useTitle';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import type { CRMMeeting } from '@/lib/crmMeetingTypes';

function MeetingRow({ meeting, onOpen }: { meeting: CRMMeeting; onOpen: () => void }) {
  const occurredAt = meeting.scheduled_start_at ?? meeting.actual_start_at ?? meeting.created_at;
  const participantCount = meeting.participants?.length ?? 0;
  return (
    <button type="button" onClick={onOpen} className="grid w-full grid-cols-[minmax(0,1fr)_auto] items-center gap-3 border-b px-3 py-3 text-left transition-colors last:border-b-0 hover:bg-muted/30 md:grid-cols-[minmax(0,1fr)_180px_160px_120px]">
      <div className="flex min-w-0 items-center gap-3">
        <MeetingPlatformIcon platform={meeting.platform} className="md:hidden" />
        <div className="min-w-0">
          <p className="truncate text-sm font-medium">{meeting.title}</p>
          <p className="mt-0.5 truncate text-xs text-muted-foreground">
            {participantCount ? `${participantCount} participant${participantCount === 1 ? '' : 's'}` : 'Participants pending'}
          </p>
        </div>
      </div>
      <div className="hidden min-w-0 text-sm text-muted-foreground md:block"><MeetingPlatformLabel platform={meeting.platform} compact /></div>
      <div className="hidden text-sm text-muted-foreground md:block">{format(new Date(occurredAt), 'MMM d, yyyy · p')}</div>
      <MeetingStatusBadge status={meeting.status} className="justify-self-end" />
    </button>
  );
}

export function MeetingsPage() {
  useTitle('Meetings');
  const workspace = useWorkspaceStore((state) => state.currentWorkspace);
  const workspaceId = workspace?.id ?? '';
  const workspaceSlug = workspace?.slug ?? '';
  const navigate = useNavigate();
  const [search, setSearch] = useState('');
  const { data: access } = useWorkspaceAccess(workspaceId);
  const permissions = usePermissions(access);
  const canEdit = permissions.has('crm.edit');
  const canAdmin = permissions.has('crm.admin');
  const [showCreate, setShowCreate] = useState(false);
  const filters = useMemo(() => ({ search: search.trim() || undefined, per_page: 50 }), [search]);
  const { data, isLoading } = useCRMMeetings(workspaceId, filters);
  const { data: settingsData } = useCRMMeetingSettings(workspaceId);

  return (
    <div className="flex h-full flex-col">
      <header className="ui-divider-bottom-fade flex flex-wrap items-center gap-2 px-3 py-2">
        <div className="relative">
          <Search01Icon className="pointer-events-none absolute left-2 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-muted-foreground" />
          <Input value={search} onChange={(event) => setSearch(event.target.value)} placeholder="Search meetings..." className="h-7 w-52 pl-7 text-xs" />
        </div>
        <div className="ml-auto flex items-center gap-1">
          {canAdmin && (
            <QuickTooltip label="Meeting settings">
              <Button variant="outline" size="icon" className="h-7 w-7" onClick={() => navigate({ to: '/w/$slug/settings/crm-meetings', params: { slug: workspaceSlug } })}>
                <Settings02Icon className="h-3.5 w-3.5" />
              </Button>
            </QuickTooltip>
          )}
          {canEdit && <Button size="sm" className="ml-1 h-7 text-xs" onClick={() => setShowCreate(true)}><PlusSignIcon className="mr-1 h-3.5 w-3.5" /> Meeting</Button>}
        </div>
      </header>

      <div className="min-h-0 flex-1 overflow-auto p-3">
        {settingsData?.settings.enabled === false && !isLoading && (
          <div className="mb-3 flex items-center justify-between gap-4 rounded-lg border bg-muted/20 p-3">
            <div>
              <p className="text-sm font-medium">Enable meeting notes</p>
              <p className="mt-0.5 text-xs text-muted-foreground">Configure meeting capture to let the Helpin notetaker join calls.</p>
            </div>
            {canAdmin && <Button size="sm" className="h-7 text-xs" variant="outline" onClick={() => navigate({ to: '/w/$slug/settings/crm-meetings', params: { slug: workspaceSlug } })}>Configure</Button>}
          </div>
        )}

        {isLoading ? (
          <div className="space-y-2">{Array.from({ length: 5 }).map((_, index) => <Skeleton key={index} className="h-16 w-full" />)}</div>
        ) : data?.data.length ? (
          <div className="overflow-hidden rounded-lg border bg-card">
            <div className="hidden grid-cols-[minmax(0,1fr)_180px_160px_120px] gap-3 border-b bg-muted/20 px-3 py-2 text-[11px] font-medium uppercase tracking-wide text-muted-foreground md:grid">
              <span>Meeting</span><span>Platform</span><span>Date</span><span className="text-right">Status</span>
            </div>
            {data.data.map((meeting) => (
              <MeetingRow key={meeting.id} meeting={meeting} onOpen={() => navigate({ to: '/w/$slug/crm/meetings/$meetingId', params: { slug: workspaceSlug, meetingId: meeting.id } })} />
            ))}
          </div>
        ) : (
          <div className="flex min-h-96 items-center justify-center p-6 text-center">
            <div className="max-w-md">
              <div className="mx-auto flex h-11 w-11 items-center justify-center rounded-lg border bg-muted/30"><Camera01Icon className="h-5 w-5 text-muted-foreground" /></div>
              <h2 className="mt-4 text-base font-semibold">{search ? 'No matching meetings' : 'Your meeting notes live here'}</h2>
              <p className="mt-2 text-sm leading-6 text-muted-foreground">{search ? 'Try a different title.' : 'Add a meeting link to capture a transcript, summary, decisions, risks, and action items.'}</p>
              {!search && canEdit && <Button className="mt-4" size="sm" onClick={() => setShowCreate(true)}><PlusSignIcon className="h-4 w-4" /> Add meeting</Button>}
            </div>
          </div>
        )}
      </div>

      {canEdit && <CreateMeetingDialog open={showCreate} onOpenChange={setShowCreate} workspaceId={workspaceId} workspaceSlug={workspaceSlug} />}
    </div>
  );
}
