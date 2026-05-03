import type { SupportConversation } from './pmTypes';
import type { NavFilter } from '@/stores/supportInboxStore';
import {
  isAIActiveConversation,
  isHumanInboxConversation,
  isMineActionableConversation,
  isResolvedByAIConversation,
} from '@/components/support/helpers';

export type ConversationAssignmentFilter = 'me' | 'mentioned_me' | 'opened_by_me' | 'unassigned' | 'others';
export type ConversationSortOrder = 'newest' | 'oldest';
export type ConversationStateFilter = 'open' | 'waiting_on_customer' | 'resolved' | 'spam';
export type ConversationAIStateFilter = 'handling' | 'handoff' | 'resolved';

export interface ConversationListFilters {
  states: ConversationStateFilter[];
  assignment: ConversationAssignmentFilter[];
  mailboxIds: string[];
  tagIds: string[];
  aiStates: ConversationAIStateFilter[];
  sort: ConversationSortOrder;
}

export const DEFAULT_CONVERSATION_LIST_FILTERS: ConversationListFilters = {
  states: ['open'],
  assignment: [],
  mailboxIds: [],
  tagIds: [],
  aiStates: [],
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
  ai?: string;
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

export function defaultAIStatesForNav(navFilter: NavFilter): ConversationAIStateFilter[] {
  switch (navFilter) {
    case 'ai_active':
      return ['handling'];
    case 'resolved_by_ai':
      return ['resolved'];
    default:
      return [];
  }
}

export function defaultAssignmentForNav(navFilter: NavFilter): ConversationAssignmentFilter[] {
  return navFilter === 'mine' ? ['me', 'mentioned_me', 'opened_by_me'] : [];
}

export function defaultConversationListFiltersForNav(navFilter: NavFilter): ConversationListFilters {
  return {
    states: defaultStatesForNav(navFilter),
    assignment: defaultAssignmentForNav(navFilter),
    mailboxIds: [],
    tagIds: [],
    aiStates: defaultAIStatesForNav(navFilter),
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
      (trimmed === 'me' ||
        trimmed === 'mentioned_me' ||
        trimmed === 'opened_by_me' ||
        trimmed === 'unassigned' ||
        trimmed === 'others') &&
      !normalized.includes(trimmed)
    ) {
      normalized.push(trimmed);
    }
  }
  return normalized;
}

export function normalizeAIStateFilters(value: unknown): ConversationAIStateFilter[] {
  const raw = Array.isArray(value) ? value : typeof value === 'string' ? value.split(',') : [];
  const normalized: ConversationAIStateFilter[] = [];
  for (const item of raw) {
    const trimmed = String(item).trim();
    const mapped =
      trimmed === 'ai_handoff' || trimmed === 'needs_human'
        ? 'handoff'
        : trimmed === 'ai_resolved' || trimmed === 'resolved_by_ai'
          ? 'resolved'
          : trimmed === 'ai_handling' || trimmed === 'ai-active' || trimmed === 'ai_active'
            ? 'handling'
            : trimmed;
    if (
      (mapped === 'handling' || mapped === 'handoff' || mapped === 'resolved') &&
      !normalized.includes(mapped)
    ) {
      normalized.push(mapped);
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
    assignment: filters && 'assignment' in filters ? normalizeAssignmentFilters(filters.assignment) : defaults.assignment,
    mailboxIds: filters?.mailboxIds ?? defaults.mailboxIds,
    tagIds: filters?.tagIds ?? defaults.tagIds,
    aiStates: filters && 'aiStates' in filters ? normalizeAIStateFilters(filters.aiStates) : defaults.aiStates,
    sort: filters?.sort ?? defaults.sort,
  };
}

export function hasConversationListChanges({
  navFilter,
  searchQuery,
  listFilters,
}: {
  navFilter: NavFilter;
  searchQuery: string;
  listFilters: Partial<ConversationListFilters> | undefined;
}): boolean {
  const normalized = normalizeConversationListFilters(listFilters, navFilter);
  return (
    searchQuery.trim().length > 0 ||
    !statesEqual(normalized.states, defaultStatesForNav(navFilter)) ||
    !stringArraysEqual(normalized.assignment, defaultAssignmentForNav(navFilter)) ||
    normalized.mailboxIds.length > 0 ||
    normalized.tagIds.length > 0 ||
    !stringArraysEqual(normalized.aiStates, defaultAIStatesForNav(navFilter)) ||
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
  if (!stringArraysEqual(normalizedFilters.assignment, defaultAssignmentForNav(navFilter))) {
    if (normalizedFilters.assignment.length > 0) filters.assignment = normalizedFilters.assignment.join(',');
  }
  if (normalizedFilters.mailboxIds.length > 0) filters.mailbox_ids = normalizedFilters.mailboxIds.join(',');
  if (normalizedFilters.tagIds.length > 0) filters.tag_ids = normalizedFilters.tagIds.join(',');
  if (normalizedFilters.aiStates.length > 0) filters.ai = normalizedFilters.aiStates.join(',');
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
      case 'resolved_by_ai':
        break;
    }
  } else {
    switch (navFilter) {
      case 'mine':
        f.filter = 'mine';
        break;
      case 'ai_active':
      case 'resolved_by_ai':
        break;
    }
  }

  if (searchQuery.trim()) f.search = searchQuery.trim();
  if (hasExplicitStates) f.statuses = selectedStates.join(',');
  if (!stringArraysEqual(normalizedFilters.assignment, defaultAssignmentForNav(navFilter))) {
    if (normalizedFilters.assignment.length > 0) f.assigned_to = normalizedFilters.assignment.join(',');
  }
  if (normalizedFilters.tagIds.length > 0) f.tag_ids = normalizedFilters.tagIds.join(',');
  if (normalizedFilters.aiStates.length > 0) f.ai = normalizedFilters.aiStates.join(',');
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

function matchesAIState(conversation: SupportConversation, aiState: ConversationAIStateFilter): boolean {
  switch (aiState) {
    case 'handling':
      return isAIActiveConversation(conversation);
    case 'handoff':
      return conversation.ai_state === 'escalated' ||
        !!conversation.ai_escalated_at ||
        (!!conversation.customer_requested_human_at && (conversation.ai_state != null || (conversation.ai_turn_count ?? 0) > 0)) ||
        (
          (conversation.flow_state === 'waiting_for_human' ||
            conversation.flow_state === 'queued_for_human' ||
            conversation.flow_state === 'after_hours_queue') &&
          (conversation.ai_state != null || (conversation.ai_turn_count ?? 0) > 0)
        );
    case 'resolved':
      return isResolvedByAIConversation(conversation);
  }
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
    aiStates?: ConversationAIStateFilter[];
  }
): SupportConversation[] {
  const { navFilter, mailboxScope, mailboxScopes = [], statusFilter = 'all', userId, searchQuery, sortOrder = 'newest', skipViewFilter = false, aiStates = [] } = options;
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

  if (aiStates.length > 0) {
    result = result.filter((conversation) => aiStates.some((aiState) => matchesAIState(conversation, aiState)));
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
