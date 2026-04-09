import type { CodingSessionLiveToolCall } from '@/lib/pmTypes';

export interface ToolCallPresentation {
  primaryLabel: string;
  secondaryLabel: string;
  chips: string[];
}

function asRecord(value: unknown): Record<string, unknown> | null {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return null;
  return value as Record<string, unknown>;
}

function asString(value: unknown): string | null {
  return typeof value === 'string' && value.trim().length > 0 ? value.trim() : null;
}

function asNumber(value: unknown): number | null {
  return typeof value === 'number' && Number.isFinite(value) ? value : null;
}

function parseArgs(argsText: string): Record<string, unknown> | null {
  if (!argsText.trim()) return null;
  try {
    return asRecord(JSON.parse(argsText));
  } catch {
    return null;
  }
}

function titleCaseToolName(toolName: string) {
  return toolName.replaceAll('_', ' ').replace(/\b\w/g, (char) => char.toUpperCase());
}

function lineRangeLabel(startLine: number | null, endLine: number | null) {
  if (startLine == null) return null;
  if (endLine != null && endLine !== startLine) return `${startLine}-${endLine}`;
  return `${startLine}`;
}

function quoted(value: string) {
  return `"${value}"`;
}

export function describeToolCall(toolCall: CodingSessionLiveToolCall): ToolCallPresentation {
  const toolName = toolCall.tool_name.toLowerCase();
  const parsed = parseArgs(toolCall.args_text);
  const path = asString(parsed?.path) ?? asString(parsed?.file_path);
  const startLine = asNumber(parsed?.start_line);
  const endLine = asNumber(parsed?.end_line);
  const range = lineRangeLabel(startLine, endLine);
  const pattern = asString(parsed?.pattern) ?? asString(parsed?.query);
  const searchPath = asString(parsed?.path);
  const command = asString(parsed?.command) ?? asString(parsed?.cmd);
  const cwd = asString(parsed?.cwd);
  const paths = Array.isArray(parsed?.paths)
    ? parsed?.paths.map((value) => asString(value)).filter((value): value is string => Boolean(value))
    : [];

  const secondaryLabel = titleCaseToolName(toolCall.tool_name);

  if (toolName === 'read_file_range' && path) {
    const primaryLabel = `Read ${path}${range ? `:${range}` : ''}`;
    const chips = [];
    if (startLine != null && endLine != null && endLine >= startLine) chips.push(`${endLine - startLine + 1} lines`);
    return { primaryLabel, secondaryLabel, chips };
  }

  if ((toolName === 'read_file' || toolName === 'write_file' || toolName === 'edit_file' || toolName === 'str_replace_editor') && path) {
    const verb = toolName === 'write_file' ? 'Write' : toolName === 'edit_file' || toolName === 'str_replace_editor' ? 'Edit' : 'Read';
    return { primaryLabel: `${verb} ${path}`, secondaryLabel, chips: [] };
  }

  if (toolName === 'read_files' && paths.length > 0) {
    return {
      primaryLabel: `Read ${paths[0]}${paths.length > 1 ? ` +${paths.length - 1} more` : ''}`,
      secondaryLabel,
      chips: [`${paths.length} files`],
    };
  }

  if ((toolName === 'ripgrep' || toolName === 'grep' || toolName === 'search_files') && pattern) {
    return {
      primaryLabel: `Search ${quoted(pattern)}${searchPath ? ` in ${searchPath}` : ''}`,
      secondaryLabel,
      chips: [],
    };
  }

  if ((toolName === 'run_command' || toolName === 'bash' || toolName.includes('shell') || toolName.includes('exec')) && command) {
    const chips = [];
    if (cwd) chips.push(cwd);
    return { primaryLabel: `Run ${command}`, secondaryLabel, chips };
  }

  if (toolName === 'list_directory' && path) {
    return { primaryLabel: `List ${path}`, secondaryLabel, chips: [] };
  }

  if (toolName === 'apply_patch') {
    return { primaryLabel: 'Apply patch', secondaryLabel, chips: [] };
  }

  if (path) {
    return { primaryLabel: `${secondaryLabel} ${path}`, secondaryLabel, chips: [] };
  }

  return {
    primaryLabel: secondaryLabel,
    secondaryLabel,
    chips: [],
  };
}
