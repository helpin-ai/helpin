import { isStatusTranscriptMessage } from '@/components/pm/CodingSession/codingSessionPresentation';
import { canonicalToolName, isToolName } from '@/lib/toolNames';
import type {
  CodingSessionLiveReasoningMessage,
  CodingSessionLiveToolCall,
  CodingSessionStreamState,
  CodingSessionTranscriptMessage,
} from '@/lib/pmTypes';

/**
 * A normalized, ordered transcript event. Both the Ask Agents dock and the
 * coding-session slider flatten a reconciled run stream into this single union
 * so they can share one row kit. Assistant text and tool calls are emitted as
 * individual segments (interleaved in turn order); reasoning/status/context/
 * user/review-decision are surface-scoped extras.
 */
export type TranscriptSegment =
  | { kind: 'assistant'; id: string; content: string; streaming?: boolean }
  | { kind: 'tool'; id: string; toolCall: CodingSessionLiveToolCall }
  | { kind: 'reasoning'; id: string; reasoning: CodingSessionLiveReasoningMessage }
  | { kind: 'status'; id: string; message: CodingSessionTranscriptMessage }
  | { kind: 'context'; id: string; message: CodingSessionTranscriptMessage }
  | { kind: 'user'; id: string; message: CodingSessionTranscriptMessage }
  | { kind: 'review_decision'; id: string; message: CodingSessionTranscriptMessage };

export type TranscriptSegmentKind = TranscriptSegment['kind'];

/** Only the stream fields the collector reads — lets the slider pass a partial. */
export type TranscriptStreamInput = Pick<
  CodingSessionStreamState,
  'transcript_messages' | 'live_turn_segments' | 'live_reasoning_message'
>;

export interface CollectSegmentsOptions {
  /** Include in-flight live turn segments + live reasoning (run still active). */
  includeLive: boolean;
  /** Which kinds to emit. Omit for all kinds. */
  include?: ReadonlySet<TranscriptSegmentKind>;
  /**
   * Leading run-context message (developer/system prompt) the slider derives
   * from the prompt artifact. Rendered first when `context` is in scope.
   */
  leadingContext?: CodingSessionTranscriptMessage | null;
}

/** Every renderable kind — the slider's scope. */
export const ALL_SEGMENT_KINDS: ReadonlySet<TranscriptSegmentKind> = new Set<TranscriptSegmentKind>([
  'assistant',
  'tool',
  'reasoning',
  'status',
  'context',
  'user',
  'review_decision',
]);

/** The dock stays lean: assistant replies + tool calls only. */
export const DOCK_SEGMENT_KINDS: ReadonlySet<TranscriptSegmentKind> = new Set<TranscriptSegmentKind>([
  'assistant',
  'tool',
]);

function isContextMessage(message: CodingSessionTranscriptMessage): boolean {
  return message.message_type === 'developer_prompt' || message.message_type === 'system_prompt';
}

function isReviewDecisionMessage(message: CodingSessionTranscriptMessage): boolean {
  return (
    message.message_type === 'review_checkpoint_resolution'
    || message.message_type === 'approval_request_resolution'
  );
}

/** Identity key for deduping a persisted tool call against the turn timeline. */
function toolCallTimelineKey(toolCall: CodingSessionLiveToolCall): string {
  return [
    canonicalToolName(toolCall.tool_name).toLowerCase(),
    toolCall.args_text.trim(),
    toolCall.result?.output_summary?.trim() ?? '',
    toolCall.result?.content?.trim() ?? '',
  ].join('\n');
}

/**
 * Flatten a reconciled run stream into an ordered segment list.
 *
 * Completed turns come from `transcript_messages` (preferring their
 * `turn_segments` so text and tool calls stay interleaved). While active we
 * append `live_reasoning_message` + `live_turn_segments`; once terminal those
 * are already folded into the transcript, so callers pass `includeLive: false`.
 */
