// Prepared workspace example, shared by the roster, findings, and work plan.
export const REVIEW_REQUEST = "What’s blocking Northstar’s rollout? Review the customer history, engineering work, and docs. Then prepare the next steps.";
export const REVIEW_STEPS = [
  'Review Maya’s support conversation',
  'Check the linked engineering work',
  'Review the Okta setup guide',
  'Create the rollout checklist',
  'Draft Maya’s customer update',
];
export const REVIEW_RUNS = [
  { id: 'review', title: 'Northstar rollout review', agent: 'Ask Agent', icon: '/brand/helpin-icon-ink.svg', role: 'Workspace agent', completeAt: 5, detail: '' },
  { id: 'customer', title: 'Customer history', agent: 'CRM agent', icon: '/new/agents/beacon.svg', role: 'Specialist', completeAt: 1, detail: 'Maya needs the Okta setup instructions before Northstar’s security review. The conversation and rollout meeting agree on this next step.' },
  { id: 'engineering', title: 'Engineering readiness', agent: 'Code reviewer', icon: '/new/agents/lens.svg', role: 'Specialist', completeAt: 2, detail: 'Group-to-role mapping is in review. SCIM provisioning is still planned. Keep these separate in the rollout checklist so Maya can see what is ready and what remains.' },
  { id: 'docs', title: 'Docs review', agent: 'Docs agent', icon: '/new/agents/quill.svg', role: 'Specialist', completeAt: 3, detail: 'The Okta setup guide is available. It still needs group-mapping troubleshooting steps before the wider rollout.' },
  { id: 'update', title: 'Customer update', agent: 'Support agent', icon: '/new/agents/echo.svg', role: 'Specialist', completeAt: 5, detail: 'Hi Maya — the Okta setup guide is ready to share. Group mapping is in review, and SCIM provisioning is planned. We’re adding troubleshooting steps to the guide and will confirm the remaining rollout requirements with you.' },
] as const;
export type ReviewRunId = typeof REVIEW_RUNS[number]['id'];
