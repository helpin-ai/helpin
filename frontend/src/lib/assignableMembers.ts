import type { AssignableMember } from './types';

export function formatAssignableMemberName(member: AssignableMember): string {
  const displayName = member.display_name?.trim() || member.email;
  if (member.status !== 'pending') {
    return displayName;
  }
  if (displayName.toLowerCase() === member.email.toLowerCase()) {
    return `${member.email} (Pending invite)`;
  }
  return `${displayName} (${member.email})`;
}

export function buildAssignableMemberOptions(members: AssignableMember[]) {
  return members.map((member) => ({
    id: member.id,
    name: formatAssignableMemberName(member),
  }));
}

export function buildAssignableMemberNameMap(members: AssignableMember[]) {
  const map = new Map<string, string>();
  for (const member of members) {
    const name = formatAssignableMemberName(member);
    map.set(member.id, name);
    if (member.user_id) {
      map.set(member.user_id, name);
    }
  }
  return map;
}
