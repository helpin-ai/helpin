import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { toast } from 'sonner';
import { CheckmarkCircle02Icon, Clock01Icon, SecurityCheckIcon, CancelCircleIcon } from '@/lib/icons';
import { UnicodeSpinner } from '@/components/pm/CodingSession/UnicodeSpinner';

import { CodingPlanPanel } from '@/components/pm/CodingSession/CodingPlanPanel';
import { CodingPreviewPanels } from '@/components/pm/CodingSession/CodingPreviewPanels';
import { CodingReviewHistoryPanel } from '@/components/pm/CodingSession/CodingReviewHistoryPanel';
import { CodingSessionHeader } from '@/components/pm/CodingSession/CodingSessionHeader';
import { CodingTranscriptPane } from '@/components/pm/CodingSession/CodingTranscriptPane';
import { NextAgentHint } from '@/components/agents/NextAgentHint';
import { UpgradeRequiredDialog } from '@/components/billing/UpgradeRequiredDialog';
import { resolveAgentPersonaKey, type AgentPersonaKey } from '@/components/agents/AgentAvatar';
import { collectCodingSessionPreviews } from '@/components/pm/CodingSession/codingSessionPreviews';
import {
  shouldShowFailedCodingSessionRecoveryNotice,
  shouldShowCodingSessionPlanPanel,
  shouldShowCodingSessionSidePanel,
} from '@/components/pm/CodingSession/codingSessionLayout';
import {
  buildCodingSessionStreamState,
  mergeCodingSessionStreamSnapshotSeed,
} from '@/components/pm/CodingSession/codingSessionStream';
import { resolveCodingSessionComposerState } from '@/components/pm/CodingSession/codingSessionComposer';
import { normalizeCodingSessionPreviewPanelKey } from '@/components/pm/CodingSession/previewPanelKeys';
import {
  codingSessionApprovalStatesByPreviewKey,
  isPersistedCodingSessionEvent,
  latestPendingCodingSessionInteraction,
  maxPersistedCodingSessionSequence,
  upsertCodingSessionEvents,
} from '@/components/pm/CodingSession/codingSessionUtils';
import { useWorkspaceMembers } from '@/hooks/queries';
import type { Agent, AgentRun, AgentRunArtifact, CodingSession, CodingSessionEvent, CodingSessionStreamSnapshot } from '@/lib/pmTypes';
import { agentService } from '@/lib/services/agentService';
import { codingSessionService } from '@/lib/services/codingSessionService';
import { cn } from '@/lib/utils';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { Button } from '@/components/ui/button';
import { getUpgradeRequiredReason, type UpgradeRequiredReason } from '@/lib/upgradeRequired';

const STATUS_ICON = {
  queued: <Clock01Icon className="h-3.5 w-3.5" />,
  running: <UnicodeSpinner name="braille" className="agent-working-chroma text-sm" />,
  paused: <SecurityCheckIcon className="h-3.5 w-3.5" />,
  completed: <CheckmarkCircle02Icon className="h-3.5 w-3.5" />,
  failed: <CancelCircleIcon className="h-3.5 w-3.5" />,
  cancelled: <CancelCircleIcon className="h-3.5 w-3.5" />,
} as const;

