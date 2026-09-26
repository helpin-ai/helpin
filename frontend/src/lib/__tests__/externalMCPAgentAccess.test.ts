import { describe, expect, it, vi } from 'vitest';
import { assignedServerTools, editableAgentVersion, mergeServerToolAccess, saveServerAgentToolAccess } from '@/lib/externalMCPAgentAccess';
import type { ExternalMCPServer } from '@/lib/externalMCPTypes';
import type { Agent, AgentPresetDefinition, AgentVersion } from '@/lib/pmTypes';

const server = { tools: [
  { runtime_alias: 'mcp__crm__read', enabled: true },
  { runtime_alias: 'mcp__crm__write', enabled: false },
] } as ExternalMCPServer;

describe('external MCP agent access', () => {
  it('counts assignments even when a workspace tool is disabled', () => {
    expect(assignedServerTools(['list_tasks', 'mcp__crm__write'], server)).toEqual(['mcp__crm__write']);
  });

  it('changes only this server’s aliases and preserves other agent tools', () => {
    expect(mergeServerToolAccess(
      ['list_tasks', 'mcp__other__search', 'mcp__crm__write'],
      server,
      ['mcp__crm__read'],
    )).toEqual(['list_tasks', 'mcp__other__search', 'mcp__crm__read']);
  });

  it('edits only active custom or workspace preset versions', () => {
    const custom = { id: 'agent-1', is_system: false, active_version_id: 'version-1' } as Agent;
    expect(editableAgentVersion(custom, [])).toEqual({ kind: 'custom', id: 'version-1' });
    const system = { id: 'agent-2', is_system: true, preset_key: 'support', preset_version_key: 'v2' } as Agent;
    const presets = [{ id: 'preset-1', scope: 'workspace', family_key: 'support', version_key: 'v2' }] as AgentPresetDefinition[];
    expect(editableAgentVersion(system, presets)).toEqual({ kind: 'preset', id: 'preset-1' });
    expect(editableAgentVersion(system, [])).toBeNull();
  });

  it('rebases an agent edit on the latest version and preserves unrelated tools', async () => {
    const agent = { id: 'agent-1', is_system: false, active_version_id: 'version-1' } as Agent;
    const updateCustomVersion = vi.fn().mockResolvedValue(undefined);
    await saveServerAgentToolAccess(agent, server, ['mcp__crm__read'], {
      getAgent: vi.fn().mockResolvedValue(agent),
      listCustomVersions: vi.fn().mockResolvedValue([{ id: 'version-1', allowed_tools: ['mcp__other__search', 'mcp__crm__write'] }] as AgentVersion[]),
      listPresets: vi.fn(),
      updateCustomVersion,
      updatePresetVersion: vi.fn(),
    });
    expect(updateCustomVersion).toHaveBeenCalledWith('agent-1', 'version-1', ['mcp__other__search', 'mcp__crm__read']);
  });

  it('updates the active workspace preset version and keeps unrelated tools', async () => {
    const agent = { id: 'agent-2', is_system: true, preset_key: 'support', preset_version_key: 'support_ws_v1' } as Agent;
    const updatePresetVersion = vi.fn().mockResolvedValue(undefined);
    await saveServerAgentToolAccess(agent, server, ['mcp__crm__read'], {
      getAgent: vi.fn().mockResolvedValue(agent),
      listCustomVersions: vi.fn(),
      listPresets: vi.fn().mockResolvedValue([{ id: 'preset-1', scope: 'workspace', family_key: 'support', version_key: 'support_ws_v1', allowed_tools: ['mcp__other__search', 'mcp__crm__write'] }] as AgentPresetDefinition[]),
      updateCustomVersion: vi.fn(),
      updatePresetVersion,
    });
    expect(updatePresetVersion).toHaveBeenCalledWith('preset-1', ['mcp__other__search', 'mcp__crm__read']);
  });

  it('refuses a stale active version instead of changing the wrong one', async () => {
    const agent = { id: 'agent-1', is_system: false, active_version_id: 'version-1' } as Agent;
    const updateCustomVersion = vi.fn();
    await expect(saveServerAgentToolAccess(agent, server, [], {
      getAgent: vi.fn().mockResolvedValue({ ...agent, active_version_id: 'version-2' }),
      listCustomVersions: vi.fn(), listPresets: vi.fn(), updateCustomVersion, updatePresetVersion: vi.fn(),
    })).rejects.toThrow('active version changed');
    expect(updateCustomVersion).not.toHaveBeenCalled();
  });
});
