import type { QueryBuilderFieldDefinition, QueryFilterGroup } from './queryBuilder';
import type { CRMSignalInboxItem } from './crmSignalInboxTypes';

export const inboxNavigation = {
  scope: [{ value: 'mine', label: 'Assigned to me' }, { value: 'my_teams', label: 'My teams' }, { value: 'unassigned', label: 'Unassigned' }, { value: 'all', label: 'Everyone' }],
  state: [{ value: 'needs_attention', label: 'Needs attention' }, { value: 'needs_approval', label: 'Needs approval' }, { value: 'open', label: 'Open' }, { value: 'waiting', label: 'Waiting' }, { value: 'paused', label: 'Paused' }, { value: 'closed', label: 'Closed' }, { value: 'all', label: 'All statuses' }],
  sort: [{ value: 'priority', label: 'Business priority' }, { value: 'recommended', label: 'Recommendation confidence' }, { value: 'newest', label: 'Newest first' }, { value: 'oldest', label: 'Oldest first' }],
  action_type: [{ value: 'deal_create', label: 'New deal' }, { value: 'deal_advance', label: 'Stage advance' }, { value: 'follow_up', label: 'Follow-up' }, { value: 'risk_alert', label: 'Risk alert' }, { value: 'enrichment', label: 'Enrichment' }],
};

export const inboxFields: QueryBuilderFieldDefinition[] = [
  { field: 'priority', label: 'Priority', type: 'enum', options: [{ value: 'high', label: 'High priority' }, { value: 'medium', label: 'Medium priority' }, { value: 'low', label: 'Low priority' }, { value: 'unscored', label: 'Not scored' }] },
  { field: 'evidence_review', label: 'Evidence review', type: 'enum', options: [{ value: 'needs_review', label: 'Evidence to review' }, { value: 'reviewed', label: 'Evidence reviewed' }, { value: 'none', label: 'No current evidence' }] },
  { field: 'attention', label: 'Attention needed', type: 'enum', options: [{ value: 'follow_up_due', label: 'Follow-up due' }, { value: 'needs_context', label: 'Needs context' }, { value: 'automation_failed', label: 'Action needs attention' }, { value: 'waiting_customer', label: 'Waiting on customer' }, { value: 'waiting_work', label: 'Work in progress' }] },
  ...inboxNavigation.action_type.map(({ value, label }): QueryBuilderFieldDefinition => ({ field: `has_${value}`, label, type: 'enum', options: [{ value: 'yes', label }] })),
];
export interface SignalsSearch {
  view?: 'evidence'; signal?: string; recommendation?: string;
  scope?: string; state?: string; category?: string; q?: string;
  priority?: string; evidence_review?: string; attention?: string; action_type?: string; sort?: string; page?: number;
}
const categories = ['all', 'sales', 'onboarding_adoption', 'expansion', 'retention'];
export function parseSignalsSearch(search: Record<string, unknown>): SignalsSearch {
  const result: SignalsSearch = {};
  if (search.view === 'evidence') result.view = 'evidence';
  for (const key of ['signal', 'recommendation'] as const) if (typeof search[key] === 'string' && search[key].length <= 100) result[key] = search[key];
  if (typeof search.q === 'string') result.q = search.q.slice(0, 500);
  for (const key of ['scope', 'state', 'sort', 'action_type'] as const) if (inboxNavigation[key].some((option) => option.value === search[key])) result[key] = search[key] as string;
  for (const key of ['priority', 'evidence_review', 'attention'] as const) if (inboxFields.find((field) => field.field === key)?.options?.some((option) => option.value === search[key])) result[key] = search[key] as string;
  if (typeof search.category === 'string' && categories.includes(search.category)) result.category = search.category;
  const page = Number(search.page);
  if (Number.isSafeInteger(page) && page > 0) result.page = page;
  return result;
}
export function reviewSignalsSearch(search: Record<string, unknown>): SignalsSearch {
  const parsed = parseSignalsSearch(search);
  return { ...parsed, view: undefined, scope: parsed.scope || 'all', state: 'needs_approval', sort: parsed.sort || 'recommended' };
}
export function inboxQueryFilter(search: SignalsSearch): QueryFilterGroup | undefined {
  const rules: QueryFilterGroup['rules'] = [];
  for (const field of inboxFields) {
    const value = field.field.startsWith('has_') ? search.action_type === field.field.slice(4) ? 'yes' : undefined : search[field.field as keyof SignalsSearch];
    if (typeof value === 'string' && field.options?.some((option) => option.value === value)) rules.push({ field: field.field, operator: 'is', value });
  }
  return rules.length ? { logic: 'and', rules } : undefined;
}
export function inboxStatus(item: CRMSignalInboxItem): string {
  const base = item.lifecycle === 'closed' ? 'Closed' : item.lifecycle === 'paused' ? 'Paused' : ({ needs_context: 'Needs context', needs_approval: 'Needs approval', follow_up_due: 'Follow-up due', waiting_customer: 'Waiting on customer', waiting_work: 'Work in progress', automation_failed: 'Action needs attention' }[item.attention]);
  return item.pending_action_count > 0 && item.attention !== 'needs_approval' || item.pending_action_count > 0 && item.lifecycle !== 'open' ? `${base} · Approval pending` : base;
}
