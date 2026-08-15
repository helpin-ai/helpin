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
  /** Whether the included runtime timeline is actively streaming. Defaults to includeLive. */
  runtimeActive?: boolean;
  /** Which kinds to emit. Omit for all kinds. */
  include?: ReadonlySet<TranscriptSegmentKind>;
  /**
   * Leading run-context message (persisted/developer/system prompt) the slider derives
   * from the prompt artifact. Rendered first when `context` is in scope.
   */
  leadingContext?: CodingSessionTranscriptMessage | null;
  /** Keep only the latest assistant prose within each user/decision interval. */
  compactAssistantProgress?: boolean;
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

function canonicalizeJSON(value: unknown): unknown {
  if (Array.isArray(value)) return value.map(canonicalizeJSON);
  if (!value || typeof value !== 'object') return value;
  return Object.fromEntries(
    Object.entries(value as Record<string, unknown>)
      .sort(([left], [right]) => left.localeCompare(right))
      .map(([key, entry]) => [key, canonicalizeJSON(entry)]),
  );
}

function normalizedToolArgs(argsText: string): string {
  const trimmed = argsText.trim();
  if (!trimmed) return '';
  try {
    return JSON.stringify(canonicalizeJSON(JSON.parse(trimmed)));
  } catch {
    return trimmed;
  }
}

/** Semantic fallback for legacy projections that did not preserve tool-call IDs. */
function toolCallSemanticKey(toolCall: CodingSessionLiveToolCall): string {
  return [
    canonicalToolName(toolCall.tool_name).toLowerCase(),
    normalizedToolArgs(toolCall.args_text),
  ].join('\n');
}

function incrementCount(counts: Map<string, number>, key: string): void {
  counts.set(key, (counts.get(key) ?? 0) + 1);
}

function compactAssistantProgress(segments: TranscriptSegment[]): TranscriptSegment[] {
  const keep = new Array<boolean>(segments.length).fill(true);
  let latestAssistantIndex: number | null = null;

  for (let index = 0; index < segments.length; index += 1) {
    const segment = segments[index];
    if (segment.kind === 'user' || segment.kind === 'review_decision') {
      latestAssistantIndex = null;
      continue;
    }
    if (segment.kind !== 'assistant') continue;
    if (latestAssistantIndex !== null) keep[latestAssistantIndex] = false;
    latestAssistantIndex = index;
  }

  return segments.filter((_, index) => keep[index]);
}

function settleInactiveRuntimeTools(
  segments: TranscriptSegment[],
  runtimeActive: boolean,
): TranscriptSegment[] {
  if (runtimeActive) return segments;
  return segments.map((segment) => (
    segment.kind === 'tool' && segment.toolCall.status === 'running'
      ? { ...segment, toolCall: { ...segment.toolCall, status: 'completed' } }
      : segment
  ));
}

