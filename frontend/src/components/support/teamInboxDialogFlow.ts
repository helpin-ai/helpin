export type TeamInboxDialogStep = {
  label: string;
  description: string;
  fields: string[];
};

export function getTeamInboxDialogSteps(): TeamInboxDialogStep[] {
  return [
    {
      label: 'Details',
      description: 'Set up the inbox identity and reply expectations.',
      fields: ['identity', 'description', 'reply_expectations'],
    },
    {
      label: 'Members & Assignment',
      description: 'Choose the linked team, additional members, and how conversations are assigned.',
      fields: ['linked_team', 'additional_members', 'assignment_mode'],
    },
    {
      label: 'AI & Routing',
      description: 'Choose whether AI replies in this inbox and how Automated routing sends conversations to it.',
      fields: ['ai_replies', 'automated_routing_notice', 'manual_routing_rules', 'ai_routing_prompt'],
    },
    {
      label: 'Email',
      description: 'Forward your support email here and choose the sender address customers see.',
      fields: ['email_forwarding', 'sender_address'],
    },
  ];
}
