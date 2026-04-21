import type {
  CodingSessionEvent,
  CodingSessionLiveAssistantMessage,
  CodingSessionLiveReasoningMessage,
  CodingSessionLiveToolCall,
  CodingSessionLiveTurnSegment,
  CodingSessionStreamSnapshot,
  CodingSessionStreamState,
  CodingSessionTranscriptMessage,
  RunPlanArtifact,
} from '@/lib/pmTypes';
import { sortCodingSessionEvents } from './codingSessionUtils';

function asRecord(value: unknown): Record<string, unknown> | null {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return null;
  return value as Record<string, unknown>;
}

function asString(value: unknown) {
  return typeof value === 'string' && value.trim().length > 0 ? value : undefined;
}

function asNumber(value: unknown) {
  return typeof value === 'number' && Number.isFinite(value) ? value : undefined;
}

function isPersistedRunMessageEvent(event: CodingSessionEvent) {
  return event.runtime_metadata?.source === 'agent_run_message';
}

function isTranscriptMessageEvent(event: CodingSessionEvent) {
  return (
    isPersistedRunMessageEvent(event)
    && (event.type === 'assistant.message.completed' || event.type === 'user.message.completed')
  );
}

function isStreamingTurnEventType(type: string) {
  return (
    type.startsWith('assistant.message.')
    || type.startsWith('reasoning.message.')
    || type.startsWith('tool.call.')
  );
}

function isSidecarActivityEvent(event: CodingSessionEvent) {
  if (isStreamingTurnEventType(event.type)) return false;
  if (event.type === 'user.message.completed' || event.type === 'assistant.message.completed') return false;
  if (event.type === 'plan.updated') return false;
  return true;
}

function firstNonEmptyString(...values: Array<string | undefined>) {
  return values.find((value) => typeof value === 'string' && value.trim().length > 0);
}

function stringifyToolInput(value: unknown): string | undefined {
  if (typeof value === 'string' && value.trim().length > 0) return value;
  if (value == null) return undefined;
  try {
    const serialized = JSON.stringify(value, null, 2);
    return typeof serialized === 'string' && serialized.trim().length > 0 ? serialized : undefined;
  } catch {
    return undefined;
  }
}

function isRunPlanStep(value: unknown): value is RunPlanArtifact['plan'][number] {
  const record = asRecord(value);
  if (!record) return false;
  return (
    typeof record.step === 'string'
    && (record.status === 'pending' || record.status === 'in_progress' || record.status === 'completed')
  );
}

function isRunPlanArtifact(value: unknown): value is RunPlanArtifact {
  const record = asRecord(value);
  if (!record) return false;
  if ('note' in record && record.note != null && typeof record.note !== 'string') return false;
  return Array.isArray(record.plan) && record.plan.every(isRunPlanStep);
}

function ensureAssistantMessage(
  current: CodingSessionLiveAssistantMessage | null,
  messageID: string,
  timestamp: string,
) {
  if (current && current.message_id === messageID) {
    if (!current.started_at) current.started_at = timestamp;
    return current;
  }
  return {
    message_id: messageID,
    content: '',
    started_at: timestamp,
    status: 'streaming',
    tool_calls: [],
  } satisfies CodingSessionLiveAssistantMessage;
}

function ensureReasoningMessage(
  current: CodingSessionLiveReasoningMessage | null,
  messageID: string,
  timestamp: string,
) {
  if (current && current.message_id === messageID) {
    if (!current.started_at) current.started_at = timestamp;
    return current;
  }
  return {
    message_id: messageID,
    content: '',
    started_at: timestamp,
    status: 'streaming',
  } satisfies CodingSessionLiveReasoningMessage;
}

function ensureToolCall(
  assistant: CodingSessionLiveAssistantMessage,
  payload: Record<string, unknown>,
  timestamp: string,
) {
  const toolCallID = asString(payload.tool_call_id) ?? `${assistant.message_id}:tool:${assistant.tool_calls.length + 1}`;
  const existing = assistant.tool_calls.find((toolCall) => toolCall.tool_call_id === toolCallID);
  if (existing) return existing;

  const nextToolCall: CodingSessionLiveToolCall = {
    tool_call_id: toolCallID,
    parent_message_id: firstNonEmptyString(asString(payload.parent_message_id), assistant.message_id),
    tool_name: asString(payload.tool_name) ?? 'tool',
    args_text: firstNonEmptyString(
      asString(payload.args_text),
      asString(payload.tool_input),
      stringifyToolInput(payload.tool_input),
      stringifyToolInput(payload.input),
    ) ?? '',
    status: 'running',
    started_at: timestamp,
  };
  assistant.tool_calls.push(nextToolCall);
  return nextToolCall;
}

