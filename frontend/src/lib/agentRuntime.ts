import type { AgentRuntimeKind } from './pmTypes';

export const AGENT_RUNTIME_LABELS: Record<AgentRuntimeKind, string> = {
  opencode: 'OpenCode',
  codex: 'Codex',
  native_sdk: 'Native SDK',
};
