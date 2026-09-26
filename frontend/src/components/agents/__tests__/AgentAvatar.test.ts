import { describe, expect, it } from 'vitest';

import { AGENT_ICON_PRESETS, getAgentPersonaMeta, resolveAgentPersonaKey } from '../AgentAvatar';

describe('AgentAvatar persona resolution', () => {
  it('assigns Mira to the marketer preset', () => {
    expect(resolveAgentPersonaKey({ presetKey: 'marketer' })).toBe('mira');
    expect(getAgentPersonaMeta({ presetKey: 'marketer' })).toMatchObject({
      key: 'mira',
      label: 'Mira',
      role: 'Marketer',
    });
  });

  it('recognizes Mira by name', () => {
    expect(resolveAgentPersonaKey({ name: 'Mira' })).toBe('mira');
  });

  it('assigns Quill to the documentation preset', () => {
    expect(resolveAgentPersonaKey({ presetKey: 'documentation_agent' })).toBe('quill');
    expect(getAgentPersonaMeta({ presetKey: 'documentation_agent' })).toMatchObject({
      key: 'quill',
      label: 'Quill',
      role: 'Documentation Agent',
    });
  });

  it('gives the Ask Agent and Sub-agent system presets their own avatars', () => {
    expect(getAgentPersonaMeta({ presetKey: 'ask_agent' })).toMatchObject({ key: 'ask', label: 'Ask Agent' });
    expect(getAgentPersonaMeta({ presetKey: 'command_agent' })).toMatchObject({ key: 'sub_agent', label: 'Sub-agent' });
    expect(resolveAgentPersonaKey({ name: 'Ask Agent' })).toBe('ask');
    expect(resolveAgentPersonaKey({ name: 'Sub-agent' })).toBe('sub_agent');
  });

  it('uses an explicitly selected custom avatar before name or preset inference', () => {
    expect(resolveAgentPersonaKey({
      agent: { name: 'Forge', preset_key: 'code_builder', icon_key: 'ocean_orbit' },
    })).toBe('ocean_orbit');
  });

  it('exposes anonymous custom avatar presets instead of product agent names', () => {
    expect(AGENT_ICON_PRESETS).toHaveLength(8);
    expect(AGENT_ICON_PRESETS.map((preset) => preset.label)).not.toContain('Forge');
    expect(AGENT_ICON_PRESETS.map((preset) => preset.label)).not.toContain('Scribe');
  });
});