function maybeCompactAssistantProgress(
  segments: TranscriptSegment[],
  enabled: boolean | undefined,
): TranscriptSegment[] {
  return enabled ? compactAssistantProgress(segments) : segments;
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
  const runtimeActive = opts.runtimeActive ?? opts.includeLive;
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
  const emittedPersistedToolIDs = new Set<string>();
  let activeStreamingAssistantSegmentId: string | null = null;

  const pushPersistedAssistant = (segment: Extract<TranscriptSegment, { kind: 'assistant' }>) => {
    persistedAssistantSegments.push({ outIndex: out.length, segment });
    out.push(segment);
  };
  const pushPersistedTool = (segment: Extract<TranscriptSegment, { kind: 'tool' }>) => {
    const toolCallID = segment.toolCall.tool_call_id.trim();
    if (toolCallID && emittedPersistedToolIDs.has(toolCallID)) return;
    if (toolCallID) emittedPersistedToolIDs.add(toolCallID);
    persistedToolSegments.push({ outIndex: out.length, segment });
    out.push(segment);
  };

  // Reconciled terminal payloads can contain both a message-level aggregate
  // tool list and the authoritative interleaved turn timeline. Pre-index the
  // timeline before rendering so an aggregate message that sorts earlier does
  // not briefly become a duplicate tool block at the top of the transcript.
  const latestTimelineOwnerByToolID = new Map<string, string>();
  for (let messageIndex = 0; messageIndex < stream.transcript_messages.length; messageIndex += 1) {
    const message = stream.transcript_messages[messageIndex];
    for (let segmentIndex = 0; segmentIndex < (message.turn_segments?.length ?? 0); segmentIndex += 1) {
      const segment = message.turn_segments?.[segmentIndex];
      if (segment?.kind !== 'tool_call' || isToolName(segment.tool_call.tool_name, 'update_plan')) continue;
      const toolCallID = segment.tool_call.tool_call_id.trim();
      if (toolCallID) latestTimelineOwnerByToolID.set(toolCallID, `${messageIndex}:${segmentIndex}`);
    }
  }

  const persistedTimelineToolIDs = new Set<string>();
  const persistedTimelineSemanticCounts = new Map<string, number>();
  for (let messageIndex = 0; messageIndex < stream.transcript_messages.length; messageIndex += 1) {
    const message = stream.transcript_messages[messageIndex];
    for (let segmentIndex = 0; segmentIndex < (message.turn_segments?.length ?? 0); segmentIndex += 1) {
      const segment = message.turn_segments?.[segmentIndex];
      if (segment?.kind !== 'tool_call' || isToolName(segment.tool_call.tool_name, 'update_plan')) continue;
      const toolCallID = segment.tool_call.tool_call_id.trim();
      if (toolCallID && latestTimelineOwnerByToolID.get(toolCallID) !== `${messageIndex}:${segmentIndex}`) continue;
      if (toolCallID) persistedTimelineToolIDs.add(toolCallID);
      incrementCount(persistedTimelineSemanticCounts, toolCallSemanticKey(segment.tool_call));
    }
  }
  const matchedFallbackSemanticCounts = new Map<string, number>();

  if (opts.includeLive && runtimeActive) {
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

  for (let messageIndex = 0; messageIndex < stream.transcript_messages.length; messageIndex += 1) {
    const message = stream.transcript_messages[messageIndex];
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
      for (let segmentIndex = 0; segmentIndex < segments.length; segmentIndex += 1) {
        const segment = segments[segmentIndex];
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
          const toolCallID = segment.tool_call.tool_call_id.trim();
          if (toolCallID && latestTimelineOwnerByToolID.get(toolCallID) !== `${messageIndex}:${segmentIndex}`) continue;
          if (include.has('tool')) pushPersistedTool({ kind: 'tool', id: segment.segment_id, toolCall: segment.tool_call });
        }
      }
      // Persisted tool calls absent from the segment timeline (e.g. apply_patch
      // recorded only on the message) still render once, deduped by content.
      if (include.has('tool')) {
        for (const toolCall of message.tool_calls ?? []) {
          if (isToolName(toolCall.tool_name, 'update_plan')) continue;
          const toolCallID = toolCall.tool_call_id.trim();
          if (toolCallID && persistedTimelineToolIDs.has(toolCallID)) continue;
          const semanticKey = toolCallSemanticKey(toolCall);
          const matchedCount = matchedFallbackSemanticCounts.get(semanticKey) ?? 0;
          if (matchedCount < (persistedTimelineSemanticCounts.get(semanticKey) ?? 0)) {
            matchedFallbackSemanticCounts.set(semanticKey, matchedCount + 1);
            continue;
          }
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
        const toolCallID = toolCall.tool_call_id.trim();
        if (toolCallID && persistedTimelineToolIDs.has(toolCallID)) continue;
        const semanticKey = toolCallSemanticKey(toolCall);
        const matchedCount = matchedFallbackSemanticCounts.get(semanticKey) ?? 0;
        if (matchedCount < (persistedTimelineSemanticCounts.get(semanticKey) ?? 0)) {
          matchedFallbackSemanticCounts.set(semanticKey, matchedCount + 1);
          continue;
        }
        pushPersistedTool({ kind: 'tool', id: toolCall.tool_call_id, toolCall });
      }
    }
  }

  if (opts.includeLive) {
    // Indexes hoisted out of `out` and re-emitted at their live position.
    const matchedPersistedIndexes = new Set<number>();
    // Indexes already represented (hoisted OR settled) — excluded from
    // further live matching either way.
    const consumedPersistedIndexes = new Set<number>();
    // The live timeline accumulates across every turn of a long-lived chat
    // run, but it only carries assistant/tool segments — never user
    // messages. Hoisting all matched segments to the tail would therefore
    // stack every user message above the whole assistant history. Segments
    // from turns that already ended (before the last user message) are
    // settled: they keep their transcript position and their live
    // counterparts are dropped. Only the current turn's tail follows the
    // live timeline's ordering.
    let lastConversationBoundaryIndex = -1;
    for (let index = out.length - 1; index >= 0; index -= 1) {
      const kind = out[index].kind;
      if (kind === 'user' || kind === 'review_decision') {
        lastConversationBoundaryIndex = index;
        break;
      }
    }
    const liveOut: TranscriptSegment[] = [];
    if (stream.live_reasoning_message && include.has('reasoning')) {
      liveOut.push({
        kind: 'reasoning',
        id: `live-reasoning:${stream.live_reasoning_message.message_id}`,
        reasoning: runtimeActive || stream.live_reasoning_message.status !== 'streaming'
          ? stream.live_reasoning_message
          : { ...stream.live_reasoning_message, status: 'completed' },
      });
    }
    for (const segment of stream.live_turn_segments) {
      if (segment.kind === 'assistant_message') {
        const content = segment.assistant_message.content.trim();
        if (!content || !include.has('assistant')) continue;

        const persisted = persistedAssistantSegments.find((candidate) => (
          !consumedPersistedIndexes.has(candidate.outIndex)
          && candidate.segment.messageId === segment.assistant_message.message_id
        ));
        if (persisted) {
          consumedPersistedIndexes.add(persisted.outIndex);
          if (persisted.outIndex < lastConversationBoundaryIndex) continue;
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
          !consumedPersistedIndexes.has(candidate.outIndex)
          && candidate.segment.toolCall.tool_call_id === segment.tool_call.tool_call_id
        )) ?? persistedToolSegments.find((candidate) => (
          !consumedPersistedIndexes.has(candidate.outIndex)
          && toolCallSemanticKey(candidate.segment.toolCall) === toolCallSemanticKey(segment.tool_call)
        ));
        if (persisted) {
          consumedPersistedIndexes.add(persisted.outIndex);
          if (persisted.outIndex < lastConversationBoundaryIndex) continue;
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
    return maybeCompactAssistantProgress(settleInactiveRuntimeTools([
      ...out.filter((_, index) => !matchedPersistedIndexes.has(index)),
      ...liveOut,
    ], runtimeActive), opts.compactAssistantProgress);
  }

  return maybeCompactAssistantProgress(out, opts.compactAssistantProgress);
}

/** True when the stream has at least one renderable segment in scope. */
export function hasRenderableSegments(
  stream: TranscriptStreamInput | null,
  opts: CollectSegmentsOptions,
): boolean {
  if (!stream) return false;
  return collectSegments(stream, opts).length > 0;
}
