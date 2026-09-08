import { describe, expect, it } from 'vitest';
import { getMemberModuleAccess } from '../memberAccess';
import type { WorkspaceModuleGrant } from '@/lib/types';

const grants = [
  { id: 'g1', module: 'support', subject_type: 'team', subject_id: 'team-1' },
  { id: 'g2', module: 'crm', subject_type: 'workspace_member', subject_id: 'member-1' },
  { id: 'g3', module: 'automation', subject_type: 'workspace_member', subject_id: 'someone-else' },
] as WorkspaceModuleGrant[];

describe('member module access', () => {
  it('combines workspace defaults, direct grants, and inherited team access', () => {
    const result = getMemberModuleAccess('member-1', 'member', [{ id: 'team-1', name: 'Support team' }], grants);
    expect(result.map(item => item.module)).toEqual(['pm', 'docs', 'crm', 'support']);
    expect(result.find(item => item.module === 'support')?.description).toBe('Via Support team');
    expect(result.find(item => item.module === 'crm')?.description).toBe('Granted directly');
  });
  it.each(['owner', 'admin'])('shows all modules for a workspace %s without grants', role => {
    expect(getMemberModuleAccess('member-1', role, [], []).map(item => item.module))
      .toEqual(['pm', 'docs', 'crm', 'support', 'automation']);
  });
  it('does not grant access from a team the member does not belong to', () => {
    expect(getMemberModuleAccess('other', 'viewer', [], grants).map(item => item.module)).toEqual(['pm', 'docs']);
  });
});