function mergeToolArgs(currentArgs: string, payload: Record<string, unknown>) {
  const argsText = asString(payload.args_text);
  if (argsText) return argsText;
  const argsDelta = asString(payload.args_delta);
  if (!argsDelta) return currentArgs;
  return `${currentArgs}${argsDelta}`;
}

function cloneToolCall(toolCall: CodingSessionLiveToolCall): CodingSessionLiveToolCall {
  return {
    ...toolCall,
    result: toolCall.result ? { ...toolCall.result } : undefined,
  };
}

function cloneAssistantMessage(message?: CodingSessionLiveAssistantMessage | null) {
  if (!message) return null;
  return {
    ...message,
    tool_calls: (message.tool_calls ?? []).map(cloneToolCall),
  } satisfies CodingSessionLiveAssistantMessage;
}

function cloneReasoningMessage(message?: CodingSessionLiveReasoningMessage | null) {
  if (!message) return null;
  return { ...message } satisfies CodingSessionLiveReasoningMessage;
}

function cloneLiveTurnSegment(segment: CodingSessionLiveTurnSegment): CodingSessionLiveTurnSegment {
  if (segment.kind === 'assistant_message') {
    return {
      ...segment,
      assistant_message: cloneAssistantMessage(segment.assistant_message)!,
    };
  }
  return {
    ...segment,
    tool_call: cloneToolCall(segment.tool_call),
  };
}

function cloneLiveTurnSegments(segments?: CodingSessionLiveTurnSegment[] | null) {
  return (segments ?? []).map(cloneLiveTurnSegment);
}

function parseLiveToolCall(value: unknown): CodingSessionLiveToolCall | null {
  const payload = asRecord(value);
  if (!payload) return null;

  const toolCallID = asString(payload.tool_call_id);
  if (!toolCallID) return null;

  return {
    tool_call_id: toolCallID,
    parent_message_id: asString(payload.parent_message_id),
    tool_name: asString(payload.tool_name) ?? 'tool',
    args_text: firstNonEmptyString(
      asString(payload.args_text),
      asString(payload.tool_input),
      stringifyToolInput(payload.tool_input),
      stringifyToolInput(payload.input),
    ) ?? '',
    status: (asString(payload.status) as CodingSessionLiveToolCall['status']) ?? 'completed',
    duration_ms: asNumber(payload.duration_ms),
    started_at: asString(payload.started_at),
    completed_at: asString(payload.completed_at),
    result: asRecord(payload.result) ? {
      message_id: asString(asRecord(payload.result)?.message_id),
      content: firstNonEmptyString(
        asString(asRecord(payload.result)?.content),
        asString(asRecord(payload.result)?.output_summary),
      ) ?? '',
      output_summary: asString(asRecord(payload.result)?.output_summary),
      error: asString(asRecord(payload.result)?.error),
    } : undefined,
  };
}

function parseLiveAssistantMessage(value: unknown): CodingSessionLiveAssistantMessage | null {
  const payload = asRecord(value);
  const messageID = asString(payload?.message_id);
  if (!payload || !messageID) return null;

  return {
    message_id: messageID,
    content: typeof payload.content === 'string' ? payload.content : '',
    started_at: asString(payload.started_at),
    completed_at: asString(payload.completed_at),
    status: (asString(payload.status) as CodingSessionLiveAssistantMessage['status']) ?? 'completed',
    tool_calls: Array.isArray(payload.tool_calls)
      ? payload.tool_calls.map(parseLiveToolCall).filter((toolCall): toolCall is CodingSessionLiveToolCall => Boolean(toolCall))
      : [],
  };
}

function parseLiveTurnSegments(value: unknown): CodingSessionLiveTurnSegment[] {
  if (!Array.isArray(value)) return [];

  const segments: CodingSessionLiveTurnSegment[] = [];
  for (const rawSegment of value) {
    const segment = asRecord(rawSegment);
    const segmentID = asString(segment?.segment_id);
    const kind = asString(segment?.kind);
    if (!segment || !segmentID || !kind) continue;

    if (kind === 'assistant_message') {
      const assistantMessage = parseLiveAssistantMessage(segment.assistant_message);
      if (!assistantMessage) continue;
      segments.push({
        segment_id: segmentID,
        kind: 'assistant_message',
        assistant_message: assistantMessage,
      });
      continue;
    }

    if (kind === 'tool_call') {
      const toolCall = parseLiveToolCall(segment.tool_call);
      if (!toolCall) continue;
      segments.push({
        segment_id: segmentID,
        kind: 'tool_call',
        tool_call: toolCall,
      });
    }
  }
  return segments;
}

