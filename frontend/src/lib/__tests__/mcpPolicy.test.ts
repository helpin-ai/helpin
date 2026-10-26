import { describe, expect, it } from 'vitest';
import {
  ensureMCPWriteScopes,
  getMCPConsentAccess,
  hasMCPWriteGrant,
  hasMCPWriteScope,
  isMCPWriteScope,
  serviceAccountScopes,
  serviceAccountWriteScopes,
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

  it('treats Help Center publishing as an explicit write scope', () => {
    expect(isMCPWriteScope('helpin.docs.publish')).toBe(true);
    expect(hasMCPWriteGrant(['helpin.docs.read', 'helpin.docs.publish'], ['docs'])).toBe(true);
    expect(hasMCPWriteGrant(['helpin.docs.publish'], ['pm'])).toBe(false);
    expect(ensureMCPWriteScopes(
      ['helpin.docs.read'],
      ['docs'],
      ['helpin.docs.read', 'helpin.docs.write', 'helpin.docs.publish'],
    )).toEqual(['helpin.docs.read', 'helpin.docs.write']);
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

describe('service account scopes', () => {
  const policy = {
    enforce_read_only: false,
    allowed_scopes: ['helpin.context.read', 'helpin.crm.read', 'helpin.crm.write', 'helpin.docs.read', 'helpin.docs.write', 'helpin.docs.publish', 'helpin.agents.read', 'helpin.agents.run'],
  };

  it('requests only read scopes by default', () => {
    expect(serviceAccountScopes(policy, ['context', 'crm'], false)).toEqual(['helpin.context.read', 'helpin.crm.read', 'helpin.docs.read', 'helpin.agents.read']);
  });

  it('adds the allowed write scope for each selected toolset when updates are allowed', () => {
    expect(serviceAccountScopes(policy, ['context', 'crm'], true)).toEqual([
      'helpin.context.read', 'helpin.crm.read', 'helpin.docs.read', 'helpin.agents.read', 'helpin.crm.write',
    ]);
  });

  it('never adds Help Center publishing or agent runs automatically', () => {
    const scopes = serviceAccountScopes(policy, ['docs', 'agents'], true);
    expect(scopes).toContain('helpin.docs.write');
    expect(scopes).not.toContain('helpin.docs.publish');
    expect(scopes).not.toContain('helpin.agents.run');
  });

  it('offers no write scopes when the policy enforces read-only or does not allow them', () => {
    expect(serviceAccountWriteScopes({ ...policy, enforce_read_only: true }, ['crm'])).toEqual([]);
    expect(serviceAccountWriteScopes({ enforce_read_only: false, allowed_scopes: ['helpin.crm.read'] }, ['crm'])).toEqual([]);
    expect(serviceAccountWriteScopes(policy, ['context'])).toEqual([]);
  });
});

describe('support reply scope', () => {
  const policy = { enforce_read_only: false, allowed_scopes: ['helpin.support.read', 'helpin.support.write', 'helpin.support.reply'] }

  it('is a write scope that is only granted on purpose', () => {
    expect(isMCPWriteScope('helpin.support.reply')).toBe(true)
    expect(serviceAccountScopes(policy, ['support'], true)).not.toContain('helpin.support.reply')
    expect(serviceAccountScopes(policy, ['support'], true, true)).toContain('helpin.support.reply')
    expect(serviceAccountScopes(policy, ['support'], false, true)).not.toContain('helpin.support.reply')
  })

  it('needs the support toolset and a policy that allows it', () => {
    expect(serviceAccountScopes(policy, ['pm'], true, true)).not.toContain('helpin.support.reply')
    expect(serviceAccountScopes({ ...policy, allowed_scopes: ['helpin.support.write'] }, ['support'], true, true)).not.toContain('helpin.support.reply')
    expect(serviceAccountScopes({ ...policy, enforce_read_only: true }, ['support'], true, true)).not.toContain('helpin.support.reply')
  })
})
