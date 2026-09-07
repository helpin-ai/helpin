import { useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react';
import { useAuthStore } from '@/stores/authStore';
import { currentAgentRunReadVersion, readSharedAgentRun } from './sharedAgentRunRead';
import { useSupportPresenceStore } from '@/stores/supportPresenceStore';
import { isDockNetworkAvailable, useDockNetworkActivity } from './useDockNetworkActivity';
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
  /** Stable endpoint/chat identity for sharing reads across mounted docks. */
  requestKey?: string;
  getSnapshot: (workspaceId: string, runId: string, signal?: AbortSignal) => Promise<{ data: CodingSession | null; error: string | null }>;
  listEvents: (
    workspaceId: string,
    runId: string,
    after: number,
    signal?: AbortSignal,
  ) => Promise<{ data: CodingSessionEventListResponse | null; error: string | null }>;
}

interface StreamRequest {
  generation: number;
  automatic: boolean;
  afterRead?: number;
  controller: AbortController;
  promise: Promise<void>;
  resolve: () => void;
}

const fetcherIds = new WeakMap<AgentRunStreamFetchers, number>();
let nextFetcherId = 0;
function fetcherIdentity(fetchers: AgentRunStreamFetchers) {
  if (fetchers.requestKey) return fetchers.requestKey;
  if (!fetcherIds.has(fetchers)) fetcherIds.set(fetchers, ++nextFetcherId);
  return fetcherIds.get(fetchers);
}

const defaultFetchers: AgentRunStreamFetchers = {
  requestKey: 'pm-agent-run',
  getSnapshot: (workspaceId, runId, signal) => agentService.getRunSnapshot(workspaceId, runId, signal),
  listEvents: (workspaceId, runId, after, signal) => agentService.listRunEvents(workspaceId, runId, after, signal),
};

export interface AgentRunStreamState {
  /** Latest authoritative run snapshot, including status and pause reason. */
  session: CodingSession | null;
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
 * - Reconcile on open, browser recovery, and socket reconnect.
 * - Poll visible active runs at `pollMs`, paused runs at most once a minute.
 *   Automatic reads stop while inactive, hidden, or offline.
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
  const currentUserId = useAuthStore((state) => state.user?.id);
  const [currentPlan, setCurrentPlan] = useState<RunPlanArtifact | null>(null);
  const [session, setSession] = useState<CodingSession | null>(null);
  const [streamState, setStreamState] = useState<CodingSessionStreamState | null>(null);
  const [pendingInteraction, setPendingInteraction] =
    useState<CodingSessionInteraction | null>(null);
  const [loading, setLoading] = useState(false);

  const persistedSeqRef = useRef(0);
  const runtimeSeqRef = useRef(0);
  const eventsRef = useRef<CodingSessionEvent[]>([]);
  const pendingRuntimeEventsRef = useRef<Map<number, CodingSessionEvent>>(new Map());
  const snapshotRef = useRef<CodingSessionStreamSnapshot | null>(null);
  const generationRef = useRef(0);
  const networkAvailable = useDockNetworkActivity();
  const activeRef = useRef(active);
  const requestRef = useRef<StreamRequest | null>(null);
  const queuedRequestRef = useRef<StreamRequest | null>(null);
  const subscribedRef = useRef(false);
  const recoveryBoundaryRef = useRef(currentAgentRunReadVersion());
  useLayoutEffect(() => {
    activeRef.current = active;
    // Capture before passive effects start any shared recovery reads.
    recoveryBoundaryRef.current = currentAgentRunReadVersion();
  });
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

  const flushContiguousRuntimeEvents = useCallback(() => {
    const contiguous: CodingSessionEvent[] = [];
    let nextSequence = runtimeSeqRef.current + 1;
    while (pendingRuntimeEventsRef.current.has(nextSequence)) {
      const event = pendingRuntimeEventsRef.current.get(nextSequence);
      pendingRuntimeEventsRef.current.delete(nextSequence);
      if (event) contiguous.push(event);
      runtimeSeqRef.current = nextSequence;
      nextSequence += 1;
    }
    if (contiguous.length > 0) {
      eventsRef.current = upsertCodingSessionEvents(eventsRef.current, contiguous);
      persistedSeqRef.current = maxPersistedCodingSessionSequence(eventsRef.current);
    }
  }, []);

