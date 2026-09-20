// One meeting story shared by the full platform preview and focused workflows.
export const MEETING_TITLE = 'Northstar Labs · SSO rollout review';
export const MEETING_SUMMARY = 'Maya outlined Northstar Labs’ requirements for an Okta SSO rollout. The team agreed to validate group-to-role mapping with an admin-only pilot. Sam will share the setup guide and pilot checklist before the wider rollout.';
export const MEETING_EVIDENCE = [
  { kind: 'Decision', title: 'Start with an admin-only pilot.', time: '12:08', person: 'maya' as const, name: 'Maya Chen', quote: 'Let’s start with our admins before we open this up to the rest of the team.', note: 'Begin the rollout with a smaller group before wider adoption.' },
  { kind: 'Open question', title: 'Confirm how groups map to roles.', time: '19:32', person: 'maya' as const, name: 'Maya Chen', quote: 'Can we map our existing Okta groups to the right roles before the pilot?', note: 'Group-to-role mapping needs validation before the rollout.' },
  { kind: 'Action item', title: 'Share the setup guide and checklist.', time: '28:14', person: 'sam' as const, name: 'Sam Rivera', quote: 'I’ll send the reviewed setup guide and pilot checklist.', note: 'Sam will share the reviewed materials as the next step.' },
];
export const FOLLOWUP_DRAFT = 'Hi Maya,\n\nThanks for walking us through Northstar’s rollout requirements today.\n\nWe’ll start with an admin-only pilot. I’ll send the reviewed Okta setup guide and pilot checklist.\n\nPlease share your security checklist so we can confirm the remaining requirements before the wider rollout.\n\nThanks,\nSam';
