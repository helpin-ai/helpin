import { meetingProcessingRecovery } from '@edition/config';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { toast } from 'sonner';

import {
  ArrowLeft02Icon,
  ArrowReloadHorizontalIcon,
  Copy01Icon,
  LinkSquare01Icon,
  PlayCircleIcon,
  StopIcon,
} from '@/lib/icons';
import { AssociationsList } from '@/components/crm/AssociationsList';
import { MeetingPlatformIcon } from '@/components/crm/MeetingPlatform';
import { MeetingRecordingPlayer, type MeetingRecordingPlayerHandle } from '@/components/crm/MeetingRecordingPlayer';
import {
  formatMeetingActionDueDate,
  formatMeetingDate,
  getMeetingPlatformLabel,
} from '@/lib/meetingPresentation';
import { MeetingProcessingState } from '@/components/crm/MeetingProcessingState';
import { MeetingStatusText } from '@/components/crm/MeetingStatusText';
import { ServerSetupNotice } from '@/components/crm/ServerSetupNotice';
import { UpgradeRequiredDialog } from '@edition';
import { MarkdownContent } from '@/components/pm/CodingSession/MarkdownContent';
import {
  QuietDetailLayout,
  QuietDetailAction,
  QuietDetailHeader,
  QuietBreadcrumbs,
  QuietEmptyState,
  QuietMetaLine,
  QuietPropertyRow,
  QuietSection,
  QuietStatusText,
  QuietTextAction,
  QuietTitleInput,
  quietUnderlineControlClassName,
} from '@/components/design-system/quiet';
import { SaveIndicator } from '@/components/pm/SaveIndicator';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Skeleton } from '@/components/ui/skeleton';
import {
  useAcceptMeetingAction,
  useCRMMeeting,
  useCRMMeetingSettings,
  useDismissMeetingAction,
  useRetryMeetingProcessing,
  useStartMeetingCapture,
  useStopMeetingCapture,
  useUpdateCRMMeeting,
} from '@/hooks/queries/useCRMMeetings';
import { useTeamWorkflow } from '@/hooks/queries/useWorkflows';
import { usePermissions, useWorkspaceAccess } from '@/hooks/queries/useSession';
import { useAccessibleTeams } from '@/hooks/useAccessibleTeams';
import { useTitle } from '@/hooks/useTitle';
import { getUpgradeRequiredReason, type UpgradeRequiredReason } from '@edition/errors';
import { resolveMeetingActionStateId } from '@/lib/meetingActionTaskTarget';
import { cn } from '@/lib/utils';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import type { CRMMeetingActionItem, CRMMeetingTranscriptSegment } from '@/lib/crmMeetingTypes';

type MeetingDetailTab = 'overview' | 'transcript';

const CAPTURE_UNAVAILABLE_ID = 'meeting-capture-unavailable';

const titleCase = (value: string) => value.replace(/_/g, ' ').replace(/\b\w/g, (letter) => letter.toUpperCase());

function IntelligenceList({ title, items }: { title: string; items?: string[] }) {
  if (!items?.length) return null;
  return (
    <section className="border-t border-quiet-divider-light py-5 first:border-t-0 first:pt-0">
      <h3 className="text-[14px] font-semibold tracking-[-0.008em] text-quiet-text-primary">{title}</h3>
      <ul className="mt-2 max-w-[760px] space-y-2 text-sm leading-[1.7] text-quiet-text-secondary">
        {items.map((item, index) => (
          <li key={`${title}-${index}`} className="flex gap-2.5">
            <span aria-hidden="true" className="mt-[9px] size-1 shrink-0 rounded-full bg-quiet-empty-glyph" />
            <span>{item}</span>
          </li>
        ))}
      </ul>
    </section>
  );
}

