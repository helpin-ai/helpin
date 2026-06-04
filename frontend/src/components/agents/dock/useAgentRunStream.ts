import { useCallback, useEffect, useRef, useState } from 'react';
import { agentService } from '@/lib/services/agentService';
import {
  latestPendingCodingSessionInteraction,
} from '@/components/pm/CodingSession/codingSessionUtils';
import { buildCodingSessionStreamState } from '@/components/pm/CodingSession/codingSessionStream';
import type {
  CodingSessionEvent,
  CodingSessionInteraction,
  CodingSessionStreamSnapshot,
  RunPlanArtifact,
} from '@/lib/pmTypes';

interface AgentRunStreamState {
  currentPlan: RunPlanArtifact | null;
  pendingInteraction: CodingSessionInteraction | null;
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
 * - Refetch when a window-level `agent_run-updated` event fires for this run id
 *   (already dispatched by useRealtimeSync from the WebSocket bridge).
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
): AgentRunStreamState {
  const [currentPlan, setCurrentPlan] = useState<RunPlanArtifact | null>(null);
  const [pendingInteraction, setPendingInteraction] =
    useState<CodingSessionInteraction | null>(null);
  const [loading, setLoading] = useState(false);

  const seqRef = useRef(0);
  const eventsRef = useRef<CodingSessionEvent[]>([]);
  const snapshotRef = useRef<CodingSessionStreamSnapshot | null>(null);
  const cancelledRef = useRef(false);
  const clearedInteractionIdsRef = useRef<Set<string>>(new Set());

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
    setLoading(true);
    try {
      const [snap, ev] = await Promise.all([
        agentService.getRunSnapshot(workspaceId, runId),
        agentService.listRunEvents(workspaceId, runId, seqRef.current),
      ]);
      if (cancelledRef.current) return;
      snapshotRef.current = snap.data?.stream_state_snapshot ?? null;

      if (ev.data?.events?.length) {
        eventsRef.current = mergeEvents(eventsRef.current, ev.data.events);
        seqRef.current = Math.max(seqRef.current, ev.data.next_sequence_no ?? 0);
      }

      // Reconcile via the same builder the coding-session drawer uses, so the
      // plan resolves whether it came through stream_state_snapshot.current_plan
      // (codex/opencode) or from update_plan tool calls in events (native_sdk).
      const built = buildCodingSessionStreamState(eventsRef.current, snapshotRef.current);
      const latestPending = latestPendingCodingSessionInteraction(eventsRef.current);
      setCurrentPlan(built.current_plan);
      setPendingInteraction(
        latestPending && clearedInteractionIdsRef.current.has(latestPending.interaction_id)
          ? null
          : latestPending,
      );
    } finally {
      if (!cancelledRef.current) setLoading(false);
    }
  }, [runId, workspaceId]);

  // Reset on run change.
  useEffect(() => {
    cancelledRef.current = false;
    seqRef.current = 0;
    eventsRef.current = [];
    snapshotRef.current = null;
    clearedInteractionIdsRef.current = new Set();
    setCurrentPlan(null);
    setPendingInteraction(null);
    return () => {
      cancelledRef.current = true;
    };
  }, [runId]);

  // Initial fetch + WS-driven refresh.
  useEffect(() => {
    if (!active || !workspaceId || !runId) return;
    void refetch();
    const handler = (event: Event) => {
      const detail = (event as CustomEvent<{ entity_id?: string }>).detail;
      if (detail?.entity_id !== runId) return;
      void refetch();
    };
    window.addEventListener('agent_run-updated', handler);
    window.addEventListener('coding_session-updated', handler);
    window.addEventListener('coding_session_event-created', handler);
    return () => {
      window.removeEventListener('agent_run-updated', handler);
      window.removeEventListener('coding_session-updated', handler);
      window.removeEventListener('coding_session_event-created', handler);
    };
  }, [active, refetch, runId, workspaceId]);

  // Light polling fallback while active.
  useEffect(() => {
    if (!active || !workspaceId || !runId || pollMs <= 0) return;
    const id = setInterval(() => {
      void refetch();
    }, pollMs);
    return () => clearInterval(id);
  }, [active, pollMs, refetch, runId, workspaceId]);

  return { currentPlan, pendingInteraction, loading, refetch, clearPendingInteraction };
}

function mergeEvents(
  prev: CodingSessionEvent[],
  next: CodingSessionEvent[],
): CodingSessionEvent[] {
  if (prev.length === 0) return next;
  const seen = new Set(prev.map((e) => e.id));
  const merged = prev.slice();
  for (const e of next) {
    if (seen.has(e.id)) continue;
    merged.push(e);
    seen.add(e.id);
  }
  return merged;
}
