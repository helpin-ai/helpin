import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { CheckmarkCircle02Icon, Clock01Icon, SecurityCheckIcon, CancelCircleIcon } from '@/lib/icons';
import { UnicodeSpinner } from '@/components/pm/CodingSession/UnicodeSpinner';

import { CodingPlanPanel } from '@/components/pm/CodingSession/CodingPlanPanel';
import { CodingPreviewPanels } from '@/components/pm/CodingSession/CodingPreviewPanels';
import { CodingSessionHeader } from '@/components/pm/CodingSession/CodingSessionHeader';
import { CodingTranscriptPane } from '@/components/pm/CodingSession/CodingTranscriptPane';
import { collectCodingSessionPreviews } from '@/components/pm/CodingSession/codingSessionPreviews';
import { buildCodingSessionStreamState } from '@/components/pm/CodingSession/codingSessionStream';
import {
  isPersistedCodingSessionEvent,
  parseCodingSessionInteraction,
  latestPendingCodingSessionInteraction,
  maxPersistedCodingSessionSequence,
  upsertCodingSessionEvents,
} from '@/components/pm/CodingSession/codingSessionUtils';
import type { CodingSession, CodingSessionEvent, CodingSessionStreamSnapshot } from '@/lib/pmTypes';
import { codingSessionService } from '@/lib/services/codingSessionService';
import { cn } from '@/lib/utils';
import { useWorkspaceStore } from '@/stores/workspaceStore';

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

  const [session, setSession] = useState<CodingSession | null>(null);
  const [events, setEvents] = useState<CodingSessionEvent[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [acting, setActing] = useState<string | null>(null);
  const sequenceRef = useRef(0);
  const seededSnapshotSessionRef = useRef<string | null>(null);
  const [streamSnapshotSeed, setStreamSnapshotSeed] = useState<CodingSessionStreamSnapshot | null>(null);

  const loadSession = useCallback(async () => {
    if (!workspaceId || !sessionId) return;
    const sessionRes = await codingSessionService.get(workspaceId, sessionId);
    if (sessionRes.error) throw new Error(sessionRes.error);
    const nextSession = sessionRes.data as CodingSession;
    if (seededSnapshotSessionRef.current !== sessionId) {
      seededSnapshotSessionRef.current = sessionId;
      setStreamSnapshotSeed(nextSession.stream_state_snapshot ?? null);
    }
    setSession(nextSession);
  }, [workspaceId, sessionId]);

  const loadEvents = useCallback(async (after = 0) => {
    if (!workspaceId || !sessionId) return;
    const eventsRes = await codingSessionService.listEvents(workspaceId, sessionId, after);
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
  }, [workspaceId, sessionId]);

  const load = useCallback(async () => {
    if (!workspaceId || !sessionId) return;
    setLoading(true);
    setError(null);
    try {
      await Promise.all([loadSession(), loadEvents(0)]);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load agent session');
    } finally {
      setLoading(false);
    }
  }, [workspaceId, sessionId, loadSession, loadEvents]);

  useEffect(() => {
    seededSnapshotSessionRef.current = null;
    setStreamSnapshotSeed(null);
    sequenceRef.current = 0;
    setEvents([]);
  }, [sessionId, workspaceId]);

  useEffect(() => {
    void load();
  }, [load]);

  const reconcileEvents = useCallback(async () => {
    if (!workspaceId || !sessionId) return;
    try {
      await loadEvents(sequenceRef.current);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to refresh session events');
    }
  }, [workspaceId, sessionId, loadEvents]);

  useEffect(() => {
    const onSessionUpdated = (raw: Event) => {
      const detail = (raw as CustomEvent).detail as { entity_id?: string; data?: Record<string, unknown> } | undefined;
      if (!detail || detail.entity_id !== sessionId) return;
      setSession((current) => current ? {
        ...current,
        status: typeof detail.data?.status === 'string' ? detail.data.status as CodingSession['status'] : current.status,
        pause_reason: typeof detail.data?.pause_reason === 'string' ? detail.data.pause_reason as CodingSession['pause_reason'] : current.pause_reason,
        updated_at: new Date().toISOString(),
      } : current);
      void loadSession();
    };
    const onSessionEvent = (raw: Event) => {
      const detail = (raw as CustomEvent).detail as { parent_id?: string; data?: CodingSessionEvent } | undefined;
      if (!detail || detail.parent_id !== sessionId || !detail.data) return;
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
  }, [loadSession, reconcileEvents, sessionId]);

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
  const interactionsById = useMemo(() => {
    const next = new Map<string, NonNullable<ReturnType<typeof parseCodingSessionInteraction>>>();
    for (const event of events) {
      const interaction = parseCodingSessionInteraction(event);
      if (interaction) next.set(interaction.interaction_id, interaction);
    }
    return next;
  }, [events]);
  const previewsByKey = useMemo(
    () => collectCodingSessionPreviews(events, streamState.live_turn_segments),
    [events, streamState.live_turn_segments],
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
    const interaction = interactionsById.get(interactionId);
    if (interaction?.interaction_kind === 'review_checkpoint') {
      const decision = typeof responsePayload.decision === 'string' ? responsePayload.decision.trim() : '';
      if (decision === 'approve') {
        await runAction('approve-review-checkpoint', () => codingSessionService.approve(workspaceId, sessionId, {
          ...(followupMessage?.trim() ? { content: followupMessage.trim(), send_message: true } : {}),
        }));
        return;
      }
      await runAction('request-review-changes', () => codingSessionService.requestChanges(workspaceId, sessionId, {
        content: followupMessage?.trim() || 'Please revise and continue.',
      }));
      return;
    }
    await runAction('resolve-interaction', () => codingSessionService.resolveInteraction(workspaceId, sessionId, interactionId, {
      response_payload: responsePayload,
      ...(followupMessage?.trim() ? { followup_message: followupMessage.trim() } : {}),
    }));
  }, [interactionsById, runAction, sessionId, workspaceId]);

  const sendMessage = useCallback(async (content: string) => {
    const result = await codingSessionService.sendMessage(workspaceId, sessionId, { content });
    if (result.error) setError(result.error);
  }, [workspaceId, sessionId]);

  const canSendMessage = session !== null && (session.status === 'running' || session.status === 'paused');

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
        onCancelRun={() => void runAction('cancel', () => codingSessionService.cancel(workspaceId, sessionId))}
      />

      {error ? (
        <div className="rounded-lg border border-destructive/30 bg-destructive/5 px-4 py-3 text-sm text-destructive">
          {error}
        </div>
      ) : null}

      <div className="grid min-h-0 flex-1 gap-4 xl:overflow-hidden xl:grid-cols-[minmax(0,1.55fr)_minmax(320px,0.9fr)]">
        <CodingTranscriptPane
          transcriptMessages={streamState.transcript_messages}
          liveAssistantMessage={streamState.live_assistant_message}
          liveReasoningMessage={streamState.live_reasoning_message}
          liveTurnSegments={streamState.live_turn_segments}
          loading={loading}
          onSendMessage={canSendMessage ? sendMessage : undefined}
          session={session}
          activeInteraction={activeInteraction}
          acting={acting}
          onAuthStart={() => void runAction('auth-start', () => codingSessionService.startDeviceCodeAuth(workspaceId, sessionId))}
          onAuthCancel={() => void runAction('auth-cancel', () => codingSessionService.cancelDeviceCodeAuth(workspaceId, sessionId))}
          onResolveInteraction={(interactionId, responsePayload, followupMessage) => void resolveInteraction(interactionId, responsePayload, followupMessage)}
        />

        <div className="min-h-0 space-y-4 overflow-y-auto">
          <CodingPlanPanel plan={streamState.current_plan} />
          <CodingPreviewPanels previewsByKey={previewsByKey} />
        </div>
      </div>
    </div>
  );
}
