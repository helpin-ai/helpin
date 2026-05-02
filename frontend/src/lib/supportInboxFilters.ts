import type { SupportConversation } from './pmTypes';
import type { NavFilter } from '@/stores/supportInboxStore';
import {
  isAIActiveConversation,
  isHumanInboxConversation,
  isHumanResolvedConversation,
  isMineActionableConversation,
  isResolvedByAIConversation,
} from '@/components/support/helpers';

export type ConversationAssignmentFilter = 'default' | 'me' | 'unassigned' | 'others';
export type ConversationAIFilter = 'default' | 'ai_handling' | 'needs_human' | 'resolved_by_ai';
export type ConversationSortOrder = 'newest' | 'oldest';
export type ConversationStateFilter = 'open' | 'waiting_on_customer' | 'resolved' | 'spam';

export interface ConversationListFilters {
  states: ConversationStateFilter[];
  assignment: ConversationAssignmentFilter;
  ai: ConversationAIFilter;
  sort: ConversationSortOrder;
}

export const DEFAULT_CONVERSATION_LIST_FILTERS: ConversationListFilters = {
  states: ['open'],
  assignment: 'default',
  ai: 'default',
  sort: 'newest',
};

export type ConversationListRequestFilters = {
  status?: string;
  filter?: string;
  mailbox_id?: string;
  flow_state?: string;
  search?: string;
  assigned_to?: string;
  sort?: string;
  statuses?: string;
};

export function defaultStatesForNav(navFilter: NavFilter): ConversationStateFilter[] {
  switch (navFilter) {
    case 'mine':
      return ['open', 'waiting_on_customer'];
    case 'waiting':
      return ['waiting_on_customer'];
    case 'resolved':
    case 'resolved_by_ai':
      return ['resolved'];
    case 'spam':
      return ['spam'];
    case 'inbox':
    case 'ai_active':
    default:
      return ['open'];
  }
}

export function defaultConversationListFiltersForNav(navFilter: NavFilter): ConversationListFilters {
  return {
    states: defaultStatesForNav(navFilter),
    assignment: 'default',
    ai: 'default',
    sort: 'newest',
  };
}

export function statesEqual(a: ConversationStateFilter[], b: ConversationStateFilter[]): boolean {
  if (a.length !== b.length) return false;
  return a.every((value, index) => value === b[index]);
}

export function normalizeConversationListFilters(
  filters: Partial<ConversationListFilters> | undefined,
  navFilter: NavFilter,
): ConversationListFilters {
  const defaults = defaultConversationListFiltersForNav(navFilter);
  return {
    states: filters?.states && filters.states.length > 0 ? filters.states : defaults.states,
    assignment: filters?.assignment ?? defaults.assignment,
    ai: filters?.ai ?? defaults.ai,
    sort: filters?.sort ?? defaults.sort,
  };
}

export function hasConversationListChanges({
  navFilter,
  selectedMailboxId,
  searchQuery,
  listFilters,
}: {
  navFilter: NavFilter;
  selectedMailboxId: string;
  searchQuery: string;
  listFilters: Partial<ConversationListFilters> | undefined;
}): boolean {
  const normalized = normalizeConversationListFilters(listFilters, navFilter);
  return (
    selectedMailboxId !== 'all' ||
    searchQuery.trim().length > 0 ||
    !statesEqual(normalized.states, defaultStatesForNav(navFilter)) ||
    normalized.assignment !== 'default' ||
    normalized.ai !== 'default' ||
    normalized.sort !== 'newest'
  );
}

export function buildSupportInboxViewFilters({
  navFilter,
  selectedMailboxId,
  searchQuery,
  listFilters,
}: {
  navFilter: NavFilter;
  selectedMailboxId: string;
  searchQuery: string;
  listFilters: ConversationListFilters;
}): Record<string, string> {
  const normalizedFilters = normalizeConversationListFilters(listFilters, navFilter);
  const filters: Record<string, string> = {
    nav_filter: navFilter,
  };
  filters.states = normalizedFilters.states.join(',');
  if (selectedMailboxId !== 'all') filters.mailbox_id = selectedMailboxId;
  if (searchQuery.trim()) filters.search = searchQuery.trim();
  if (normalizedFilters.assignment !== 'default') filters.assignment = normalizedFilters.assignment;
  if (normalizedFilters.ai !== 'default') filters.ai = normalizedFilters.ai;
  if (normalizedFilters.sort !== 'newest') filters.sort = normalizedFilters.sort;
  return filters;
}

