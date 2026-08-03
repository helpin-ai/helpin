const LEGACY_SUB_AGENT_NAMES = new Set(['command agent', 'command agent (one-shot)']);

export function displayAgentName(name: string): string {
  const trimmed = name.trim();
  return LEGACY_SUB_AGENT_NAMES.has(trimmed.toLowerCase()) ? 'Sub-agent' : trimmed;
}
