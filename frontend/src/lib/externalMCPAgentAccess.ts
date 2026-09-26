import type { ExternalMCPServer } from '@/lib/externalMCPTypes';
import type { Agent, AgentPresetDefinition, AgentVersion } from '@/lib/pmTypes';

export function assignedServerTools(allowedTools: string[], server: ExternalMCPServer): string[] {
  const allowed = new Set(allowedTools);
  return (server.tools ?? []).map((tool) => tool.runtime_alias).filter((alias) => allowed.has(alias));
}

export function mergeServerToolAccess(currentTools: string[], server: ExternalMCPServer, selectedAliases: string[]): string[] {
  const serverAliases = new Set((server.tools ?? []).map((tool) => tool.runtime_alias));
  const selected = new Set(selectedAliases.filter((alias) => serverAliases.has(alias)));
  return [
    ...currentTools.filter((alias) => !serverAliases.has(alias)),
    ...(server.tools ?? []).map((tool) => tool.runtime_alias).filter((alias) => selected.has(alias)),
  ];
}

export function editableAgentVersion(agent: Agent, presets: AgentPresetDefinition[]): { kind: 'custom' | 'preset'; id: string } | null {
  if (!agent.is_system) return agent.active_version_id ? { kind: 'custom', id: agent.active_version_id } : null;
  const active = presets.find((preset) => preset.scope === 'workspace'
    && preset.id
    && preset.family_key === agent.preset_key
    && preset.version_key === agent.preset_version_key);
  return active?.id ? { kind: 'preset', id: active.id } : null;
}

export type AgentAccessWriter = {
  getAgent: (agentId: string) => Promise<Agent>;
  listCustomVersions: (agentId: string) => Promise<AgentVersion[]>;
  listPresets: () => Promise<AgentPresetDefinition[]>;
  updateCustomVersion: (agentId: string, versionId: string, allowedTools: string[]) => Promise<unknown>;
  updatePresetVersion: (versionId: string, allowedTools: string[]) => Promise<unknown>;
};

export async function saveServerAgentToolAccess(
  agent: Agent,
  server: ExternalMCPServer,
  selectedAliases: string[],
  writer: AgentAccessWriter,
): Promise<void> {
  const fresh = await writer.getAgent(agent.id);
  if (fresh.is_system !== agent.is_system
    || fresh.active_version_id !== agent.active_version_id
    || fresh.preset_version_key !== agent.preset_version_key) {
    throw new Error('The agent’s active version changed. Reopen access and try again.');
  }

  if (!fresh.is_system) {
    if (!fresh.active_version_id) throw new Error('This agent has no active version to edit.');
    const versions = await writer.listCustomVersions(fresh.id);
    const version = versions.find((candidate) => candidate.id === fresh.active_version_id);
    if (!version) throw new Error('The active agent version is unavailable.');
    await writer.updateCustomVersion(fresh.id, version.id, mergeServerToolAccess(version.allowed_tools ?? [], server, selectedAliases));
    return;
  }

  const presets = await writer.listPresets();
  const editable = editableAgentVersion(fresh, presets);
  if (!editable || editable.kind !== 'preset') throw new Error('Duplicate the product version in the agent editor to change its tools.');
  const preset = presets.find((candidate) => candidate.id === editable.id);
  if (!preset) throw new Error('The active workspace version is unavailable.');
  await writer.updatePresetVersion(editable.id, mergeServerToolAccess(preset.allowed_tools ?? [], server, selectedAliases));
}
