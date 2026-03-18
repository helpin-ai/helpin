import type { AssignableMember, WorkspaceTeam } from '@/lib/types';

export type MentionableTeam = Pick<WorkspaceTeam, 'id' | 'name' | 'handle'>;

export interface MentionSuggestionItem {
  id: string;
  type: 'member' | 'team';
  handle: string;
  label: string;
  secondaryText?: string;
  avatarUrl?: string;
}

const normalizeMentionHandle = (value: string) =>
  value
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9._\-\s]/g, '')
    .replace(/\s+/g, '.')
    .replace(/^\.+|\.+$/g, '');

export const getMemberMentionHandle = (member: AssignableMember) => {
  const displayNameHandle = normalizeMentionHandle(member.display_name);
  if (displayNameHandle) {
    return displayNameHandle;
  }

  const emailHandle = normalizeMentionHandle(member.email.split('@')[0] ?? '');
  if (emailHandle) {
    return emailHandle;
  }

  return normalizeMentionHandle(member.user_id ?? member.id);
};

const buildMemberSuggestion = (member: AssignableMember): MentionSuggestionItem | null => {
  const handle = getMemberMentionHandle(member);
  if (!handle) {
    return null;
  }

  return {
    id: member.user_id ?? member.id,
    type: 'member',
    handle,
    label: member.display_name || member.email,
    secondaryText: member.email,
    avatarUrl: member.avatar_url,
  };
};

const buildTeamSuggestion = (team: MentionableTeam): MentionSuggestionItem | null => {
  const handle = normalizeMentionHandle(team.handle ?? '');
  if (!handle) {
    return null;
  }

  return {
    id: team.id,
    type: 'team',
    handle,
    label: team.name,
    secondaryText: `@${handle}`,
  };
};

const matchesMentionQuery = (item: MentionSuggestionItem, query: string) => {
  if (!query) {
    return true;
  }

  return [
    item.handle,
    item.label.toLowerCase(),
    item.secondaryText?.toLowerCase() ?? '',
  ].some((value) => value.includes(query));
};

const compareMentionSuggestions = (query: string) => (a: MentionSuggestionItem, b: MentionSuggestionItem) => {
  const aStartsWith = a.handle.startsWith(query) ? 0 : 1;
  const bStartsWith = b.handle.startsWith(query) ? 0 : 1;
  if (aStartsWith !== bStartsWith) {
    return aStartsWith - bStartsWith;
  }

  if (a.type !== b.type) {
    return a.type === 'member' ? -1 : 1;
  }

  return a.label.localeCompare(b.label);
};

export function filterMentionTeams(
  teams: MentionableTeam[],
  selectedTeamIds: string[] = [],
): MentionableTeam[] {
  const selectedIds = new Set(selectedTeamIds.filter(Boolean));
  const candidates = selectedIds.size > 0
    ? teams.filter((team) => selectedIds.has(team.id))
    : teams;

  return candidates.filter((team) => Boolean(normalizeMentionHandle(team.handle ?? '')));
}

export function getMentionSuggestions(
  query: string | null | undefined,
  members: AssignableMember[],
  teams: MentionableTeam[] = [],
  limit = 8,
): MentionSuggestionItem[] {
  const normalizedQuery = normalizeMentionHandle(query ?? '');
  const items = [
    ...members
      .map(buildMemberSuggestion)
      .filter((item): item is MentionSuggestionItem => item !== null),
    ...teams
      .map(buildTeamSuggestion)
      .filter((item): item is MentionSuggestionItem => item !== null),
  ]
    .filter((item) => matchesMentionQuery(item, normalizedQuery))
    .sort(compareMentionSuggestions(normalizedQuery));

  return items.slice(0, limit);
}
