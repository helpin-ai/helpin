import { isPublishedPreviewToolName, parsePublishedPreviewPayload, parseToolInvocationPublishedPreview, type PublishedPreview } from '@/components/pm/runPreviews';
import type { CodingSessionEvent, CodingSessionLiveTurnSegment } from '@/lib/pmTypes';

function asRecord(value: unknown): Record<string, unknown> | null {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return null;
  return value as Record<string, unknown>;
}

function asString(value: unknown) {
  return typeof value === 'string' ? value : '';
}

function parsePreviewFromMessageEvent(event: CodingSessionEvent): PublishedPreview | null {
  if (event.type !== 'assistant.message.completed') return null;
  if (event.runtime_metadata?.source !== 'agent_run_message') return null;

  const payload = asRecord(event.payload) ?? {};
  const surroundingText = asString(payload.content).trim();
  const invocations = Array.isArray(payload.tool_invocations) ? payload.tool_invocations : [];

  for (let index = invocations.length - 1; index >= 0; index -= 1) {
    const invocation = asRecord(invocations[index]);
    if (!invocation || !isPublishedPreviewToolName(invocation.tool_name)) continue;
    const preview = parsePublishedPreviewPayload(invocation.input, surroundingText);
    if (preview) return preview;
  }

  return null;
}

function parsePreviewFromArtifactEvent(event: CodingSessionEvent): PublishedPreview | null {
  if (event.type !== 'preview.updated') return null;
  const payload = asRecord(event.payload) ?? {};
  return parsePublishedPreviewPayload(payload.content);
}

function parsePreviewFromLiveSegment(segment: CodingSessionLiveTurnSegment): PublishedPreview | null {
  if (segment.kind !== 'tool_call') return null;
  if (!isPublishedPreviewToolName(segment.tool_call.tool_name)) return null;
  if (segment.tool_call.status === 'failed') return null;
  let input: unknown = segment.tool_call.args_text;
  try {
    input = JSON.parse(segment.tool_call.args_text);
  } catch {
    input = segment.tool_call.args_text;
  }
  return parseToolInvocationPublishedPreview({
    tool_name: segment.tool_call.tool_name,
    input,
  });
}

export function collectCodingSessionPreviews(
  events: CodingSessionEvent[],
  liveTurnSegments: CodingSessionLiveTurnSegment[],
) {
  const previews = new Map<string, PublishedPreview>();

  for (let index = events.length - 1; index >= 0; index -= 1) {
    const event = events[index];
    const preview = parsePreviewFromArtifactEvent(event) ?? parsePreviewFromMessageEvent(event);
    if (!preview || previews.has(preview.panelKey)) continue;
    previews.set(preview.panelKey, preview);
  }

  for (const segment of liveTurnSegments) {
    const preview = parsePreviewFromLiveSegment(segment);
    if (!preview) continue;
    previews.set(preview.panelKey, preview);
  }

  return previews;
}
