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
  | { kind: 'assistant'; id: string; messageId?: string; content: string; streaming?: boolean }
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
   * Leading run-context message (persisted/developer/system prompt) the slider derives
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

/** The dock stays lean while preserving conversational chronology. */
export const DOCK_SEGMENT_KINDS: ReadonlySet<TranscriptSegmentKind> = new Set<TranscriptSegmentKind>([
  'assistant',
  'tool',
  'user',
]);

function isContextMessage(message: CodingSessionTranscriptMessage): boolean {
  return (
    (message.message_type === 'prompt' && message.role !== 'user')
    || message.message_type === 'developer_prompt'
    || message.message_type === 'system_prompt'
  );
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
  const persistedAssistantSegments: Array<{
    outIndex: number;
    segment: Extract<TranscriptSegment, { kind: 'assistant' }>;
  }> = [];
  const persistedToolSegments: Array<{
    outIndex: number;
    segment: Extract<TranscriptSegment, { kind: 'tool' }>;
  }> = [];
  let activeStreamingAssistantSegmentId: string | null = null;

  const pushPersistedAssistant = (segment: Extract<TranscriptSegment, { kind: 'assistant' }>) => {
    persistedAssistantSegments.push({ outIndex: out.length, segment });
    out.push(segment);
  };
  const pushPersistedTool = (segment: Extract<TranscriptSegment, { kind: 'tool' }>) => {
    persistedToolSegments.push({ outIndex: out.length, segment });
    out.push(segment);
  };

  if (opts.includeLive) {
    for (let index = stream.live_turn_segments.length - 1; index >= 0; index -= 1) {
      const segment = stream.live_turn_segments[index];
      if (segment.kind === 'assistant_message') {
        if (!include.has('assistant')) continue;
        if (segment.assistant_message.content.trim() && segment.assistant_message.status === 'streaming') {
          activeStreamingAssistantSegmentId = segment.segment_id;
        }
        break;
      }
      if (!include.has('tool') || isToolName(segment.tool_call.tool_name, 'update_plan')) continue;
      break;
    }
  }

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
    // Runtime-backed chat messages use message_type="prompt" for the user's
    // submitted turn. isContextMessage deliberately treats that combination
    // as conversation, while developer/system prompts and review decisions
    // retain their specialized presentation.
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
            pushPersistedAssistant({
              kind: 'assistant',
              id: segment.segment_id,
              messageId: segment.assistant_message.message_id || message.message_id,
              content,
            });
          }
        } else if (segment.kind === 'tool_call' && !isToolName(segment.tool_call.tool_name, 'update_plan')) {
          seenToolKeys.add(toolCallTimelineKey(segment.tool_call));
          if (include.has('tool')) pushPersistedTool({ kind: 'tool', id: segment.segment_id, toolCall: segment.tool_call });
        }
      }
      // Persisted tool calls absent from the segment timeline (e.g. apply_patch
      // recorded only on the message) still render once, deduped by content.
      if (include.has('tool')) {
        for (const toolCall of message.tool_calls ?? []) {
          if (isToolName(toolCall.tool_name, 'update_plan')) continue;
          if (seenToolKeys.has(toolCallTimelineKey(toolCall))) continue;
          pushPersistedTool({ kind: 'tool', id: toolCall.tool_call_id, toolCall });
        }
      }
      continue;
    }

    if (message.content.trim() && include.has('assistant')) {
      const content = message.content.trim();
      pushPersistedAssistant({ kind: 'assistant', id: message.event_id, messageId: message.message_id, content });
    }
    if (include.has('tool')) {
      for (const toolCall of message.tool_calls ?? []) {
        if (isToolName(toolCall.tool_name, 'update_plan')) continue;
        pushPersistedTool({ kind: 'tool', id: toolCall.tool_call_id, toolCall });
      }
    }
  }

  if (opts.includeLive) {
    const matchedPersistedIndexes = new Set<number>();
    const liveOut: TranscriptSegment[] = [];
    if (stream.live_reasoning_message && include.has('reasoning')) {
      liveOut.push({
        kind: 'reasoning',
        id: `live-reasoning:${stream.live_reasoning_message.message_id}`,
        reasoning: stream.live_reasoning_message,
      });
    }
    for (const segment of stream.live_turn_segments) {
      if (segment.kind === 'assistant_message') {
        const content = segment.assistant_message.content.trim();
        if (!content || !include.has('assistant')) continue;

        const persisted = persistedAssistantSegments.find((candidate) => (
          !matchedPersistedIndexes.has(candidate.outIndex)
          && candidate.segment.messageId === segment.assistant_message.message_id
        ));
        if (persisted) {
          matchedPersistedIndexes.add(persisted.outIndex);
          // The persisted body is authoritative and complete, but the live
          // timeline supplies its correct position among tool calls.
          liveOut.push(persisted.segment);
          continue;
        }

        liveOut.push({
          kind: 'assistant',
          id: `live:${segment.segment_id}`,
          messageId: segment.assistant_message.message_id,
          content,
          streaming: segment.segment_id === activeStreamingAssistantSegmentId,
        });
      } else if (segment.kind === 'tool_call' && !isToolName(segment.tool_call.tool_name, 'update_plan')) {
        if (!include.has('tool')) continue;
        const persisted = persistedToolSegments.find((candidate) => (
          !matchedPersistedIndexes.has(candidate.outIndex)
          && candidate.segment.toolCall.tool_call_id === segment.tool_call.tool_call_id
        ));
        if (persisted) {
          matchedPersistedIndexes.add(persisted.outIndex);
          liveOut.push(persisted.segment);
        } else {
          liveOut.push({ kind: 'tool', id: `live:${segment.segment_id}`, toolCall: segment.tool_call });
        }
      }
    }

    // A cumulative live snapshot is the best ordering source while a run is
    // active. Remove persisted rows represented by that timeline, then insert
    // their authoritative content at the corresponding live positions.
    return [
      ...out.filter((_, index) => !matchedPersistedIndexes.has(index)),
      ...liveOut,
    ];
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
