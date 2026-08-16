import type { CommandBarPageContext } from '@/lib/pmTypes';

type ContextType = CommandBarPageContext['entity_type'];

const STARTER_SUGGESTIONS: Record<ContextType, string[]> = {
  support_conversation: [
    'Draft a reply to the customer',
    'Investigate the issue and likely cause',
    'Summarize the conversation and recommend next steps',
  ],
  task: [
    'Review this task and suggest next steps',
    'Break this work into an implementation plan',
    'Identify blockers and risks',
  ],
  epic: [
    'Summarize progress and next priorities',
    'Review open work and identify blockers',
    'Create a delivery plan',
  ],
  document: [
    'Summarize this document',
    'Improve this document for clarity',
    'Find gaps and recommend updates',
  ],
  crm_contact: [
    'Summarize this customer context',
    'Suggest the best next outreach',
    'Identify risks and opportunities',
  ],
  crm_deal: [
    'Summarize this deal and recommend next steps',
    'Prepare a follow-up message',
    'Identify risks and opportunities',
  ],
  repository: [
    'Explain how this repository is structured',
    'Investigate a bug or improvement',
    'Identify the next change to make',
  ],
  workspace: [
    'Help me prioritize the next work',
    'Find relevant work or documentation',
    'Investigate an issue',
  ],
};

export function starterSuggestionsForContext(contextType?: ContextType): string[] {
  return STARTER_SUGGESTIONS[contextType ?? 'workspace'];
}
