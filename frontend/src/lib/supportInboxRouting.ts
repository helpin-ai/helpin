import type { NavFilter } from '@/stores/supportInboxStore';
import { defaultAIStatesForNav, defaultAssignmentForNav, defaultStatesForNav, statesEqual, stringArraysEqual } from '@/lib/supportInboxFilters';
import type { ConversationListFilters } from '@/lib/supportInboxFilters';

export type SupportInboxRouteSearch = {
  view?: string;
  inbox?: string;
  team_inbox?: string;
  status?: string;
  q?: string;
  conversation?: string;
  custom_view?: string;
  assigned_to?: string;
  ai?: string;
  sort?: string;
  states?: string;
  mailbox_ids?: string;
  tag_ids?: string;
  system_tags?: string;
};

const VIEW_BY_FILTER: Record<NavFilter, string> = {
  inbox: 'inbox',
  mine: 'mine',
  waiting: 'waiting',
  resolved: 'resolved',
  spam: 'spam',
  ai_active: 'ai-handling',
  resolved_by_ai: 'ai-resolved',
};

const FILTER_BY_VIEW: Record<string, NavFilter> = {
  all: 'inbox',
  inbox: 'inbox',
  assigned: 'mine',
  mine: 'mine',
  my: 'mine',
  my_inbox: 'mine',
  unassigned: 'inbox',
  team: 'inbox',
  custom: 'inbox',
  mentions: 'mine',
  waiting: 'waiting',
  resolved: 'resolved',
  spam: 'spam',
  ai: 'ai_active',
  'ai-active': 'ai_active',
  'ai-handling': 'ai_active',
  ai_active: 'ai_active',
  'ai-resolved': 'resolved_by_ai',
  resolved_by_ai: 'resolved_by_ai',
};

export function normalizeSupportInboxRouteSearch(search: Record<string, unknown>): SupportInboxRouteSearch {
  return {
    view: typeof search.view === 'string' ? search.view : undefined,
    inbox: typeof search.inbox === 'string' ? search.inbox : undefined,
    team_inbox: typeof search.team_inbox === 'string' ? search.team_inbox : undefined,
    status: typeof search.status === 'string' ? search.status : undefined,
    q: typeof search.q === 'string' ? search.q : undefined,
    conversation: typeof search.conversation === 'string' ? search.conversation : undefined,
    custom_view: typeof search.custom_view === 'string' ? search.custom_view : undefined,
    assigned_to: typeof search.assigned_to === 'string' ? search.assigned_to : undefined,
    ai: typeof search.ai === 'string' ? search.ai : undefined,
    sort: typeof search.sort === 'string' ? search.sort : undefined,
    states: typeof search.states === 'string' ? search.states : undefined,
    mailbox_ids: typeof search.mailbox_ids === 'string' ? search.mailbox_ids : undefined,
    tag_ids: typeof search.tag_ids === 'string' ? search.tag_ids : undefined,
    system_tags: typeof search.system_tags === 'string' ? search.system_tags : undefined,
  };
}

export function navFilterFromView(view?: string): NavFilter {
  if (!view) return 'inbox';
  return FILTER_BY_VIEW[view.toLowerCase()] ?? 'inbox';
}

export function viewFromNavFilter(filter: NavFilter): string {
  return VIEW_BY_FILTER[filter];
}

export function buildSupportInboxSearch({
  navFilter,
  selectedMailboxId,
  statusFilter,
  searchQuery,
  activeCustomViewId,
  listFilters,
  includeFilterParams = true,
}: {
  navFilter: NavFilter;
  selectedMailboxId: string;
  statusFilter: string;
  searchQuery: string;
  activeCustomViewId?: string | null;
  listFilters?: ConversationListFilters;
  includeFilterParams?: boolean;
}): SupportInboxRouteSearch {
  const search: SupportInboxRouteSearch = {};
  if (activeCustomViewId) {
    search.view = 'custom';
    search.custom_view = activeCustomViewId;
  } else if (navFilter === 'inbox' && selectedMailboxId && selectedMailboxId !== 'all') {
    search.view = 'team';
    search.team_inbox = selectedMailboxId;
  } else if (navFilter !== 'inbox') {
    search.view = viewFromNavFilter(navFilter);
  }
  if (!includeFilterParams) {
    return search;
  }
  if (statusFilter && statusFilter !== 'all') {
    search.status = statusFilter;
  }
  if (searchQuery.trim()) {
    search.q = searchQuery.trim();
  }
  if (listFilters?.assignment && !stringArraysEqual(listFilters.assignment, defaultAssignmentForNav(navFilter))) {
    if (listFilters.assignment.length > 0) search.assigned_to = listFilters.assignment.join(',');
  }
  if (listFilters?.sort && listFilters.sort !== 'newest') {
    search.sort = listFilters.sort;
  }
  if (listFilters?.states && !statesEqual(listFilters.states, defaultStatesForNav(navFilter))) {
    search.states = listFilters.states.join(',');
  }
  if (listFilters?.mailboxIds && listFilters.mailboxIds.length > 0) {
    search.mailbox_ids = listFilters.mailboxIds.join(',');
  }
  if (listFilters?.tagIds && listFilters.tagIds.length > 0) {
    search.tag_ids = listFilters.tagIds.join(',');
  }
  if (listFilters?.aiStates && !stringArraysEqual(listFilters.aiStates, defaultAIStatesForNav(navFilter))) {
    if (listFilters.aiStates.length > 0) {
      search.ai = listFilters.aiStates.join(',');
    }
  }
  return search;
}
