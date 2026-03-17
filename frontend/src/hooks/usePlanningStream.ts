import { useEffect, useRef, useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { useWSStore, type WSEvent } from './useWebSocket';
import { queryKeys } from '@/lib/queryKeys';
import type { PlanningStreamEvent, ToolInvocation } from '@/lib/pmTypes';

export function usePlanningStream(wsId: string, sessionId: string | undefined) {
  const [isStreaming, setIsStreaming] = useState(false);
  const [turnPending, setTurnPending] = useState(false);
  const [activeToolCall, setActiveToolCall] = useState<{ tool_name: string } | null>(null);
  const [toolResults, setToolResults] = useState<ToolInvocation[]>([]);
  const [error, setError] = useState<string | null>(null);
  const send = useWSStore((s) => s.send);
  const qc = useQueryClient();
  const receivedAnyEvent = useRef(false);
  const seenEventIds = useRef<string[]>([]);

  // Mark that we're waiting for the agent to respond (bridges the gap
  // between sending a message and the first stream token arriving).
  const markTurnPending = () => setTurnPending(true);

  const settleTurn = () => {
    if (!sessionId) return;
    qc.invalidateQueries({ queryKey: queryKeys.pm.planningMessages(wsId, sessionId) });
    qc.invalidateQueries({ queryKey: queryKeys.pm.planningSession(wsId, sessionId) });
    setTimeout(() => {
      setIsStreaming(false);
      setTurnPending(false);
      setToolResults([]);
      setActiveToolCall(null);
    }, 300);
  };

  const isDuplicateEvent = (eventId?: string) => {
    if (!eventId) return false;
    if (seenEventIds.current.includes(eventId)) {
      return true;
    }
    seenEventIds.current.push(eventId);
    if (seenEventIds.current.length > 200) {
      seenEventIds.current = seenEventIds.current.slice(-200);
    }
    return false;
  };

  useEffect(() => {
    seenEventIds.current = [];
  }, [sessionId]);

  // Subscribe on mount, unsubscribe on unmount.
  // Also re-subscribe when `send` changes (WS reconnect).
  useEffect(() => {
    if (!sessionId || !send) return;
    send({ type: 'subscribe_session', session_id: sessionId });
    return () => {
      // Only unsubscribe if the socket is still live (send hasn't been nulled).
      send({ type: 'unsubscribe_session', session_id: sessionId });
    };
  }, [sessionId, send]);

  // Listen for stream events
  useEffect(() => {
    if (!sessionId) return;
    const handler = (e: Event) => {
      const event = (e as CustomEvent<PlanningStreamEvent>).detail;
      if (event.session_id !== sessionId) return;
      if (isDuplicateEvent(event.event_id)) return;
      receivedAnyEvent.current = true;

      switch (event.type) {
        case 'assistant_message_started':
          setIsStreaming(true);
          setTurnPending(false);
          setError(null);
          break;
        case 'assistant_message_delta':
          setIsStreaming(true);
          setTurnPending(false);
          setError(null);
          break;
        case 'tool_call_started':
          setIsStreaming(true);
          setTurnPending(false);
          setActiveToolCall({ tool_name: event.tool_name ?? '' });
          break;
        case 'tool_call_finished':
          setActiveToolCall(null);
          setToolResults((prev) => [
            ...prev,
            {
              tool_name: event.tool_name ?? '',
              input: {},
              output_summary: event.output_summary ?? '',
              duration_ms: event.duration_ms ?? 0,
            },
          ]);
          break;
        case 'assistant_message_completed':
          setIsStreaming(true);
          break;
        case 'turn_completed': {
          settleTurn();
          break;
        }
        case 'error':
          setIsStreaming(false);
          setTurnPending(false);
          setError(event.error ?? 'Unknown error');
          break;
      }
    };
    window.addEventListener('planning-stream', handler);
    return () => window.removeEventListener('planning-stream', handler);
  }, [sessionId, settleTurn]);

  useEffect(() => {
    if (!sessionId) return;
    const handler = (e: Event) => {
      const event = (e as CustomEvent<WSEvent>).detail;
      if (event.parent_id !== sessionId) return;
      if (isDuplicateEvent(event.event_id)) return;
      if (event.data?.role !== 'assistant') return;
      settleTurn();
    };
    window.addEventListener('planning-session-message', handler);
    return () => window.removeEventListener('planning-session-message', handler);
  }, [sessionId, settleTurn]);

  return { isStreaming, turnPending, markTurnPending, activeToolCall, toolResults, error };
}
