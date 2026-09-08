import type { WorkspaceModule, WorkspaceModuleGrant } from '@/lib/types';

export const MEMBER_MODULES: Array<{ module: WorkspaceModule; label: string }> = [
  { module: 'pm', label: 'Projects' },
  { module: 'docs', label: 'Docs' },
  { module: 'crm', label: 'CRM' },
  { module: 'support', label: 'Support' },
  { module: 'automation', label: 'Automation' },
];

export function getMemberModuleAccess(
  memberId: string,
  role: string,
  teams: Array<{ id: string; name: string }>,
  grants: WorkspaceModuleGrant[],
) {
  return MEMBER_MODULES.flatMap<{ module: WorkspaceModule; label: string; description: string }>(({ module, label }) => {
    if (module === 'pm' || module === 'docs') {
      return [{ module, label, description: 'Available to all workspace members' }];
    }
    if (role === 'owner' || role === 'admin') {
      return [{ module, label, description: `Included with the workspace ${role} role` }];
    }
    const matching = grants.filter(grant => grant.module === module);
    if (matching.some(grant => grant.subject_type === 'workspace_member' && grant.subject_id === memberId)) {
      return [{ module, label, description: 'Granted directly' }];
    }
    const inheritedTeams = teams.filter(team => matching.some(grant => grant.subject_type === 'team' && grant.subject_id === team.id));
    return inheritedTeams.length
      ? [{ module, label, description: `Via ${inheritedTeams.map(team => team.name).join(', ')}` }]
      : [];
  });
}
