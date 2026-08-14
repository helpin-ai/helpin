import { useMemo, useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { format } from 'date-fns';
import { toast } from 'sonner';
import {
  ArrowLeft02Icon,
  Copy01Icon,
  Loading01Icon,
  PlayCircleIcon,
  StopIcon,
} from '@/lib/icons';
import { AssociationsList } from '@/components/crm/AssociationsList';
import { UpgradeRequiredDialog } from '@/components/billing/UpgradeRequiredDialog';
import { MarkdownContent } from '@/components/pm/CodingSession/MarkdownContent';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Skeleton } from '@/components/ui/skeleton';
import {
  useAcceptMeetingAction,
  useCRMMeeting,
  useDismissMeetingAction,
  useRetryMeetingProcessing,
  useStartMeetingCapture,
  useStopMeetingCapture,
} from '@/hooks/queries/useCRMMeetings';
import { usePermissions, useWorkspaceAccess } from '@/hooks/queries/useSession';
import { useAccessibleTeams } from '@/hooks/useAccessibleTeams';
import { useTitle } from '@/hooks/useTitle';
import { crmMeetingService } from '@/lib/services/crmMeetingService';
import { getUpgradeRequiredReason, type UpgradeRequiredReason } from '@/lib/upgradeRequired';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import type { CRMMeetingActionItem, CRMMeetingStatus } from '@/lib/crmMeetingTypes';

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

const titleCase = (value: string) => value.replace(/_/g, ' ').replace(/\b\w/g, (letter) => letter.toUpperCase());

function IntelligenceList({ title, items }: { title: string; items?: string[] }) {
  if (!items?.length) return null;
  return (
    <div>
      <h3 className="text-sm font-semibold">{title}</h3>
      <ul className="mt-2 space-y-2 text-sm leading-6 text-muted-foreground">
        {items.map((item, index) => <li key={`${title}-${index}`} className="flex gap-2"><span className="mt-2 h-1.5 w-1.5 shrink-0 rounded-full bg-primary/70" /><span>{item}</span></li>)}
      </ul>
    </div>
  );
}

function ActionItemRow({
  item,
  teamId,
  teams,
  busy,
  canCreateTask,
  canDismiss,
  onTeamChange,
  onAccept,
  onDismiss,
}: {
  item: CRMMeetingActionItem;
  teamId: string;
  teams: Array<{ id: string; name: string }>;
  busy: boolean;
  canCreateTask: boolean;
  canDismiss: boolean;
  onTeamChange: (value: string) => void;
  onAccept: () => void;
  onDismiss: () => void;
}) {
  return (
    <div className="rounded-lg border p-3">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <p className="text-sm font-medium">{item.title}</p>
          {item.details && <p className="mt-1 text-xs leading-5 text-muted-foreground">{item.details}</p>}
          <div className="mt-2 flex flex-wrap gap-2 text-[11px] text-muted-foreground">
            {item.assignee_name && <span>Owner: {item.assignee_name}</span>}
            {item.due_date && <span>Due {format(new Date(`${item.due_date}T00:00:00`), 'MMM d')}</span>}
          </div>
        </div>
        {item.status !== 'pending' && <Badge variant="secondary">{titleCase(item.status)}</Badge>}
      </div>
      {item.evidence?.excerpt && <blockquote className="mt-3 border-l-2 pl-3 text-xs italic text-muted-foreground">{item.evidence.excerpt}</blockquote>}
      {item.status === 'pending' && (canCreateTask || canDismiss) && (
        <div className="mt-3 flex flex-wrap items-center gap-2">
          {canCreateTask && <>
            <Select value={teamId} onValueChange={onTeamChange}>
              <SelectTrigger className="h-8 min-w-44 flex-1"><SelectValue placeholder="Select destination team" /></SelectTrigger>
              <SelectContent>{teams.map((team) => <SelectItem key={team.id} value={team.id}>{team.name}</SelectItem>)}</SelectContent>
            </Select>
            <Button size="sm" className="h-8" onClick={onAccept} disabled={!teamId || busy}>Create task</Button>
          </>}
          {canDismiss && <Button size="sm" variant="ghost" className="h-8" onClick={onDismiss} disabled={busy}>Dismiss</Button>}
        </div>
      )}
    </div>
  );
}

