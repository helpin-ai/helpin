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
  return [message.event_id, message.message_id, message.client_message_id && `client:${message.client_message_id}`]
    .filter((value): value is string => !!value);
}

function persistedIdentityKeys(message: AgentRunMessage): string[] {
  return [message.id, message.runtime_message_id, `msg:${message.id}`, message.client_message_id && `client:${message.client_message_id}`]
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

export interface PendingDockChatMessage {
  id: string;
  content: string;
  timestamp: string;
  actor_user_id?: string;
}

/** Compact history replaces the raw progress previously received while streaming. */
function reconcileWorkSummaries(messages: AgentRunMessage[]): AgentRunMessage[] {
  const sequence = (message: AgentRunMessage) => message.dock_chat_sequence ?? message.sequence_no;
  const ordered = [...messages].sort((left, right) => sequence(left) - sequence(right)
    || Number(!!right.dock_work_summary) - Number(!!left.dock_work_summary));
  const covered = new Set<string>();
  for (const [index, summary] of ordered.entries()) {
    if (!summary.dock_work_summary) continue;
    const finalID = summary.id.startsWith('work:') ? summary.id.slice(5) : summary.dock_work_summary.message_id;
    const target = ordered.find(message => message.id === summary.dock_work_summary?.message_id);
    const end = Math.max(sequence(summary), target ? sequence(target) : sequence(summary));
    const boundary = findLastMatchingIndex(ordered.slice(0, index), message => message.role !== 'assistant');
    const summaryTime = timestamp(summary.created_at);
    const startTime = summaryTime === null ? null : summaryTime - summary.dock_work_summary.duration_ms;
    for (const message of ordered.slice(boundary + 1)) {
      if (sequence(message) > end) break;
      if (message.role !== 'assistant' || message.dock_work_summary || message.id === finalID
        || message.message_type === 'assistant_final' || message.message_type === 'status'
        || message.run_id !== summary.run_id || message.dock_chat_id !== summary.dock_chat_id) continue;
      // A page may start mid-turn. Without its user boundary, require proof
      // that this progress belongs to the summary's time interval.
      const time = timestamp(message.created_at);
      if (boundary < 0 && (startTime === null || time === null || time < startTime)) continue;
      covered.add(message.id);
    }
  }
  return ordered.filter(message => !covered.has(message.id));
}

/** Merge stable chat rows with only the genuinely newer tail of a runtime snapshot. */
export function mergePersistedChatMessages(
  stream: CodingSessionStreamState | null,
  messages: AgentRunMessage[],
  options: { pendingMessage?: PendingDockChatMessage | null; failedClientMessageIds?: ReadonlySet<string> } = {},
): CodingSessionStreamState | null {
  if (!stream && messages.length === 0 && !options.pendingMessage) return null;
  const isVisible = (message: Pick<CodingSessionTranscriptMessage, 'client_message_id' | 'delivery_status'>) => (
    message.delivery_status !== 'failed'
    && (message.delivery_status === 'sent'
      || !(message.client_message_id && options.failedClientMessageIds?.has(message.client_message_id)))
  );

  const orderedMessages = reconcileWorkSummaries(messages.filter(isVisible));
  const persisted = orderedMessages
    .filter((message) => message.role === 'user' || message.role === 'assistant' || message.message_type === 'status')
    .map((message) => ({
      event_id: `msg:${message.id}`,
      message_id: message.runtime_message_id || message.id,
      client_message_id: message.client_message_id,
      delivery_status: message.delivery_status,
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
    .filter(isVisible)
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
    if (segment.kind === 'assistant_message' && segment.assistant_message.message_type === 'assistant_final'
      && segment.assistant_message.message_id === stream?.turn_state?.answer_message_id) return true;
    if (segmentIdentityKeys(segment).some((key) => persistedSegmentKeys.has(key) || persistedMessageKeys.has(key))) return true;
    if (persisted.length === 0 || segmentIsActive(segment)) return true;
    if (segment.segment_id === timestampLessCompletionFallbackID) return true;
    const value = segmentStartedAt(segment);
    return latestPersistedTime !== null && value !== null && value > latestPersistedTime;
  });

  // The same submitted turn keeps its React identity when its websocket event
  // arrives before the HTTP acknowledgement or the saved message page.
  const transcriptMessages: CodingSessionTranscriptMessage[] = [...persisted, ...extras].map((message) => (
    message.role === 'user' && message.client_message_id
      ? { ...message, event_id: `client:${message.client_message_id}`,
          ...(message.client_message_id === options.pendingMessage?.id && message.delivery_status !== 'sent'
            ? { delivery_status: 'pending' as const } : {}) }
      : message
  ));
  const pending = resolveVisiblePendingEcho(options.pendingMessage ?? null, transcriptMessages);
  if (pending) {
    transcriptMessages.push({
      event_id: `client:${pending.id}`, client_message_id: pending.id,
      role: 'user', content: pending.content, timestamp: pending.timestamp,
      message_type: 'user_reply', delivery_status: 'pending', actor_user_id: pending.actor_user_id,
      sequence_no: transcriptMessages.reduce((maximum, message) => Math.max(maximum, message.sequence_no), 0) + 1,
    });
  }

  return {
    transcript_messages: transcriptMessages,
    ...(stream?.turn_state ? { turn_state: stream.turn_state } : {}),
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
  return reconcileWorkSummaries([...messages.values()]);
}

/** Never paint an optimistic user bubble beside its accepted durable row. */
export function resolveVisiblePendingEcho<T extends { id: string }>(
  pending: T | null,
  messages: Pick<AgentRunMessage, 'client_message_id'>[],
): T | null {
  if (!pending) return null;
  return messages.some((message) => message.client_message_id === pending.id) ? null : pending;
}
