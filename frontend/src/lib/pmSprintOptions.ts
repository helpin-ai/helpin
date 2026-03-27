import type { SprintWithStats } from '@/lib/pmTypes';
import type { WorkspaceTeam } from '@/lib/types';

export interface SprintOptionGroup {
  key: string;
  label: string;
  options: Array<{ value: string; label: string }>;
}

export function buildSprintOptionGroups(
  sprints: SprintWithStats[],
  teams: WorkspaceTeam[],
  teamId?: string | null,
): SprintOptionGroup[] {
  if (teamId) {
    const teamName = teams.find((team) => team.id === teamId)?.name ?? 'Team sprints';
    const options = sprints
      .filter((entry) => entry.sprint.team_id === teamId)
      .map((entry) => ({ value: entry.sprint.id, label: entry.sprint.name }));

    return options.length > 0 ? [{ key: teamId, label: teamName, options }] : [];
  }

  const orderedGroups: SprintOptionGroup[] = [];
  const grouped = new Map<string, SprintOptionGroup>();

  for (const team of teams) {
    const key = team.id;
    const group = { key, label: team.name, options: [] as SprintOptionGroup['options'] };
    grouped.set(key, group);
    orderedGroups.push(group);
  }

  const workspaceGroup = {
    key: '__workspace__',
    label: 'Workspace',
    options: [] as SprintOptionGroup['options'],
  };
  grouped.set(workspaceGroup.key, workspaceGroup);
  orderedGroups.push(workspaceGroup);

  for (const entry of sprints) {
    const key = entry.sprint.team_id ?? workspaceGroup.key;
    const fallback = {
      key,
      label: key === workspaceGroup.key ? 'Workspace' : 'Other team',
      options: [] as SprintOptionGroup['options'],
    };
    const group = grouped.get(key) ?? fallback;
    if (!grouped.has(key)) {
      grouped.set(key, group);
      orderedGroups.push(group);
    }
    group.options.push({ value: entry.sprint.id, label: entry.sprint.name });
  }

  return orderedGroups.filter((group) => group.options.length > 0);
}