export function MeetingDetailPage({ meetingId }: { meetingId: string }) {
  const workspace = useWorkspaceStore((state) => state.currentWorkspace);
  const workspaceId = workspace?.id ?? '';
  const slug = workspace?.slug ?? '';
  const navigate = useNavigate();
  const { data: access } = useWorkspaceAccess(workspaceId);
  const permissions = usePermissions(access);
  const canEditCRM = permissions.has('crm.edit');
  const canEditPM = permissions.has('pm.edit');
  const { data, isLoading, refetch } = useCRMMeeting(workspaceId, meetingId);
  const startCapture = useStartMeetingCapture(workspaceId, meetingId);
  const stopCapture = useStopMeetingCapture(workspaceId, meetingId);
  const retryProcessing = useRetryMeetingProcessing(workspaceId, meetingId);
  const acceptAction = useAcceptMeetingAction(workspaceId, meetingId);
  const dismissAction = useDismissMeetingAction(workspaceId, meetingId);
  const { teams } = useAccessibleTeams(workspaceId);
  const [teamByAction, setTeamByAction] = useState<Record<string, string>>({});
  const [upgradeReason, setUpgradeReason] = useState<UpgradeRequiredReason | null>(null);
  const [recordingLoading, setRecordingLoading] = useState(false);
  useTitle(data?.meeting.title ?? 'Meeting');

  const defaultTeamId = teams[0]?.id ?? '';
  const transcriptText = data?.transcript?.plain_text ?? '';
  const occurredAt = data?.meeting.actual_start_at ?? data?.meeting.scheduled_start_at ?? data?.meeting.created_at;
  const duration = useMemo(() => {
    const seconds = data?.meeting.duration_seconds ?? 0;
    if (!seconds) return null;
    const minutes = Math.floor(seconds / 60);
    return `${minutes}m ${seconds % 60}s`;
  }, [data?.meeting.duration_seconds]);

  const runCommand = async (command: () => Promise<unknown>, success: string) => {
    try {
      await command();
      toast.success(success);
    } catch (error) {
      const reason = getUpgradeRequiredReason(error);
      if (reason) {
        setUpgradeReason(reason);
        return;
      }
      toast.error(error instanceof Error ? error.message : 'Unable to update meeting');
    }
  };

  const copyText = async (value: string, label: string) => {
    await navigator.clipboard.writeText(value);
    toast.success(`${label} copied`);
  };

  const openRecording = async () => {
    setRecordingLoading(true);
    try {
      const response = await crmMeetingService.getRecording(workspaceId, meetingId);
      if (response.error || !response.data?.url) throw new Error(response.error ?? 'Recording is unavailable');
      window.open(response.data.url, '_blank', 'noopener,noreferrer');
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Unable to open recording');
    } finally {
      setRecordingLoading(false);
    }
  };

  const accept = async (item: CRMMeetingActionItem) => {
    const teamId = teamByAction[item.id] ?? defaultTeamId;
    try {
      await acceptAction.mutateAsync({ itemId: item.id, payload: { team_id: teamId } });
      toast.success('Project task created');
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Unable to create task');
    }
  };

  const dismiss = async (item: CRMMeetingActionItem) => {
    try {
      await dismissAction.mutateAsync(item.id);
      toast.success('Action item dismissed');
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Unable to dismiss action item');
    }
  };

  if (isLoading) return <div className="space-y-4 p-5"><Skeleton className="h-12 w-2/3" /><Skeleton className="h-72 w-full" /></div>;
  if (!data) return <div className="flex h-full items-center justify-center text-sm text-muted-foreground">Meeting not found.</div>;

  const meeting = data.meeting;
  const canStart = ['scheduled', 'failed'].includes(meeting.status);
  const canStop = ['joining', 'waiting', 'recording'].includes(meeting.status);
  const canRetry = ['failed', 'blocked_usage'].includes(meeting.summary_status);

  return (
    <div className="h-full overflow-auto">
      <div className="sticky top-0 z-10 flex flex-wrap items-center justify-between gap-3 border-b bg-background/95 px-4 py-3 backdrop-blur">
        <div className="flex min-w-0 items-center gap-3">
          <Button variant="ghost" size="icon" className="h-8 w-8" onClick={() => navigate({ to: '/w/$slug/crm/meetings', params: { slug } })}><ArrowLeft02Icon className="h-4 w-4" /></Button>
          <div className="min-w-0">
            <div className="flex items-center gap-2"><h1 className="truncate text-base font-semibold">{meeting.title}</h1><Badge variant="secondary" className={statusClasses[meeting.status]}>{titleCase(meeting.status)}</Badge></div>
            <p className="mt-0.5 text-xs text-muted-foreground">{titleCase(meeting.platform)} · {format(new Date(occurredAt ?? meeting.created_at), 'MMM d, yyyy, p')}{duration ? ` · ${duration}` : ''}</p>
          </div>
        </div>
        <div className="flex items-center gap-2">
          {meeting.recording_object_key && <Button variant="outline" size="sm" onClick={openRecording} disabled={recordingLoading}>{recordingLoading ? <Loading01Icon className="h-4 w-4 animate-spin" /> : <PlayCircleIcon className="h-4 w-4" />} Recording</Button>}
          {canEditCRM && canStart && <Button size="sm" onClick={() => runCommand(() => startCapture.mutateAsync(), 'Helpin is joining the meeting')} disabled={startCapture.isPending}><PlayCircleIcon className="h-4 w-4" /> Start capture</Button>}
          {canEditCRM && canStop && <Button size="sm" variant="destructive" onClick={() => runCommand(() => stopCapture.mutateAsync(), 'Capture is finalizing')} disabled={stopCapture.isPending}><StopIcon className="h-4 w-4" /> Stop</Button>}
          {canEditCRM && canRetry && <Button size="sm" onClick={() => runCommand(() => retryProcessing.mutateAsync(), 'Meeting processing restarted')} disabled={retryProcessing.isPending}>Retry processing</Button>}
        </div>
      </div>

      <div className="mx-auto grid max-w-[1500px] gap-5 p-5 lg:grid-cols-[minmax(0,1fr)_300px]">
        <main className="min-w-0 space-y-5">
          {meeting.failure_message && <div className="rounded-lg border border-destructive/30 bg-destructive/5 p-4 text-sm text-destructive"><p className="font-medium">Meeting capture needs attention</p><p className="mt-1">{meeting.failure_message}</p></div>}
          {meeting.summary_status === 'blocked_usage' && <div className="rounded-lg border border-amber-500/30 bg-amber-500/5 p-4 text-sm text-amber-700 dark:text-amber-300"><p className="font-medium">Transcript captured; AI processing is paused</p><p className="mt-1">Upgrade or add AI capacity, then retry processing. The transcript remains available.</p></div>}

          <Card>
            <CardHeader className="flex-row items-start justify-between gap-3">
              <div><CardTitle className="text-base">Meeting intelligence</CardTitle><CardDescription>Helpin-generated outcomes from the canonical transcript.</CardDescription></div>
              {data.intelligence?.summary_markdown && <Button variant="ghost" size="sm" onClick={() => copyText(data.intelligence?.summary_markdown ?? '', 'Summary')}><Copy01Icon className="h-4 w-4" /> Copy</Button>}
            </CardHeader>
            <CardContent>
              {data.intelligence ? (
                <div className="space-y-6">
                  <MarkdownContent content={data.intelligence.summary_markdown} className="text-sm leading-7" />
                  <div className="grid gap-6 md:grid-cols-2">
                    <IntelligenceList title="Key points" items={data.intelligence.key_points} />
                    <IntelligenceList title="Decisions" items={data.intelligence.decisions} />
                    <IntelligenceList title="Risks" items={data.intelligence.risks} />
                    <IntelligenceList title="Next steps" items={data.intelligence.next_steps} />
                    <IntelligenceList title="Objections" items={data.intelligence.objections} />
                  </div>
                  {(data.intelligence.follow_up_draft.subject || data.intelligence.follow_up_draft.body) && (
                    <div className="rounded-lg border bg-muted/20 p-4">
                      <div className="flex items-center justify-between gap-3"><h3 className="text-sm font-semibold">Follow-up draft</h3><Button variant="ghost" size="sm" onClick={() => copyText([data.intelligence?.follow_up_draft.subject, data.intelligence?.follow_up_draft.body].filter(Boolean).join('\n\n'), 'Follow-up')}><Copy01Icon className="h-4 w-4" /> Copy</Button></div>
                      {data.intelligence.follow_up_draft.subject && <p className="mt-3 text-sm font-medium">{data.intelligence.follow_up_draft.subject}</p>}
                      {data.intelligence.follow_up_draft.body && <p className="mt-2 whitespace-pre-wrap text-sm leading-6 text-muted-foreground">{data.intelligence.follow_up_draft.body}</p>}
                    </div>
                  )}
                </div>
              ) : (
                <div className="py-10 text-center text-sm text-muted-foreground">{meeting.summary_status === 'processing' ? 'Generating summary, decisions, risks, and next steps…' : 'Intelligence will appear when the transcript is processed.'}</div>
              )}
            </CardContent>
          </Card>

          <Card>
            <CardHeader><CardTitle className="text-base">Action items</CardTitle><CardDescription>Review proposed work before creating canonical project tasks.</CardDescription></CardHeader>
            <CardContent className="space-y-3">
              {data.action_items.length ? data.action_items.map((item) => (
                <ActionItemRow key={item.id} item={item} teams={teams} teamId={teamByAction[item.id] ?? defaultTeamId} busy={acceptAction.isPending || dismissAction.isPending} canCreateTask={canEditCRM && canEditPM} canDismiss={canEditCRM} onTeamChange={(value) => setTeamByAction((current) => ({ ...current, [item.id]: value }))} onAccept={() => accept(item)} onDismiss={() => dismiss(item)} />
              )) : <p className="py-6 text-center text-sm text-muted-foreground">No action items were identified.</p>}
            </CardContent>
          </Card>

          <Card>
            <CardHeader className="flex-row items-start justify-between gap-3">
              <div><CardTitle className="text-base">Transcript</CardTitle><CardDescription>{data.transcript?.language ? `Language: ${data.transcript.language}` : 'Speaker-attributed meeting transcript'}</CardDescription></div>
              {transcriptText && <Button variant="ghost" size="sm" onClick={() => copyText(transcriptText, 'Transcript')}><Copy01Icon className="h-4 w-4" /> Copy</Button>}
            </CardHeader>
            <CardContent>
              {data.transcript?.segments?.length ? (
                <div className="space-y-4">{data.transcript.segments.map((segment) => <div key={segment.id} className="grid gap-1 sm:grid-cols-[120px_minmax(0,1fr)]"><div className="text-xs font-medium">{segment.speaker_name || 'Speaker'}<span className="ml-2 text-[10px] font-normal text-muted-foreground">{Math.floor(segment.start_seconds / 60)}:{String(Math.floor(segment.start_seconds % 60)).padStart(2, '0')}</span></div><p className="text-sm leading-6 text-muted-foreground">{segment.text}</p></div>)}</div>
              ) : transcriptText ? <p className="whitespace-pre-wrap text-sm leading-7 text-muted-foreground">{transcriptText}</p> : <p className="py-8 text-center text-sm text-muted-foreground">Transcript is not available yet.</p>}
            </CardContent>
          </Card>
        </main>

        <aside className="min-w-0 space-y-5">
          <Card>
            <CardHeader><CardTitle className="text-sm">Capture details</CardTitle></CardHeader>
            <CardContent className="space-y-3 text-xs">
              <div className="flex justify-between gap-3"><span className="text-muted-foreground">Provider</span><span className="font-medium">{data.capture ? titleCase(data.capture.provider) : 'Not started'}</span></div>
              <div className="flex justify-between gap-3"><span className="text-muted-foreground">Visibility</span><span className="font-medium">{titleCase(meeting.visibility)}</span></div>
              <div className="flex justify-between gap-3"><span className="text-muted-foreground">Audio</span><span className="font-medium">{meeting.record_audio ? 'Recorded' : 'Transcript only'}</span></div>
              <div className="flex justify-between gap-3"><span className="text-muted-foreground">Participants</span><span className="font-medium">{meeting.participants?.length ?? 0}</span></div>
              <a href={meeting.meeting_url} target="_blank" rel="noreferrer" className="block truncate pt-1 text-primary hover:underline">Open original meeting</a>
            </CardContent>
          </Card>
          <Card>
            <CardHeader><CardTitle className="text-sm">Associations</CardTitle><CardDescription>Connect this meeting to CRM records and project work.</CardDescription></CardHeader>
            <CardContent className="pt-0"><AssociationsList workspaceId={workspaceId} slug={slug} associations={data.associations} currentObjectType="meeting" currentObjectId={meetingId} onAssociationRemoved={() => void refetch()} editable={canEditCRM} /></CardContent>
          </Card>
        </aside>
      </div>

      <UpgradeRequiredDialog open={upgradeReason !== null} onOpenChange={(open) => { if (!open) setUpgradeReason(null); }} reason={upgradeReason} />
    </div>
  );
}
