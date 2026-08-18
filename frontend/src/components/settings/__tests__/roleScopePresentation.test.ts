import { describe, expect, it } from 'vitest';
import {
  ASSIGNABLE_ORGANIZATION_ROLES,
  canEditOrganizationMemberRole,
  canRemoveOrganizationMember,
  organizationRoleLabel,
  workspaceRoleLabel,
} from '../roleScopePresentation';

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

  it('only offers non-owner organization roles', () => {
    expect(ASSIGNABLE_ORGANIZATION_ROLES).toEqual(['admin', 'member', 'viewer']);
  });

  it('lets owners edit non-owners, including admins', () => {
    expect(canEditOrganizationMemberRole('owner', 'admin', false)).toBe(true);
  });

  it('lets admins edit members and viewers but not peer admins', () => {
    expect(canEditOrganizationMemberRole('admin', 'admin', false)).toBe(false);
    expect(canEditOrganizationMemberRole('admin', 'member', false)).toBe(true);
    expect(canEditOrganizationMemberRole('admin', 'viewer', false)).toBe(true);
  });

  it('keeps owners, self, and non-admin actors read-only', () => {
    expect(canEditOrganizationMemberRole('owner', 'owner', false)).toBe(false);
    expect(canEditOrganizationMemberRole('admin', 'admin', true)).toBe(false);
    expect(canEditOrganizationMemberRole('member', 'viewer', false)).toBe(false);
    expect(canEditOrganizationMemberRole('viewer', 'member', false)).toBe(false);
  });

  it('preserves the stricter removal rule for admins', () => {
    expect(canRemoveOrganizationMember('owner', 'admin', false)).toBe(true);
    expect(canRemoveOrganizationMember('admin', 'admin', false)).toBe(false);
    expect(canRemoveOrganizationMember('admin', 'member', false)).toBe(true);
  });
});
