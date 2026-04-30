import type { NavFilter } from '@/stores/supportInboxStore';

export type SupportInboxRouteSearch = {
  view?: string;
  inbox?: string;
  status?: string;
  q?: string;
  conversation?: string;
};

const VIEW_BY_FILTER: Record<NavFilter, string> = {
  all: 'all',
  my_inbox: 'assigned',
  unassigned: 'unassigned',
  mentions: 'mentions',
  ai_active: 'ai-handling',
  resolved_by_ai: 'ai-resolved',
};

const FILTER_BY_VIEW: Record<string, NavFilter> = {
  all: 'all',
  assigned: 'my_inbox',
  mine: 'my_inbox',
  my: 'my_inbox',
  my_inbox: 'my_inbox',
  unassigned: 'unassigned',
  mentions: 'mentions',
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
    status: typeof search.status === 'string' ? search.status : undefined,
    q: typeof search.q === 'string' ? search.q : undefined,
    conversation: typeof search.conversation === 'string' ? search.conversation : undefined,
  };
}

export function navFilterFromView(view?: string): NavFilter {
  if (!view) return 'all';
  return FILTER_BY_VIEW[view.toLowerCase()] ?? 'all';
}

export function viewFromNavFilter(filter: NavFilter): string {
  return VIEW_BY_FILTER[filter];
}

export function buildSupportInboxSearch({
  navFilter,
  selectedMailboxId,
  statusFilter,
  searchQuery,
}: {
  navFilter: NavFilter;
  selectedMailboxId: string;
  statusFilter: string;
  searchQuery: string;
}): SupportInboxRouteSearch {
  const search: SupportInboxRouteSearch = {};
  if (navFilter !== 'all') {
    search.view = viewFromNavFilter(navFilter);
  }
  if (selectedMailboxId && selectedMailboxId !== 'all') {
    search.inbox = selectedMailboxId;
  }
  if (statusFilter && statusFilter !== 'all') {
    search.status = statusFilter;
  }
  if (searchQuery.trim()) {
    search.q = searchQuery.trim();
  }
  return search;
}
