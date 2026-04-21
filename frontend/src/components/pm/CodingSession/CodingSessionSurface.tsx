import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { toast } from 'sonner';
import { CheckmarkCircle02Icon, Clock01Icon, SecurityCheckIcon, CancelCircleIcon } from '@/lib/icons';
import { UnicodeSpinner } from '@/components/pm/CodingSession/UnicodeSpinner';

import { CodingPlanPanel } from '@/components/pm/CodingSession/CodingPlanPanel';
import { CodingPreviewPanels } from '@/components/pm/CodingSession/CodingPreviewPanels';
import { CodingSessionHeader } from '@/components/pm/CodingSession/CodingSessionHeader';
import { CodingTranscriptPane } from '@/components/pm/CodingSession/CodingTranscriptPane';
import { NextAgentHint } from '@/components/agents/NextAgentHint';
import { collectCodingSessionPreviews } from '@/components/pm/CodingSession/codingSessionPreviews';
import { buildCodingSessionStreamState } from '@/components/pm/CodingSession/codingSessionStream';
import {
  isPersistedCodingSessionEvent,
  latestPendingCodingSessionInteraction,
  maxPersistedCodingSessionSequence,
  upsertCodingSessionEvents,
} from '@/components/pm/CodingSession/codingSessionUtils';
import type { Agent, AgentRunArtifact, CodingSession, CodingSessionEvent, CodingSessionStreamSnapshot } from '@/lib/pmTypes';
import { agentService } from '@/lib/services/agentService';
import { codingSessionService } from '@/lib/services/codingSessionService';
import { cn } from '@/lib/utils';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { Button } from '@/components/ui/button';

const STATUS_ICON = {
  queued: <Clock01Icon className="h-3.5 w-3.5" />,
  running: <UnicodeSpinner name="braille" className="text-sm text-primary" />,
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
  const sequenceRef = useRef(0);
  const seededSnapshotSessionRef = useRef<string | null>(null);
  const [streamSnapshotSeed, setStreamSnapshotSeed] = useState<CodingSessionStreamSnapshot | null>(null);

  useEffect(() => {
    setActiveSessionId(sessionId);
  }, [sessionId]);

  const loadSession = useCallback(async () => {
    if (!workspaceId || !activeSessionId) return;
    const sessionRes = await codingSessionService.get(workspaceId, activeSessionId);
    if (sessionRes.error) throw new Error(sessionRes.error);
    const nextSession = sessionRes.data as CodingSession;
    if (seededSnapshotSessionRef.current !== activeSessionId) {
      seededSnapshotSessionRef.current = activeSessionId;
      setStreamSnapshotSeed(nextSession.stream_state_snapshot ?? null);
    }
    setSession(nextSession);
  }, [workspaceId, activeSessionId]);

  const loadEvents = useCallback(async (after = 0) => {
    if (!workspaceId || !activeSessionId) return;
    const eventsRes = await codingSessionService.listEvents(workspaceId, activeSessionId, after);
    if (eventsRes.error) throw new Error(eventsRes.error);
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
    seededSnapshotSessionRef.current = null;
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
      setSession((current) => current ? {
        ...current,
        parent_run_id: typeof detail.data?.parent_run_id === 'string'
          ? detail.data.parent_run_id || undefined
          : current.parent_run_id,
        status: typeof detail.data?.status === 'string' ? detail.data.status as CodingSession['status'] : current.status,
        pause_reason: typeof detail.data?.pause_reason === 'string' ? detail.data.pause_reason as CodingSession['pause_reason'] : current.pause_reason,
        error_message: typeof detail.data?.error_message === 'string'
          ? detail.data.error_message || undefined
          : current.error_message,
        updated_at: new Date().toISOString(),
      } : current);
      void loadSession();
      void loadArtifacts();
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
  const activeInteraction = useMemo(
    () => latestPendingCodingSessionInteraction(events),
    [events],
  );
  const previewsByKey = useMemo(
    () => collectCodingSessionPreviews(events, streamState.live_turn_segments),
    [events, streamState.live_turn_segments],
  );
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

  const canSendMessage = session !== null && (
    session.status === 'running'
    || session.status === 'paused'
    || session.status === 'failed'
    || session.status === 'cancelled'
  );
  const terminalContinuation = session !== null && (session.status === 'failed' || session.status === 'cancelled');
  const messagePlaceholder = terminalContinuation
    ? 'This run ended. Type instructions to continue from the previous progress… (⌘↵ to send)'
    : 'Reply to agent… (⌘↵ to send)';

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

  const handoffCandidates = useMemo(
    () => (handoffAgents ?? []).filter((agent) => agent.allowed_targets.includes('task')),
    [handoffAgents],
  );
  const completedSessionAgent = useMemo(() => {
    if (!session || !handoffAgents) return null;
    return handoffAgents.find((agent) => agent.id === session.agent_id) ?? null;
  }, [session, handoffAgents]);

  const startHandoffRun = useCallback(async (agent: Agent) => {
    if (!session) return;
    const res = await agentService.runTask(workspaceId, session.target_id, { agent_id: agent.id });
    if (res.error) {
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

      {canSuggestHandoff && completedSessionAgent ? (
        <NextAgentHint
          completedAgent={completedSessionAgent}
          candidates={handoffCandidates}
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

      <div className="grid min-h-0 flex-1 gap-4 xl:overflow-hidden xl:grid-cols-[minmax(0,1.55fr)_minmax(320px,0.9fr)]">
        <CodingTranscriptPane
          promptArtifact={promptArtifact}
          reviewArtifacts={reviewArtifacts}
          transcriptMessages={streamState.transcript_messages}
          liveAssistantMessage={streamState.live_assistant_message}
          liveReasoningMessage={streamState.live_reasoning_message}
          liveTurnSegments={streamState.live_turn_segments}
          loading={loading}
          onSendMessage={canSendMessage ? sendMessage : undefined}
          sendingMessage={sendingMessage}
          session={session}
          activeInteraction={activeInteraction}
          acting={acting}
          messagePlaceholder={messagePlaceholder}
          onAuthStart={() => void runAction('auth-start', () => codingSessionService.startDeviceCodeAuth(workspaceId, activeSessionId))}
          onAuthCancel={() => void runAction('auth-cancel', () => codingSessionService.cancelDeviceCodeAuth(workspaceId, activeSessionId))}
          onResolveInteraction={(interactionId, responsePayload, followupMessage) => void resolveInteraction(interactionId, responsePayload, followupMessage)}
        />

        <div className="min-h-0 space-y-4 overflow-y-auto">
          <CodingPlanPanel plan={streamState.current_plan} runStatus={session?.status} />
          <CodingPreviewPanels previewsByKey={previewsByKey} />
        </div>
      </div>
    </div>
  );
}