function nextAssistantSegmentID(segments: CodingSessionLiveTurnSegment[], messageID: string) {
  const count = segments.filter((segment) => (
    segment.kind === 'assistant_message' && segment.assistant_message.message_id === messageID
  )).length;
  return `${messageID}:segment:${count + 1}`;
}

function latestAssistantSegment(
  segments: CodingSessionLiveTurnSegment[],
  messageID: string,
) {
  for (let index = segments.length - 1; index >= 0; index -= 1) {
    const segment = segments[index];
    if (segment?.kind !== 'assistant_message') continue;
    if (segment.assistant_message.message_id === messageID) return segment.assistant_message;
  }
  return null;
}

function appendAssistantSegment(
  segments: CodingSessionLiveTurnSegment[],
  messageID: string,
  content: string,
  timestamp: string,
) {
  if (!content) return null;
  const lastSegment = segments[segments.length - 1];
  if (lastSegment?.kind === 'assistant_message' && lastSegment.assistant_message.message_id === messageID) {
    lastSegment.assistant_message.content += content;
    lastSegment.assistant_message.status = 'streaming';
    if (!lastSegment.assistant_message.started_at) {
      lastSegment.assistant_message.started_at = timestamp;
    }
    return lastSegment.assistant_message;
  }

  const assistantSegment: CodingSessionLiveAssistantMessage = {
    message_id: messageID,
    content,
    started_at: timestamp,
    status: 'streaming',
    tool_calls: [],
  };
  segments.push({
    segment_id: nextAssistantSegmentID(segments, messageID),
    kind: 'assistant_message',
    assistant_message: assistantSegment,
  });
  return assistantSegment;
}

function deriveAssistantSegmentDelta(previousContent: string, fullContent: string) {
  if (!fullContent) return null;
  if (!previousContent) return fullContent;
  if (fullContent.startsWith(previousContent)) return fullContent.slice(previousContent.length);
  return null;
}

function appendOrMarkCompletedAssistantSegment(
  segments: CodingSessionLiveTurnSegment[],
  messageID: string,
  previousContent: string,
  fullContent: string,
  timestamp: string,
) {
  const delta = deriveAssistantSegmentDelta(previousContent, fullContent);
  if (typeof delta === 'string' && delta.length > 0) {
    const segment = appendAssistantSegment(segments, messageID, delta, timestamp);
    if (segment) {
      segment.status = 'completed';
      segment.completed_at = timestamp;
    }
    return;
  }

  const segment = latestAssistantSegment(segments, messageID);
  if (segment) {
    segment.status = 'completed';
    segment.completed_at = timestamp;
    return;
  }

  if (!fullContent.trim()) return;
  const fallback = appendAssistantSegment(segments, messageID, fullContent, timestamp);
  if (fallback) {
    fallback.status = 'completed';
    fallback.completed_at = timestamp;
  }
}

function ensureToolCallSegment(
  segments: CodingSessionLiveTurnSegment[],
  payload: Record<string, unknown>,
  timestamp: string,
  parentMessageID: string,
) {
  const toolCallID = asString(payload.tool_call_id) ?? `${parentMessageID}:tool:1`;
  const existing = segments.find((segment) => segment.kind === 'tool_call' && segment.tool_call.tool_call_id === toolCallID);
  if (existing?.kind === 'tool_call') {
    if (!existing.tool_call.started_at) existing.tool_call.started_at = timestamp;
    existing.tool_call.parent_message_id = firstNonEmptyString(
      asString(payload.parent_message_id),
      existing.tool_call.parent_message_id,
      parentMessageID,
    );
    existing.tool_call.tool_name = firstNonEmptyString(asString(payload.tool_name), existing.tool_call.tool_name) ?? 'tool';
    return existing.tool_call;
  }

  const toolCall: CodingSessionLiveToolCall = {
    tool_call_id: toolCallID,
    parent_message_id: firstNonEmptyString(asString(payload.parent_message_id), parentMessageID),
    tool_name: asString(payload.tool_name) ?? 'tool',
    args_text: firstNonEmptyString(
      asString(payload.args_text),
      asString(payload.tool_input),
      stringifyToolInput(payload.tool_input),
      stringifyToolInput(payload.input),
    ) ?? '',
    status: 'running',
    started_at: timestamp,
  };
  segments.push({
    segment_id: toolCallID,
    kind: 'tool_call',
    tool_call: toolCall,
  });
  return toolCall;
}