  const reconcile = useCallback(async (signal: AbortSignal, generation: number, afterRead: number | undefined) => {
    if (!workspaceId || !runId) return;
    const [snap, ev] = await readSharedAgentRun(
      JSON.stringify([currentUserId, workspaceId, runId, fetcherIdentity(fetchers)]),
      persistedSeqRef.current,
      (after, sharedSignal) => Promise.all([
        fetchers.getSnapshot(workspaceId, runId, sharedSignal),
        fetchers.listEvents(workspaceId, runId, after, sharedSignal),
      ]),
      signal,
      afterRead,
    );
    if (generationRef.current !== generation || signal.aborted) return;

    // Dock endpoints resolve the chat's active run. A chat can roll over to
    // a successor while this request is in flight, so never merge a response
    // that identifies a different host run.
    if (typeof snap.data?.id === 'string' && snap.data.id !== runId) return;
    if (snap.data) setSession(snap.data);
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

      // The snapshot is authoritative through its runtime watermark. Discard
      // buffered websocket events it already contains, then append only a
      // contiguous tail. This lets snapshots bridge legitimate gaps caused by
      // runtime events that are not projected to the browser.
      if (terminal) {
        pendingRuntimeEventsRef.current.clear();
      } else {
        for (const sequence of pendingRuntimeEventsRef.current.keys()) {
          if (sequence <= incomingSequence) pendingRuntimeEventsRef.current.delete(sequence);
        }
        flushContiguousRuntimeEvents();
      }
    }

    publishStreamState();
  }, [currentUserId, fetchers, flushContiguousRuntimeEvents, publishStreamState, runId, workspaceId]);

  const refetch = useCallback((automatic = false, boundary: number | null = currentAgentRunReadVersion()): Promise<void> => {
    if (!workspaceId || !runId || (automatic && (!activeRef.current || !isDockNetworkAvailable()))) {
      return Promise.resolve();
    }
    const generation = generationRef.current;
    const afterRead = boundary ?? undefined;
    const queued = queuedRequestRef.current;
    if (queued?.generation === generation) {
      queued.automatic &&= automatic;
      if (afterRead !== undefined) queued.afterRead = Math.max(queued.afterRead ?? -1, afterRead);
      return queued.promise;
    }
    let resolve!: () => void;
    const promise = new Promise<void>((done) => { resolve = done; });
    const request: StreamRequest = {
      generation, automatic, afterRead, controller: new AbortController(), promise, resolve,
    };
    const start = (next: StreamRequest) => {
      if (generationRef.current !== next.generation) {
        next.resolve();
        return;
      }
      if (next.automatic && (!activeRef.current || !isDockNetworkAvailable())) {
        next.resolve();
        setLoading(false);
        return;
      }
      requestRef.current = next;
      setLoading(true);
      void reconcile(next.controller.signal, next.generation, next.afterRead).catch(() => {
        // A later action, recovery event, or fallback retries failed reads.
      }).finally(() => {
        // Each caller waits for its own cycle, not an endless polling loop.
        next.resolve();
        if (requestRef.current !== next) return;
        requestRef.current = null;
        const followup = queuedRequestRef.current;
        queuedRequestRef.current = null;
        if (followup) start(followup);
        else if (generationRef.current === next.generation) setLoading(false);
      });
    };
    if (requestRef.current?.generation === generation) queuedRequestRef.current = request;
    else start(request);
    return promise;
  }, [reconcile, runId, workspaceId]);

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
    if (runtimeV2 && event.sequence_no > 0) {
      if (event.sequence_no > runtimeSeqRef.current + 1) {
        // Do not expose a later text fragment before its missing predecessor.
        // Reconciliation will either supply the missing event or replace this
        // buffer with a snapshot whose watermark covers the gap.
        pendingRuntimeEventsRef.current.set(event.sequence_no, event);
        return true;
      }

      eventsRef.current = upsertCodingSessionEvents(eventsRef.current, [event]);
      persistedSeqRef.current = maxPersistedCodingSessionSequence(eventsRef.current);
      if (event.sequence_no === runtimeSeqRef.current + 1) {
        runtimeSeqRef.current = event.sequence_no;
        flushContiguousRuntimeEvents();
      }
      publishStreamState();
      return false;
    }

    eventsRef.current = upsertCodingSessionEvents(eventsRef.current, [event]);
    persistedSeqRef.current = maxPersistedCodingSessionSequence(eventsRef.current);
    publishStreamState();
    return false;
  }, [flushContiguousRuntimeEvents, publishStreamState, runId]);

  // Reset on workspace, chat fetcher, or run change. The generation token
  // prevents a predecessor request from committing after the new run mounts.
  useEffect(() => {
    generationRef.current += 1;
    requestRef.current?.controller.abort();
    requestRef.current = null;
    queuedRequestRef.current?.resolve();
    queuedRequestRef.current = null;
    subscribedRef.current = false;
    persistedSeqRef.current = 0;
    runtimeSeqRef.current = 0;
    eventsRef.current = [];
    pendingRuntimeEventsRef.current = new Map();
    snapshotRef.current = null;
    clearedInteractionIdsRef.current = new Set();
    setCurrentPlan(null);
    setSession(null);
    setStreamState(null);
    setPendingInteraction(null);
    setLoading(false);
    return () => {
      generationRef.current += 1;
      requestRef.current?.controller.abort();
      requestRef.current = null;
      queuedRequestRef.current?.resolve();
      queuedRequestRef.current = null;
    };
  }, [currentUserId, fetchers, runId, workspaceId]);

  // Initial fetch + WS-driven updates.
  useEffect(() => {
    if (!active || !networkAvailable || !workspaceId || !runId) return;
    void refetch(true, subscribedRef.current ? recoveryBoundaryRef.current : null);
    subscribedRef.current = true;
    let refreshTimer: ReturnType<typeof setTimeout> | null = null;
    let refreshAfterRead = currentAgentRunReadVersion();
    const scheduleReconcile = () => {
      if (!isDockNetworkAvailable()) return;
      // All consumers capture the same boundary when the event arrives, before
      // the first consumer's deferred request can advance the shared version.
      refreshAfterRead = currentAgentRunReadVersion();
      if (refreshTimer) return;
      refreshTimer = setTimeout(() => {
        refreshTimer = null;
        void refetch(true, refreshAfterRead);
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
    const unsubscribeConnection = useSupportPresenceStore.subscribe((state, previous) => {
      if (state.wsConnected && !previous.wsConnected) scheduleReconcile();
    });
    window.addEventListener('focus', scheduleReconcile);
    window.addEventListener('agent_run-updated', onRunUpdated);
    window.addEventListener('coding_session-updated', onRunUpdated);
    window.addEventListener('coding_session_event-created', onSessionEvent);
    return () => {
      if (refreshTimer) clearTimeout(refreshTimer);
      unsubscribeConnection();
      window.removeEventListener('focus', scheduleReconcile);
      window.removeEventListener('agent_run-updated', onRunUpdated);
      window.removeEventListener('coding_session-updated', onRunUpdated);
      window.removeEventListener('coding_session_event-created', onSessionEvent);
    };
  }, [active, networkAvailable, ingestRealtimeEvent, refetch, runId, workspaceId]);

  // Light polling fallback while active.
  const sessionStatus = session?.status;
  useEffect(() => {
    if (!active || !networkAvailable || !workspaceId || !runId || pollMs <= 0) return;
    if (sessionStatus && ['completed', 'failed', 'cancelled'].includes(sessionStatus)) return;
    const id = setInterval(() => {
      // Periodic safety reads can share a bounded read already in progress.
      void refetch(true, null);
    }, sessionStatus === 'paused' ? Math.max(60_000, pollMs) : pollMs);
    return () => clearInterval(id);
  }, [active, networkAvailable, pollMs, refetch, runId, sessionStatus, workspaceId]);

  return { session, currentPlan, streamState, pendingInteraction, loading, refetch, clearPendingInteraction };
}
