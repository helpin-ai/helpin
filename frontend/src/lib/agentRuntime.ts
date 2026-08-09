import type { AgentRuntimeKind } from './pmTypes';

export const AGENT_RUNTIME_LABELS: Record<AgentRuntimeKind, string> = {
  opencode: 'OpenCode',
  codex: 'Codex',
  native_sdk: 'Native SDK',
};

export const AGENT_RUNTIME_HELP_TEXT: Record<AgentRuntimeKind, string> = {
  native_sdk: 'Recommended for Helpin product tools and on-demand skill loading. Keeps the prompt concise and lets the agent read available skills as needed.',
  codex: 'Best for code and repository work. Helpin product tools and on-demand skill loading are limited compared with Native SDK.',
  opencode: 'Best for code and repository work. Helpin product tools and on-demand skill loading are limited compared with Native SDK.',
};

export const MIN_NATIVE_TOOL_STEPS = 1;
export const MAX_NATIVE_TOOL_STEPS = 1000;

export function parseNativeToolStepLimit(value: string): number | undefined {
  const trimmed = value.trim();
  if (!trimmed) return undefined;
  const parsed = Number(trimmed);
  if (!Number.isInteger(parsed) || parsed < MIN_NATIVE_TOOL_STEPS || parsed > MAX_NATIVE_TOOL_STEPS) {
    return undefined;
  }
  return parsed;
}

export function isValidNativeToolStepLimit(runtimeKind: AgentRuntimeKind, value: string): boolean {
  return runtimeKind !== 'native_sdk' || !value.trim() || parseNativeToolStepLimit(value) !== undefined;
}
