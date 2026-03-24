import type { AgentRunArtifact, AgentRunMessage } from '@/lib/pmTypes';

export type PreviewFormat = 'markdown' | 'json';

export interface PublishedPreview {
  panelKey: string;
  title: string;
  format: PreviewFormat;
  content: unknown;
  replace: boolean;
  surroundingText: string;
}

const PREVIEW_TOOL_NAMES = new Set([
  'publish_preview',
  'preview_md',
  'preview_json',
  'publish_prd_draft',
  'publish_story_plan',
  'publish_story_plan_doc',
]);

export function isPublishedPreviewToolName(value: unknown): boolean {
  return PREVIEW_TOOL_NAMES.has(asString(value));
}

function asRecord(value: unknown): Record<string, unknown> | null {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return null;
  return value as Record<string, unknown>;
}

function asString(value: unknown): string | null {
  return typeof value === 'string' && value.trim().length > 0 ? value.trim() : null;
}

function normalizePreviewFormat(value: unknown): PreviewFormat | null {
  const normalized = asString(value)?.toLowerCase();
  if (normalized === 'markdown' || normalized === 'json') return normalized;
  return null;
}

function parseJsonPreviewContent(content: unknown): unknown {
  if (typeof content !== 'string') return content;
  const trimmed = content.trim();
  if (!trimmed.startsWith('{') && !trimmed.startsWith('[')) return content;
  try {
    return JSON.parse(trimmed);
  } catch {
    return content;
  }
}

function parsePreviewInput(input: unknown, surroundingText: string): PublishedPreview | null {
  const payload = asRecord(input);
  if (!payload) return null;

  const panelKey = asString(payload.panel_key)?.toLowerCase();
  const title = asString(payload.title);
  const format = normalizePreviewFormat(payload.format);
  if (!panelKey || !title || !format || !Object.prototype.hasOwnProperty.call(payload, 'content')) {
    return null;
  }

  const content = format === 'json' ? parseJsonPreviewContent(payload.content) : payload.content;
  if (format === 'markdown') {
    if (typeof content !== 'string' || content.trim().length === 0) return null;
  }

  return {
    panelKey,
    title,
    format,
    content,
    replace: payload.replace !== false,
    surroundingText: surroundingText.trim(),
  };
}

export function parsePublishedPreviewRawInput(raw: string): PublishedPreview | null {
  if (!raw.trim()) return null;
  try {
    return parsePreviewInput(JSON.parse(raw), '');
  } catch {
    return null;
  }
}

export function parseMessagePublishedPreview(
  message: Pick<AgentRunMessage, 'content' | 'tool_invocations'>,
  panelKey?: string,
): PublishedPreview | null {
  const expectedPanelKey = panelKey?.trim().toLowerCase() ?? '';
  const invocations = Array.isArray(message.tool_invocations) ? message.tool_invocations : [];
  for (let index = invocations.length - 1; index >= 0; index -= 1) {
    const invocation = asRecord(invocations[index]);
    if (!invocation || !isPublishedPreviewToolName(invocation.tool_name)) continue;
    const preview = parsePreviewInput(invocation.input, message.content);
    if (!preview) continue;
    if (expectedPanelKey && preview.panelKey !== expectedPanelKey) continue;
    return preview;
  }
  return null;
}

function parseArtifactAssistantMessageSequenceNo(
  artifact: Pick<AgentRunArtifact, 'metadata'> | null | undefined,
): number | null {
  if (!artifact?.metadata || typeof artifact.metadata !== 'object' || Array.isArray(artifact.metadata)) {
    return null;
  }
  const value = artifact.metadata.assistant_message_sequence_no;
  return typeof value === 'number' && Number.isFinite(value) ? value : null;
}

export function parseArtifactPublishedPreview(
  artifact: Pick<AgentRunArtifact, 'artifact_type' | 'inline_content'> | null | undefined,
): PublishedPreview | null {
  if (!artifact || artifact.artifact_type !== 'run_preview' || !artifact.inline_content) {
    return null;
  }
  try {
    return parsePreviewInput(JSON.parse(artifact.inline_content), '');
  } catch {
    return null;
  }
}

export function resolveMessagePublishedPreview(
  message: Pick<AgentRunMessage, 'content' | 'tool_invocations' | 'sequence_no'>,
  artifacts: Array<Pick<AgentRunArtifact, 'artifact_type' | 'inline_content' | 'metadata'>>,
  panelKey?: string,
): PublishedPreview | null {
  const expectedPanelKey = panelKey?.trim().toLowerCase() ?? '';
  for (let index = artifacts.length - 1; index >= 0; index -= 1) {
    const artifact = artifacts[index];
    if (parseArtifactAssistantMessageSequenceNo(artifact) !== message.sequence_no) continue;
    const preview = parseArtifactPublishedPreview(artifact);
    if (!preview) continue;
    if (expectedPanelKey && preview.panelKey !== expectedPanelKey) continue;
    return preview;
  }
  return parseMessagePublishedPreview(message, panelKey);
}
