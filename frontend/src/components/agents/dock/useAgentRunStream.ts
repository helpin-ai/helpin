import { useCallback, useEffect, useRef, useState } from 'react';
import { agentService } from '@/lib/services/agentService';
import {
  latestPendingCodingSessionInteraction,
  maxPersistedCodingSessionSequence,
  upsertCodingSessionEvents,
} from '@/components/pm/CodingSession/codingSessionUtils';
import {
  buildCodingSessionStreamState,
  mergeCodingSessionStreamSnapshotSeed,
} from '@/components/pm/CodingSession/codingSessionStream';
import type {
  CodingSession,
  CodingSessionEvent,
  CodingSessionEventListResponse,
  CodingSessionInteraction,
  CodingSessionStreamSnapshot,
  CodingSessionStreamState,
  RunPlanArtifact,
} from '@/lib/pmTypes';

/**
 * Pluggable snapshot/event fetchers. The default hits the PM-gated
 * /pm/agent-runs endpoints; the dock chat view supplies chat-scoped fetchers
 * (/dock/chats/{id}/run*) so users without PM permissions can stream their
 * own chat.
 */
export interface AgentRunStreamFetchers {
  getSnapshot: (workspaceId: string, runId: string) => Promise<{ data: CodingSession | null; error: string | null }>;
  listEvents: (
    workspaceId: string,
    runId: string,
    after: number,
  ) => Promise<{ data: CodingSessionEventListResponse | null; error: string | null }>;
}

const defaultFetchers: AgentRunStreamFetchers = {
  getSnapshot: (workspaceId, runId) => agentService.getRunSnapshot(workspaceId, runId),
  listEvents: (workspaceId, runId, after) => agentService.listRunEvents(workspaceId, runId, after),
};

interface AgentRunStreamState {
  currentPlan: RunPlanArtifact | null;
  pendingInteraction: CodingSessionInteraction | null;
  /**
   * Full reconciled transcript + live-turn state, the same shape the coding
   * session drawer renders from. Null until the first fetch resolves. Lets the
   * Ask Agents dock render an agent's output inline instead of behind a sheet.
   */
  streamState: CodingSessionStreamState | null;
  /** True while at least one fetch is in flight. */
  loading: boolean;
  /** Force a snapshot/events reconciliation for this run. */
  refetch: () => Promise<void>;
  /** Optimistically hide a submitted interaction while the backend catches up. */
  clearPendingInteraction: (interactionId: string) => void;
}

/**
 * Subscribes to live plan + pending-interaction state for a single agent run.
 *
 * Strategy:
 * - On mount (when active): fetch /snapshot + /events.
 * - Apply `coding_session_event-created` payloads immediately, using the host
 *   run id carried by the websocket parent rather than the event's own id.
 * - Reconcile snapshots after status changes or sequence gaps.
 * - Light polling fallback every `pollMs` while the run is active, in case a
 *   WS event is dropped. Disabled when `active === false`.
 *
 * Returns null fields when nothing is known yet — callers should treat them as
 * "no live state" and fall back to the static run summary.
 */
