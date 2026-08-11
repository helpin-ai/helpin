import { describe, expect, it } from 'vitest';
import { organizationRoleLabel, workspaceRoleLabel } from '../roleScopePresentation';

describe('role scope presentation', () => {
  it('makes workspace roles explicit', () => {
    expect(workspaceRoleLabel('owner')).toBe('Workspace owner');
    expect(workspaceRoleLabel('admin')).toBe('Workspace admin');
    expect(workspaceRoleLabel('member')).toBe('Workspace member');
    expect(workspaceRoleLabel('viewer')).toBe('Workspace viewer');
  });

  it('makes organization roles explicit', () => {
    expect(organizationRoleLabel('owner')).toBe('Organization owner');
    expect(organizationRoleLabel('admin')).toBe('Organization admin');
    expect(organizationRoleLabel('member')).toBe('Organization member');
    expect(organizationRoleLabel('viewer')).toBe('Organization viewer');
  });

  it('preserves unknown role values', () => {
    expect(workspaceRoleLabel('custom')).toBe('custom');
    expect(organizationRoleLabel('custom')).toBe('custom');
  });
});