export function collectSegments(
  stream: TranscriptStreamInput,
  opts: CollectSegmentsOptions,
): TranscriptSegment[] {
  const include = opts.include ?? ALL_SEGMENT_KINDS;
  const out: TranscriptSegment[] = [];

  if (opts.leadingContext && include.has('context')) {
    out.push({ kind: 'context', id: `context:${opts.leadingContext.event_id}`, message: opts.leadingContext });
  }

  for (const message of stream.transcript_messages) {
    if (isStatusTranscriptMessage(message)) {
      if (include.has('status')) out.push({ kind: 'status', id: message.event_id, message });
      continue;
    }
    if (isContextMessage(message)) {
      if (include.has('context')) out.push({ kind: 'context', id: message.event_id, message });
      continue;
    }
    if (isReviewDecisionMessage(message)) {
      if (include.has('review_decision') && message.content.trim()) {
        out.push({ kind: 'review_decision', id: message.event_id, message });
      }
      continue;
    }
    if (message.role === 'user') {
      if (include.has('user') && message.content.trim()) {
        out.push({ kind: 'user', id: message.event_id, message });
      }
      continue;
    }
    if (message.role !== 'assistant') continue;

    const segments = message.turn_segments?.length ? message.turn_segments : null;
    if (segments) {
      const seenToolKeys = new Set<string>();
      for (const segment of segments) {
        if (segment.kind === 'assistant_message') {
          const content = segment.assistant_message.content.trim();
          if (content && include.has('assistant')) {
            out.push({ kind: 'assistant', id: segment.segment_id, content });
          }
        } else if (segment.kind === 'tool_call' && !isToolName(segment.tool_call.tool_name, 'update_plan')) {
          seenToolKeys.add(toolCallTimelineKey(segment.tool_call));
          if (include.has('tool')) out.push({ kind: 'tool', id: segment.segment_id, toolCall: segment.tool_call });
        }
      }
      // Persisted tool calls absent from the segment timeline (e.g. apply_patch
      // recorded only on the message) still render once, deduped by content.
      if (include.has('tool')) {
        for (const toolCall of message.tool_calls ?? []) {
          if (isToolName(toolCall.tool_name, 'update_plan')) continue;
          if (seenToolKeys.has(toolCallTimelineKey(toolCall))) continue;
          out.push({ kind: 'tool', id: toolCall.tool_call_id, toolCall });
        }
      }
      continue;
    }

    if (message.content.trim() && include.has('assistant')) {
      out.push({ kind: 'assistant', id: message.event_id, content: message.content.trim() });
    }
    if (include.has('tool')) {
      for (const toolCall of message.tool_calls ?? []) {
        if (isToolName(toolCall.tool_name, 'update_plan')) continue;
        out.push({ kind: 'tool', id: toolCall.tool_call_id, toolCall });
      }
    }
  }

  if (opts.includeLive) {
    if (stream.live_reasoning_message && include.has('reasoning')) {
      out.push({
        kind: 'reasoning',
        id: `live-reasoning:${stream.live_reasoning_message.message_id}`,
        reasoning: stream.live_reasoning_message,
      });
    }
    for (const segment of stream.live_turn_segments) {
      if (segment.kind === 'assistant_message') {
        const content = segment.assistant_message.content.trim();
        if (content && include.has('assistant')) {
          out.push({
            kind: 'assistant',
            id: `live:${segment.segment_id}`,
            content,
            streaming: segment.assistant_message.status === 'streaming',
          });
        }
      } else if (segment.kind === 'tool_call' && !isToolName(segment.tool_call.tool_name, 'update_plan')) {
        if (include.has('tool')) out.push({ kind: 'tool', id: `live:${segment.segment_id}`, toolCall: segment.tool_call });
      }
    }
  }

  return out;
}

/** True when the stream has at least one renderable segment in scope. */
export function hasRenderableSegments(
  stream: TranscriptStreamInput | null,
  opts: CollectSegmentsOptions,
): boolean {
  if (!stream) return false;
  return collectSegments(stream, opts).length > 0;
}
