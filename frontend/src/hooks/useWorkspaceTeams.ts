import { useCallback, useEffect, useState } from 'react';
import { settingsService } from '@/lib/services/settingsService';
import type { WorkspaceTeam, WorkspacePerson, TeamMembership, TeamUserMembership } from '@/lib/types';

interface WorkspaceTeamsResult {
  teams: WorkspaceTeam[];
  people: WorkspacePerson[];
  memberships: TeamMembership[];
  userMemberships: TeamUserMembership[];
  loading: boolean;
  getTeamMembers: (teamId: string) => WorkspacePerson[];
  findTeamName: (teamId: string | undefined) => string | undefined;
}

// Module-level cache keyed by workspace ID.
let cachedWorkspaceId: string | null = null;
let cachedTeams: WorkspaceTeam[] = [];
let cachedPeople: WorkspacePerson[] = [];
let cachedMemberships: TeamMembership[] = [];
let cachedUserMemberships: TeamUserMembership[] = [];
let fetchPromise: Promise<void> | null = null;
let listeners: (() => void)[] = [];

/** Clear the teams cache and notify all mounted hook instances to re-fetch. */
export function invalidateWorkspaceTeamsCache() {
  cachedWorkspaceId = null;
  cachedTeams = [];
  cachedPeople = [];
  cachedMemberships = [];
  cachedUserMemberships = [];
  fetchPromise = null;
  listeners.forEach((l) => l());
}

export function useWorkspaceTeams(workspaceId: string | undefined): WorkspaceTeamsResult {
  const [teams, setTeams] = useState<WorkspaceTeam[]>(cachedWorkspaceId === workspaceId ? cachedTeams : []);
  const [people, setPeople] = useState<WorkspacePerson[]>(cachedWorkspaceId === workspaceId ? cachedPeople : []);
  const [memberships, setMemberships] = useState<TeamMembership[]>(cachedWorkspaceId === workspaceId ? cachedMemberships : []);
  const [userMemberships, setUserMemberships] = useState<TeamUserMembership[]>(cachedWorkspaceId === workspaceId ? cachedUserMemberships : []);
  const [loading, setLoading] = useState(false);
  const [version, setVersion] = useState(0);

  // Subscribe to cache invalidation events.
  useEffect(() => {
    const listener = () => setVersion((v) => v + 1);
    listeners.push(listener);
    return () => { listeners = listeners.filter((l) => l !== listener); };
  }, []);

  const load = useCallback(async () => {
    if (!workspaceId) return;
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
          cachedUserMemberships = data.user_memberships;
        }
      })();
    }

    await fetchPromise;
    fetchPromise = null;
    setTeams(cachedTeams);
    setPeople(cachedPeople);
    setMemberships(cachedMemberships);
    setUserMemberships(cachedUserMemberships);
    setLoading(false);
  }, [workspaceId]);

  useEffect(() => {
    if (!workspaceId) return;

    // Return cached data if it matches.
    if (cachedWorkspaceId === workspaceId && cachedTeams.length > 0) {
      setTeams(cachedTeams);
      setPeople(cachedPeople);
      setMemberships(cachedMemberships);
      setUserMemberships(cachedUserMemberships);
      return;
    }

    load();
  }, [workspaceId, version, load]);

  const getTeamMembers = (teamId: string): WorkspacePerson[] => {
    const memberIds = new Set(memberships.filter((m) => m.team_id === teamId).map((m) => m.person_id));
    return people.filter((p) => memberIds.has(p.id));
  };

  const findTeamName = (teamId: string | undefined): string | undefined => {
    if (!teamId) return undefined;
    return teams.find((t) => t.id === teamId)?.name;
  };

  return { teams, people, memberships, userMemberships, loading, getTeamMembers, findTeamName };
}