function transcriptToolCallsFromPayload(payload: Record<string, unknown>, messageID: string) {
  const invocations = Array.isArray(payload.tool_invocations) ? payload.tool_invocations : [];
  const toolCalls = invocations.flatMap((value, index) => {
    const invocation = asRecord(value);
    if (!invocation) return [];

    const outputSummary = asString(invocation.output_summary) ?? '';
    const toolName = asString(invocation.tool_name) ?? 'tool';
    const argsText = stringifyToolInput(invocation.input) ?? '';

    return [{
      tool_call_id: `${messageID}:tool:${index + 1}`,
      parent_message_id: messageID,
      tool_name: toolName,
      args_text: argsText,
      status: 'completed',
      result: {
        content: outputSummary,
        output_summary: outputSummary,
      },
      duration_ms: asNumber(invocation.duration_ms),
    } satisfies CodingSessionLiveToolCall];
  });

  return toolCalls.length > 0 ? toolCalls : undefined;
}

function transcriptMessageFromEvent(event: CodingSessionEvent): CodingSessionTranscriptMessage | null {
  if (isTranscriptMessageEvent(event)) {
    const payload = asRecord(event.payload) ?? {};
    const role = event.type === 'user.message.completed' ? 'user' : 'assistant';

    return {
      event_id: event.id,
      message_id: asString(payload.message_id),
      role,
      content: firstNonEmptyString(asString(payload.content), asString(payload.text)) ?? '',
      message_type: asString(payload.message_type),
      timestamp: event.timestamp,
      sequence_no: event.sequence_no,
      tool_calls: role === 'assistant'
        ? transcriptToolCallsFromPayload(payload, asString(payload.message_id) ?? event.id)
        : undefined,
      turn_segments: role === 'assistant' ? parseLiveTurnSegments(payload.turn_segments) : undefined,
    };
  }
  const interactionTranscript = transcriptInteractionResolutionMessageFromEvent(event);
  if (interactionTranscript) return interactionTranscript;
  return null;
}

function transcriptInteractionResolutionMessageFromEvent(event: CodingSessionEvent): CodingSessionTranscriptMessage | null {
  if (event.type !== 'interaction.resolved') return null;
  const payload = asRecord(event.payload) ?? {};
  if (asString(payload.interaction_kind) !== 'review_checkpoint') return null;

  const content = reviewCheckpointResolutionTranscriptContent(
    asRecord(payload.request_payload),
    asRecord(payload.response_payload),
  );
  if (!content) return null;

  return {
    event_id: event.id,
    message_id: asString(payload.interaction_id) ?? event.id,
    role: 'user',
    content,
    message_type: 'review_checkpoint_resolution',
    timestamp: event.timestamp,
    sequence_no: event.sequence_no,
  };
}

function reviewCheckpointResolutionTranscriptContent(
  requestPayload: Record<string, unknown> | null,
  responsePayload: Record<string, unknown> | null,
) {
  if (!responsePayload) return '';
  const decision = asString(responsePayload.decision);
  if (!decision) return '';
  const selectionMode = (asString(responsePayload.selection_mode) ?? '').toLowerCase();
  const note = asString(responsePayload.message);
  const findings = Array.isArray(requestPayload?.findings) ? requestPayload.findings : [];
  const selectedFindingIDs = Array.isArray(responsePayload.selected_finding_ids)
    ? responsePayload.selected_finding_ids.flatMap((value) => {
      const id = asString(value);
      return id ? [id] : [];
    })
    : [];

  const normalizedFindings = findings.flatMap((rawFinding) => {
    const finding = asRecord(rawFinding);
    const id = asString(finding?.id);
    const title = asString(finding?.title);
    if (!id || !title) return [];
    return [{
      id,
      title,
      codeLocation: asString(finding?.code_location),
    }];
  });

  const selectedFindings = selectionMode === 'selected'
    ? normalizedFindings.filter((finding) => selectedFindingIDs.includes(finding.id))
    : normalizedFindings;

  const lines: string[] = [];
  if (decision === 'approve') {
    if (selectedFindings.length > 0) {
      lines.push(selectionMode === 'selected'
        ? 'Approved selected review findings for implementation:'
        : 'Approved all review findings for implementation:');
      lines.push(...selectedFindings.map((finding) => (
        finding.codeLocation ? `- ${finding.title} \`${finding.codeLocation}\`` : `- ${finding.title}`
      )));
    } else {
      lines.push('Approved the review checkpoint.');
    }
  } else {
    if (selectedFindings.length > 0) {
      lines.push(selectionMode === 'selected'
        ? 'Requested changes on selected review findings:'
        : 'Requested changes on the review findings:');
      lines.push(...selectedFindings.map((finding) => (
        finding.codeLocation ? `- ${finding.title} \`${finding.codeLocation}\`` : `- ${finding.title}`
      )));
    } else {
      lines.push('Requested changes on the review checkpoint.');
    }
  }

  if (note) {
    lines.push('');
    lines.push(`Note: ${note}`);
  }
  return lines.join('\n');
}

