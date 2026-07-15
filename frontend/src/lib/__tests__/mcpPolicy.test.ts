import { describe, expect, it } from 'vitest';
import { ensureMCPWriteScopes, hasMCPWriteScope, isMCPWriteScope } from '@/lib/mcpPolicy';

const AVAILABLE_SCOPES = [
  'helpin.context.read',
  'helpin.pm.read',
  'helpin.pm.write',
  'helpin.docs.read',
  'helpin.docs.write',
  'helpin.crm.read',
  'helpin.crm.write',
  'helpin.support.read',
  'helpin.agents.read',
  'helpin.agents.run',
];

describe('MCP policy scope helpers', () => {
  it('adds write scopes for selected write-capable toolsets', () => {
    expect(ensureMCPWriteScopes(
      ['helpin.context.read', 'helpin.pm.read', 'helpin.docs.read'],
      ['context', 'pm', 'docs'],
      AVAILABLE_SCOPES,
    )).toEqual([
      'helpin.context.read',
      'helpin.pm.read',
      'helpin.docs.read',
      'helpin.pm.write',
      'helpin.docs.write',
    ]);
  });

  it('does not add unavailable, duplicate, or read-only product scopes', () => {
    expect(ensureMCPWriteScopes(
      ['helpin.pm.write'],
      ['pm', 'crm', 'support', 'context'],
      ['helpin.context.read', 'helpin.pm.write'],
    )).toEqual(['helpin.pm.write']);
  });

  it('recognizes mutation and agent-run scopes', () => {
    expect(isMCPWriteScope('helpin.docs.write')).toBe(true);
    expect(isMCPWriteScope('helpin.agents.run')).toBe(true);
    expect(hasMCPWriteScope(['helpin.context.read', 'helpin.pm.read'])).toBe(false);
    expect(hasMCPWriteScope(['helpin.context.read', 'helpin.crm.write'])).toBe(true);
  });
});
