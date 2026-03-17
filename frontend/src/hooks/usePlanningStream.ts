import { useEffect, useRef, useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { useWSStore } from './useWebSocket';
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

  // Mark that we're waiting for the agent to respond (bridges the gap
  // between sending a message and the first stream token arriving).
  const markTurnPending = () => setTurnPending(true);

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
      receivedAnyEvent.current = true;

      switch (event.type) {
        case 'token':
          setIsStreaming(true);
          setTurnPending(false);
          setError(null);
          break;
        case 'tool_start':
          setIsStreaming(true);
          setTurnPending(false);
          setActiveToolCall({ tool_name: event.tool_name ?? '' });
          break;
        case 'tool_result':
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
        case 'turn_complete': {
          // Invalidate queries first, then clear streaming state after a short delay
          // so the persisted message has time to load before the streaming bubble disappears.
          qc.invalidateQueries({ queryKey: queryKeys.pm.planningMessages(wsId, sessionId) });
          qc.invalidateQueries({ queryKey: queryKeys.pm.planningSession(wsId, sessionId) });
          // Delay clearing to avoid blank flash between streaming and persisted message
          setTimeout(() => {
            setIsStreaming(false);
            setTurnPending(false);
            setToolResults([]);
            setActiveToolCall(null);
          }, 500);
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
  }, [sessionId, wsId, qc]);

  return { isStreaming, turnPending, markTurnPending, activeToolCall, toolResults, error };
}
