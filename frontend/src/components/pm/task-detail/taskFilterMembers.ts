import type { AssignableMember, TeamUserMembership } from '@/lib/types';

export function filterAssignableMembersForTeam(
  members: AssignableMember[],
  teamId: string | null | undefined,
  userMemberships: TeamUserMembership[],
) {
  const activeMembers = members.filter((member) => member.status === 'active');

  if (!teamId) {
    return activeMembers;
  }

  const allowedUserIds = new Set(
    userMemberships
      .filter((membership) => membership.team_id === teamId)
      .map((membership) => membership.user_id),
  );

  return activeMembers.filter((member) => member.user_id && allowedUserIds.has(member.user_id));
}
