import type {
  AgentRunMessage,
  CodingSessionLiveTurnSegment,
  CodingSessionStreamState,
  CodingSessionTranscriptMessage,
} from '@/lib/pmTypes';
import { findLastMatchingIndex } from './findLastMatchingIndex';

function timestamp(value: string | undefined): number | null {
  const parsed = Date.parse(value ?? '');
  return Number.isFinite(parsed) ? parsed : null;
}

function transcriptIdentityKeys(message: CodingSessionTranscriptMessage): string[] {
  return [message.event_id, message.message_id].filter((value): value is string => !!value);
}

function persistedIdentityKeys(message: AgentRunMessage): string[] {
  return [message.id, message.runtime_message_id, `msg:${message.id}`]
    .filter((value): value is string => !!value);
}

function segmentIdentityKeys(segment: CodingSessionLiveTurnSegment): string[] {
  if (segment.kind === 'assistant_message') {
    return [segment.segment_id, segment.assistant_message.message_id]
      .filter((value): value is string => !!value);
  }
  return [segment.segment_id, segment.tool_call.tool_call_id]
    .filter((value): value is string => !!value);
}

function segmentStartedAt(segment: CodingSessionLiveTurnSegment): number | null {
  return timestamp(segment.kind === 'assistant_message'
    ? segment.assistant_message.started_at
    : segment.tool_call.started_at);
}

function segmentIsActive(segment: CodingSessionLiveTurnSegment): boolean {
  return segment.kind === 'assistant_message'
    ? segment.assistant_message.status === 'streaming'
    : segment.tool_call.status === 'running';
}

function isConversationBoundary(message: CodingSessionTranscriptMessage): boolean {
  return message.role === 'user'
    || message.message_type === 'review_checkpoint_resolution'
    || message.message_type === 'approval_request_resolution';
}

/**
 * A retained snapshot may order completed work only when it fully represents
 * the durable current interval. Partial cumulative snapshots must not reorder
 * otherwise stable history.
 */
export function hasAuthoritativeDockRuntimeTimeline(stream: CodingSessionStreamState): boolean {
  const runtimeAssistantIDs = new Set<string>();
  const runtimeToolIDs = new Set<string>();
  for (const segment of stream.live_turn_segments) {
    if (segment.kind === 'assistant_message') {
      const id = segment.assistant_message.message_id.trim();
      if (id) runtimeAssistantIDs.add(id);
    } else {
      const id = segment.tool_call.tool_call_id.trim();
      if (id) runtimeToolIDs.add(id);
    }
  }
  if (runtimeAssistantIDs.size === 0 && runtimeToolIDs.size === 0) return false;

  const lastBoundary = findLastMatchingIndex(stream.transcript_messages, isConversationBoundary);
  const interval = stream.transcript_messages.slice(lastBoundary + 1);
  const durableAssistantIDs = new Set<string>();
  const durableToolIDs = new Set<string>();
  for (const message of interval) {
    if (message.role !== 'assistant') continue;
    const messageID = message.message_id?.trim();
    if (messageID) durableAssistantIDs.add(messageID);
    for (const segment of message.turn_segments ?? []) {
      if (segment.kind === 'assistant_message') {
        const id = segment.assistant_message.message_id.trim();
        if (id) durableAssistantIDs.add(id);
      } else {
        const id = segment.tool_call.tool_call_id.trim();
        if (id) durableToolIDs.add(id);
      }
    }
    for (const toolCall of message.tool_calls ?? []) {
      const id = toolCall.tool_call_id.trim();
      if (id) durableToolIDs.add(id);
    }
  }

  return [...durableAssistantIDs].every((id) => runtimeAssistantIDs.has(id))
    && [...durableToolIDs].every((id) => runtimeToolIDs.has(id));
}

