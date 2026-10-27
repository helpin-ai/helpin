import type { AskAIRequest } from '@/components/assistant/AskAIPanel'

/** OPEN_ASK_AI_EVENT opens the Ask AI panel from anywhere in the help center. */
export const OPEN_ASK_AI_EVENT = 'open-help-assistant'

/** openAskAI opens the Ask AI panel, optionally asking a question. */
export function openAskAI(request: AskAIRequest = {}) {
  window.dispatchEvent(new CustomEvent<AskAIRequest>(OPEN_ASK_AI_EVENT, { detail: request }))
}
