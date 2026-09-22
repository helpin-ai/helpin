// One meeting story shared by the full platform preview and focused workflows.
export const MEETING_TITLE = 'Northstar Labs · SSO rollout review';
export const MEETING_SUMMARY = 'Northstar will start with an admin-only SSO pilot before considering a wider rollout. Group-to-role mapping still needs validation. Sam will share the setup guide and pilot checklist. Maya will provide the remaining security requirements.';
export const MEETING_EVIDENCE = [
  { kind: 'Decision', title: 'Run an admin-only SSO pilot first.', time: '12:08', person: 'maya' as const, name: 'Maya Chen', quote: 'Let’s begin with the admin team, then decide whether we’re ready for a wider rollout.', note: 'A wider rollout is a later decision—not something already approved.' },
  { kind: 'Open question', title: 'Confirm the group-to-role mapping.', time: '19:32', person: 'maya' as const, name: 'Maya Chen', quote: 'We still need to check whether our Okta groups can map to the roles we use.', note: 'Validate the mapping before confirming the rollout approach.' },
  { kind: 'Action item', title: 'Send the setup guide and pilot checklist.', time: '28:14', person: 'sam' as const, name: 'Sam Rivera', quote: 'I’ll take care of sending the setup guide and the checklist for the pilot.', note: 'Suggested · Not yet a task' },
];
export const FOLLOWUP_DRAFT = 'Hi Maya,\n\nWe agreed to begin with an admin-only SSO pilot before considering a wider rollout.\n\nI’ll send the setup guide and pilot checklist. We also need to confirm how your existing Okta groups map to the product roles.\n\nPlease share your remaining security requirements so we can review those alongside the pilot setup.\n\nThanks,\nSam';
