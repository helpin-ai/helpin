import type { TranscriptSegment, TranscriptStreamInput } from './segments';

export function transcriptSegmentTimes(stream: TranscriptStreamInput): Map<string, number> {
  const times = new Map<string, number>();
  const remember = (id: string | undefined, value: string | undefined) => {
    if (!id || !value) return;
    const timestamp = Date.parse(value);
    if (Number.isFinite(timestamp)) times.set(id, timestamp);
  };

  for (const message of stream.transcript_messages) {
    remember(message.event_id, message.timestamp);
    remember(message.message_id, message.timestamp);
    for (const segment of message.turn_segments ?? []) {
      remember(segment.segment_id, message.timestamp);
      if (segment.kind === 'assistant_message') {
        remember(segment.assistant_message.message_id, segment.assistant_message.started_at ?? message.timestamp);
      } else {
        remember(segment.tool_call.tool_call_id, segment.tool_call.started_at ?? message.timestamp);
      }
    }
  }
  for (const segment of stream.live_turn_segments) {
    if (segment.kind === 'assistant_message') {
      remember(segment.segment_id, segment.assistant_message.started_at);
      remember(segment.assistant_message.message_id, segment.assistant_message.started_at);
    } else {
      remember(segment.segment_id, segment.tool_call.started_at);
      remember(segment.tool_call.tool_call_id, segment.tool_call.started_at);
    }
  }
  return times;
}

export function segmentTimestamp(segment: TranscriptSegment, times: Map<string, number>): number | null {
  if (segment.kind === 'user' || segment.kind === 'status' || segment.kind === 'context' || segment.kind === 'review_decision') {
    const timestamp = Date.parse(segment.message.timestamp);
    return Number.isFinite(timestamp) ? timestamp : null;
  }
  if (segment.kind === 'assistant') {
    return times.get(segment.id) ?? (segment.messageId ? times.get(segment.messageId) : undefined) ?? null;
  }
  if (segment.kind === 'tool') {
    const timestamp = Date.parse(segment.toolCall.started_at ?? '');
    return Number.isFinite(timestamp) ? timestamp : times.get(segment.id) ?? times.get(segment.toolCall.tool_call_id) ?? null;
  }
  const timestamp = Date.parse(segment.reasoning.started_at ?? '');
  return Number.isFinite(timestamp) ? timestamp : times.get(segment.id) ?? null;
}