export function useAgentRunStream(
  workspaceId: string | undefined,
  runId: string | null | undefined,
  active: boolean,
  pollMs = 5_000,
  fetchers: AgentRunStreamFetchers = defaultFetchers,
): AgentRunStreamState {
  const [currentPlan, setCurrentPlan] = useState<RunPlanArtifact | null>(null);
  const [streamState, setStreamState] = useState<CodingSessionStreamState | null>(null);
  const [pendingInteraction, setPendingInteraction] =
    useState<CodingSessionInteraction | null>(null);
  const [loading, setLoading] = useState(false);

  const persistedSeqRef = useRef(0);
  const runtimeSeqRef = useRef(0);
  const eventsRef = useRef<CodingSessionEvent[]>([]);
  const snapshotRef = useRef<CodingSessionStreamSnapshot | null>(null);
  const generationRef = useRef(0);
  const loadingCountRef = useRef(0);
  const clearedInteractionIdsRef = useRef<Set<string>>(new Set());

  const publishStreamState = useCallback(() => {
    const built = buildCodingSessionStreamState(eventsRef.current, snapshotRef.current);
    const latestPending = latestPendingCodingSessionInteraction(eventsRef.current);
    setStreamState(built);
    setCurrentPlan(built.current_plan);
    setPendingInteraction(
      latestPending && clearedInteractionIdsRef.current.has(latestPending.interaction_id)
        ? null
        : latestPending,
    );
  }, []);

  const clearPendingInteraction = useCallback((interactionId: string) => {
    const trimmed = interactionId.trim();
    if (!trimmed) return;
    clearedInteractionIdsRef.current.add(trimmed);
    setPendingInteraction((current) =>
      current?.interaction_id === trimmed ? null : current,
    );
  }, []);

  const refetch = useCallback(async () => {
    if (!workspaceId || !runId) return;
    const generation = generationRef.current;
    loadingCountRef.current += 1;
    setLoading(true);
    try {
      const [snap, ev] = await Promise.all([
        fetchers.getSnapshot(workspaceId, runId),
        fetchers.listEvents(workspaceId, runId, persistedSeqRef.current),
      ]);
      if (generationRef.current !== generation) return;

      // Dock endpoints resolve the chat's active run. A chat can roll over to
      // a successor while this request is in flight, so never merge a response
      // that identifies a different host run.
      if (typeof snap.data?.id === 'string' && snap.data.id !== runId) return;
      const acceptedEvents = (ev.data?.events ?? []).filter(
        (event) => event.session_id === runId && event.run_id === runId,
      );
      if (acceptedEvents.length > 0) {
        eventsRef.current = upsertCodingSessionEvents(eventsRef.current, acceptedEvents);
        persistedSeqRef.current = maxPersistedCodingSessionSequence(eventsRef.current);
      }

      // The event-list response does not identify which active dock run it
      // resolved. Only use the snapshot embedded in the run-identified session
      // response; otherwise a rollover between these two requests can leak the
      // successor snapshot into the predecessor stream.
      const incomingSnapshot = snap.data?.stream_state_snapshot ?? null;
      const incomingSequence = incomingSnapshot?.through_sequence ?? 0;
      const terminal = snap.data
        ? ['completed', 'failed', 'cancelled'].includes(snap.data.status)
        : false;
      // A response that started before a websocket delta must not erase that
      // newer live state. Terminal snapshots remain authoritative so completed
      // live segments can be cleared.
      if (
        terminal
        || runtimeSeqRef.current === 0
        || incomingSequence >= runtimeSeqRef.current
      ) {
        snapshotRef.current = mergeCodingSessionStreamSnapshotSeed(
          snapshotRef.current,
          incomingSnapshot,
        );
        runtimeSeqRef.current = Math.max(runtimeSeqRef.current, incomingSequence);
      }

      publishStreamState();
    } finally {
      if (generationRef.current === generation) {
        loadingCountRef.current = Math.max(0, loadingCountRef.current - 1);
        setLoading(loadingCountRef.current > 0);
      }
    }
  }, [fetchers, publishStreamState, runId, workspaceId]);

  const ingestRealtimeEvent = useCallback((event: CodingSessionEvent) => {
    if (!runId || event.session_id !== runId || event.run_id !== runId) return false;
    const hostRunId = typeof event.payload?.host_run_id === 'string'
      ? event.payload.host_run_id.trim()
      : '';
    if (hostRunId && hostRunId !== runId) return false;

    const source = typeof event.runtime_metadata?.source === 'string'
      ? event.runtime_metadata.source.trim()
      : '';
    const runtimeV2 = source === 'agent-runtime-v2';
    const hasRuntimeGap = runtimeV2
      && event.sequence_no > 0
      && (
        (runtimeSeqRef.current === 0 && event.sequence_no > 1)
        || (runtimeSeqRef.current > 0 && event.sequence_no > runtimeSeqRef.current + 1)
      );

    eventsRef.current = upsertCodingSessionEvents(eventsRef.current, [event]);
    persistedSeqRef.current = maxPersistedCodingSessionSequence(eventsRef.current);
    if (runtimeV2 && event.sequence_no > 0) {
      runtimeSeqRef.current = Math.max(runtimeSeqRef.current, event.sequence_no);
    }
    publishStreamState();
    return hasRuntimeGap;
  }, [publishStreamState, runId]);

  // Reset on workspace, chat fetcher, or run change. The generation token
  // prevents a predecessor request from committing after the new run mounts.
  useEffect(() => {
    generationRef.current += 1;
    loadingCountRef.current = 0;
    persistedSeqRef.current = 0;
    runtimeSeqRef.current = 0;
    eventsRef.current = [];
    snapshotRef.current = null;
    clearedInteractionIdsRef.current = new Set();
    setCurrentPlan(null);
    setStreamState(null);
    setPendingInteraction(null);
    setLoading(false);
    return () => {
      generationRef.current += 1;
    };
  }, [fetchers, runId, workspaceId]);

  // Initial fetch + WS-driven updates.
  useEffect(() => {
    if (!active || !workspaceId || !runId) return;
    void refetch();
    let refreshTimer: ReturnType<typeof setTimeout> | null = null;
    const scheduleReconcile = () => {
      if (refreshTimer) return;
      refreshTimer = setTimeout(() => {
        refreshTimer = null;
        void refetch();
      }, 100);
    };
    const onRunUpdated = (event: Event) => {
      const detail = (event as CustomEvent<{ entity_id?: string }>).detail;
      if (detail?.entity_id !== runId) return;
      scheduleReconcile();
    };
    const onSessionEvent = (event: Event) => {
      const detail = (event as CustomEvent<{
        parent_id?: string;
        data?: CodingSessionEvent;
      }>).detail;
      if (detail?.parent_id !== runId || !detail.data) return;
      if (ingestRealtimeEvent(detail.data)) scheduleReconcile();
    };
    window.addEventListener('agent_run-updated', onRunUpdated);
    window.addEventListener('coding_session-updated', onRunUpdated);
    window.addEventListener('coding_session_event-created', onSessionEvent);
    return () => {
      if (refreshTimer) clearTimeout(refreshTimer);
      window.removeEventListener('agent_run-updated', onRunUpdated);
      window.removeEventListener('coding_session-updated', onRunUpdated);
      window.removeEventListener('coding_session_event-created', onSessionEvent);
    };
  }, [active, ingestRealtimeEvent, refetch, runId, workspaceId]);

  // Light polling fallback while active.
  useEffect(() => {
    if (!active || !workspaceId || !runId || pollMs <= 0) return;
    const id = setInterval(() => {
      void refetch();
    }, pollMs);
    return () => clearInterval(id);
  }, [active, pollMs, refetch, runId, workspaceId]);

  return { currentPlan, streamState, pendingInteraction, loading, refetch, clearPendingInteraction };
}
