// Prepared examples. These tool names and composer patterns match Helpin's app.
export const RUN_PHASE_TIMES = [1200, 2800, 4600, 6400, 8200, 14800] as const;
export const RUN_CYCLE_MS = 20500;
export const RUN_COMPLETE_PHASE = RUN_PHASE_TIMES.length;

export const REVIEW_RUNS = [
  {
    id: 'review', title: 'Plan Northstar’s next steps', agent: 'Helpin AI', role: 'Coordinator',
    icon: '/brand/helpin-icon-ink.svg', context: 'Northstar rollout',
    request: 'Check what’s blocking Northstar’s rollout. Bring together the customer history, product work, and renewal notes, then prepare the next steps.',
    acknowledgement: 'I’ll connect the relevant work and ask the specialist agents to help.',
    tools: [
      { name: 'search_workspace', source: 'Support · Projects · CRM', label: 'Find Northstar’s related work', result: 'Found Maya’s conversation, EXP-142, and the renewal record.' },
      { name: 'read_document', source: 'Internal docs', label: 'Read the rollout checklist', result: 'Northstar needs the full contact export before going live.' },
      { name: 'start_agent_run', source: 'Echo · Forge · Quill', label: 'Ask specialists to check the blockers', result: 'Support history, export code, and the customer guide reviewed.' },
    ],
    resultTitle: 'A clear path to unblock the rollout',
    result: 'The incomplete export is the blocker. Keep the fix in EXP-142, retrieve the remaining pages, and add a regression test.\n\nQuill has identified missing guidance for reporting incomplete exports. Update Maya on the fix before following up about the renewal.',
    completion: 'Next steps ready for your review',
  },
  {
    id: 'customer', title: 'Answer Maya’s export question', agent: 'Echo', role: 'Support',
    icon: '/new/agents/echo.svg', context: 'Conversation #1245',
    request: 'Maya has asked about her export again. Check what she’s already tried and draft a helpful reply.',
    acknowledgement: 'I’ll read the earlier conversation and check the linked task before replying.',
    tools: [
      { name: 'list_conversation_messages', source: 'Support', label: 'Read Maya’s earlier messages', result: 'The smaller export worked, but Maya needs all 18,400 contacts.' },
      { name: 'get_task_context', source: 'Projects', label: 'Check the progress of EXP-142', result: 'The export fix is being investigated. No release is confirmed.' },
      { name: 'draft_support_reply', source: 'Support', label: 'Prepare a reply for review', result: 'Draft saved to Maya’s conversation. Not sent.' },
    ],
    resultTitle: 'A reply that picks up where Maya left off',
    result: 'Hi Maya, we know the smaller export doesn’t give you the complete list you need. We’ve linked your report to the existing engineering task.\n\nWe’ll update you here once the full-export fix has been released.',
    completion: 'Reply drafted · Not sent',
  },
  {
    id: 'engineering', title: 'Investigate the incomplete export', agent: 'Forge', role: 'Coding',
    icon: '/new/agents/forge.svg', context: 'EXP-142',
    request: 'Investigate EXP-142. Why does Maya’s export stop early, and what needs to change?',
    acknowledgement: 'I’ll check the customer’s report and trace the export code.',
    tools: [
      { name: 'get_task_context', source: 'Projects · Support', label: 'Read the task and customer report', result: 'The export stops at 10,000 of the expected 18,400 contacts.' },
      { name: 'repository_search', source: 'Code · GitHub', label: 'Find the contact export implementation', result: 'Found the export handler and its pagination helper.' },
      { name: 'read_files', source: 'Code · GitHub', label: 'Trace how the export retrieves contacts', result: 'The handler returns the first page without fetching the rest.' },
    ],
    resultTitle: 'The cause and a proposed fix',
    result: 'The export stops after the first page of contacts. Retrieve every page before finishing the file.\n\nAdd a regression test above 10,000 contacts and a test for a failed page. Keep the work in EXP-142 so Maya’s report stays connected to the fix.',
    completion: 'Investigation ready for review',
  },
  {
    id: 'docs', title: 'Improve the export guide', agent: 'Quill', role: 'Docs',
    icon: '/new/agents/quill.svg', context: 'Contact export guide',
    request: 'Use Maya’s unanswered question to improve our export guide. Prepare the changes for review.',
    acknowledgement: 'I’ll compare the guide with Maya’s report and draft the missing guidance.',
    tools: [
      { name: 'list_conversation_messages', source: 'Support', label: 'Read the unanswered export question', result: 'Maya needed help reporting missing contacts.' },
      { name: 'read_document', source: 'Help center', label: 'Check the current export guide', result: 'The guide explains exporting, but not what to do when records are missing.' },
      { name: 'publish_document_change_proposal', source: 'Docs', label: 'Prepare an article update', result: 'A troubleshooting section is ready for the team to review.' },
    ],
    resultTitle: 'A new section for the export guide',
    result: 'Missing contacts in your export? Share the expected contact count, your export filters, and when you ran the export with support.\n\nThese details help the team investigate without asking you to repeat the same information.',
    completion: 'Article update prepared · Not published',
  },
  {
    id: 'update', title: 'Prepare Maya’s renewal follow-up', agent: 'Beacon', role: 'CRM',
    icon: '/new/agents/beacon.svg', context: 'Northstar renewal',
    request: 'Check Northstar’s renewal and recent conversations. Draft the right follow-up for Maya.',
    acknowledgement: 'I’ll check the deal, support history, and meeting notes first.',
    tools: [
      { name: 'get_crm_deal', source: 'CRM', label: 'Read Northstar’s renewal record', result: 'Sam owns the renewal. Maya is the main contact.' },
      { name: 'list_conversation_messages', source: 'Support', label: 'Check Maya’s unresolved issue', result: 'Maya is still waiting for the full contact export.' },
      { name: 'search_workspace', source: 'Meetings', label: 'Find the rollout meeting notes', result: 'Maya asked to resolve the export before discussing the renewal.' },
    ],
    resultTitle: 'A follow-up that respects what Maya needs',
    result: 'Hi Maya, we know you’re waiting for the full export. We’ll update you once the fix is released, then pick up the renewal conversation when you’re ready.\n\nSam, follow up after the export issue is resolved.',
    completion: 'Follow-up drafted · Not sent',
  },
] as const;
export type ReviewRunId = typeof REVIEW_RUNS[number]['id'];
export type DemoTool = { name: string; source: string; label: string; result: string };