function ActionItemRow({
  workspaceId,
  item,
  teamId,
  stateId,
  teams,
  busy,
  canCreateTask,
  canDismiss,
  configuring,
  onTeamChange,
  onStateChange,
  onConfigure,
  onCancelConfigure,
  onAccept,
  onDismiss,
}: {
  workspaceId: string;
  item: CRMMeetingActionItem;
  teamId: string;
  stateId?: string;
  teams: Array<{ id: string; name: string }>;
  busy: boolean;
  canCreateTask: boolean;
  canDismiss: boolean;
  configuring: boolean;
  onTeamChange: (value: string) => void;
  onStateChange: (value: string) => void;
  onConfigure: () => void;
  onCancelConfigure: () => void;
  onAccept: (workflowId: string, workflowStateId: string) => void;
  onDismiss: () => void;
}) {
  const { data: workflow, isLoading: workflowLoading, isError: workflowError } = useTeamWorkflow(
    workspaceId,
    teamId,
    item.status === 'pending' && canCreateTask,
  );
  const workflowStates = useMemo(
    () => [...(workflow?.states ?? [])].sort((left, right) => left.position - right.position),
    [workflow?.states],
  );
  const selectedStateId = resolveMeetingActionStateId(workflow, stateId);
  const dueDateLabel = formatMeetingActionDueDate(item.due_date);
  const selectedTeamName = teams.find((team) => team.id === teamId)?.name;
  const selectedStateName = workflowStates.find((state) => state.id === selectedStateId)?.name;
  const destinationLabel = [selectedTeamName, selectedStateName].filter(Boolean).join(' · ');
  const canSubmit = Boolean(teamId && workflow && selectedStateId && !workflowLoading && !busy);

  return (
    <article className="group/action flex h-full flex-col rounded-lg border border-border/70 bg-card px-4 py-3.5 transition-all hover:border-border hover:shadow-sm">
      <div className="flex items-start justify-between gap-4">
        <div className="min-w-0">
          <p className="text-[13.5px] font-semibold leading-5 tracking-[-0.008em] text-quiet-text-primary">{item.title}</p>
          {item.details ? <p className="mt-1 max-w-[680px] text-[12.5px] leading-5 text-quiet-text-tertiary">{item.details}</p> : null}
          <QuietMetaLine
            className="mt-2"
            items={[
              item.assignee_name ? `Owner: ${item.assignee_name}` : null,
              dueDateLabel ? `Due ${dueDateLabel}` : null,
            ]}
          />
        </div>
        {item.status !== 'pending' ? (
          <QuietStatusText tone={item.status === 'accepted' ? 'positive' : 'neutral'}>
            {titleCase(item.status)}
          </QuietStatusText>
        ) : null}
      </div>

      {item.evidence?.excerpt ? (
        <details className="group/evidence mt-2.5 max-w-[680px]">
          <summary className="w-fit cursor-pointer list-none border-b border-quiet-field pb-0.5 text-[11.5px] text-quiet-text-tertiary hover:border-quiet-text-primary hover:text-quiet-text-primary focus-visible:border-b-2 focus-visible:border-quiet-text-primary focus-visible:outline-none">
            View transcript evidence
          </summary>
          <blockquote className="mt-2 border-l border-quiet-divider-strong pl-3 text-[12.5px] italic leading-5 text-quiet-text-tertiary">
            “{item.evidence.excerpt}”
          </blockquote>
        </details>
      ) : null}

      {item.status === 'pending' && (canCreateTask || canDismiss) ? (
        <div className="mt-3 border-t border-quiet-divider-light pt-3">
          {canCreateTask && configuring ? (
            <div className="motion-safe:animate-in motion-safe:fade-in motion-safe:duration-200">
              <div className="mb-3">
                <p className="text-[12px] font-semibold uppercase tracking-[0.06em] text-quiet-muted">Task destination</p>
                <p className="mt-1 text-[12.5px] leading-5 text-quiet-text-tertiary">Choose where this follow-up should enter project work.</p>
              </div>
              <div className="grid gap-4 sm:grid-cols-2">
                <div>
                  <label htmlFor={`meeting-action-${item.id}-team`} className="text-[12px] font-semibold uppercase tracking-[0.06em] text-quiet-muted">Team</label>
                  <Select value={teamId} onValueChange={onTeamChange}>
                    <SelectTrigger id={`meeting-action-${item.id}-team`} size="sm" className={cn(quietUnderlineControlClassName, 'mt-1.5 w-full justify-between')}>
                      <SelectValue placeholder="Select team" />
                    </SelectTrigger>
                    <SelectContent>{teams.map((team) => <SelectItem key={team.id} value={team.id}>{team.name}</SelectItem>)}</SelectContent>
                  </Select>
                </div>
                <div>
                  <label htmlFor={`meeting-action-${item.id}-state`} className="text-[12px] font-semibold uppercase tracking-[0.06em] text-quiet-muted">State</label>
                  <Select value={selectedStateId || undefined} onValueChange={onStateChange} disabled={!teamId || workflowLoading || workflowStates.length === 0}>
                    <SelectTrigger id={`meeting-action-${item.id}-state`} size="sm" className={cn(quietUnderlineControlClassName, 'mt-1.5 w-full justify-between')}>
                      <SelectValue placeholder={workflowLoading ? 'Loading states…' : 'Select state'} />
                    </SelectTrigger>
                    <SelectContent>{workflowStates.map((state) => <SelectItem key={state.id} value={state.id}>{state.name}</SelectItem>)}</SelectContent>
                  </Select>
                </div>
              </div>
              {workflowError ? <p className="mt-2 text-[12.5px] text-quiet-accent">Workflow states could not be loaded for this team. Try another team.</p> : null}
              <div className="mt-4 flex flex-wrap justify-end gap-4">
                <QuietTextAction onClick={onCancelConfigure} disabled={busy}>Cancel</QuietTextAction>
                <QuietTextAction
                  className="border-b border-quiet-field pb-0.5 font-medium"
                  onClick={() => workflow && onAccept(workflow.workflow.id, selectedStateId)}
                  disabled={!canSubmit}
                >
                  Create task
                </QuietTextAction>
              </div>
            </div>
          ) : (
            <div className="flex flex-wrap items-center justify-between gap-x-5 gap-y-3">
              {canCreateTask ? (
                <p className="text-[11.5px] text-quiet-muted">
                  {workflowLoading ? 'Preparing task destination…' : destinationLabel ? `Creates in ${destinationLabel}` : 'Choose a task destination'}
                </p>
              ) : <span />}
              <div className="ml-auto flex flex-wrap items-center gap-4">
                {canDismiss ? <QuietTextAction onClick={onDismiss} disabled={busy}>Dismiss</QuietTextAction> : null}
                {canCreateTask && destinationLabel ? <QuietTextAction onClick={onConfigure} disabled={busy}>Change destination</QuietTextAction> : null}
                {canCreateTask ? (
                  <QuietTextAction
                    className="border-b border-quiet-field pb-0.5 font-medium"
                    onClick={() => workflow && onAccept(workflow.workflow.id, selectedStateId)}
                    disabled={!canSubmit}
                  >
                    Create task
                  </QuietTextAction>
                ) : null}
              </div>
            </div>
          )}
        </div>
      ) : null}
    </article>
  );
}

