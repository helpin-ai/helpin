// Prepared workspace example shared by the roster, findings, and work plan.
export const REVIEW_REQUEST = "Review what’s blocking Northstar’s rollout. Check the support history, code, docs, and renewal notes. Prepare the next steps and an update for Maya.";
export const REVIEW_DRAFT = "Hi Maya, we’ve confirmed why the full export is stopping early. The issue is being handled in the existing engineering task.\n\nWe know the smaller export does not give you the complete list you need. We’ll update you here once the fix has been released.";
export const REVIEW_STEPS = [
  'Review Maya’s support conversation',
  'Check the export code and guide',
  'Review the renewal conversation',
  'Prepare the next steps for EXP-142',
  'Draft Maya’s customer update',
];
export const REVIEW_RUNS = [
  { id: 'review', title: 'Northstar rollout review', agent: 'Ask Agent', icon: '/brand/helpin-icon-ink.svg', role: 'Coordinator', completeAt: 5, detail: '' },
  { id: 'customer', title: 'Support agent · Echo', agent: 'Echo · Support', icon: '/new/agents/echo.svg', role: 'Specialist', completeAt: 1, detail: 'Handed off from Maya’s chat: the smaller export worked, but she still needs the full list of 18,400 contacts.' },
  { id: 'engineering', title: 'Coding agent · Forge', agent: 'Forge · Coding', icon: '/new/agents/forge.svg', role: 'Specialist', completeAt: 2, detail: 'The current implementation returns the first page without retrieving the remaining contacts.' },
  { id: 'docs', title: 'Docs agent · Quill', agent: 'Quill · Docs', icon: '/new/agents/quill.svg', role: 'Specialist', completeAt: 2, detail: 'The guide explains how to start an export, but not how to report missing contacts.' },
  { id: 'update', title: 'CRM agent · Beacon', agent: 'Beacon · CRM', icon: '/new/agents/beacon.svg', role: 'Specialist', completeAt: 3, detail: 'Maya wants an export update before discussing the renewal.' },
] as const;
export type ReviewRunId = typeof REVIEW_RUNS[number]['id'];
