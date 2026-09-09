import type { CRMSignalInboxItem } from './crmSignalInboxTypes';
import { signalCategories } from './crmSituationPresentation';

export const inboxNavigation = {
  scope: [{ value: 'all', label: 'Everyone' }, { value: 'mine', label: 'Assigned to me' }, { value: 'my_teams', label: 'My teams' }, { value: 'unassigned', label: 'Unassigned' }],
  state: [{ value: 'needs_attention', label: 'Needs attention' }, { value: 'needs_approval', label: 'Needs approval' }, { value: 'open', label: 'Open' }, { value: 'waiting', label: 'Waiting' }, { value: 'paused', label: 'Paused' }, { value: 'closed', label: 'Closed' }, { value: 'all', label: 'All statuses' }],
  sort: [{ value: 'priority', label: 'High priority' }, { value: 'newest', label: 'Newest first' }, { value: 'oldest', label: 'Oldest first' }],
};

export const inboxFilterDefinitions = [
  { key: 'scope', label: 'Assignment', options: inboxNavigation.scope, singleSelect: true },
  { key: 'state', label: 'Signal status', options: inboxNavigation.state, singleSelect: true },
  { key: 'category', label: 'Category', options: signalCategories, singleSelect: true },
] satisfies { key: 'scope' | 'state' | 'category'; label: string; options: { value: string; label: string }[]; singleSelect: boolean }[];

export interface SignalsSearch {
  view?: 'evidence'; signal?: string; recommendation?: string; group?: string;
  scope?: string; state?: string; category?: string; q?: string;
  sort?: string; page?: number;
}
const categories = ['all', 'sales', 'onboarding_adoption', 'expansion', 'retention'];
export function parseSignalsSearch(search: Record<string, unknown>): SignalsSearch {
  const result: SignalsSearch = {};
  if (search.view === 'evidence') result.view = 'evidence';
  for (const key of ['signal', 'recommendation', 'group'] as const) if (typeof search[key] === 'string' && search[key].length <= 100) result[key] = search[key];
  if (typeof search.q === 'string') result.q = search.q.slice(0, 500);
  for (const key of ['scope', 'state', 'sort'] as const) if (inboxNavigation[key].some((option) => option.value === search[key])) result[key] = search[key] as string;
  if (typeof search.category === 'string' && categories.includes(search.category)) result.category = search.category;
  const page = Number(search.page);
  if (Number.isSafeInteger(page) && page > 0) result.page = page;
  // Retired controls must never constrain a view invisibly through an old URL.
  if (['priority', 'evidence_review', 'attention', 'action_type', 'filter'].some((key) => search[key] !== undefined) || search.sort === 'recommended') result.page = 1;
  return result;
}
export function reviewSignalsSearch(search: Record<string, unknown>): SignalsSearch {
  const parsed = parseSignalsSearch(search);
  return { ...parsed, view: undefined, scope: parsed.scope || 'all', state: 'needs_approval', sort: parsed.sort || 'priority' };
}
export function inboxStatus(item: CRMSignalInboxItem): string {
  if (item.kind === 'evidence') return item.evidence_review === 'reviewed' ? 'Evidence reviewed' : 'Evidence to review';
  const base = item.lifecycle === 'closed' ? 'Closed' : item.lifecycle === 'paused' ? 'Paused' : ({ needs_context: 'Needs context', needs_approval: 'Needs approval', follow_up_due: 'Follow-up due', waiting_customer: 'Waiting on customer', waiting_work: 'Work in progress', automation_failed: 'Action needs attention' }[item.attention]);
  return item.pending_action_count > 0 && item.attention !== 'needs_approval' || item.pending_action_count > 0 && item.lifecycle !== 'open' ? `${base} · Approval pending` : base;
}
