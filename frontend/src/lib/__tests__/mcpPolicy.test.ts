import { describe, expect, it } from 'vitest';
import {
  ensureMCPWriteScopes,
  getMCPConsentAccess,
  hasMCPWriteGrant,
  hasMCPWriteScope,
  isMCPWriteScope,
} from '@/lib/mcpPolicy';

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
    expect(hasMCPWriteGrant(['helpin.docs.write'], ['docs'])).toBe(true);
    expect(hasMCPWriteGrant(['helpin.docs.write'], ['context'])).toBe(false);
  });

  it('applies selected workspace restrictions before authorization', () => {
    expect(getMCPConsentAccess(
      ['helpin.context.read', 'helpin.pm.read', 'helpin.pm.write', 'helpin.docs.read'],
      ['context', 'pm', 'docs'],
      {
        allowed_scopes: ['helpin.context.read', 'helpin.pm.read', 'helpin.pm.write'],
        allowed_toolsets: ['context', 'pm'],
        read_only_required: false,
      },
      false,
    )).toEqual({
      scopes: ['helpin.context.read', 'helpin.pm.read', 'helpin.pm.write'],
      toolsets: ['context', 'pm'],
      readOnly: false,
      canRequestWrites: true,
    });
  });

  it('does not promise writes when workspace policy forces read-only', () => {
    expect(getMCPConsentAccess(
      ['helpin.context.read', 'helpin.docs.read', 'helpin.docs.write'],
      ['context', 'docs'],
      {
        allowed_scopes: ['helpin.context.read', 'helpin.docs.read', 'helpin.docs.write'],
        allowed_toolsets: ['context', 'docs'],
        read_only_required: true,
      },
      false,
    )).toEqual({
      scopes: ['helpin.context.read', 'helpin.docs.read'],
      toolsets: ['context', 'docs'],
      readOnly: true,
      canRequestWrites: false,
    });
  });
});
