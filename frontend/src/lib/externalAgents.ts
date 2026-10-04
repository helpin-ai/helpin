import type { Agent } from '@/lib/pmTypes';
import type { AgentCardSummary } from '@/lib/externalAgentTypes';

export const A2A_LEARN_MORE_URL = 'https://a2a-protocol.org';

/** External agents run outside Helpin and receive work over the A2A protocol. */
export function isExternalAgent(agent?: Pick<Agent, 'runtime_kind'> | null): boolean {
  return agent?.runtime_kind === 'a2a';
}

/**
 * External (A2A) agents are configured in Settings → External agents, so the
 * agents page shows them read-only and removes them only from Settings.
 */
export function agentEditorKind(agent: Pick<Agent, 'is_system' | 'runtime_kind'>): 'system' | 'custom' | 'external' {
  if (isExternalAgent(agent)) return 'external';
  return agent.is_system ? 'system' : 'custom';
}

/** External (A2A) agents bring their own model, so Helpin's AI connection choice does not apply. */
export function taskAgentRunUsesAIConnection(agent: Pick<Agent, 'runtime_kind'> | null | undefined) {
  return !isExternalAgent(agent);
}

/** Accepts an Agent Card URL or the agent's base URL; the server resolves the well-known card path. */
export function isValidExternalAgentURL(value: string) {
  try {
    const url = new URL(value.trim());
    return (url.protocol === 'https:' || url.protocol === 'http:') && Boolean(url.hostname);
  } catch {
    return false;
  }
}

export function externalAgentHost(value: string) {
  try {
    return new URL(value).host;
  } catch {
    return value;
  }
}

export function externalAgentProtocolLabel(card: Pick<AgentCardSummary, 'protocol_binding' | 'protocol_version'>) {
  const version = card.protocol_version ? `A2A ${card.protocol_version}` : 'A2A';
  return card.protocol_binding ? `${version} · ${card.protocol_binding}` : version;
}

export function externalAgentCapabilityLabel(card: Pick<AgentCardSummary, 'capabilities'>) {
  const supported = [
    card.capabilities?.streaming ? 'streaming' : null,
    card.capabilities?.push_notifications ? 'push notifications' : null,
  ].filter(Boolean);
  if (supported.length === 0) return 'No streaming or push notifications';
  const joined = supported.join(' and ');
  return `Supports ${joined}`;
}

export function externalAgentTeamSummary(teamIds: string[], findTeamName: (id: string) => string | undefined) {
  if (teamIds.length === 0) return 'All teams';
  const names = teamIds.map((id) => findTeamName(id) ?? 'Unknown team');
  return names.join(', ');
}