/** Merge stable chat rows with only the genuinely newer tail of a runtime snapshot. */
export function mergePersistedChatMessages(
  stream: CodingSessionStreamState | null,
  messages: AgentRunMessage[],
): CodingSessionStreamState | null {
  if (!stream && messages.length === 0) return null;

  const orderedMessages = [...messages].sort(
    (left, right) => (left.dock_chat_sequence ?? left.sequence_no) - (right.dock_chat_sequence ?? right.sequence_no),
  );
  const persisted = orderedMessages
    .filter((message) => message.role === 'user' || message.role === 'assistant' || message.message_type === 'status')
    .map((message) => ({
      event_id: `msg:${message.id}`,
      message_id: message.runtime_message_id || message.id,
      role: message.role as 'user' | 'assistant',
      content: message.content,
      message_type: message.message_type,
      timestamp: message.created_at,
      sequence_no: message.dock_chat_sequence ?? message.sequence_no,
      actor_user_id: message.actor_user_id,
      dock_work_summary: message.dock_work_summary,
      turn_segments: message.role === 'assistant' ? message.turn_segments : undefined,
    }));

  const persistedMessageKeys = new Set(orderedMessages.flatMap(persistedIdentityKeys));
  const persistedSegmentKeys = new Set(
    orderedMessages.flatMap((message) => (message.turn_segments ?? []).flatMap(segmentIdentityKeys)),
  );
  const latestPersistedTime = orderedMessages.reduce<number | null>((latest, message) => {
    const value = timestamp(message.created_at);
    if (value === null) return latest;
    return latest === null ? value : Math.max(latest, value);
  }, null);
  const maxSequence = persisted.reduce((maximum, message) => Math.max(maximum, message.sequence_no), 0);
  const lastUserIndex = findLastMatchingIndex(orderedMessages, (message) => message.role === 'user');
  const currentIntervalHasDurableAssistant = orderedMessages
    .slice(lastUserIndex + 1)
    .some((message) => message.role === 'assistant');
  const timestampLessCompletionFallbackID = currentIntervalHasDurableAssistant
    ? null
    : [...(stream?.live_turn_segments ?? [])].reverse().find((segment) => (
      segment.kind === 'assistant_message'
      && segment.assistant_message.status === 'completed'
      && segmentStartedAt(segment) === null
    ))?.segment_id ?? null;

  // Runtime snapshots may contain the entire chat, while the durable endpoint
  // intentionally returns only the newest page. Never append old snapshot
  // history after that page. Snapshot messages are admitted only when they can
  // be proven to be newer than the durable tail.
  const extras = (stream?.transcript_messages ?? [])
    .filter((message) => !transcriptIdentityKeys(message).some((key) => persistedMessageKeys.has(key)))
    .filter((message) => {
      if (persisted.length === 0) return true;
      const value = timestamp(message.timestamp);
      return latestPersistedTime !== null && value !== null && value > latestPersistedTime;
    })
    .sort((left, right) => {
      const leftTime = timestamp(left.timestamp);
      const rightTime = timestamp(right.timestamp);
      if (leftTime !== null && rightTime !== null && leftTime !== rightTime) return leftTime - rightTime;
      return left.sequence_no - right.sequence_no;
    })
    .map((message, index) => ({ ...message, sequence_no: maxSequence + index + 1 }));

  // Paused runs can retain a cumulative live timeline. Keep segments already
  // represented by the durable page for tool/text interleaving, plus active or
  // provably newer tail segments; discard unmatched historical leftovers.
  const liveTurnSegments = (stream?.live_turn_segments ?? []).filter((segment) => {
    if (segmentIdentityKeys(segment).some((key) => persistedSegmentKeys.has(key) || persistedMessageKeys.has(key))) return true;
    if (persisted.length === 0 || segmentIsActive(segment)) return true;
    if (segment.segment_id === timestampLessCompletionFallbackID) return true;
    const value = segmentStartedAt(segment);
    return latestPersistedTime !== null && value !== null && value > latestPersistedTime;
  });

  return {
    transcript_messages: [...persisted, ...extras],
    live_assistant_message: stream?.live_assistant_message ?? null,
    live_reasoning_message: stream?.live_reasoning_message ?? null,
    live_turn_segments: liveTurnSegments,
    activity_events: stream?.activity_events ?? [],
    current_plan: stream?.current_plan ?? null,
    completed_tool_calls: stream?.completed_tool_calls ?? [],
  };
}

export function mergeMessagePages(current: AgentRunMessage[], incoming: AgentRunMessage[]) {
  const messages = new Map(current.map((message) => [message.id, message]));
  for (const message of incoming) messages.set(message.id, message);
  return [...messages.values()].sort(
    (left, right) => (left.dock_chat_sequence ?? left.sequence_no) - (right.dock_chat_sequence ?? right.sequence_no),
  );
}

/** Never paint an optimistic user bubble beside its accepted durable row. */
export function resolveVisiblePendingEcho<T extends { id: string }>(
  pending: T | null,
  messages: Pick<AgentRunMessage, 'client_message_id'>[],
): T | null {
  if (!pending) return null;
  return messages.some((message) => message.client_message_id === pending.id) ? null : pending;
}
