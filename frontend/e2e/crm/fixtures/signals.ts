import type { Page } from '@playwright/test';
import type { CRMSuggestion } from '../../../src/lib/crmTypes';
import type { CRMSituationCategory, CRMSituationItem } from '../../../src/lib/crmSituationTypes';
import type { CRMSignalInboxItem } from '../../../src/lib/crmSignalInboxTypes';
import { installPlaybookMocks } from './playbooks';

export async function installSignalMocks(page: Page, options: { readOnly?: boolean; empty?: boolean; failure?: boolean; conflict?: boolean; actionFailure?: boolean; actionType?: CRMSuggestion['suggestion_type']; unclassified?: boolean; standalone?: boolean } = {}) {
  const base = await installPlaybookMocks(page, { readOnly: options.readOnly });
  const signal = base.signal;
  delete signal.situation.playbook_id;
  delete signal.situation.playbook_version_id;
  delete signal.situation.playbook_milestones;
  const action: CRMSuggestion = {
    id: 'action-1', workspace_id: 'ws-playbooks', suggestion_type: options.actionType || 'follow_up', title: 'Confirm the buying team’s timeline',
    description: 'James asked for pricing ahead of the team’s budget review.',
    context: { deal_id: 'deal-1', deal_name: 'Harbor expansion', pipeline_id: 'pipeline-1', stage_id: 'stage-1', target_stage_id: 'stage-2', company_id: 'company-1', amount: 12000 },
    status: 'pending', confidence: .92, revision: 'shown-revision', execution_status: 'pending', created_at: '2026-09-07T08:00:00Z', updated_at: '2026-09-07T08:00:00Z',
  };
  signal.actions = [action]; signal.pending_action_count = 1; signal.effective_attention = 'needs_approval';
  signal.evidence = [{ id: 'evidence-1', summary: 'James requested pricing for a 40-seat rollout.', evidence_excerpt: 'Could you send pricing before Friday’s budget review?', source_type: 'email', source_thread_id: 'email-thread-1', company_id: 'company-1', evidence_identity_trust: 'verified', detected_at: '2026-09-07T08:00:00Z' }];
  const records: CRMSituationItem[] = [signal, ...(['onboarding_adoption', 'expansion', 'retention'] as CRMSituationCategory[]).map((category, i) => ({ ...structuredClone(signal), category, actions: [], evidence: [], pending_action_count: 0, effective_attention: 'follow_up_due' as const, company_name: ['Northstar', 'Summit', 'Brightside'][i], situation: { ...structuredClone(signal.situation), id: `signal-${i + 2}`, title: ['First workspace not yet activated', 'Team is approaching its seat limit', 'Renewal decision is due this month'][i], next_step: ['Agree the onboarding session', 'Confirm the team’s expansion plan', 'Discuss renewal with the account owner'][i], commercial_motion: (['onboarding', 'expansion', 'renewal'] as const)[i], next_checkpoint_at: '2026-09-10T09:00:00Z' } }))];
  if (options.unclassified) records.push({ ...structuredClone(signal), category: '', situation: { ...structuredClone(signal.situation), id: 'unclassified', commercial_motion: 'needs_context', title: 'Confirm the customer relationship' } });
  const linked: string[] = [];
  if (options.standalone) { signal.actions = []; signal.pending_action_count = 0; action.title = 'Review a standalone recommendation'; }
  await page.route(/\/api\/crm\/(situations|signal-inbox)(\/|\?)/, async (route) => {
    const request = route.request(); const url = new URL(request.url()); const path = url.pathname.replace(/^\/api/, '');
    const headers = { 'access-control-allow-origin': request.headers().origin || 'http://127.0.0.1:5193', 'access-control-allow-credentials': 'true' };
    const json = (data: unknown, status = 200) => route.fulfill({ status, contentType: 'application/json', headers, body: JSON.stringify(data) });
    if (request.method() === 'OPTIONS') return route.fallback();
    if (request.method() === 'POST' && (path.includes('/actions/') || path.includes('/recommendations/'))) {
      base.writes.push({ path, body: request.postDataJSON() });
      if (options.conflict) { action.revision = 'new-revision'; action.title = 'Review the revised customer request'; return json({ error: 'This action has changed. Reload it before deciding.' }, 409); }
      if (request.postDataJSON().revision !== action.revision) return json({ error: 'Stale action' }, 409);
      action.status = path.endsWith('/dismiss') ? 'dismissed' : 'accepted';
      action.dismissal_reason = request.postDataJSON().reason;
      action.execution_status = options.actionFailure ? 'failed' : ['deal_create', 'deal_advance'].includes(action.suggestion_type) ? 'succeeded' : 'manual_required';
      action.revision = 'decided-revision'; signal.pending_action_count = 0;
      signal.failed_action_count = action.execution_status === 'failed' ? 1 : 0;
      signal.manual_action_count = action.execution_status === 'manual_required' && action.status === 'accepted' ? 1 : 0;
      return json(action);
    }
    if (request.method() !== 'GET') return route.fallback();
    base.reads.push(request.url());
    if (path === '/crm/signal-inbox/recommendations/action-1') return json({ action, linked_situations: linked });
    if (path === '/crm/signal-inbox') {
      if (options.failure) return json({ error: 'Signals could not be loaded.' }, 503);
      const search = url.searchParams.get('q')?.toLowerCase() || '';
      const category = url.searchParams.get('category') || 'all';
      const rows: CRMSignalInboxItem[] = records.map((item) => ({ id: item.situation.id, kind: 'situation', title: item.situation.title, next_step: item.situation.next_step, category: item.category, customer_name: item.company_name, owner_name: item.owner_name, owner_member_id: item.situation.owner_member_id, owner_available: item.owner_available, priority: item.situation.priority, priority_band: item.situation.priority >= 15 ? 'high' : item.situation.priority >= 8 ? 'medium' : 'low', lifecycle: item.situation.lifecycle, attention: item.effective_attention, pending_action_count: item.pending_action_count, evidence_review: item.evidence?.length ? item.evidence.every((source) => source.reviewed_at) ? 'reviewed' : 'needs_review' : 'none', created_at: item.situation.created_at }));
      if (options.standalone && !linked.length && action.status !== 'dismissed' && action.execution_status !== 'succeeded') rows.push({ id: action.id, kind: 'recommendation', title: action.title, next_step: action.description || '', category: '', customer_name: '', owner_name: '', owner_member_id: null, owner_available: false, priority: null, priority_band: 'unscored', lifecycle: 'open', attention: action.status === 'pending' ? 'needs_approval' : 'needs_context', pending_action_count: action.status === 'pending' ? 1 : 0, evidence_review: 'none', created_at: action.created_at });
      const filter = JSON.parse(url.searchParams.get('filter') || '{"rules":[]}') as { rules: { field: string; value: string }[] };
      const state = url.searchParams.get('state');
      const scope = url.searchParams.get('scope');
      const scoped = options.empty ? [] : rows.filter((item) => (scope === 'unassigned' ? !item.owner_member_id : scope === 'all' || !!item.owner_member_id) && `${item.title} ${item.customer_name}`.toLowerCase().includes(search) && (state !== 'needs_approval' || item.pending_action_count > 0) && filter.rules.every((rule) => rule.field.startsWith('has_') ? item.pending_action_count > 0 && action.suggestion_type === rule.field.slice(4) : (item as unknown as Record<string, unknown>)[rule.field === 'priority' ? 'priority_band' : rule.field] === rule.value));
      const counts = { all: scoped.length, sales: 0, onboarding_adoption: 0, expansion: 0, retention: 0 };
      for (const item of scoped) if (item.category) counts[item.category]++;
      const selected = scoped.filter((item) => category === 'all' || item.category === category);
      const currentPage = Number(url.searchParams.get('page') || 1);
      return json({ data: selected.slice((currentPage - 1) * 25, currentPage * 25), total: selected.length, page: currentPage, page_size: 25, category_counts: counts, uncategorized_count: scoped.filter((item) => !item.category).length, pending_action_total: 1 });
    }
    if (path.endsWith('/history')) return json({ data: [{ id: 'change-1', situation_id: signal.situation.id, revision: 1, operation: 'created', actor_kind: 'signal', actor_member_id: null, reason: '', before: null, after: signal.situation, in_flight_action_count: 0, created_at: '2026-09-07T08:00:00Z' }], next_before_revision: null });
    const found = records.find((item) => path === `/crm/situations/${item.situation.id}`);
    return found ? json(found) : json({ error: 'This signal could not be found.' }, 404);
  });
  await page.route('**/api/crm/signals/evidence-1/review?**', async (route) => {
    if (route.request().method() === 'OPTIONS') return route.fallback();
    base.writes.push({ path: '/crm/signals/evidence-1/review', body: route.request().postDataJSON() });
    signal.evidence![0].reviewed_at = '2026-09-07T12:00:00Z';
    return route.fulfill({ contentType: 'application/json', headers: { 'access-control-allow-origin': route.request().headers().origin || '', 'access-control-allow-credentials': 'true' }, body: '{}' });
  });
  await page.route('**/api/crm/pipelines/pipeline-1?**', async (route) => route.fulfill({ contentType: 'application/json', headers: { 'access-control-allow-origin': route.request().headers().origin || '', 'access-control-allow-credentials': 'true' }, body: JSON.stringify({ id: 'pipeline-1', name: 'Sales pipeline', stages: [{ id: 'stage-1', name: 'Discovery' }, { id: 'stage-2', name: 'Proposal' }] }) }));
  await page.route('**/api/crm/companies/company-1?**', async (route) => route.fulfill({ contentType: 'application/json', headers: { 'access-control-allow-origin': route.request().headers().origin || '', 'access-control-allow-credentials': 'true' }, body: JSON.stringify({ id: 'company-1', name: 'Harbor Labs' }) }));
  await page.route('**/api/crm/deals/deal-1?**', async (route) => route.fulfill({ contentType: 'application/json', headers: { 'access-control-allow-origin': route.request().headers().origin || '', 'access-control-allow-credentials': 'true' }, body: JSON.stringify({ id: 'deal-1', name: 'Harbor expansion', pipeline_id: 'pipeline-1', stage_id: 'stage-1' }) }));
  return { ...base, action, records, linked };
}
