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
  'publish_task_plan',
  'publish_task_plan_doc',
  'publish_story_plan',
  'publish_story_plan_doc',
]);

export function isPublishedPreviewToolName(value: unknown): boolean {
  const normalized = asString(value);
  return normalized ? PREVIEW_TOOL_NAMES.has(normalized) : false;
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

  const rawPanelKey = asString(payload.panel_key)?.toLowerCase();
  const panelKey = rawPanelKey === 'story_plan'
    ? 'task_plan'
    : rawPanelKey === 'story_plan_doc'
      ? 'task_plan_doc'
      : rawPanelKey;
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

function deepCloneJson(value: unknown): unknown {
  if (Array.isArray(value)) {
    return value.map(deepCloneJson);
  }
  if (value && typeof value === 'object') {
    return Object.fromEntries(
      Object.entries(value as Record<string, unknown>).map(([key, entry]) => [key, deepCloneJson(entry)]),
    );
  }
  return value;
}

function extractToolPreviewContent(payload: Record<string, unknown>, format: PreviewFormat): unknown {
  for (const key of ['content', 'body', 'markdown', 'text']) {
    if (Object.prototype.hasOwnProperty.call(payload, key) && payload[key] != null) {
      return payload[key];
    }
  }

  for (const key of ['preview', 'payload']) {
    const nested = asRecord(payload[key]);
    if (!nested) continue;
    const nestedContent = extractToolPreviewContent(nested, format);
    if (nestedContent != null) return nestedContent;
    if (format === 'json') {
      const stripped = stripPreviewMetaFields(nested);
      if (stripped != null) return stripped;
    }
  }

  if (format === 'json') {
    return stripPreviewMetaFields(payload);
  }

  return null;
}

function stripPreviewMetaFields(payload: Record<string, unknown>): unknown {
  const metaKeys = new Set([
    'slot',
    'panel_key',
    'panelKey',
    'title',
    'panelTitle',
    'format',
    'preview_format',
    'replace',
    'content',
    'body',
    'markdown',
    'text',
  ]);
  const entries = Object.entries(payload).filter(([key]) => !metaKeys.has(key));
  if (entries.length === 0) return null;
  return Object.fromEntries(entries.map(([key, value]) => [key, deepCloneJson(value)]));
}

function defaultPreviewTitle(panelKey: string): string {
  switch (panelKey) {
    case 'prd_draft':
      return 'PRD Draft';
    case 'task_plan':
      return 'Task Plan';
    case 'task_plan_doc':
      return 'Task Planning Document';
    default:
      return '';
  }
}

function buildFixedToolPreviewInput(toolName: string, input: unknown): unknown {
  const payload = asRecord(input);
  if (!payload) return input;

  let panelKey: string | null = null;
  let format: PreviewFormat | null = null;
  switch (toolName) {
    case 'publish_prd_draft':
      panelKey = 'prd_draft';
      format = 'markdown';
      break;
    case 'publish_task_plan':
    case 'publish_story_plan':
      panelKey = 'task_plan';
      format = 'json';
      break;
    case 'publish_task_plan_doc':
    case 'publish_story_plan_doc':
      panelKey = 'task_plan_doc';
      format = 'markdown';
      break;
    default:
      return input;
  }

  const nextPayload: Record<string, unknown> = { ...payload };
  if (!Object.prototype.hasOwnProperty.call(nextPayload, 'panel_key')) {
    nextPayload.panel_key = panelKey;
  }
  if (!Object.prototype.hasOwnProperty.call(nextPayload, 'title')) {
    nextPayload.title = asString(nextPayload.panelTitle) ?? defaultPreviewTitle(panelKey);
  }
  if (!Object.prototype.hasOwnProperty.call(nextPayload, 'format')) {
    nextPayload.format = format;
  }
  if (!Object.prototype.hasOwnProperty.call(nextPayload, 'content')) {
    const content = extractToolPreviewContent(payload, format);
    if (content != null) {
      nextPayload.content = content;
    }
  }

  return nextPayload;
}

export function parsePublishedPreviewPayload(input: unknown, surroundingText = ''): PublishedPreview | null {
  return parsePreviewInput(input, surroundingText);
}

export function parsePublishedPreviewRawInput(raw: string): PublishedPreview | null {
  if (!raw.trim()) return null;
  try {
    return parsePreviewInput(JSON.parse(raw), '');
  } catch {
    return null;
  }
}

export function parseToolInvocationPublishedPreview(
  invocation: { tool_name?: unknown; input?: unknown },
  surroundingText = '',
): PublishedPreview | null {
  const toolName = asString(invocation.tool_name);
  const normalizedInput = toolName ? buildFixedToolPreviewInput(toolName, invocation.input) : invocation.input;
  return parsePreviewInput(normalizedInput, surroundingText);
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
    const preview = parseToolInvocationPublishedPreview(invocation, message.content);
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
