const WORKSPACE_ROLE_LABELS: Record<string, string> = {
  owner: 'Workspace owner',
  admin: 'Workspace admin',
  manager: 'Workspace manager',
  member: 'Workspace member',
  viewer: 'Workspace viewer',
};

const ORGANIZATION_ROLE_LABELS: Record<string, string> = {
  owner: 'Organization owner',
  admin: 'Organization admin',
  member: 'Organization member',
  viewer: 'Organization viewer',
};

export const ASSIGNABLE_ORGANIZATION_ROLES = ['admin', 'member', 'viewer'] as const;

export function canEditOrganizationMemberRole(
  actorRole: string | undefined,
  targetRole: string,
  isSelf: boolean,
): boolean {
  if (isSelf || targetRole === 'owner') return false;
  if (actorRole === 'owner') return true;
  return actorRole === 'admin' && targetRole !== 'admin';
}

export function canRemoveOrganizationMember(
  actorRole: string | undefined,
  targetRole: string,
  isSelf: boolean,
): boolean {
  if (isSelf || targetRole === 'owner') return false;
  if (actorRole === 'owner') return true;
  return actorRole === 'admin' && targetRole !== 'admin';
}

export function workspaceRoleLabel(role: string): string {
  const normalized = role.trim().toLowerCase();
  return WORKSPACE_ROLE_LABELS[normalized] ?? role;
}

export function organizationRoleLabel(role: string): string {
  const normalized = role.trim().toLowerCase();
  return ORGANIZATION_ROLE_LABELS[normalized] ?? role;
}
