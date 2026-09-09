import type { Page } from '@playwright/test';
import { blankPlaybook } from '../../../src/lib/crmPlaybookPresentation';
import type { CRMPlaybookDefinition, CRMPlaybookItem, CRMPlaybookVersion, CRMPlaybookAutomationOverview, CRMPlaybookAutomationBinding } from '../../../src/lib/crmPlaybookTypes';
import type { CRMSituationItem } from '../../../src/lib/crmSituationTypes';

export async function installPlaybookMocks(page: Page, options: { readOnly?: boolean; draft?: boolean; empty?: boolean; failure?: boolean; saveFailures?: number; conflict?: boolean } = {}) {
  const definition: CRMPlaybookDefinition = {
    ...blankPlaybook(), journey: 'buying_intent', name: 'Buying-intent follow-up', objective: 'Agree a qualified next step with the customer',
    responsibilities: { ...blankPlaybook().responsibilities, escalation_member_id: 'member-1' },
    milestones: [{ key: 'intent', name: 'Intent confirmed', success_criteria: 'The customer confirms a relevant need.' }, { key: 'next_step', name: 'Next step agreed', success_criteria: 'The customer agrees to a specific next step.' }],
  };
  const version: CRMPlaybookVersion = { id: 'version-2', playbook_id: 'book-1', version: 2, definition: structuredClone(definition), published_at: '2026-09-06T10:00:00Z', published_by_member_id: 'member-1' };
  const oldVersion: CRMPlaybookVersion = { ...version, id: 'version-1', version: 1, definition: { ...structuredClone(definition), milestones: [{ key: 'legacy', name: 'Original customer commitment', success_criteria: 'Customer confirmation from the original published version.' }] } };
  const item: CRMPlaybookItem = {
    execution_enabled: false, playbook: { id: 'book-1', workspace_id: 'ws-playbooks', revision: 3, draft: structuredClone(definition), published_version_id: options.draft ? null : version.id, accepting_customers: !options.draft, created_by_member_id: 'member-1', updated_by_member_id: 'member-1', created_at: '2026-09-06T10:00:00Z', updated_at: '2026-09-06T10:00:00Z' },
    open_count: 1, paused_count: 0, closed_count: 0, published_version: options.draft ? undefined : version,
  };
  const signal: CRMSituationItem = {
    situation: { id: 'signal-1', workspace_id: 'ws-playbooks', origin_kind: 'signal', title: 'Pricing requested for a 40-seat rollout', objective: 'Agree the buying team’s next step', commercial_motion: 'conversion', company_id: 'company-1', contact_id: null, deal_id: null, revision: 4, priority: 80, created_by_member_id: null, created_at: '2026-09-06T10:00:00Z', updated_at: '2026-09-06T10:00:00Z', owner_member_id: 'member-1', next_action_owner_member_id: 'member-1', lifecycle: 'open', attention: 'needs_context', next_step: 'Confirm the evaluation timeline with James', next_checkpoint_at: null, playbook_id: 'book-1', playbook_version_id: 'version-1', playbook_applied_at: '2026-09-06T10:00:00Z', playbook_milestones: [{ key: 'legacy', status: 'pending', summary: '', assessed_by_member_id: null, assessed_at: null, basis: null }], outcome_kind: null, outcome_summary: null, outcome_basis: null, duplicate_of_situation_id: null, closed_at: null, closed_by_member_id: null },
    category: 'sales', company_name: 'Harbor Labs', contact_name: '', deal_name: '', owner_name: 'Maya Patel', next_action_owner_name: 'Maya Patel', owner_available: true, next_action_owner_available: true, pending_action_count: 0, failed_action_count: 0, uncertain_action_count: 0, manual_action_count: 0, executing_action_count: 0, effective_attention: 'needs_context',
  };
  const candidate: CRMSituationItem = { ...structuredClone(signal), company_name: 'Northstar', situation: { ...structuredClone(signal.situation), id: 'signal-2', company_id: 'company-2', playbook_id: undefined, playbook_version_id: undefined, playbook_milestones: undefined } };
  const writes: { path: string; body: Record<string, any> }[] = [];
  const reads: string[] = [];
  const automation: CRMPlaybookAutomationOverview = { settings: null, connection: null, runtime_available: true };
  let binding: CRMPlaybookAutomationBinding | null = null;
  const receipts = new Map<string, unknown>();
  let saveFailures = options.saveFailures ?? 0;
  let enrolled = false;
  const signalList = (data: CRMSituationItem[]) => ({ data, total: data.length, page: 1, page_size: 25, category_counts: { all: data.length, sales: data.length, onboarding_adoption: 0, expansion: 0, retention: 0 }, uncategorized_count: 0, pending_action_total: 0 });
  await page.route('**/api/**', async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const path = url.pathname.replace(/^\/api/, '');
    const headers = { 'access-control-allow-origin': request.headers().origin || 'http://127.0.0.1:5193', 'access-control-allow-credentials': 'true' };
    const json = async (data: unknown, status = 200) => route.fulfill({ status, contentType: 'application/json', headers, body: JSON.stringify(data) });
    if (request.method() === 'OPTIONS') return route.fulfill({ status: 204, headers: { ...headers, 'access-control-allow-headers': 'authorization, content-type', 'access-control-allow-methods': 'GET, POST, OPTIONS' } });
    // Analytics shares the API prefix, but is not a CRM command.
    if (path === '/v1/event') return json({});
    if (request.method() !== 'GET') {
      const body = request.postDataJSON();
      writes.push({ path, body });
      const key = body.command_key || body.creation_key;
      if (receipts.has(key)) return json(receipts.get(key));
      if (options.conflict && body.operation === 'update_draft') return json({ error: 'This Playbook has changed. Reload it before continuing.' }, 409);
      if (path.endsWith('/automation/setup')) return json({ ...body, flow_id: 'flow-1', agent_id: 'beacon-1', flow_name: 'Buying-intent follow-up', agent_name: 'Beacon', runtime_kind: 'codex', connection_version: 0, review_fingerprint: 'reviewed-exact-settings', skills: [{ key: 'crm_record_operations', title: 'CRM record operations', role: 'core', version: 'v1' }, { key: 'crm_buying_intent_follow_up', title: 'Buying-intent follow-up', role: 'job', version: 'v1' }], execution_enabled: false });
      if (path.endsWith('/automation/connections')) {
        automation.connection = { id: 'connection-1', playbook_id: 'book-1', playbook_version_id: body.playbook_version_id, version: 1, fingerprint: 'published-settings', execution_enabled: false, published_by_member_id: 'member-1', published_at: new Date().toISOString() };
        automation.agent_name = 'Beacon'; automation.agent_id = 'beacon-1'; automation.flow_id = 'flow-1'; automation.flow_name = 'Buying-intent follow-up';
        return json({ connection: automation.connection, replayed: false });
      }
      if (path.endsWith('/automation/settings')) {
        automation.settings = { ...body, playbook_id: 'book-1', revision: (automation.settings?.revision || 0) + 1, authorized_by_member_id: 'member-1', updated_at: new Date().toISOString() };
        return json({ settings: automation.settings, created_at: new Date().toISOString() });
      }
      if (path === '/crm/situations/signal-1/automation') {
        binding = { situation_id: 'signal-1', playbook_id: 'book-1', connection_id: body.connection_id, enabled: body.enabled, generation: (binding?.generation || 0) + 1, no_progress_runs: 0, blocker: '' };
        return json({ binding, created_at: new Date().toISOString() });
      }
      if (body.operation === 'update_draft' && saveFailures-- > 0) return json({ error: 'Temporary save failure. Try again.' }, 503);
      if (path === '/crm/playbooks') { item.playbook.draft = body.definition; item.playbook.published_version_id = null; item.published_version = undefined; item.playbook.accepting_customers = false; receipts.set(key, structuredClone(item.playbook)); return json(item.playbook, 201); }
      if (path.endsWith('/apply')) { enrolled = true; return json({ change: { after: candidate.situation }, replayed: false }); }
      if (path.endsWith('/milestones')) { signal.situation.playbook_milestones![0] = { key: body.milestone_key, status: body.status, summary: body.summary, assessed_by_member_id: 'member-1', assessed_at: '2026-09-07T12:00:00Z', basis: 'human_assessment' }; signal.situation.revision++; return json({ change: { after: signal.situation }, replayed: false }); }
      if (path.startsWith('/crm/situations/')) {
        if (body.operation === 'update') {
          if (body.changes.owner) { signal.situation.owner_member_id = body.changes.owner.member_id; signal.owner_name = body.changes.owner.member_id === 'member-2' ? 'Jordan Reed' : 'Maya Patel'; }
          if (body.changes.next_action_owner) signal.situation.next_action_owner_member_id = body.changes.next_action_owner.member_id;
          if (body.changes.next_step !== undefined) signal.situation.next_step = body.changes.next_step;
          if (body.changes.attention !== undefined) signal.situation.attention = body.changes.attention;
          if (body.changes.checkpoint !== undefined) signal.situation.next_checkpoint_at = body.changes.checkpoint.at;
        } else signal.situation.lifecycle = body.operation === 'pause' ? 'paused' : body.operation === 'resume' ? 'open' : 'closed';
        signal.situation.revision++;
        if (body.outcome) { signal.situation.outcome_kind = body.outcome.kind; signal.situation.outcome_summary = body.outcome.summary; }
        return json({ change: { after: signal.situation }, replayed: false });
      }
      if (body.operation === 'update_draft') item.playbook.draft = body.definition;
      if (body.operation === 'publish') { item.published_version = { ...version, version: version.version + 1, definition: structuredClone(item.playbook.draft) }; item.playbook.published_version_id = item.published_version.id; }
      if (body.operation === 'set_enrollment') item.playbook.accepting_customers = body.accepting_customers;
      item.playbook.revision++;
      const receipt = { change: { after: structuredClone(item.playbook) }, replayed: false };
      receipts.set(key, receipt);
      return json(receipt);
    }
    reads.push(request.url());
    if (path.endsWith('/me')) return json({ membership: { id: 'member-1', role: options.readOnly ? 'viewer' : 'admin' }, permissions: options.readOnly ? ['crm.read'] : ['crm.read', 'crm.edit', 'crm.admin', 'pm.admin.automations'], modules: ['crm', 'automation'], team_memberships: [] });
    if (path.endsWith('/assignable-members')) return json([{ id: 'member-1', display_name: 'Maya Patel', email: 'maya@example.test', role: 'admin', status: 'active' }, { id: 'member-2', display_name: 'Jordan Reed', email: 'jordan@example.test', role: 'member', status: 'active' }]);
    if (path === '/crm/pipelines') return json([]);
    if (path === '/crm/playbooks/templates') return json([definition, { ...definition, name: 'Sales-to-success handoff', journey: 'sales_handoff' }, { ...definition, name: 'Renewal-risk recovery', journey: 'renewal_recovery' }]);
    if (path === '/crm/playbooks') {
      if (options.failure) return json({ error: 'Playbooks could not be loaded.' }, 503);
      const state = url.searchParams.get('state');
      const visible = !options.empty && (!url.searchParams.get('q') || item.playbook.draft.name.toLowerCase().includes(url.searchParams.get('q')!.toLowerCase())) && (state === 'all' || !state || state === 'draft' && !item.published_version || state === 'accepting' && item.playbook.accepting_customers || state === 'stopped' && item.published_version && !item.playbook.accepting_customers);
      return json({ data: visible ? [item] : [], total: visible ? 1 : 0, page: 1, page_size: 25 });
    }
    if (path === '/crm/playbooks/book-1') return json(item);
    if (path === '/crm/playbooks/book-1/automation') return json(automation);
    if (path === '/crm/playbooks/book-1/automation/activity') return json({ data: [], total: 0, page: 1 });
    if (path === '/crm/situations/signal-1/automation') return json(binding);
    if (path.endsWith('/versions')) return json(url.searchParams.has('before') ? { data: [oldVersion], next_before_version: null } : { data: [version], next_before_version: 2 });
    if (path.endsWith('/preview')) return json({ version_id: url.searchParams.get('version_id'), playbook_revision: item.playbook.revision, definition, signals: signalList(enrolled ? [] : [candidate]), scope: 'existing_signals', execution_enabled: false });
    if (path.endsWith('/participants')) return json(signalList(options.empty ? [] : [signal]));
    if (path.endsWith('/history')) return json({ data: [], next_before_revision: null });
    if (path === '/crm/situations/signal-1') return json(signal);
    return json({ error: `Unhandled test API: ${path}` }, 404);
  });
  return { writes, reads, item, signal, automation, setBinding: (value: CRMPlaybookAutomationBinding | null) => { binding = value; } };
}
