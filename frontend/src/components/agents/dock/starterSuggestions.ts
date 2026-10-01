import type { CommandBarPageContext } from '@/lib/pmTypes';

type ContextType = CommandBarPageContext['entity_type'];

export interface StarterSuggestion {
  label: string;
  prompt: string;
}

const STARTER_SUGGESTIONS: Record<ContextType, StarterSuggestion[]> = {
  support_coverage_gap: [
    { label: 'Prepare a fix', prompt: 'Investigate this coverage gap using the linked conversations and documents. Prepare the right fix for review. Ask only for essential missing facts. Keep documentation unpublished and leave the gap open.' },
    { label: 'Check the evidence', prompt: 'Check the current evidence for this gap. Explain what is missing and whether the recommended fix still fits. Link the source conversations and documents.' },
    { label: 'Find existing guidance', prompt: 'Find existing documentation that could answer this customer need. Explain whether we should improve it or fix retrieval, with links to the relevant articles.' },
  ],
  support_conversation: [
    { label: 'Draft a reply', prompt: 'Review this support conversation and draft a helpful, accurate reply to the customer. Flag anything that needs clarification before sending.' },
    { label: 'Investigate the issue', prompt: 'Investigate the customer issue using this conversation. Identify the likely cause, supporting evidence, and the best next step.' },
    { label: 'Summarize next steps', prompt: 'Summarize this conversation, including key facts, unresolved questions, and the recommended next steps.' },
  ],
  task: [
    { label: 'Review next steps', prompt: 'Review this task, its current context, and related work. Recommend the clearest next steps, owners, and any blockers.' },
    { label: 'Plan the work', prompt: 'Break this task into a practical implementation plan with ordered steps, dependencies, and any important risks.' },
    { label: 'Find blockers', prompt: 'Analyze this task for missing information, dependencies, risks, and blockers that need to be resolved before work can proceed.' },
  ],
  epic: [
    { label: 'Review priorities', prompt: 'Review this epic, its work items, and current progress. Recommend the highest-priority next work and explain why.' },
    { label: 'Find blockers', prompt: 'Identify delivery risks, blocked work, missing dependencies, and decisions that could affect this epic.' },
    { label: 'Create delivery plan', prompt: 'Create a practical delivery plan for this epic, including milestones, dependencies, sequencing, and risks.' },
  ],
  document: [
    { label: 'Summarize document', prompt: 'Summarize this document clearly, including its purpose, key decisions, important details, and open questions.' },
    { label: 'Improve clarity', prompt: 'Review this document for clarity, structure, and completeness. Recommend specific improvements without changing its intended meaning.' },
    { label: 'Find gaps', prompt: 'Identify missing information, ambiguities, outdated sections, and recommended updates in this document.' },
  ],
  crm_contact: [
    { label: 'Summarize customer', prompt: 'Summarize this customer context, including recent activity, important details, opportunities, and risks.' },
    { label: 'Plan outreach', prompt: 'Recommend the best next outreach to this contact, including the goal, timing, message angle, and supporting context.' },
    { label: 'Find risks', prompt: 'Identify relationship risks, opportunities, missing information, and recommended next actions for this contact.' },
  ],
  crm_deal: [
    { label: 'Review next steps', prompt: 'Review this deal and recommend the most valuable next steps to move it forward, including owners, timing, and risks.' },
    { label: 'Draft follow-up', prompt: 'Draft a concise, tailored follow-up message for this deal using its current context, open questions, and next objective.' },
    { label: 'Assess risks', prompt: 'Assess this deal for risks, objections, missing stakeholders, missing information, and opportunities to improve the chance of success.' },
  ],
  repository: [
    { label: 'Explain the codebase', prompt: 'Explain how this repository is structured, the role of its key areas, and where a developer should start for the current work.' },
    { label: 'Investigate an issue', prompt: 'Investigate a likely bug or improvement in this repository. Gather evidence first, identify the probable cause, and recommend a focused next step.' },
    { label: 'Identify next change', prompt: 'Review the repository context and identify the highest-value next change, including its likely scope, dependencies, and risks.' },
  ],
  workspace: [
    { label: 'Prioritize work', prompt: 'Review the available workspace context and recommend the highest-priority work to focus on next, explaining the trade-offs.' },
    { label: 'Find relevant work', prompt: 'Find relevant work, documents, and context for the topic I need help with, then summarize the most useful results.' },
    { label: 'Investigate an issue', prompt: 'Investigate an issue in the workspace by gathering relevant context, identifying likely causes, and recommending the best next step.' },
  ],
};

export function starterSuggestionsForContext(contextType?: ContextType): StarterSuggestion[] {
  return STARTER_SUGGESTIONS[contextType ?? 'workspace'];
}
