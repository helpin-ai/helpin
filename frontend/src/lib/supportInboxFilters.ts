import type { SupportConversation, SupportSystemTag } from './pmTypes';
import type { NavFilter } from '@/stores/supportInboxStore';
import {
  isAIActiveConversation,
  isHumanInboxConversation,
  isMineActionableConversation,
  isResolvedByAIConversation,
} from '@/components/support/helpers';

export type ConversationAssignmentFilter = 'me' | 'unassigned' | 'others';
export type ConversationSortOrder = 'newest' | 'oldest';
export type ConversationStateFilter = 'open' | 'waiting_on_customer' | 'resolved' | 'spam';

export interface ConversationListFilters {
  states: ConversationStateFilter[];
  assignment: ConversationAssignmentFilter[];
  mailboxIds: string[];
  tagIds: string[];
  systemTags: SupportSystemTag[];
  sort: ConversationSortOrder;
}

export const DEFAULT_CONVERSATION_LIST_FILTERS: ConversationListFilters = {
  states: ['open'],
  assignment: [],
  mailboxIds: [],
  tagIds: [],
  systemTags: [],
  sort: 'newest',
};

export type ConversationListRequestFilters = {
  status?: string;
  filter?: string;
  mailbox_id?: string;
  mailbox_ids?: string;
  flow_state?: string;
  search?: string;
  assigned_to?: string;
  sort?: string;
  statuses?: string;
  tag_ids?: string;
  system_tags?: string;
};

export function defaultStatesForNav(navFilter: NavFilter): ConversationStateFilter[] {
  switch (navFilter) {
    case 'inbox':
      return ['open', 'waiting_on_customer'];
    case 'mine':
      return ['open', 'waiting_on_customer'];
    case 'waiting':
      return ['waiting_on_customer'];
    case 'resolved':
    case 'resolved_by_ai':
      return ['resolved'];
    case 'spam':
      return ['spam'];
    case 'ai_active':
    default:
      return ['open'];
  }
}

export function defaultConversationListFiltersForNav(navFilter: NavFilter): ConversationListFilters {
  return {
    states: defaultStatesForNav(navFilter),
    assignment: [],
    mailboxIds: [],
    tagIds: [],
    systemTags: [],
    sort: 'newest',
  };
}

export function statesEqual(a: ConversationStateFilter[], b: ConversationStateFilter[]): boolean {
  if (a.length !== b.length) return false;
  return a.every((value, index) => value === b[index]);
}

export function stringArraysEqual(a: string[], b: string[]): boolean {
  if (a.length !== b.length) return false;
  return a.every((value, index) => value === b[index]);
}

function normalizeAssignmentFilters(value: unknown): ConversationAssignmentFilter[] {
  const raw = Array.isArray(value) ? value : typeof value === 'string' ? value.split(',') : [];
  const normalized: ConversationAssignmentFilter[] = [];
  for (const item of raw) {
    const trimmed = String(item).trim();
    if (
      (trimmed === 'me' || trimmed === 'unassigned' || trimmed === 'others') &&
      !normalized.includes(trimmed)
    ) {
      normalized.push(trimmed);
    }
  }
  return normalized;
}

export function normalizeConversationListFilters(
  filters: Partial<ConversationListFilters> | undefined,
  navFilter: NavFilter,
): ConversationListFilters {
  const defaults = defaultConversationListFiltersForNav(navFilter);
  return {
    states: filters?.states && filters.states.length > 0 ? filters.states : defaults.states,
    assignment: normalizeAssignmentFilters(filters?.assignment),
    mailboxIds: filters?.mailboxIds ?? defaults.mailboxIds,
    tagIds: filters?.tagIds ?? defaults.tagIds,
    systemTags: filters?.systemTags ?? defaults.systemTags,
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
    normalized.assignment.length > 0 ||
    normalized.mailboxIds.length > 0 ||
    normalized.tagIds.length > 0 ||
    normalized.systemTags.length > 0 ||
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
  if (normalizedFilters.assignment.length > 0) filters.assignment = normalizedFilters.assignment.join(',');
  if (normalizedFilters.mailboxIds.length > 0) filters.mailbox_ids = normalizedFilters.mailboxIds.join(',');
  if (normalizedFilters.tagIds.length > 0) filters.tag_ids = normalizedFilters.tagIds.join(',');
  if (normalizedFilters.systemTags.length > 0) filters.system_tags = normalizedFilters.systemTags.join(',');
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
  const selectedStates = normalizedFilters.states;
  const hasExplicitStates = !statesEqual(selectedStates, defaultStatesForNav(navFilter));

  if (normalizedFilters.mailboxIds.includes('all')) {
    // Explicit global override, mainly for Inbox where the default is Main inbox.
  } else if (normalizedFilters.mailboxIds.length > 0) {
    f.mailbox_ids = normalizedFilters.mailboxIds.join(',');
  } else if (selectedMailboxId !== 'all') {
    f.mailbox_id = selectedMailboxId;
  } else if (navFilter === 'inbox') {
    f.mailbox_id = 'shared';
  }

  if (!hasExplicitStates) {
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
  if (normalizedFilters.assignment.length > 0) f.assigned_to = normalizedFilters.assignment.join(',');
  if (normalizedFilters.tagIds.length > 0) f.tag_ids = normalizedFilters.tagIds.join(',');
  if (normalizedFilters.systemTags.length > 0) f.system_tags = normalizedFilters.systemTags.join(',');
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

function matchesMailboxScopes(conversation: SupportConversation, mailboxScopes: string[]): boolean {
  if (mailboxScopes.length === 0) return true;
  if (mailboxScopes.includes('all')) return true;
  return mailboxScopes.some((scope) => matchesMailboxScope(conversation, scope));
}

export function filterSupportConversations(
  conversations: SupportConversation[],
  options: {
    navFilter: NavFilter;
    mailboxScope: string;
    mailboxScopes?: string[];
    statusFilter?: string;
    userId?: string;
    searchQuery: string;
    sortOrder?: ConversationSortOrder;
    skipViewFilter?: boolean;
  }
): SupportConversation[] {
  const { navFilter, mailboxScope, mailboxScopes = [], statusFilter = 'all', userId, searchQuery, sortOrder = 'newest', skipViewFilter = false } = options;
  let result = conversations.filter((conversation) =>
    mailboxScopes.length > 0 ? matchesMailboxScopes(conversation, mailboxScopes) : matchesMailboxScope(conversation, mailboxScope)
  );

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
      result = result.filter((conversation) => conversation.status === 'resolved');
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
