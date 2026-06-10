import { describe, expect, it } from 'vitest';

import { getAgentPersonaMeta, resolveAgentPersonaKey } from '../AgentAvatar';

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
});