export function CodingSessionSurface({
  sessionId,
  embedded = false,
  showBackToRuns = true,
}: {
  sessionId: string;
  embedded?: boolean;
  showBackToRuns?: boolean;
}) {
  const workspace = useWorkspaceStore((state) => state.currentWorkspace);
  const workspaceId = workspace?.id ?? '';
  const workspaceSlug = workspace?.slug ?? '';
  const [activeSessionId, setActiveSessionId] = useState(sessionId);

  const [session, setSession] = useState<CodingSession | null>(null);
  const [events, setEvents] = useState<CodingSessionEvent[]>([]);
  const [artifacts, setArtifacts] = useState<AgentRunArtifact[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [acting, setActing] = useState<string | null>(null);
  const [sendingMessage, setSendingMessage] = useState(false);
  const [handoffAgents, setHandoffAgents] = useState<Agent[] | null>(null);
  const [handoffRuns, setHandoffRuns] = useState<AgentRun[] | null>(null);
  const [handoffRunsTargetId, setHandoffRunsTargetId] = useState<string | null>(null);
  const [upgradeDialogReason, setUpgradeDialogReason] = useState<UpgradeRequiredReason | null>(null);
  const sequenceRef = useRef(0);
  const [streamSnapshotSeed, setStreamSnapshotSeed] = useState<CodingSessionStreamSnapshot | null>(null);

  useEffect(() => {
    setActiveSessionId(sessionId);
  }, [sessionId]);

  const loadSession = useCallback(async () => {
    if (!workspaceId || !activeSessionId) return;
    const sessionRes = await codingSessionService.get(workspaceId, activeSessionId);
    if (sessionRes.error) throw new Error(sessionRes.error);
    const nextSession = sessionRes.data as CodingSession;
    setStreamSnapshotSeed((current) => mergeCodingSessionStreamSnapshotSeed(current, nextSession.stream_state_snapshot ?? null));
    setSession(nextSession);
  }, [workspaceId, activeSessionId]);

  const loadEvents = useCallback(async (after = 0) => {
    if (!workspaceId || !activeSessionId) return;
    const eventsRes = await codingSessionService.listEvents(workspaceId, activeSessionId, after);
    if (eventsRes.error) throw new Error(eventsRes.error);
    setStreamSnapshotSeed((current) => mergeCodingSessionStreamSnapshotSeed(
      current,
      eventsRes.data?.stream_state_snapshot ?? null,
    ));
    const nextEvents = eventsRes.data?.events ?? [];
    if (after > 0) {
      setEvents((current) => {
        const merged = upsertCodingSessionEvents(current, nextEvents);
        sequenceRef.current = maxPersistedCodingSessionSequence(merged);
        return merged;
      });
    } else {
      setEvents(nextEvents);
      sequenceRef.current = maxPersistedCodingSessionSequence(nextEvents);
    }
  }, [workspaceId, activeSessionId]);

  const loadArtifacts = useCallback(async () => {
    if (!workspaceId || !activeSessionId) return;
    const artifactsRes = await codingSessionService.listArtifacts(workspaceId, activeSessionId);
    if (artifactsRes.error) throw new Error(artifactsRes.error);
    setArtifacts(artifactsRes.data ?? []);
  }, [workspaceId, activeSessionId]);

  const load = useCallback(async () => {
    if (!workspaceId || !activeSessionId) return;
    setLoading(true);
    setError(null);
    try {
      await Promise.all([loadSession(), loadEvents(0), loadArtifacts()]);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load agent session');
    } finally {
      setLoading(false);
    }
  }, [workspaceId, activeSessionId, loadSession, loadEvents, loadArtifacts]);

  useEffect(() => {
    setStreamSnapshotSeed(null);
    sequenceRef.current = 0;
    setEvents([]);
    setArtifacts([]);
    setSession(null);
  }, [activeSessionId, workspaceId]);

  useEffect(() => {
    void load();
  }, [load]);

  const reconcileEvents = useCallback(async () => {
    if (!workspaceId || !activeSessionId) return;
    try {
      await Promise.all([loadEvents(sequenceRef.current), loadArtifacts()]);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to refresh session events');
    }
  }, [workspaceId, activeSessionId, loadEvents, loadArtifacts]);

  useEffect(() => {
    const onSessionUpdated = (raw: Event) => {
      const detail = (raw as CustomEvent).detail as { entity_id?: string; data?: Record<string, unknown> } | undefined;
      if (!detail || detail.entity_id !== activeSessionId) return;
      const nextStatus = typeof detail.data?.status === 'string' ? detail.data.status as CodingSession['status'] : undefined;
      const nextPauseReason = typeof detail.data?.pause_reason === 'string' ? detail.data.pause_reason as CodingSession['pause_reason'] : undefined;
      const shouldReloadSession = (
        nextStatus === 'paused'
        || nextStatus === 'completed'
        || nextStatus === 'failed'
        || nextStatus === 'cancelled'
        || (nextPauseReason && nextPauseReason !== 'none')
      );
      setSession((current) => current ? {
        ...current,
        parent_run_id: typeof detail.data?.parent_run_id === 'string'
          ? detail.data.parent_run_id || undefined
          : current.parent_run_id,
        status: nextStatus ?? current.status,
        pause_reason: nextPauseReason ?? current.pause_reason,
        execution_stage: typeof detail.data?.execution_stage === 'string'
          ? detail.data.execution_stage || undefined
          : current.execution_stage,
        last_heartbeat_at: typeof detail.data?.last_heartbeat_at === 'string'
          ? detail.data.last_heartbeat_at || undefined
          : current.last_heartbeat_at,
        error_message: typeof detail.data?.error_message === 'string'
          ? detail.data.error_message || undefined
          : current.error_message,
        updated_at: new Date().toISOString(),
      } : current);
      if (shouldReloadSession) {
        void loadSession();
        void loadArtifacts();
      }
    };
    const onSessionEvent = (raw: Event) => {
      const detail = (raw as CustomEvent).detail as { parent_id?: string; data?: CodingSessionEvent } | undefined;
      if (!detail || detail.parent_id !== activeSessionId || !detail.data) return;
      const event = detail.data;
      if (
        isPersistedCodingSessionEvent(event)
        && event.sequence_no > 0
        && sequenceRef.current > 0
        && event.sequence_no > sequenceRef.current + 1
      ) {
        void reconcileEvents();
      }
      setEvents((current) => {
        const merged = upsertCodingSessionEvents(current, [event]);
        sequenceRef.current = maxPersistedCodingSessionSequence(merged);
        return merged;
      });
      if (
        event.type === 'auth.updated'
        || event.type === 'approval.requested'
        || event.type === 'input.requested'
        || event.type === 'interaction.requested'
        || event.type === 'interaction.resolved'
        || event.type === 'interaction.cancelled'
      ) {
        void loadSession();
      }
    };
    window.addEventListener('coding_session-updated', onSessionUpdated);
    window.addEventListener('coding_session_event-created', onSessionEvent);
    return () => {
      window.removeEventListener('coding_session-updated', onSessionUpdated);
      window.removeEventListener('coding_session_event-created', onSessionEvent);
    };
  }, [activeSessionId, loadSession, reconcileEvents]);

  useEffect(() => {
    if (!session || session.status !== 'running') return;
    const timer = window.setInterval(() => {
      void reconcileEvents();
      void loadSession();
    }, 5000);
    return () => window.clearInterval(timer);
  }, [session, reconcileEvents, loadSession]);

  const streamState = useMemo(
    () => buildCodingSessionStreamState(events, streamSnapshotSeed),
    [events, streamSnapshotSeed],
  );
  const { data: workspaceMembers } = useWorkspaceMembers(session?.workspace_id ?? workspaceId);
  const activeInteraction = useMemo(
    () => (session && (session.status === 'running' || session.status === 'paused')
      ? latestPendingCodingSessionInteraction(events)
      : null),
    [events, session],
  );
  const approvalStatesByPreviewKey = useMemo(
    () => codingSessionApprovalStatesByPreviewKey(events),
    [events],
  );
  const resolverNamesByUserId = useMemo(() => {
    const map = new Map<string, string>();
    for (const member of workspaceMembers ?? []) {
      if (!member.user_id) continue;
      map.set(member.user_id, member.full_name || member.email || member.user_id);
    }
    return map;
  }, [workspaceMembers]);
  const previewsByKey = useMemo(
    () => collectCodingSessionPreviews(events, streamState.live_turn_segments),
    [events, streamState.live_turn_segments],
  );
  const approvalPreviewPanelKey = useMemo<string | null>(() => {
    if (!activeInteraction || activeInteraction.interaction_kind !== 'approval_request') return null;
    const previewPanelKey = normalizeCodingSessionPreviewPanelKey(
      typeof activeInteraction.request_payload?.preview_panel_key === 'string'
        ? activeInteraction.request_payload.preview_panel_key
        : '',
    );
    if (!previewPanelKey || !previewsByKey.has(previewPanelKey)) return null;
    return previewPanelKey;
  }, [activeInteraction, previewsByKey]);
  const approvalPreview = approvalPreviewPanelKey ? previewsByKey.get(approvalPreviewPanelKey) ?? null : null;
  const [openPreviewRequest, setOpenPreviewRequest] = useState<{ panelKey: string | null; requestId: number }>({
    panelKey: null,
    requestId: 0,
  });
  const handleViewPreview = useCallback((panelKey: string) => {
    setOpenPreviewRequest((current) => ({
      panelKey,
      requestId: current.requestId + 1,
    }));
    if (typeof document === 'undefined') return;
    const target = document.querySelector<HTMLElement>(`[data-preview-panel-key="${panelKey}"]`);
    if (!target) return;
    target.scrollIntoView({ behavior: 'smooth', block: 'start' });
    target.setAttribute('data-preview-flash', 'true');
    window.setTimeout(() => target.removeAttribute('data-preview-flash'), 1400);
  }, []);
  const handleReviewApproval = useCallback(() => {
    if (typeof document === 'undefined') return;
    const target = document.querySelector<HTMLElement>('[data-coding-session-interruption-panel]');
    if (!target) return;
    target.scrollIntoView({ behavior: 'smooth', block: 'end' });
    target.setAttribute('data-review-flash', 'true');
    window.setTimeout(() => target.removeAttribute('data-review-flash'), 1400);
  }, []);
  const promptArtifact = useMemo(() => {
    for (let index = artifacts.length - 1; index >= 0; index -= 1) {
      const artifact = artifacts[index];
      if (artifact?.artifact_type === 'codex_prompt' || artifact?.artifact_type === 'opencode_prompt') {
        return artifact;
      }
    }
    return null;
  }, [artifacts]);
  const reviewArtifacts = useMemo(
    () => {
      const parseAssistantSequenceNo = (artifact: AgentRunArtifact) => {
        if (!artifact.inline_content) return 0;
        try {
          const parsed = JSON.parse(artifact.inline_content) as Record<string, unknown>;
          const value = parsed.assistant_message_sequence_no;
          return typeof value === 'number' && Number.isFinite(value) ? value : 0;
        } catch {
          return 0;
        }
      };

      const decisionBySequence = new Map<number, AgentRunArtifact>();
      for (const artifact of artifacts) {
        if (artifact?.artifact_type !== 'review_decision') continue;
        const seq = parseAssistantSequenceNo(artifact);
        if (seq > 0) {
          decisionBySequence.set(seq, artifact);
        }
      }

      return artifacts
        .filter((artifact) => artifact?.artifact_type === 'review_findings')
        .sort((a, b) => (a.created_at < b.created_at ? -1 : a.created_at > b.created_at ? 1 : 0))
        .map((artifact) => {
          const seq = parseAssistantSequenceNo(artifact);
          return {
            artifact,
            decisionArtifact: seq > 0 ? decisionBySequence.get(seq) ?? null : null,
          };
        });
    },
    [artifacts],
  );
  const showPlanPanel = shouldShowCodingSessionPlanPanel(streamState.current_plan);
  const showSidePanel = shouldShowCodingSessionSidePanel(streamState.current_plan, previewsByKey.size, reviewArtifacts.length);
  const showRecoveryNotice = shouldShowFailedCodingSessionRecoveryNotice(session?.status, streamState.current_plan, previewsByKey.size);

  const runAction = useCallback(async (name: string, fn: () => Promise<{ error: string | null }>) => {
    setActing(name);
    try {
      const result = await fn();
      if (result.error) throw new Error(result.error);
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Action failed');
    } finally {
      setActing(null);
    }
  }, [load]);

  const resolveInteraction = useCallback(async (
    interactionId: string,
    responsePayload: Record<string, unknown>,
    followupMessage?: string,
  ) => {
    await runAction('resolve-interaction', () => codingSessionService.resolveInteraction(workspaceId, activeSessionId, interactionId, {
      response_payload: responsePayload,
      ...(followupMessage?.trim() ? { followup_message: followupMessage.trim() } : {}),
    }));
  }, [runAction, activeSessionId, workspaceId]);

  const continueRun = useCallback(async (content?: string) => {
    if (!workspaceId || !activeSessionId) return;
    const result = await codingSessionService.continue(workspaceId, activeSessionId, content?.trim() ? { content: content.trim() } : {});
    if (result.error) {
      throw new Error(result.error);
    }
    const nextRunId = result.data?.id;
    if (!nextRunId) {
      throw new Error('Continuation did not return a new run');
    }
    setError(null);
    setActiveSessionId(nextRunId);
  }, [workspaceId, activeSessionId]);

  const sendMessage = useCallback(async (content: string) => {
    if (!workspaceId || !activeSessionId || !session) return;
    setSendingMessage(true);
    try {
      if (session.status === 'failed' || session.status === 'cancelled') {
        await continueRun(content);
        return;
      }
      const result = await codingSessionService.sendMessage(workspaceId, activeSessionId, { content });
      if (result.error) throw new Error(result.error);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to send message');
    } finally {
      setSendingMessage(false);
    }
  }, [workspaceId, activeSessionId, session, continueRun]);

  const messageComposer = useMemo(
    () => resolveCodingSessionComposerState(session, activeInteraction, loading),
    [activeInteraction, loading, session],
  );
  const terminalContinuation = session !== null && (session.status === 'failed' || session.status === 'cancelled');

  const canSuggestHandoff = session !== null
    && session.status === 'completed'
    && session.target_type === 'task';

  useEffect(() => {
    if (!canSuggestHandoff || !workspaceId || handoffAgents !== null) return;
    let cancelled = false;
    void (async () => {
      const res = await agentService.list(workspaceId);
      if (cancelled) return;
      setHandoffAgents(res.data ?? []);
    })();
    return () => { cancelled = true; };
  }, [canSuggestHandoff, workspaceId, handoffAgents]);

  useEffect(() => {
    if (!canSuggestHandoff || !workspaceId || !session?.target_id) return;
    if (handoffRuns !== null && handoffRunsTargetId === session.target_id) return;
    let cancelled = false;
    void (async () => {
      const res = await agentService.listTargetRuns(workspaceId, 'task', session.target_id);
      if (cancelled) return;
      setHandoffRuns(res.data ?? []);
      setHandoffRunsTargetId(session.target_id);
    })();
    return () => { cancelled = true; };
  }, [canSuggestHandoff, workspaceId, session?.target_id, handoffRuns, handoffRunsTargetId]);

  useEffect(() => {
    if (!session?.target_id || session.target_type !== 'task') return;
    const handler = (event: Event) => {
      const detail = (event as CustomEvent).detail as { parent_type?: string; parent_id?: string } | undefined;
      if (detail?.parent_type === 'task' && detail.parent_id === session.target_id) {
        setHandoffRuns(null);
        setHandoffRunsTargetId(null);
      }
    };
    window.addEventListener('agent_run-updated', handler);
    window.addEventListener('agent_run-created', handler);
    return () => {
      window.removeEventListener('agent_run-updated', handler);
      window.removeEventListener('agent_run-created', handler);
    };
  }, [session?.target_id, session?.target_type]);

  const handoffCandidates = useMemo(
    () => (handoffAgents ?? []).filter((agent) => agent.allowed_targets.includes('task')),
    [handoffAgents],
  );
  const completedSessionAgent = useMemo(() => {
    if (!session || !handoffAgents) return null;
    return handoffAgents.find((agent) => agent.id === session.agent_id) ?? null;
  }, [session, handoffAgents]);
  const completedPersonaKeys = useMemo(() => {
    if (!handoffAgents || !handoffRuns) return undefined;
    if (handoffRunsTargetId !== session?.target_id) return undefined;
    const agentById = new Map(handoffAgents.map((agent) => [agent.id, agent]));
    const keys = new Set<AgentPersonaKey>();
    for (const run of handoffRuns) {
      const agent = agentById.get(run.agent_id);
      if (agent) {
        keys.add(resolveAgentPersonaKey({ agent }));
      }
    }
    return keys;
  }, [handoffAgents, handoffRuns, handoffRunsTargetId, session?.target_id]);

  const startHandoffRun = useCallback(async (agent: Agent) => {
    if (!session) return;
    const res = await agentService.runTask(workspaceId, session.target_id, { agent_id: agent.id });
    if (res.error) {
      const reason = getUpgradeRequiredReason(res.error);
      if (reason) {
        setUpgradeDialogReason(reason);
        return;
      }
      toast.error(res.error);
      return;
    }
    if (res.data?.id) {
      setActiveSessionId(res.data.id);
    }
  }, [session, workspaceId]);

  return (
    <div className={cn(
      'flex flex-col gap-4',
      embedded
        ? 'h-full min-h-0 overflow-hidden p-4'
        : 'min-h-full overflow-y-auto p-4 md:p-6 xl:h-full xl:min-h-0 xl:overflow-hidden xl:p-6',
    )}>
      <CodingSessionHeader
        session={session}
        statusIcon={STATUS_ICON[session?.status ?? 'queued'] ?? <Clock01Icon className="h-3.5 w-3.5" />}
        workspaceSlug={showBackToRuns ? workspaceSlug : undefined}
        onRefresh={() => void load()}
        refreshing={loading}
        acting={acting}
        onCancelRun={() => void runAction('cancel', () => codingSessionService.cancel(workspaceId, activeSessionId))}
      />

      {canSuggestHandoff && completedSessionAgent && completedPersonaKeys ? (
        <NextAgentHint
          completedAgent={completedSessionAgent}
          candidates={handoffCandidates}
          completedPersonaKeys={completedPersonaKeys}
          onRun={startHandoffRun}
          density="comfortable"
        />
      ) : null}

      {error ? (
        <div className="rounded-lg border border-destructive/30 bg-destructive/5 px-4 py-3 text-sm text-destructive">
          {error}
        </div>
      ) : null}

      {!error && session?.error_message ? (
        <div className="rounded-lg border border-destructive/30 bg-destructive/5 px-4 py-3 text-sm text-destructive">
          <div className="flex flex-wrap items-start justify-between gap-3">
            <div className="space-y-1">
              <p className="font-medium">Run failed</p>
              <p>{session.error_message}</p>
              {terminalContinuation ? (
                <p className="text-xs text-destructive/80">
                  Type a follow-up below to continue from this run, or retry immediately.
                </p>
              ) : null}
            </div>
            {terminalContinuation ? (
              <Button
                variant="outline"
                size="sm"
                className="border-destructive/30 text-destructive hover:bg-destructive/10 hover:text-destructive"
                disabled={sendingMessage}
                onClick={() => {
                  void (async () => {
                    setSendingMessage(true);
                    try {
                      await continueRun();
                    } catch (err) {
                      setError(err instanceof Error ? err.message : 'Failed to continue run');
                    } finally {
                      setSendingMessage(false);
                    }
                  })();
                }}
              >
                Retry
              </Button>
            ) : null}
          </div>
        </div>
      ) : null}
      <UpgradeRequiredDialog
        open={upgradeDialogReason !== null}
        onOpenChange={(open) => {
          if (!open) setUpgradeDialogReason(null);
        }}
        reason={upgradeDialogReason}
      />

      <div className={cn(
        'grid min-h-0 flex-1 gap-4 xl:overflow-hidden',
        showSidePanel
          ? 'xl:grid-cols-[minmax(0,1.55fr)_minmax(320px,0.9fr)]'
          : 'xl:grid-cols-1',
      )}>
        <CodingTranscriptPane
          promptArtifact={promptArtifact}
          reviewArtifacts={activeInteraction ? reviewArtifacts : []}
          transcriptMessages={streamState.transcript_messages}
          liveAssistantMessage={streamState.live_assistant_message}
          liveReasoningMessage={streamState.live_reasoning_message}
          liveTurnSegments={streamState.live_turn_segments}
          loading={loading}
          onSendMessage={messageComposer.enabled ? sendMessage : undefined}
          sendingMessage={sendingMessage}
          session={session}
          activeInteraction={activeInteraction}
          acting={acting}
          messageComposer={messageComposer}
          availablePreviewPanelKey={approvalPreviewPanelKey}
          attachedPreview={approvalPreview}
          onViewPreview={handleViewPreview}
          onAuthStart={() => void runAction('auth-start', () => codingSessionService.startDeviceCodeAuth(workspaceId, activeSessionId))}
          onAuthCancel={() => void runAction('auth-cancel', () => codingSessionService.cancelDeviceCodeAuth(workspaceId, activeSessionId))}
          onResolveInteraction={(interactionId, responsePayload, followupMessage) => void resolveInteraction(interactionId, responsePayload, followupMessage)}
        />

        {showSidePanel ? (
          <div
            className="min-h-0 space-y-4 overflow-y-auto pb-[calc(env(safe-area-inset-bottom)+5rem)]"
            data-coding-session-side-panel
          >
            {showRecoveryNotice ? (
              <div
                className="rounded-lg border border-amber-200/80 bg-amber-50 px-3 py-2 text-xs text-amber-900 dark:border-amber-900/60 dark:bg-amber-950/25 dark:text-amber-200"
                data-coding-session-recovered-output
              >
                <div className="font-medium">Recovered output before failure</div>
                <p className="mt-0.5 text-amber-800/80 dark:text-amber-200/75">
                  This plan or draft was captured before the run stopped. Review it as partial work, then continue or retry the run when ready.
                </p>
              </div>
            ) : null}
            {showPlanPanel ? (
              <CodingPlanPanel plan={streamState.current_plan} runStatus={session?.status} />
            ) : null}
            <CodingPreviewPanels
              previewsByKey={previewsByKey}
              attachedApprovalInteraction={activeInteraction}
              approvalStatesByPreviewKey={approvalStatesByPreviewKey}
              resolverNamesByUserId={resolverNamesByUserId}
              onReviewApproval={handleReviewApproval}
              openPreviewPanelKey={openPreviewRequest.panelKey}
              openPreviewRequestId={openPreviewRequest.requestId}
            />
            <CodingReviewHistoryPanel reviewArtifacts={reviewArtifacts} />
          </div>
        ) : null}
      </div>
    </div>
  );
}