export function buildConversationListRequestFilters({
  navFilter,
  selectedMailboxId,
  searchQuery,
  listFilters = DEFAULT_CONVERSATION_LIST_FILTERS,
}: {
  navFilter: NavFilter;
  selectedMailboxId: string;
  searchQuery: string;
  listFilters?: ConversationListFilters;
}): ConversationListRequestFilters | undefined {
  const f: ConversationListRequestFilters = {};
  const normalizedFilters = normalizeConversationListFilters(listFilters, navFilter);
  const hasExplicitAI = normalizedFilters.ai !== 'default';
  const selectedStates = normalizedFilters.states;
  const hasExplicitStates = !statesEqual(selectedStates, defaultStatesForNav(navFilter));

  if (selectedMailboxId !== 'all') {
    f.mailbox_id = selectedMailboxId;
  }

  if (hasExplicitAI) {
    switch (normalizedFilters.ai) {
      case 'ai_handling':
        f.flow_state = 'ai_handling';
        break;
      case 'needs_human':
        f.flow_state = 'waiting_for_human';
        break;
      case 'resolved_by_ai':
        f.flow_state = 'resolved_by_ai';
        break;
    }
  } else if (!hasExplicitStates) {
    switch (navFilter) {
      case 'inbox':
        f.filter = 'inbox';
        break;
      case 'mine':
        f.filter = 'mine';
        break;
      case 'waiting':
        f.status = 'waiting_on_customer';
        break;
      case 'resolved':
        f.filter = 'resolved';
        break;
      case 'spam':
        f.status = 'spam';
        break;
      case 'ai_active':
        f.flow_state = 'ai_handling';
        break;
      case 'resolved_by_ai':
        f.flow_state = 'resolved_by_ai';
        break;
    }
  } else {
    switch (navFilter) {
      case 'mine':
        f.filter = 'mine';
        break;
      case 'ai_active':
        f.flow_state = 'ai_handling';
        break;
      case 'resolved_by_ai':
        f.flow_state = 'resolved_by_ai';
        break;
    }
  }

  if (searchQuery.trim()) f.search = searchQuery.trim();
  if (hasExplicitStates) f.statuses = selectedStates.join(',');
  if (normalizedFilters.assignment !== 'default') f.assigned_to = normalizedFilters.assignment;
  if (normalizedFilters.sort !== 'newest') f.sort = normalizedFilters.sort;
  return Object.keys(f).length > 0 ? f : undefined;
}

function matchesMailboxScope(conversation: SupportConversation, mailboxScope: string): boolean {
  if (mailboxScope === 'all') {
    return true;
  }
  if (mailboxScope === 'shared') {
    return !conversation.mailbox_id;
  }
  return conversation.mailbox_id === mailboxScope;
}

export function filterSupportConversations(
  conversations: SupportConversation[],
  options: {
    navFilter: NavFilter;
    mailboxScope: string;
    statusFilter?: string;
    userId?: string;
    searchQuery: string;
    sortOrder?: ConversationSortOrder;
    skipViewFilter?: boolean;
  }
): SupportConversation[] {
  const { navFilter, mailboxScope, statusFilter = 'all', userId, searchQuery, sortOrder = 'newest', skipViewFilter = false } = options;
  let result = conversations.filter((conversation) => matchesMailboxScope(conversation, mailboxScope));

  if (statusFilter !== 'all') {
    result = result.filter((conversation) => conversation.status === statusFilter);
  }

  if (!skipViewFilter) {
    if (navFilter === 'inbox') {
      result = result.filter(isHumanInboxConversation);
    } else if (navFilter === 'mine') {
      result = result.filter((conversation) => isMineActionableConversation(conversation, userId));
    } else if (navFilter === 'waiting') {
      result = result.filter((conversation) => conversation.status === 'waiting_on_customer');
    } else if (navFilter === 'resolved') {
      result = result.filter(isHumanResolvedConversation);
    } else if (navFilter === 'spam') {
      result = result.filter((conversation) => conversation.status === 'spam');
    } else if (navFilter === 'ai_active') {
      result = result.filter(isAIActiveConversation);
    } else if (navFilter === 'resolved_by_ai') {
      result = result.filter(isResolvedByAIConversation);
    }
  }

  if (searchQuery.trim()) {
    const query = searchQuery.toLowerCase();
    result = result.filter(
      (conversation) =>
        conversation.subject.toLowerCase().includes(query) ||
        conversation.customer_name?.toLowerCase().includes(query) ||
        conversation.customer_email?.toLowerCase().includes(query) ||
        String(conversation.display_id).includes(query)
    );
  }

  result.sort((a, b) => {
    const resolvedStatuses = new Set(['resolved']);
    const aResolved = resolvedStatuses.has(a.status) ? 1 : 0;
    const bResolved = resolvedStatuses.has(b.status) ? 1 : 0;
    if (aResolved !== bResolved) return aResolved - bResolved;
    const aTime = new Date(a.updated_at).getTime();
    const bTime = new Date(b.updated_at).getTime();
    return sortOrder === 'oldest' ? aTime - bTime : bTime - aTime;
  });

  return result;
}
