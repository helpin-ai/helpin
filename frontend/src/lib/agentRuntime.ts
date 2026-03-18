import type { AgentRuntimeKind } from './pmTypes';

export const AGENT_RUNTIME_LABELS: Record<AgentRuntimeKind, string> = {
  opencode: 'OpenCode',
  native_sdk: 'Native SDK',
};