function liveAssistantMatchesTranscript(
  liveAssistant: CodingSessionLiveAssistantMessage,
  transcriptMessages: CodingSessionTranscriptMessage[],
) {
  if (liveAssistant.status !== 'completed') return false;
  const liveContent = liveAssistant.content.trim();
  if (!liveContent) return false;

  return transcriptMessages.some((message) => (
    message.role === 'assistant'
    && message.timestamp >= (liveAssistant.started_at ?? '')
    && message.content.trim() === liveContent
  ));
}

function parsePlanArtifact(argsText: string): RunPlanArtifact | null {
  try {
    const parsed = JSON.parse(argsText) as unknown;
    return isRunPlanArtifact(parsed) ? parsed : null;
  } catch {
    return null;
  }
}

const TASK_PLAN_DOC_PUBLISH_TOOLS = new Set(['publish_task_plan_doc', 'publish_story_plan_doc']);
const REVIEW_CHECKPOINT_TOOLS = new Set(['request_review_checkpoint', 'request_human_approval']);

function parsePlanArtifactValue(value: unknown): RunPlanArtifact | null {
  if (typeof value === 'string') return parsePlanArtifact(value);
  return isRunPlanArtifact(value) ? value : null;
}

function extractPlanFromLiveTurnSegments(
  segments: CodingSessionLiveTurnSegment[],
): RunPlanArtifact | null {
  let currentPlan: RunPlanArtifact | null = null;
  for (const segment of segments) {
    if (segment.kind !== 'tool_call') continue;
    if (segment.tool_call.tool_name !== 'update_plan') continue;
    if (!segment.tool_call.args_text) continue;
    const parsed = parsePlanArtifact(segment.tool_call.args_text);
    if (parsed) currentPlan = parsed;
  }
  return currentPlan;
}

function extractPlanFromEvents(events: CodingSessionEvent[]): RunPlanArtifact | null {
  let currentPlan: RunPlanArtifact | null = null;
  for (const event of events) {
    if (event.type !== 'plan.updated' && event.type !== 'activity.updated') continue;
    const payload = asRecord(event.payload);
    const parsed = parsePlanArtifactValue(payload?.content);
    if (parsed) currentPlan = parsed;
  }
  return currentPlan;
}

function normalizePlanStepText(value: string) {
  return value.toLowerCase().replace(/\s+/g, ' ').trim();
}

function isTaskPlanDocumentStep(stepText: string) {
  return (
    stepText.includes('planning document')
    || stepText.includes('task plan doc')
    || stepText.includes('task planning doc')
    || stepText.includes('story plan doc')
    || stepText.includes('plan document')
  );
}

function isDraftPlanDocumentStep(stepText: string) {
  if (!isTaskPlanDocumentStep(stepText)) return false;
  return stepText.includes('draft') || (stepText.includes('publish') && !stepText.includes('review') && !stepText.includes('approval'));
}

function isReviewStep(stepText: string) {
  if (!(stepText.includes('review') || stepText.includes('approval'))) return false;
  return stepText.includes('publish') || stepText.includes('request') || stepText.includes('checkpoint') || stepText.includes('wait');
}

function hasReviewCheckpointEvent(events: CodingSessionEvent[]) {
  return events.some((event) => {
    if (event.type === 'approval.requested') return true;
    if (!event.type.startsWith('interaction.')) return false;
    const payload = asRecord(event.payload);
    return asString(payload?.interaction_kind) === 'review_checkpoint';
  });
}

function reconcileObservedPlanState(
  plan: RunPlanArtifact | null,
  toolCalls: CodingSessionLiveToolCall[],
  events: CodingSessionEvent[],
): RunPlanArtifact | null {
  if (!plan || plan.plan.length === 0) return plan;

  const completedToolNames = new Set(
    toolCalls
      .filter((toolCall) => toolCall.status === 'completed')
      .map((toolCall) => toolCall.tool_name.toLowerCase()),
  );

  const publishedTaskPlanDoc = [...TASK_PLAN_DOC_PUBLISH_TOOLS].some((toolName) => completedToolNames.has(toolName));
  const requestedReviewCheckpoint = [...REVIEW_CHECKPOINT_TOOLS].some((toolName) => completedToolNames.has(toolName))
    || hasReviewCheckpointEvent(events);

  if (!publishedTaskPlanDoc && !requestedReviewCheckpoint) return plan;

  let changed = false;
  const nextPlan: RunPlanArtifact = {
    ...plan,
    plan: plan.plan.map((step) => {
      const normalizedStep = normalizePlanStepText(step.step);
      if (publishedTaskPlanDoc && step.status !== 'completed' && isDraftPlanDocumentStep(normalizedStep)) {
        changed = true;
        return { ...step, status: 'completed' };
      }
      if (requestedReviewCheckpoint && step.status !== 'completed' && isReviewStep(normalizedStep)) {
        changed = true;
        return { ...step, status: 'completed' };
      }
      return step;
    }),
  };

  return changed ? nextPlan : plan;
}

