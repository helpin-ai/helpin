import { useMemo, useRef, useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { format } from 'date-fns';
import { toast } from 'sonner';
import {
  ArrowLeft02Icon,
  Copy01Icon,
  LinkSquare01Icon,
  PlayCircleIcon,
  StopIcon,
} from '@/lib/icons';
import { AssociationsList } from '@/components/crm/AssociationsList';
import { MeetingPlatformIcon } from '@/components/crm/MeetingPlatform';
import { MeetingRecordingPlayer, type MeetingRecordingPlayerHandle } from '@/components/crm/MeetingRecordingPlayer';
import { getMeetingPlatformLabel } from '@/lib/meetingPresentation';
import { MeetingProcessingState } from '@/components/crm/MeetingProcessingState';
import { MeetingStatusBadge } from '@/components/crm/MeetingStatusBadge';
import { UpgradeRequiredDialog } from '@/components/billing/UpgradeRequiredDialog';
import { MarkdownContent } from '@/components/pm/CodingSession/MarkdownContent';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardAction, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
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
import { getUpgradeRequiredReason, type UpgradeRequiredReason } from '@/lib/upgradeRequired';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import type { CRMMeetingActionItem } from '@/lib/crmMeetingTypes';

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
  const recordingPlayerRef = useRef<MeetingRecordingPlayerHandle | null>(null);
  const [playbackSeconds, setPlaybackSeconds] = useState(0);
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
  const participantContext = data.intelligence?.participants_context?.length
    ? data.intelligence.participants_context
    : meeting.participants
      .map((participant) => participant.name && participant.email
        ? `${participant.name} (${participant.email})`
        : participant.name || participant.email || '')
      .filter(Boolean);
  const openQuestions = data.intelligence?.open_questions?.length
    ? data.intelligence.open_questions
    : [...new Set([...(data.intelligence?.objections ?? []), ...(data.intelligence?.risks ?? [])])];

  return (
    <div className="flex h-full flex-col overflow-hidden">
      <div className="flex shrink-0 flex-wrap items-center justify-between gap-3 border-b border-border/60 px-4 py-2.5">
        <div className="flex min-w-0 items-center gap-3">
          <Button variant="ghost" size="icon" className="h-8 w-8" onClick={() => navigate({ to: '/w/$slug/crm/meetings', params: { slug } })}><ArrowLeft02Icon className="h-4 w-4" /></Button>
          <MeetingPlatformIcon platform={meeting.platform} size="sm" />
          <div className="min-w-0">
            <div className="flex items-center gap-2"><h1 className="truncate text-base font-semibold">{meeting.title}</h1><MeetingStatusBadge status={meeting.status} /></div>
            <p className="mt-0.5 text-xs text-muted-foreground">{getMeetingPlatformLabel(meeting.platform)} · {format(new Date(occurredAt ?? meeting.created_at), 'MMM d, yyyy, p')}{duration ? ` · ${duration}` : ''}</p>
          </div>
        </div>
        <div className="flex items-center gap-2">
          <Button asChild variant="outline" size="sm"><a href={meeting.meeting_url} target="_blank" rel="noreferrer"><MeetingPlatformIcon platform={meeting.platform} size="sm" className="-ml-1 border-0 shadow-none" /> Open {getMeetingPlatformLabel(meeting.platform)} <LinkSquare01Icon className="h-3.5 w-3.5" /></a></Button>
          {meeting.recording_object_key && <Button variant="outline" size="sm" onClick={() => recordingPlayerRef.current?.focus()}><PlayCircleIcon className="h-4 w-4" /> Recording</Button>}
          {canEditCRM && canStart && <Button size="sm" onClick={() => runCommand(() => startCapture.mutateAsync(), 'Helpin is joining the meeting')} disabled={startCapture.isPending}><PlayCircleIcon className="h-4 w-4" /> Start capture</Button>}
          {canEditCRM && canStop && <Button size="sm" variant="destructive" onClick={() => runCommand(() => stopCapture.mutateAsync(), 'Capture is finalizing')} disabled={stopCapture.isPending}><StopIcon className="h-4 w-4" /> Stop</Button>}
          {canEditCRM && canRetry && <Button size="sm" onClick={() => runCommand(() => retryProcessing.mutateAsync(), 'Meeting processing restarted')} disabled={retryProcessing.isPending}>Retry processing</Button>}
        </div>
      </div>

      <div className="mx-auto grid min-h-0 w-full max-w-[1500px] flex-1 gap-5 overflow-auto p-4 lg:grid-cols-[minmax(0,1fr)_300px] lg:p-6">
        <main className="min-w-0 space-y-5">
          {meeting.failure_message && <div className="rounded-lg border border-destructive/30 bg-destructive/5 p-4 text-sm text-destructive"><p className="font-medium">Meeting capture needs attention</p><p className="mt-1">{meeting.failure_message}</p></div>}
          {meeting.recording_object_key && (
            <MeetingRecordingPlayer
              ref={recordingPlayerRef}
              workspaceId={workspaceId}
              meetingId={meetingId}
              onTimeUpdate={setPlaybackSeconds}
            />
          )}
          {meeting.summary_status === 'blocked_usage' && <div className="rounded-lg border border-amber-500/30 bg-amber-500/5 p-4 text-sm text-amber-700 dark:text-amber-300"><p className="font-medium">Transcript captured; AI processing is paused</p><p className="mt-1">Upgrade or add AI capacity, then retry processing. The transcript remains available.</p></div>}

          <Card>
            <CardHeader>
              <CardTitle className="text-base">Meeting notes</CardTitle>
              <CardDescription>A clear record of the discussion, decisions, and next steps.</CardDescription>
              {data.intelligence?.summary_markdown && <CardAction><Button variant="outline" size="sm" className="h-7 text-xs" onClick={() => copyText(data.intelligence?.summary_markdown ?? '', 'Overview')}><Copy01Icon className="h-3.5 w-3.5" /> Copy overview</Button></CardAction>}
            </CardHeader>
            <CardContent>
              {data.intelligence ? (
                <div className="space-y-7 motion-safe:animate-in motion-safe:fade-in motion-safe:slide-in-from-bottom-1 motion-safe:duration-500">
                  <section>
                    <h3 className="text-sm font-semibold">Overview</h3>
                    <MarkdownContent content={data.intelligence.summary_markdown} className="mt-2 text-sm leading-7" />
                  </section>
                  <IntelligenceList title="Participants + Context" items={participantContext} />
                  <IntelligenceList title="Key Discussion Points" items={data.intelligence.key_points} />
                  <IntelligenceList title="Decisions Made" items={data.intelligence.decisions} />
                  <IntelligenceList title="Open Questions / Issues" items={openQuestions} />
                  {(data.intelligence.next_steps.length > 0 || data.action_items.length > 0) && (
                    <section>
                      <h3 className="text-sm font-semibold">Action Items &amp; Next Steps</h3>
                      {data.intelligence.next_steps.length > 0 && (
                        <ul className="mt-2 space-y-2 text-sm leading-6 text-muted-foreground">
                          {data.intelligence.next_steps.map((item, index) => <li key={`next-step-${index}`} className="flex gap-2"><span className="mt-2 h-1.5 w-1.5 shrink-0 rounded-full bg-primary/70" /><span>{item}</span></li>)}
                        </ul>
                      )}
                      {data.action_items.length > 0 && (
                        <div className="mt-3 space-y-3">
                          {data.action_items.map((item) => (
                            <ActionItemRow key={item.id} item={item} teams={teams} teamId={teamByAction[item.id] ?? defaultTeamId} busy={acceptAction.isPending || dismissAction.isPending} canCreateTask={canEditCRM && canEditPM} canDismiss={canEditCRM} onTeamChange={(value) => setTeamByAction((current) => ({ ...current, [item.id]: value }))} onAccept={() => accept(item)} onDismiss={() => dismiss(item)} />
                          ))}
                        </div>
                      )}
                    </section>
                  )}
                  <IntelligenceList title="Rapport" items={data.intelligence.rapport} />
                </div>
              ) : (
                (['joining', 'waiting', 'recording', 'finalizing', 'processing'].includes(meeting.status) || meeting.summary_status === 'processing') ? <MeetingProcessingState status={meeting.status} summaryStatus={meeting.summary_status} /> : <div className="py-10 text-center text-sm text-muted-foreground">Meeting notes will appear when the transcript is processed.</div>
              )}
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle className="text-base">Transcript</CardTitle>
              <CardDescription>{data.transcript?.language ? 'Language: ' + data.transcript.language : 'Speaker-attributed meeting transcript'}</CardDescription>
              {transcriptText && <CardAction><Button variant="outline" size="sm" className="h-7 text-xs" onClick={() => copyText(transcriptText, 'Transcript')}><Copy01Icon className="h-3.5 w-3.5" /> Copy transcript</Button></CardAction>}
            </CardHeader>
            <CardContent>
              {data.transcript?.segments?.length ? (
                <div className="space-y-2">
                  {data.transcript.segments.map((segment) => {
                    const segmentEnd = Math.max(segment.end_seconds, segment.start_seconds + 0.5);
                    const active = Boolean(meeting.recording_object_key) && playbackSeconds >= segment.start_seconds && playbackSeconds < segmentEnd;
                    const timestamp = `${Math.floor(segment.start_seconds / 60)}:${String(Math.floor(segment.start_seconds % 60)).padStart(2, '0')}`;
                    return (
                      <div key={segment.id} className={`grid gap-1 rounded-lg px-2 py-2 transition-colors sm:grid-cols-[120px_minmax(0,1fr)] ${active ? 'bg-primary/5 ring-1 ring-primary/15' : ''}`}>
                        <div className="text-xs font-medium">
                          {segment.speaker_name || 'Speaker'}
                          {meeting.recording_object_key ? (
                            <button
                              type="button"
                              className="ml-2 rounded px-1 text-[10px] font-normal text-primary hover:bg-primary/10 hover:underline"
                              title="Play from this moment"
                              onClick={() => recordingPlayerRef.current?.seekTo(segment.start_seconds)}
                            >
                              {timestamp}
                            </button>
                          ) : <span className="ml-2 text-[10px] font-normal text-muted-foreground">{timestamp}</span>}
                        </div>
                        <p className="text-sm leading-6 text-muted-foreground">{segment.text}</p>
                      </div>
                    );
                  })}
                </div>
              ) : transcriptText ? <p className="whitespace-pre-wrap text-sm leading-7 text-muted-foreground">{transcriptText}</p> : <p className="py-8 text-center text-sm text-muted-foreground">Transcript is not available yet.</p>}
            </CardContent>
          </Card>
        </main>

        <aside className="min-w-0 space-y-5">
          <Card>
            <CardHeader><CardTitle className="text-sm">Capture details</CardTitle></CardHeader>
            <CardContent className="space-y-3 text-xs">
              <div className="flex justify-between gap-3"><span className="text-muted-foreground">Visibility</span><span className="font-medium">{titleCase(meeting.visibility)}</span></div>
              <div className="flex justify-between gap-3"><span className="text-muted-foreground">Recording</span><span className="font-medium">{meeting.recording_object_key ? (meeting.recording_content_type?.startsWith('video/') ? 'Video' : 'Audio') : meeting.record_audio ? 'Requested' : 'Transcript only'}</span></div>
              <div className="flex justify-between gap-3"><span className="text-muted-foreground">Participants</span><span className="font-medium">{meeting.participants?.length ?? 0}</span></div>
              <div className="flex items-center justify-between gap-3"><span className="text-muted-foreground">Platform</span><span className="inline-flex items-center gap-1.5 font-medium"><MeetingPlatformIcon platform={meeting.platform} size="sm" className="border-0 shadow-none" />{getMeetingPlatformLabel(meeting.platform)}</span></div>
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
