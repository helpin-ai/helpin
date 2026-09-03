import { useCallback, useMemo } from 'react';
import { useWorkspaceSettings } from '@/hooks/queries/useSettings';
import type {
  TeamMembership,
  TeamUserMembership,
  WorkspacePerson,
  WorkspaceSettings,
  WorkspaceTeam,
} from '@/lib/types';

interface WorkspaceTeamsResult {
  teams: WorkspaceTeam[];
  people: WorkspacePerson[];
  memberships: TeamMembership[];
  userMemberships: TeamUserMembership[];
  loading: boolean;
  getTeamMembers: (teamId: string) => WorkspacePerson[];
  findTeamName: (teamId: string | null | undefined) => string | undefined;
}

interface TeamsSlice {
  teams: WorkspaceTeam[];
  people: WorkspacePerson[];
  memberships: TeamMembership[];
  userMemberships: TeamUserMembership[];
}

const EMPTY_SLICE: TeamsSlice = { teams: [], people: [], memberships: [], userMemberships: [] };

export function capitalizeTeamName(name: string): string {
  return name ? name.charAt(0).toUpperCase() + name.slice(1) : name;
}

function selectTeamsSlice(settings: WorkspaceSettings): TeamsSlice {
  return {
    teams: settings.teams.map((team) => ({ ...team, name: capitalizeTeamName(team.name) })),
    people: settings.people,
    memberships: settings.memberships,
    userMemberships: settings.user_memberships,
  };
}

/**
 * Teams, people and memberships for a workspace.
 *
 * Reads from the workspace settings query (already loaded by the workspace
 * layout), so there is no separate fetch or module-level cache. Any
 * `invalidateQueries(queryKeys.workspaces.settings(wsId))` refreshes it.
 */
export function useWorkspaceTeams(workspaceId: string | undefined): WorkspaceTeamsResult {
  const { data, isLoading } = useWorkspaceSettings(workspaceId ?? '');

  const slice = useMemo(() => (data ? selectTeamsSlice(data) : EMPTY_SLICE), [data]);
  const { teams, people, memberships, userMemberships } = slice;

  const getTeamMembers = useCallback((teamId: string): WorkspacePerson[] => {
    const memberIds = new Set(memberships.filter((m) => m.team_id === teamId).map((m) => m.person_id));
    return people.filter((p) => memberIds.has(p.id));
  }, [memberships, people]);

  const findTeamName = useCallback((teamId: string | null | undefined): string | undefined => {
    if (!teamId) return undefined;
    return teams.find((t) => t.id === teamId)?.name;
  }, [teams]);

  return { teams, people, memberships, userMemberships, loading: isLoading, getTeamMembers, findTeamName };
}
