import { useEffect, useState } from 'react';
import { settingsService } from '@/lib/services/settingsService';
import type { WorkspaceTeam, WorkspacePerson, TeamMembership } from '@/lib/types';

interface WorkspaceTeamsResult {
  teams: WorkspaceTeam[];
  people: WorkspacePerson[];
  memberships: TeamMembership[];
  loading: boolean;
  getTeamMembers: (teamId: string) => WorkspacePerson[];
  findTeamName: (teamId: string | undefined) => string | undefined;
}

// Module-level cache keyed by workspace ID.
let cachedWorkspaceId: string | null = null;
let cachedTeams: WorkspaceTeam[] = [];
let cachedPeople: WorkspacePerson[] = [];
let cachedMemberships: TeamMembership[] = [];
let fetchPromise: Promise<void> | null = null;

export function useWorkspaceTeams(workspaceId: string | undefined): WorkspaceTeamsResult {
  const [teams, setTeams] = useState<WorkspaceTeam[]>(cachedWorkspaceId === workspaceId ? cachedTeams : []);
  const [people, setPeople] = useState<WorkspacePerson[]>(cachedWorkspaceId === workspaceId ? cachedPeople : []);
  const [memberships, setMemberships] = useState<TeamMembership[]>(cachedWorkspaceId === workspaceId ? cachedMemberships : []);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (!workspaceId) return;

    // Return cached data if it matches.
    if (cachedWorkspaceId === workspaceId && cachedTeams.length > 0) {
      setTeams(cachedTeams);
      setPeople(cachedPeople);
      setMemberships(cachedMemberships);
      return;
    }

    const load = async () => {
      // Deduplicate concurrent fetches.
      if (!fetchPromise || cachedWorkspaceId !== workspaceId) {
        setLoading(true);
        fetchPromise = (async () => {
          const { data } = await settingsService.getAll(workspaceId);
          if (data) {
            cachedWorkspaceId = workspaceId;
            cachedTeams = data.teams;
            cachedPeople = data.people;
            cachedMemberships = data.memberships;
          }
        })();
      }

      await fetchPromise;
      fetchPromise = null;
      setTeams(cachedTeams);
      setPeople(cachedPeople);
      setMemberships(cachedMemberships);
      setLoading(false);
    };

    load();
  }, [workspaceId]);

  const getTeamMembers = (teamId: string): WorkspacePerson[] => {
    const memberIds = new Set(memberships.filter((m) => m.team_id === teamId).map((m) => m.person_id));
    return people.filter((p) => memberIds.has(p.id));
  };

  const findTeamName = (teamId: string | undefined): string | undefined => {
    if (!teamId) return undefined;
    return teams.find((t) => t.id === teamId)?.name;
  };

  return { teams, people, memberships, loading, getTeamMembers, findTeamName };
}
