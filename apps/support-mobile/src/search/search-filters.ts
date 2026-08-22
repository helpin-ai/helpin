import type { SupportConversationSearchParams } from '@helpin-ai/support-core'

export interface ConversationSearchFilters {
  sort: 'relevance' | 'newest' | 'oldest'
  statuses: string[]
  priorities: string[]
  assignedTo: string[]
  mailboxIds: string[]
  tagIds: string[]
  aiStates: string[]
  customerEmail: string
  title: string
  createdFrom: string
  createdTo: string
}

export const EMPTY_CONVERSATION_SEARCH_FILTERS: ConversationSearchFilters = {
  sort: 'relevance',
  statuses: [],
  priorities: [],
  assignedTo: [],
  mailboxIds: [],
  tagIds: [],
  aiStates: [],
  customerEmail: '',
  title: '',
  createdFrom: '',
  createdTo: '',
}

function csv(values: string[]) {
  return values.length > 0 ? values.join(',') : undefined
}

function trimmed(value: string) {
  const result = value.trim()
  return result || undefined
}

export function buildConversationSearchParams(
  query: string,
  filters: ConversationSearchFilters,
): SupportConversationSearchParams {
  return {
    q: trimmed(query),
    sort: filters.sort,
    statuses: csv(filters.statuses),
    priorities: csv(filters.priorities),
    assigned_to: csv(filters.assignedTo),
    mailbox_ids: csv(filters.mailboxIds),
    tag_ids: csv(filters.tagIds),
    ai: csv(filters.aiStates),
    customer_email: trimmed(filters.customerEmail),
    title: trimmed(filters.title),
    created_from: trimmed(filters.createdFrom),
    created_to: trimmed(filters.createdTo),
  }
}

export function activeConversationSearchFilterCount(filters: ConversationSearchFilters) {
  return [
    filters.statuses.length,
    filters.priorities.length,
    filters.assignedTo.length,
    filters.mailboxIds.length,
    filters.tagIds.length,
    filters.aiStates.length,
    filters.customerEmail.trim() ? 1 : 0,
    filters.title.trim() ? 1 : 0,
    filters.createdFrom ? 1 : 0,
    filters.createdTo ? 1 : 0,
    filters.sort !== 'relevance' ? 1 : 0,
  ].reduce((total, value) => total + value, 0)
}