function extractPlanAndToolCalls(
  transcriptMessages: CodingSessionTranscriptMessage[],
  liveAssistant: CodingSessionLiveAssistantMessage | null,
  liveTurnSegments: CodingSessionLiveTurnSegment[],
): { currentPlan: RunPlanArtifact | null; completedToolCalls: CodingSessionLiveToolCall[]; allToolCalls: CodingSessionLiveToolCall[] } {
  const allToolCalls: CodingSessionLiveToolCall[] = [];

  for (const message of transcriptMessages) {
    if (message.tool_calls) {
      allToolCalls.push(...message.tool_calls);
    }
  }
  if (liveAssistant) {
    allToolCalls.push(...liveAssistant.tool_calls);
  }

  // Dedupe by tool_call_id (keep last occurrence)
  const seen = new Map<string, CodingSessionLiveToolCall>();
  for (const tc of allToolCalls) {
    seen.set(tc.tool_call_id, tc);
  }
  const deduped = Array.from(seen.values());

  // Extract latest plan
  let currentPlan: RunPlanArtifact | null = null;
  for (const tc of deduped) {
    if (tc.tool_name === 'update_plan' && tc.args_text) {
      const parsed = parsePlanArtifact(tc.args_text);
      if (parsed) currentPlan = parsed;
    }
  }
  currentPlan ??= extractPlanFromLiveTurnSegments(liveTurnSegments);

  // Collect completed non-plan tool calls sorted by completion time
  const completedToolCalls = deduped
    .filter((tc) => tc.tool_name !== 'update_plan' && (tc.status === 'completed' || tc.status === 'failed'))
    .sort((a, b) => {
      const ta = a.completed_at ?? a.started_at ?? '';
      const tb = b.completed_at ?? b.started_at ?? '';
      return ta < tb ? -1 : ta > tb ? 1 : 0;
    });

  return { currentPlan, completedToolCalls, allToolCalls: deduped };
}

