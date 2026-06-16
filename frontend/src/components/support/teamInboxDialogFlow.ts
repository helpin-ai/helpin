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
      label: 'Routing',
      description: 'Control how Automated routing sends conversations to this inbox.',
      fields: ['automated_routing_notice', 'manual_routing_rules', 'ai_routing_prompt'],
    },
  ];
}
