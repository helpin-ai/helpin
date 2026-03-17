import { useMemo } from 'react';
import { useWorkspaceTeams } from '@/hooks/useWorkspaceTeams';
import { useWorkspaceAccess, usePermissions } from '@/hooks/queries/useSession';
import type { WorkspaceTeam } from '@/lib/types';

/**
 * useAccessibleTeams returns teams filtered by the current user's access level.
 * Admins/owners see all teams. Members/viewers see only their own teams.
 */
export function useAccessibleTeams(workspaceId: string) {
  const teamsResult = useWorkspaceTeams(workspaceId);
  const { data: access } = useWorkspaceAccess(workspaceId);
  const { isAdmin } = usePermissions(access);

  const accessibleTeams: WorkspaceTeam[] = useMemo(() => {
    if (isAdmin) return teamsResult.teams;
    const myTeamIds = new Set(
      (access?.team_memberships ?? []).map((tm) => tm.team_id),
    );
    return teamsResult.teams.filter((t) => myTeamIds.has(t.id));
  }, [teamsResult.teams, access?.team_memberships, isAdmin]);

  const hasTeams = accessibleTeams.length > 0;

  return {
    ...teamsResult,
    /** All workspace teams (unfiltered) */
    allTeams: teamsResult.teams,
    /** Teams the current user can access */
    teams: accessibleTeams,
    /** Whether the user has any accessible teams */
    hasTeams,
    /** Whether the user is admin/owner (sees all teams) */
    isAdmin,
  };
}