export function buildCodingSessionStreamState(
  events: CodingSessionEvent[],
  snapshot?: CodingSessionStreamSnapshot | null,
): CodingSessionStreamState {
  const sortedEvents = sortCodingSessionEvents(events);
  const transcriptMessages: CodingSessionTranscriptMessage[] = [];
  const activityEvents: CodingSessionEvent[] = [];
  let liveAssistantMessage = cloneAssistantMessage(snapshot?.live_assistant_message);
  let liveReasoningMessage = cloneReasoningMessage(snapshot?.live_reasoning_message);
  const liveTurnSegments = cloneLiveTurnSegments(snapshot?.live_turn_segments);
  let currentPlanLive: RunPlanArtifact | null = parsePlanArtifactValue(snapshot?.current_plan);

  if (liveTurnSegments.length === 0 && liveAssistantMessage) {
    if (liveAssistantMessage.content) {
      liveTurnSegments.push({
        segment_id: nextAssistantSegmentID([], liveAssistantMessage.message_id),
        kind: 'assistant_message',
        assistant_message: {
          ...cloneAssistantMessage(liveAssistantMessage)!,
          tool_calls: [],
        },
      });
    }
    for (const toolCall of liveAssistantMessage.tool_calls) {
      liveTurnSegments.push({
        segment_id: toolCall.tool_call_id,
        kind: 'tool_call',
        tool_call: cloneToolCall(toolCall),
      });
    }
  }

  for (const event of sortedEvents) {
    const transcriptMessage = transcriptMessageFromEvent(event);
    if (transcriptMessage) {
      transcriptMessages.push(transcriptMessage);
      continue;
    }

    if (isPersistedRunMessageEvent(event) && event.type.startsWith('tool.call.')) {
      continue;
    }

    if (isSidecarActivityEvent(event)) {
      activityEvents.push(event);
      continue;
    }

    const payload = asRecord(event.payload) ?? {};
    switch (event.type) {
      case 'assistant.message.started': {
        const messageID = asString(payload.message_id) ?? `assistant:${event.id}`;
        liveAssistantMessage = ensureAssistantMessage(null, messageID, event.timestamp);
        break;
      }

      case 'assistant.message.delta': {
        const messageID = asString(payload.message_id) ?? liveAssistantMessage?.message_id ?? `assistant:${event.id}`;
        liveAssistantMessage = ensureAssistantMessage(liveAssistantMessage, messageID, event.timestamp);
        const previousContent = liveAssistantMessage.content;
        // Use raw coalescing (not asString) to preserve whitespace-only deltas like " " or " found".
        const deltaContent = typeof payload.content === 'string' ? payload.content : (typeof payload.text === 'string' ? payload.text : '');
        liveAssistantMessage.content += deltaContent;
        liveAssistantMessage.status = 'streaming';
        if (deltaContent || previousContent.length === 0) {
          appendAssistantSegment(liveTurnSegments, messageID, deltaContent, event.timestamp);
        }
        break;
      }

      case 'assistant.message.completed': {
        const messageID = asString(payload.message_id) ?? liveAssistantMessage?.message_id ?? `assistant:${event.id}`;
        liveAssistantMessage = ensureAssistantMessage(liveAssistantMessage, messageID, event.timestamp);
        const previousContent = liveAssistantMessage.content;
        const content = firstNonEmptyString(asString(payload.content), asString(payload.text));
        if (content && content.length >= liveAssistantMessage.content.length) {
          liveAssistantMessage.content = content;
        }
        liveAssistantMessage.status = 'completed';
        liveAssistantMessage.completed_at = event.timestamp;
        appendOrMarkCompletedAssistantSegment(
          liveTurnSegments,
          messageID,
          previousContent,
          liveAssistantMessage.content,
          event.timestamp,
        );
        break;
      }

      case 'reasoning.message.started': {
        const messageID = asString(payload.message_id) ?? `reasoning:${event.id}`;
        liveReasoningMessage = ensureReasoningMessage(null, messageID, event.timestamp);
        liveReasoningMessage.encrypted_value = asString(payload.encrypted_value);
        break;
      }

      case 'reasoning.message.delta': {
        const messageID = asString(payload.message_id) ?? liveReasoningMessage?.message_id ?? `reasoning:${event.id}`;
        liveReasoningMessage = ensureReasoningMessage(liveReasoningMessage, messageID, event.timestamp);
        const reasoningDelta = typeof payload.content === 'string' ? payload.content : (typeof payload.text === 'string' ? payload.text : '');
        liveReasoningMessage.content += reasoningDelta;
        liveReasoningMessage.encrypted_value = firstNonEmptyString(
          asString(payload.encrypted_value),
          liveReasoningMessage.encrypted_value,
        );
        liveReasoningMessage.status = 'streaming';
        break;
      }

      case 'reasoning.message.completed': {
        const messageID = asString(payload.message_id) ?? liveReasoningMessage?.message_id ?? `reasoning:${event.id}`;
        liveReasoningMessage = ensureReasoningMessage(liveReasoningMessage, messageID, event.timestamp);
        const content = firstNonEmptyString(asString(payload.content), asString(payload.text));
        if (content && content.length >= liveReasoningMessage.content.length) {
          liveReasoningMessage.content = content;
        }
        liveReasoningMessage.encrypted_value = firstNonEmptyString(
          asString(payload.encrypted_value),
          liveReasoningMessage.encrypted_value,
        );
        liveReasoningMessage.status = 'completed';
        liveReasoningMessage.completed_at = event.timestamp;
        break;
      }

      case 'tool.call.started': {
        const parentMessageID = asString(payload.parent_message_id)
          ?? liveAssistantMessage?.message_id
          ?? `assistant:${asString(payload.tool_call_id) ?? event.id}`;
        liveAssistantMessage = ensureAssistantMessage(liveAssistantMessage, parentMessageID, event.timestamp);
        const toolCall = ensureToolCall(liveAssistantMessage, payload, event.timestamp);
        const liveToolSegment = ensureToolCallSegment(liveTurnSegments, payload, event.timestamp, parentMessageID);
        toolCall.args_text = firstNonEmptyString(
          asString(payload.args_text),
          asString(payload.tool_input),
          toolCall.args_text,
        ) ?? '';
        toolCall.status = 'running';
        liveToolSegment.args_text = firstNonEmptyString(
          asString(payload.args_text),
          asString(payload.tool_input),
          liveToolSegment.args_text,
        ) ?? '';
        liveToolSegment.status = 'running';
        break;
      }

      case 'tool.call.args.delta': {
        const parentMessageID = asString(payload.parent_message_id)
          ?? liveAssistantMessage?.message_id
          ?? `assistant:${asString(payload.tool_call_id) ?? event.id}`;
        liveAssistantMessage = ensureAssistantMessage(liveAssistantMessage, parentMessageID, event.timestamp);
        const toolCall = ensureToolCall(liveAssistantMessage, payload, event.timestamp);
        const liveToolSegment = ensureToolCallSegment(liveTurnSegments, payload, event.timestamp, parentMessageID);
        toolCall.args_text = mergeToolArgs(toolCall.args_text, payload);
        liveToolSegment.args_text = mergeToolArgs(liveToolSegment.args_text, payload);
        break;
      }

      case 'tool.call.result': {
        const parentMessageID = asString(payload.parent_message_id)
          ?? liveAssistantMessage?.message_id
          ?? `assistant:${asString(payload.tool_call_id) ?? event.id}`;
        liveAssistantMessage = ensureAssistantMessage(liveAssistantMessage, parentMessageID, event.timestamp);
        const toolCall = ensureToolCall(liveAssistantMessage, payload, event.timestamp);
        const liveToolSegment = ensureToolCallSegment(liveTurnSegments, payload, event.timestamp, parentMessageID);
        toolCall.result = {
          message_id: asString(payload.result_message_id),
          content: firstNonEmptyString(asString(payload.content), asString(payload.output_summary)) ?? '',
          output_summary: asString(payload.output_summary),
          error: asString(payload.error),
        };
        liveToolSegment.result = {
          message_id: asString(payload.result_message_id),
          content: firstNonEmptyString(asString(payload.content), asString(payload.output_summary)) ?? '',
          output_summary: asString(payload.output_summary),
          error: asString(payload.error),
        };
        break;
      }

      case 'tool.call.completed':
      case 'tool.call.failed': {
        const parentMessageID = asString(payload.parent_message_id)
          ?? liveAssistantMessage?.message_id
          ?? `assistant:${asString(payload.tool_call_id) ?? event.id}`;
        liveAssistantMessage = ensureAssistantMessage(liveAssistantMessage, parentMessageID, event.timestamp);
        const toolCall = ensureToolCall(liveAssistantMessage, payload, event.timestamp);
        const liveToolSegment = ensureToolCallSegment(liveTurnSegments, payload, event.timestamp, parentMessageID);
        toolCall.status = event.type === 'tool.call.failed' ? 'failed' : 'completed';
        toolCall.completed_at = event.timestamp;
        toolCall.duration_ms = asNumber(payload.duration_ms);
        toolCall.result = {
          message_id: asString(payload.result_message_id) ?? toolCall.result?.message_id,
          content: firstNonEmptyString(
            asString(payload.content),
            asString(payload.output_summary),
            toolCall.result?.content,
          ) ?? '',
          output_summary: firstNonEmptyString(
            asString(payload.output_summary),
            toolCall.result?.output_summary,
          ),
          error: firstNonEmptyString(asString(payload.error), toolCall.result?.error),
        };
        liveToolSegment.status = event.type === 'tool.call.failed' ? 'failed' : 'completed';
        liveToolSegment.completed_at = event.timestamp;
        liveToolSegment.duration_ms = asNumber(payload.duration_ms);
        liveToolSegment.result = {
          message_id: asString(payload.result_message_id) ?? liveToolSegment.result?.message_id,
          content: firstNonEmptyString(
            asString(payload.content),
            asString(payload.output_summary),
            liveToolSegment.result?.content,
          ) ?? '',
          output_summary: firstNonEmptyString(
            asString(payload.output_summary),
            liveToolSegment.result?.output_summary,
          ),
          error: firstNonEmptyString(asString(payload.error), liveToolSegment.result?.error),
        };
        break;
      }

      case 'plan.updated': {
        const planJson = asString(payload.content);
        if (planJson) {
          const parsed = parsePlanArtifact(planJson);
          if (parsed) currentPlanLive = parsed;
        }
        break;
      }

      default:
        break;
    }
  }

  if (liveAssistantMessage && liveAssistantMatchesTranscript(liveAssistantMessage, transcriptMessages)) {
    liveAssistantMessage = null;
    liveTurnSegments.length = 0;
  }

  if (
    liveReasoningMessage
    && liveReasoningMessage.status === 'completed'
    && !liveReasoningMessage.content.trim()
    && !liveReasoningMessage.encrypted_value
  ) {
    liveReasoningMessage = null;
  }

  const { currentPlan, completedToolCalls, allToolCalls } = extractPlanAndToolCalls(
    transcriptMessages,
    liveAssistantMessage,
    liveTurnSegments,
  );
  const eventPlan = extractPlanFromEvents(sortedEvents);
  const reconciledPlan = reconcileObservedPlanState(
    currentPlanLive ?? currentPlan ?? eventPlan,
    allToolCalls,
    sortedEvents,
  );

  return {
    transcript_messages: transcriptMessages,
    live_assistant_message: liveAssistantMessage,
    live_reasoning_message: liveReasoningMessage,
    live_turn_segments: liveTurnSegments.filter((segment) => (
      segment.kind !== 'tool_call' || segment.tool_call.tool_name !== 'update_plan'
    )),
    activity_events: activityEvents,
    current_plan: reconciledPlan,
    completed_tool_calls: completedToolCalls,
  };
}
