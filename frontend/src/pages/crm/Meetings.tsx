import { useMemo, useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { format } from 'date-fns';
import { Camera01Icon, PlusSignIcon, Search01Icon, Settings02Icon } from '@/lib/icons';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Skeleton } from '@/components/ui/skeleton';
import { CreateMeetingDialog } from '@/components/crm/CreateMeetingDialog';
import { useCRMMeetings, useCRMMeetingSettings } from '@/hooks/queries/useCRMMeetings';
import { usePermissions, useWorkspaceAccess } from '@/hooks/queries/useSession';
import { useTitle } from '@/hooks/useTitle';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import type { CRMMeeting, CRMMeetingStatus } from '@/lib/crmMeetingTypes';

const statusClasses: Record<CRMMeetingStatus, string> = {
  scheduled: 'bg-muted text-muted-foreground',
  joining: 'bg-blue-500/10 text-blue-600',
  waiting: 'bg-amber-500/10 text-amber-600',
  recording: 'bg-red-500/10 text-red-600',
  finalizing: 'bg-violet-500/10 text-violet-600',
  processing: 'bg-violet-500/10 text-violet-600',
  ready: 'bg-emerald-500/10 text-emerald-600',
  failed: 'bg-destructive/10 text-destructive',
  cancelled: 'bg-muted text-muted-foreground',
};

function MeetingRow({ meeting, onOpen }: { meeting: CRMMeeting; onOpen: () => void }) {
  const occurredAt = meeting.scheduled_start_at ?? meeting.actual_start_at ?? meeting.created_at;
  return (
    <button type="button" onClick={onOpen} className="grid w-full grid-cols-[minmax(0,1fr)_auto] gap-4 border-b px-4 py-4 text-left transition-colors hover:bg-muted/40 md:grid-cols-[minmax(0,1fr)_150px_120px_120px]">
      <div className="min-w-0">
        <p className="truncate text-sm font-medium">{meeting.title}</p>
        <p className="mt-1 truncate text-xs text-muted-foreground">{meeting.meeting_url}</p>
      </div>
      <Badge variant="secondary" className={statusClasses[meeting.status]}>{meeting.status.replace('_', ' ')}</Badge>
      <div className="hidden text-sm text-muted-foreground md:block">{meeting.platform.replace('_', ' ')}</div>
      <div className="hidden text-sm text-muted-foreground md:block">{format(new Date(occurredAt), 'MMM d, p')}</div>
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
    <div className="flex h-full flex-col overflow-hidden">
      <div className="flex flex-wrap items-center justify-between gap-3 border-b px-4 py-3">
        <div>
          <h1 className="text-lg font-semibold">Meetings</h1>
          <p className="text-sm text-muted-foreground">Capture conversations and turn commitments into CRM intelligence and project work.</p>
        </div>
        <div className="flex gap-2">
          {canAdmin && <Button variant="outline" size="sm" onClick={() => navigate({ to: '/w/$slug/settings/crm-meetings', params: { slug: workspaceSlug } })}>
            <Settings02Icon className="h-4 w-4" /> Settings
          </Button>}
          {canEdit && <Button size="sm" onClick={() => setShowCreate(true)}><PlusSignIcon className="h-4 w-4" /> Add meeting</Button>}
        </div>
      </div>

      {!settingsData?.settings.enabled && !isLoading && (
        <div className="mx-4 mt-4 flex items-center justify-between gap-4 rounded-lg border bg-muted/20 p-4">
          <div>
            <p className="text-sm font-medium">Enable Meeting Intelligence</p>
            <p className="mt-1 text-xs text-muted-foreground">Configure Recall to let the Helpin notetaker join calls. Vexa remains available behind the same provider switch.</p>
          </div>
          {canAdmin && <Button size="sm" variant="outline" onClick={() => navigate({ to: '/w/$slug/settings/crm-meetings', params: { slug: workspaceSlug } })}>Configure</Button>}
        </div>
      )}

      <div className="border-b p-4">
        <div className="relative max-w-md">
          <Search01Icon className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <Input value={search} onChange={(event) => setSearch(event.target.value)} placeholder="Search meetings" className="pl-9" />
        </div>
      </div>

      <div className="min-h-0 flex-1 overflow-auto">
        {isLoading ? (
          <div className="space-y-2 p-4">{Array.from({ length: 5 }).map((_, index) => <Skeleton key={index} className="h-16 w-full" />)}</div>
        ) : data?.data.length ? (
          <div className="mx-auto mt-4 max-w-6xl overflow-hidden rounded-lg border bg-card">
            {data.data.map((meeting) => (
              <MeetingRow key={meeting.id} meeting={meeting} onOpen={() => navigate({ to: '/w/$slug/crm/meetings/$meetingId', params: { slug: workspaceSlug, meetingId: meeting.id } })} />
            ))}
          </div>
        ) : (
          <div className="flex h-full min-h-96 items-center justify-center p-6 text-center">
            <div className="max-w-md">
              <div className="mx-auto flex h-11 w-11 items-center justify-center rounded-lg border bg-muted/30"><Camera01Icon className="h-5 w-5 text-muted-foreground" /></div>
              <h2 className="mt-4 text-base font-semibold">{search ? 'No matching meetings' : 'Your meeting intelligence lives here'}</h2>
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