export const CHAT_RUN = {
  id: 'chat', title: 'Prepare Maya’s customer update', agent: 'Helpin AI', role: 'Assistant',
  icon: '/brand/helpin-icon-ink.svg', context: 'Conversation #1245',
  request: 'What should I tell Maya about her export? Check the latest task update and draft a reply.',
  acknowledgement: 'I’ll check what Maya last heard and whether the fix is ready.',
  tools: [
    { name: 'list_conversation_messages', source: 'Support', label: 'Read Maya’s latest conversation', result: 'Maya is waiting for a complete export of 18,400 contacts.' },
    { name: 'get_task_context', source: 'Projects', label: 'Check the latest update on EXP-142', result: 'The pagination issue is confirmed. The fix has not been released.' },
    { name: 'read_document', source: 'Internal docs', label: 'Check the customer update guidelines', result: 'Explain what is confirmed and avoid promising a release date.' },
  ],
  resultTitle: 'Here’s a reply you can review',
  result: 'Hi Maya, we’ve confirmed why your export stops early, and the team is working on the fix. You don’t need to repeat the smaller-export workaround.\n\nWe’ll update you here once the complete export is ready. Thanks for sharing the details that helped us investigate.',
  completion: 'Reply ready for review · Not sent',
} as const;
export type DemoRunId = ReviewRunId | 'chat';
