export type MailboxMemberOption = {
  id: string;
};

export type MailboxTeamOption = {
  id: string;
};

export type MailboxSupportAccessGrant = {
  module: string;
  subject_type: string;
  subject_id: string;
};

export type MailboxTeamUserMembership = {
  team_id: string;
  user_id: string;
};

export type MailboxSupportMemberOption = MailboxMemberOption & {
  user_id?: string;
  role?: string;
};

export function splitMailboxMembersBySelection<T>(
  members: T[],
  selectedIDs: string[],
  getMemberID: (member: T) => string = (member) => (member as MailboxMemberOption).id,
): { selected: T[]; available: T[] } {
  const byID = new Map(members.map((member) => [getMemberID(member), member]));
  const selected = selectedIDs
    .map((id) => byID.get(id))
    .filter((member): member is T => Boolean(member));
  const selectedSet = new Set(selected.map((member) => getMemberID(member)));
  const available = members.filter((member) => !selectedSet.has(getMemberID(member)));

  return { selected, available };
}

function isSupportGrant(grant: MailboxSupportAccessGrant) {
  return grant.module === 'support';
}

function isWorkspaceAdminRole(role: string | undefined) {
  return role === 'owner' || role === 'admin';
}

export function getSupportAccessTeamIDs(grants: MailboxSupportAccessGrant[]) {
  return new Set(
    grants
      .filter((grant) => isSupportGrant(grant) && grant.subject_type === 'team')
      .map((grant) => grant.subject_id),
  );
}

export function getSupportAccessMemberIDs(grants: MailboxSupportAccessGrant[]) {
  return new Set(
    grants
      .filter((grant) => isSupportGrant(grant) && grant.subject_type === 'workspace_member')
      .map((grant) => grant.subject_id),
  );
}

export function filterSupportAccessibleTeams<T extends MailboxTeamOption>(
  teams: T[],
  grants: MailboxSupportAccessGrant[],
) {
  const supportTeamIDs = getSupportAccessTeamIDs(grants);
  return teams.filter((team) => supportTeamIDs.has(team.id));
}

export function buildSupportTeamOptions<T extends MailboxTeamOption>(
  teams: T[],
  grants: MailboxSupportAccessGrant[],
) {
  const supportTeamIDs = getSupportAccessTeamIDs(grants);
  return teams.map((team) => {
    const hasSupportAccess = supportTeamIDs.has(team.id);
    return {
      team,
      hasSupportAccess,
      disabledReason: hasSupportAccess
        ? null
        : 'This team does not have access to the Support module.',
    };
  });
}

export function memberHasSupportAccess<T extends MailboxSupportMemberOption>(
  member: T,
  grants: MailboxSupportAccessGrant[],
  teamMemberships: MailboxTeamUserMembership[],
) {
  if (isWorkspaceAdminRole(member.role)) return true;

  const supportMemberIDs = getSupportAccessMemberIDs(grants);
  if (supportMemberIDs.has(member.id)) return true;

  if (!member.user_id) return false;

  const supportTeamIDs = getSupportAccessTeamIDs(grants);
  return teamMemberships.some(
    (membership) =>
      membership.user_id === member.user_id && supportTeamIDs.has(membership.team_id),
  );
}

export function filterSupportAccessibleMembers<T extends MailboxSupportMemberOption>(
  members: T[],
  grants: MailboxSupportAccessGrant[],
  teamMemberships: MailboxTeamUserMembership[],
) {
  return members.filter((member) => memberHasSupportAccess(member, grants, teamMemberships));
}

export function buildSupportMemberOptions<T extends MailboxSupportMemberOption>(
  members: T[],
  grants: MailboxSupportAccessGrant[],
  teamMemberships: MailboxTeamUserMembership[],
) {
  return members.map((member) => {
    const hasSupportAccess = memberHasSupportAccess(member, grants, teamMemberships);
    return {
      member,
      hasSupportAccess,
      disabledReason: hasSupportAccess
        ? null
        : 'This member does not have access to the Support module.',
    };
  });
}

export function filterMembersOutsideLinkedTeam<T extends MailboxSupportMemberOption>(
  members: T[],
  linkedTeamID: string,
  teamMemberships: MailboxTeamUserMembership[],
) {
  if (!linkedTeamID || linkedTeamID === 'none') return members;

  const linkedTeamUserIDs = new Set(
    teamMemberships
      .filter((membership) => membership.team_id === linkedTeamID)
      .map((membership) => membership.user_id),
  );

  return members.filter((member) => !member.user_id || !linkedTeamUserIDs.has(member.user_id));
}
