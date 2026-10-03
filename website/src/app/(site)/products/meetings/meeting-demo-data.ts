// One meeting story shared by the full platform preview and focused workflows.
export const MEETING_TITLE = 'Northstar Labs · SSO rollout review';
export const MEETING_SUMMARY = 'Admin-only SSO pilot agreed. Role mapping needs a check. Sam owns the guide; Maya will share security requirements.';
export const MEETING_EVIDENCE = [
  { kind: 'Decision', title: 'Run an admin-only SSO pilot first.', time: '12:08', person: 'maya' as const, name: 'Maya Chen', quote: 'Let’s begin with the admin team, then decide whether we’re ready for a wider rollout.', note: 'A wider rollout still needs a decision.' },
  { kind: 'Open question', title: 'Confirm the group-to-role mapping.', time: '19:32', person: 'maya' as const, name: 'Maya Chen', quote: 'We still need to check whether our Okta groups can map to the roles we use.', note: 'Validate the mapping before confirming the rollout approach.' },
  { kind: 'Action item', title: 'Send the setup guide and pilot checklist.', time: '28:14', person: 'sam' as const, name: 'Sam Rivera', quote: 'I’ll take care of sending the setup guide and the checklist for the pilot.', note: 'Suggested · Not yet a task' },
];
export const FOLLOWUP_DRAFT = 'Hi Maya,\n\nWe’ll start with the admin pilot. I’ll send the guide and checklist. Please share the remaining security requirements while we check role mapping.\n\nThanks,\nSam';
