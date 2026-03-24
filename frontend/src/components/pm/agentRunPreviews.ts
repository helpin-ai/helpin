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

function parsePreviewInput(input: unknown, surroundingText: string): PublishedPreview | null {
  const payload = asRecord(input);
  if (!payload) return null;

  const panelKey = asString(payload.panel_key)?.toLowerCase();
  const title = asString(payload.title);
  const format = normalizePreviewFormat(payload.format);
  if (!panelKey || !title || !format || !Object.prototype.hasOwnProperty.call(payload, 'content')) {
    return null;
  }

  const content = payload.content;
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
    if (!invocation || invocation.tool_name !== 'publish_preview') continue;
    const preview = parsePreviewInput(invocation.input, message.content);
    if (!preview) continue;
    if (expectedPanelKey && preview.panelKey !== expectedPanelKey) continue;
    return preview;
  }
  return null;
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
