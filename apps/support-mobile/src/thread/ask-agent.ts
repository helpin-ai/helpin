import type { SupportConversation, SupportDockPageContext } from '@helpin-ai/support-core'

export const SUPPORT_AGENT_STARTERS = [
  {
    label: 'Draft a reply',
    prompt: 'Review this support conversation and draft a helpful, accurate reply to the customer. Flag anything that needs clarification before sending.',
  },
  {
    label: 'Investigate the issue',
    prompt: 'Investigate the customer issue using this conversation. Identify the likely cause, supporting evidence, and the best next step.',
  },
  {
    label: 'Summarize next steps',
    prompt: 'Summarize this conversation, including key facts, unresolved questions, and the recommended next steps.',
  },
] as const

export function buildSupportDockPageContext(conversation: SupportConversation): SupportDockPageContext {
  const displayTitle = [
    conversation.subject,
    conversation.customer_name,
    conversation.customer_email,
  ].find((value) => value?.trim())?.trim() || `Conversation ${conversation.id}`
  return {
    entity_type: 'support_conversation',
    entity_id: conversation.id,
    display_title: displayTitle,
  }
}

export function newSupportDockClientMessageId(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') return crypto.randomUUID()
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (character) => {
    const random = Math.floor(Math.random() * 16)
    const value = character === 'x' ? random : (random & 0x3) | 0x8
    return value.toString(16)
  })
}

export function supportAgentRunLabel(status?: string | null): string {
  if (status === 'queued' || status === 'running') return 'Agent is working…'
  if (status === 'paused') return 'Agent is waiting for input'
  if (status === 'failed') return 'The last agent turn failed'
  if (status === 'cancelled') return 'The last agent turn was cancelled'
  return 'Ask about this conversation'
}
