import type { SupportMailbox } from '@/lib/pmTypes';
import type { MailboxSupportMemberOption, MailboxTeamUserMembership } from './teamInboxDialogMembers';

export const inboxAssignmentOptions = [
  { value: 'manual', label: 'Manual', description: 'Leave conversations unassigned for someone to pick up.' },
  { value: 'specific_member', label: 'Specific member', description: 'Automatically assign to one selected member.' },
  { value: 'round_robin', label: 'Round robin', description: 'Take turns assigning conversations to selected members.' },
] as const;

export type InboxAssignmentMode = SupportMailbox['assignment_mode'];

// Input members must already have Support module access.
export function filterInboxAssignmentMembers<T extends MailboxSupportMemberOption & { status: string }>(
  members: T[], linkedTeamID: string, additionalIDs: string[], memberships: MailboxTeamUserMembership[],
): T[] {
  const teamUsers = new Set(memberships.filter(m => m.team_id === linkedTeamID).map(m => m.user_id));
  const additional = new Set(additionalIDs);
  return members.filter(member => member.status === 'active' && member.user_id && (
    member.role === 'owner' || member.role === 'admin' ||
    (member.role === 'member' && (additional.has(member.id) || teamUsers.has(member.user_id)))
  ));
}

export function resolveInboxAssignmentSelection(
  selectedIDs: string[] | null, eligibleMembers: { id: string }[], legacyMemberIDs: string[],
): string[] {
  const eligible = new Set(eligibleMembers.map(member => member.id));
  return (selectedIDs ?? legacyMemberIDs).filter(id => eligible.has(id));
}