function TranscriptRow({
  segment,
  active,
  playable,
  onSeek,
}: {
  segment: CRMMeetingTranscriptSegment;
  active: boolean;
  playable: boolean;
  onSeek: () => void;
}) {
  const timestamp = `${Math.floor(segment.start_seconds / 60)}:${String(Math.floor(segment.start_seconds % 60)).padStart(2, '0')}`;
  return (
    <div className={cn(
      'grid gap-1 rounded-lg px-2 py-2 transition-colors sm:grid-cols-[120px_minmax(0,1fr)]',
      active && 'bg-primary/5 ring-1 ring-primary/15',
    )}>
      <div className="text-xs font-medium">
        {segment.speaker_name || 'Speaker'}
        {playable ? (
          <button
            type="button"
            className="ml-2 rounded px-1 text-[10px] font-normal text-primary hover:bg-primary/10 hover:underline"
            title="Play from this moment"
            onClick={onSeek}
          >
            {timestamp}
          </button>
        ) : <span className="ml-2 text-[10px] font-normal text-muted-foreground">{timestamp}</span>}
      </div>
      <p className="text-sm leading-6 text-muted-foreground">{segment.text}</p>
    </div>
  );
}

function MeetingDetailState({
  title,
  description,
  onBack,
  onRetry,
}: {
  title: string;
  description: string;
  onBack: () => void;
  onRetry?: () => void;
}) {
  return (
    <div className="flex h-full flex-col overflow-hidden">
      <QuietDetailHeader
        breadcrumbs={<QuietBreadcrumbs items={[{ id: 'meetings', label: 'Meetings', onClick: onBack }]} onBack={onBack} backLabel="Back to meetings" />}
        title="Meeting"
      />
      <div className="flex-1 overflow-auto p-4 md:p-6">
        <div className="mx-auto max-w-4xl">
          <QuietEmptyState
            title={title}
            description={description}
            action={(
              <div className="flex flex-wrap gap-4">
                {onRetry ? <QuietTextAction className="border-b border-quiet-field pb-0.5" onClick={onRetry}>Try again</QuietTextAction> : null}
                <QuietTextAction onClick={onBack}><ArrowLeft02Icon className="h-3.5 w-3.5" />Back to meetings</QuietTextAction>
              </div>
            )}
          />
        </div>
      </div>
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
  const meetingQuery = useCRMMeeting(workspaceId, meetingId);
  const { data } = meetingQuery;
  const settingsQuery = useCRMMeetingSettings(workspaceId);
  const captureUnavailable = settingsQuery.data?.settings.capture_configured === false;
  const startCapture = useStartMeetingCapture(workspaceId, meetingId);
  const stopCapture = useStopMeetingCapture(workspaceId, meetingId);
  const retryProcessing = useRetryMeetingProcessing(workspaceId, meetingId);
  const updateMeeting = useUpdateCRMMeeting(workspaceId, meetingId);
  const acceptAction = useAcceptMeetingAction(workspaceId, meetingId);
  const dismissAction = useDismissMeetingAction(workspaceId, meetingId);
  const { teams } = useAccessibleTeams(workspaceId);
  const [stateByAction, setStateByAction] = useState<Record<string, string>>({});
  const [teamByAction, setTeamByAction] = useState<Record<string, string>>({});
  const [configuringActionId, setConfiguringActionId] = useState<string | null>(null);
  const [upgradeReason, setUpgradeReason] = useState<UpgradeRequiredReason | null>(null);
  const [activeTab, setActiveTab] = useState<MeetingDetailTab>('overview');
  const recordingPlayerRef = useRef<MeetingRecordingPlayerHandle | null>(null);
  const [playbackSeconds, setPlaybackSeconds] = useState(0);
  const [titleDraftState, setTitleDraftState] = useState<{ meetingId: string; value: string } | null>(null);
  const [titleSaveError, setTitleSaveError] = useState<string | null>(null);
  const [editedMeetingId, setEditedMeetingId] = useState<string | null>(null);
  const titleSaveTimerRef = useRef<ReturnType<typeof setTimeout>>(undefined);
  const titleDraft = titleDraftState?.meetingId === meetingId ? titleDraftState.value : data?.meeting.title ?? '';
  useTitle(titleDraft.trim() || 'Meeting');

  const backToMeetings = () => navigate({ to: '/w/$slug/crm/meetings', params: { slug } });
  const defaultTeamId = teams[0]?.id ?? '';
  const transcriptText = data?.transcript?.plain_text ?? '';
  const occurredAt = data?.meeting.actual_start_at ?? data?.meeting.scheduled_start_at ?? data?.meeting.created_at;
  const duration = useMemo(() => {
    const seconds = data?.meeting.duration_seconds ?? 0;
    if (!seconds) return null;
    const minutes = Math.floor(seconds / 60);
    return `${minutes}m ${seconds % 60}s`;
  }, [data?.meeting.duration_seconds]);

  const persistMeetingTitle = useCallback(async (value: string) => {
    const nextTitle = value.trim();
    if (!nextTitle || nextTitle === data?.meeting.title) return;
    try {
      await updateMeeting.mutateAsync({ title: nextTitle });
      setTitleDraftState({ meetingId, value: nextTitle });
      setTitleSaveError(null);
    } catch (error) {
      setTitleSaveError(error instanceof Error ? error.message : 'Failed to save title');
    }
  }, [data?.meeting.title, meetingId, updateMeeting]);

  const scheduleMeetingTitleSave = (value: string) => {
    setTitleDraftState({ meetingId, value });
    setEditedMeetingId(meetingId);
    setTitleSaveError(null);
    if (titleSaveTimerRef.current) clearTimeout(titleSaveTimerRef.current);
    titleSaveTimerRef.current = setTimeout(() => void persistMeetingTitle(value), 800);
  };

  const flushMeetingTitleSave = () => {
    if (titleSaveTimerRef.current) clearTimeout(titleSaveTimerRef.current);
    const nextTitle = titleDraft.trim();
    if (!nextTitle) {
      setTitleDraftState({ meetingId, value: data?.meeting.title ?? '' });
      setTitleSaveError('Meeting title cannot be empty');
      return;
    }
    void persistMeetingTitle(nextTitle);
  };

  useEffect(() => () => {
    if (titleSaveTimerRef.current) clearTimeout(titleSaveTimerRef.current);
  }, [meetingId]);

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

  const updateActionTeam = (itemId: string, teamId: string) => {
    setTeamByAction((current) => ({ ...current, [itemId]: teamId }));
    setStateByAction((current) => {
      const next = { ...current };
      delete next[itemId];
      return next;
    });
  };

  const accept = async (item: CRMMeetingActionItem, workflowId: string, workflowStateId: string) => {
    const teamId = teamByAction[item.id] ?? defaultTeamId;
    try {
      await acceptAction.mutateAsync({ itemId: item.id, payload: { team_id: teamId, workflow_id: workflowId, workflow_state_id: workflowStateId } });
      setConfiguringActionId(null);
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

  if (meetingQuery.isLoading) {
    return (
      <div className="flex h-full flex-col overflow-hidden">
        <QuietDetailHeader
          breadcrumbs={<QuietBreadcrumbs items={[{ id: 'meetings', label: 'Meetings', onClick: backToMeetings }]} onBack={backToMeetings} backLabel="Back to meetings" />}
          avatar={<Skeleton className="h-10 w-10 rounded-[10px]" />}
          title={<Skeleton className="h-8 w-64 max-w-full rounded-none" />}
          meta={<Skeleton className="h-3 w-48 rounded-none" />}
        />
        <div className="flex-1 overflow-auto p-4 md:p-6">
          <div className="mx-auto max-w-6xl">
            <div className="mt-6 border-t border-quiet-divider-strong py-6">
              <Skeleton className="h-4 w-32 rounded-none" />
              <Skeleton className="mt-3 h-48 w-full rounded-none" />
            </div>
          </div>
        </div>
      </div>
    );
  }
  if (meetingQuery.isError) {
    return <MeetingDetailState title="Meeting could not be loaded" description="This meeting is temporarily unavailable. Its capture and notes have not been changed." onBack={backToMeetings} onRetry={() => void meetingQuery.refetch()} />;
  }
  if (!data) {
    return <MeetingDetailState title="Meeting not found" description="This meeting may have been deleted or you may no longer have access to it." onBack={backToMeetings} />;
  }

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

  const captureAction = canEditCRM && canStart ? (
    <QuietDetailAction
      tone="primary"
      icon={<PlayCircleIcon className="h-4 w-4" />}
      label="Start capture"
      onClick={() => runCommand(() => startCapture.mutateAsync(), 'Helpin is joining the meeting')}
      disabled={startCapture.isPending || captureUnavailable}
      aria-describedby={captureUnavailable ? CAPTURE_UNAVAILABLE_ID : undefined}
    />
  ) : canEditCRM && canStop ? (
    <QuietDetailAction
      tone="primary"
      icon={<StopIcon className="h-4 w-4" />}
      label="Stop capture"
      onClick={() => runCommand(() => stopCapture.mutateAsync(), 'Capture is finalizing')}
      disabled={stopCapture.isPending}
    />
  ) : canEditCRM && canRetry ? (
    <QuietDetailAction
      tone="primary"
      icon={<ArrowReloadHorizontalIcon className="h-4 w-4" />}
      label="Retry processing"
      onClick={() => runCommand(() => retryProcessing.mutateAsync(), 'Meeting processing restarted')}
      disabled={retryProcessing.isPending}
    />
  ) : null;
  const recordingPlayer = meeting.recording_object_key ? (
    <MeetingRecordingPlayer
      ref={recordingPlayerRef}
      workspaceId={workspaceId}
      meetingId={meetingId}
      onTimeUpdate={setPlaybackSeconds}
    />
  ) : null;

  return (
    <div className="flex h-full flex-col overflow-hidden">
      <QuietDetailHeader
        breadcrumbs={<QuietBreadcrumbs items={[{ id: 'meetings', label: 'Meetings', onClick: backToMeetings }]} onBack={backToMeetings} backLabel="Back to meetings" />}
        avatar={<MeetingPlatformIcon platform={meeting.platform} presentation="quiet" className="h-10 w-10" />}
        title={canEditCRM ? (
          <QuietTitleInput
            aria-label="Meeting name"
            presentation="header"
            className="max-w-[42rem] border-b-transparent pb-0.5 hover:border-quiet-field focus-visible:border-quiet-text-primary"
            value={titleDraft}
            onChange={(event) => scheduleMeetingTitleSave(event.target.value)}
            onBlur={flushMeetingTitleSave}
            onKeyDown={(event) => {
              if (event.key === 'Enter') {
                event.preventDefault();
                event.currentTarget.blur();
              } else if (event.key === 'Escape') {
                event.preventDefault();
                if (titleSaveTimerRef.current) clearTimeout(titleSaveTimerRef.current);
                setTitleDraftState({ meetingId, value: meeting.title });
                setTitleSaveError(null);
                event.currentTarget.blur();
              }
            }}
          />
        ) : meeting.title}
        meta={(
          <QuietMetaLine items={[
            getMeetingPlatformLabel(meeting.platform),
            formatMeetingDate(occurredAt ?? meeting.created_at, 'MMM d, yyyy · p'),
            duration,
            `${meeting.participants?.length ?? 0} participant${meeting.participants?.length === 1 ? '' : 's'}`,
          ]} />
        )}
        status={<MeetingStatusText status={meeting.status} presentation="badge" />}
        state={editedMeetingId === meetingId ? <SaveIndicator saving={updateMeeting.isPending} error={titleSaveError} presentation="quiet" /> : null}
        actions={(
          <>
            <QuietDetailAction
              href={meeting.meeting_url}
              target="_blank"
              rel="noreferrer"
              icon={<LinkSquare01Icon className="h-3.5 w-3.5" />}
              label={`Open ${getMeetingPlatformLabel(meeting.platform)}`}
            />
            {meeting.recording_object_key ? (
              <QuietDetailAction
                icon={<PlayCircleIcon className="h-3.5 w-3.5" />}
                label="Recording"
                onClick={() => recordingPlayerRef.current?.focus()}
              />
            ) : null}
            {captureAction}
          </>
        )}
      />

      <QuietDetailLayout
        className="min-h-0 flex-1 overflow-y-auto lg:overflow-hidden"
        main={(
          <div className="flex min-h-0 flex-col lg:h-full">
            <div className="shrink-0 overflow-x-auto border-b border-quiet-divider-strong px-4 sm:px-6 lg:px-8">
              <Tabs value={activeTab} onValueChange={(value) => setActiveTab(value as MeetingDetailTab)} className="min-w-max gap-0">
                <TabsList variant="quiet" aria-label="Meeting detail views" className="border-b-0">
                  <TabsTrigger value="overview">Overview</TabsTrigger>
                  <TabsTrigger value="transcript">
                    Transcript
                    {data.transcript?.segments?.length ? <span className="text-[12px] font-normal tabular-nums text-quiet-muted">{data.transcript.segments.length}</span> : null}
                  </TabsTrigger>
                </TabsList>
              </Tabs>
            </div>

            <main className="min-h-0 flex-1 lg:overflow-y-auto">
              {captureUnavailable && canStart ? (
                <QuietSection title="Meeting capture">
                  <ServerSetupNotice
                    id={CAPTURE_UNAVAILABLE_ID}
                    title="Meeting capture isn’t set up on this server."
                    slug={slug}
                    isServerAdmin={permissions.isServerAdmin}
                  />
                </QuietSection>
              ) : null}
              {meeting.failure_message ? (
                <QuietSection title="Capture needs attention">
                  <QuietStatusText tone="blocker" className="text-quiet-accent">Capture failed</QuietStatusText>
                  <p className="mt-1 max-w-[680px] text-sm leading-[1.6] text-quiet-text-tertiary">{meeting.failure_message}</p>
                </QuietSection>
              ) : null}
              {meeting.summary_status === 'blocked_usage' ? (
                <QuietSection title="Processing paused">
                  <QuietStatusText tone="blocker" className="text-quiet-accent">AI capacity required</QuietStatusText>
                  <p className="mt-1 max-w-[680px] text-sm leading-[1.6] text-quiet-text-tertiary">
                    {meetingProcessingRecovery}
                  </p>
                </QuietSection>
              ) : null}
              {activeTab === 'overview' ? (
                <QuietSection>
                  {data.intelligence ? (
                    <div className="motion-safe:animate-in motion-safe:fade-in motion-safe:duration-300">
                      <div className="mb-4 flex flex-wrap items-center gap-3 border-b border-quiet-divider-light pb-3">
                        <QuietStatusText tone="positive" className="text-quiet-positive">Based on transcript</QuietStatusText>
                        <QuietTextAction className="border-b border-quiet-field pb-0.5" onClick={() => setActiveTab('transcript')}>View transcript</QuietTextAction>
                        <QuietTextAction className="ml-auto" onClick={() => copyText(data.intelligence?.summary_markdown ?? '', 'Overview')}>
                          <Copy01Icon className="h-3.5 w-3.5" />Copy overview
                        </QuietTextAction>
                      </div>
                      <section className="pb-5">
                        <MarkdownContent content={data.intelligence.summary_markdown} className="max-w-[760px] text-sm leading-[1.7] text-quiet-text-secondary [text-wrap:pretty]" />
                      </section>
                      {recordingPlayer || data.action_items.length > 0 ? (
                        <div className="border-t border-quiet-divider-light">
                          {recordingPlayer}
                          {data.action_items.length > 0 ? (
                            <section className="border-b border-quiet-divider-strong py-5">
                              <div className="flex items-baseline gap-2">
                                <h3 className="text-[20px] font-semibold leading-tight tracking-[-0.018em] text-quiet-text-primary">Action items</h3>
                                <span className="text-[11.5px] tabular-nums text-quiet-muted">{data.action_items.length}</span>
                              </div>
                              <p className="mt-1.5 max-w-[680px] text-[12.5px] leading-5 text-quiet-text-tertiary">
                                Follow-up work captured from the transcript. Create a task directly, or change its destination first.
                              </p>
                              <div className="mt-4 grid gap-3 md:grid-cols-2">
                                {data.action_items.map((item) => (
                                  <ActionItemRow
                                    key={item.id}
                                    workspaceId={workspaceId}
                                    item={item}
                                    teams={teams}
                                    teamId={teamByAction[item.id] ?? defaultTeamId}
                                    stateId={stateByAction[item.id]}
                                    busy={acceptAction.isPending || dismissAction.isPending}
                                    canCreateTask={canEditCRM && canEditPM}
                                    canDismiss={canEditCRM}
                                    configuring={configuringActionId === item.id}
                                    onTeamChange={(value) => updateActionTeam(item.id, value)}
                                    onStateChange={(value) => setStateByAction((current) => ({ ...current, [item.id]: value }))}
                                    onConfigure={() => setConfiguringActionId(item.id)}
                                    onCancelConfigure={() => setConfiguringActionId(null)}
                                    onAccept={(workflowId, workflowStateId) => accept(item, workflowId, workflowStateId)}
                                    onDismiss={() => dismiss(item)}
                                  />
                                ))}
                              </div>
                            </section>
                          ) : null}
                        </div>
                      ) : null}
                      <IntelligenceList title="Decisions made" items={data.intelligence.decisions} />
                      <IntelligenceList title="Key discussion points" items={data.intelligence.key_points} />
                      <IntelligenceList title="Next steps" items={data.intelligence.next_steps} />
                      <IntelligenceList title="Open questions and issues" items={openQuestions} />
                      <IntelligenceList title="Participants and context" items={participantContext} />
                      <IntelligenceList title="Rapport" items={data.intelligence.rapport} />
                    </div>
                  ) : ['joining', 'waiting', 'recording', 'finalizing', 'processing'].includes(meeting.status) || meeting.summary_status === 'processing' ? (
                    <>
                      {recordingPlayer}
                      <MeetingProcessingState status={meeting.status} summaryStatus={meeting.summary_status} />
                    </>
                  ) : (
                    <>
                      {recordingPlayer}
                      <QuietEmptyState
                        className="border-b-0"
                        title="Meeting notes are not ready yet"
                        description="Once a transcript is processed, Helpin will summarize the discussion, decisions, and follow-up work here."
                      />
                    </>
                  )}
                </QuietSection>
              ) : (
                <>
                  {recordingPlayer ? <div className="px-4 sm:px-6 lg:px-8">{recordingPlayer}</div> : null}
                  <QuietSection
                    title="Transcript"
                    count={data.transcript?.segments?.length || undefined}
                    action={transcriptText ? (
                      <QuietTextAction onClick={() => copyText(transcriptText, 'Transcript')}>
                        <Copy01Icon className="h-3.5 w-3.5" />Copy transcript
                      </QuietTextAction>
                    ) : undefined}
                  >
                    <p className="mb-4 text-sm leading-[1.6] text-quiet-text-tertiary">
                      {data.transcript?.language ? `Speaker-attributed transcript · ${data.transcript.language}` : 'Speaker-attributed meeting transcript'}
                    </p>
                    {data.transcript?.segments?.length ? (
                      <div className="space-y-2">
                        {data.transcript.segments.map((segment) => {
                          const segmentEnd = Math.max(segment.end_seconds, segment.start_seconds + 0.5);
                          const active = Boolean(meeting.recording_object_key) && playbackSeconds >= segment.start_seconds && playbackSeconds < segmentEnd;
                          return (
                            <TranscriptRow
                              key={segment.id}
                              segment={segment}
                              active={active}
                              playable={Boolean(meeting.recording_object_key)}
                              onSeek={() => recordingPlayerRef.current?.seekTo(segment.start_seconds)}
                            />
                          );
                        })}
                      </div>
                    ) : transcriptText ? (
                      <p className="max-w-[760px] whitespace-pre-wrap border-t border-quiet-divider-light pt-4 text-sm leading-[1.7] text-quiet-text-secondary">{transcriptText}</p>
                    ) : (
                      <QuietEmptyState className="border-b-0" title="Transcript is not available yet" description="The speaker-attributed transcript will appear here after meeting capture finishes." />
                    )}
                  </QuietSection>
                </>
              )}
            </main>
          </div>
        )}
        rail={(
          <div>
            <QuietSection title="Capture details" className="px-4 sm:px-5 lg:px-5" bodyClassName="-mx-4 sm:-mx-5 lg:-mx-5">
              <QuietPropertyRow label="Visibility" value={titleCase(meeting.visibility)} />
              <QuietPropertyRow label="Recording" value={meeting.recording_object_key ? (meeting.recording_content_type?.startsWith('video/') ? 'Video' : 'Audio') : meeting.record_audio ? 'Requested' : 'Transcript only'} />
              <QuietPropertyRow label="Participants" value={meeting.participants?.length ?? 0} />
              <QuietPropertyRow label="Platform" value={<span className="inline-flex items-center gap-1.5"><MeetingPlatformIcon platform={meeting.platform} size="sm" presentation="quiet" />{getMeetingPlatformLabel(meeting.platform)}</span>} />
            </QuietSection>
            <QuietSection title="Associations" className="px-4 sm:px-5 lg:px-5">
              <p className="text-[12.5px] leading-5 text-quiet-text-tertiary">Connect this meeting to CRM records and project work.</p>
              <AssociationsList workspaceId={workspaceId} slug={slug} associations={data.associations} currentObjectType="meeting" currentObjectId={meetingId} onAssociationRemoved={() => void meetingQuery.refetch()} editable={canEditCRM} />
            </QuietSection>
          </div>
        )}
      />

      <UpgradeRequiredDialog open={upgradeReason !== null} onOpenChange={(open) => { if (!open) setUpgradeReason(null); }} reason={upgradeReason} />
    </div>
  );
}
