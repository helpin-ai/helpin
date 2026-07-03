export const CHAT_WIDGET_ROUTING_ASSIGNMENT_DESCRIPTION =
  'New widget conversations start in Shared Inbox. Configure human handoff ownership in Routing & Assignment.';

export function buildChatWidgetHandoffSummary({
  mailboxName,
  behavior,
  teamName,
}: {
  mailboxName: string;
  behavior: string;
  teamName?: string | null;
}) {
  if (behavior === 'round_robin') {
    return `When AI hands off to a human, the conversation moves to ${mailboxName} and is assigned by round robin.`;
  }
  if (behavior === 'assign_to_team') {
    return `When AI hands off to a human, the conversation moves to ${mailboxName} and is assigned to ${teamName || 'the selected team'}.`;
  }
  return `When AI hands off to a human, the conversation moves to ${mailboxName} and remains unassigned.`;
}
