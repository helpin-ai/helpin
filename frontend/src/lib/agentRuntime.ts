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
