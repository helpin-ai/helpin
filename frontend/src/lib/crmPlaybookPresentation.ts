import type { CRMPlaybookDefinition, CRMPlaybookItem } from './crmPlaybookTypes';
import type { CRMSituationAttention, CRMSituationMotion } from './crmSituationTypes';

export const playbookFilterDefinitions = [{
  key: 'state' as const,
  label: 'Playbook status',
  singleSelect: true,
  options: [
    { value: 'all', label: 'All statuses' },
    { value: 'draft', label: 'Draft' },
    { value: 'accepting', label: 'Accepting signals' },
    { value: 'stopped', label: 'Enrollment stopped' },
  ],
}];

export const playbookMotions: { value: CRMSituationMotion; label: string }[] = [
  { value: 'prospecting', label: 'Prospecting' }, { value: 'conversion', label: 'Conversion' },
  { value: 'onboarding', label: 'Onboarding' }, { value: 'adoption', label: 'Adoption' },
  { value: 'expansion', label: 'Expansion' }, { value: 'renewal', label: 'Renewal' }, { value: 'retention', label: 'Retention' },
];
export const attentionLabels: Record<CRMSituationAttention, string> = {
  needs_context: 'Needs context', needs_approval: 'Needs approval', follow_up_due: 'Follow-up due',
  waiting_customer: 'Waiting on customer', waiting_work: 'Work in progress', automation_failed: 'Needs attention',
};
export function playbookStatus(item: CRMPlaybookItem) {
  return !item.playbook.published_version_id ? 'Draft' : item.playbook.accepting_customers ? 'Accepting signals' : 'Enrollment stopped';
}
export function blankPlaybook(): CRMPlaybookDefinition {
  return {
    name: 'Untitled playbook', description: '', journey: 'custom', objective: '',
    eligibility: { commercial_motions: ['prospecting', 'conversion'] },
    responsibilities: { owner_role: 'signal_owner', approver_role: 'next_action_owner', escalation_member_id: null },
    milestones: [],
    policy: { outbound_messages: 'approval_required', crm_changes: 'approval_required', pm_tasks: 'not_allowed', check_after_hours: 48, escalate_after_hours: 168, stop_conditions: ['customer_declined', 'objective_achieved'] },
  };
}
// Keep keys across edits, reordering, retries and publications: participant progress uses them.
export function newMilestoneKey() { return `milestone_${crypto.randomUUID().replaceAll('-', '')}`; }
export function publishIssues(definition: CRMPlaybookDefinition): string[] {
  const issues: string[] = [];
  if (!definition.name.trim()) issues.push('Give the playbook a name.');
  if (!definition.objective.trim()) issues.push('Describe the customer outcome.');
  if (!definition.eligibility.commercial_motions.length) issues.push('Choose at least one sales or success stage.');
  if (!definition.responsibilities.escalation_member_id) issues.push('Choose who handles escalations.');
  if (!definition.milestones?.length) issues.push('Add at least one milestone.');
  if (definition.milestones?.some((milestone) => !milestone.name.trim() || !milestone.success_criteria.trim())) issues.push('Give every milestone a name and success criteria.');
  return issues;
}

// Object property order must not create an "unpublished changes" indicator.
export function definitionFingerprint(value: unknown): string {
  if (Array.isArray(value)) return `[${value.map(definitionFingerprint).join(',')}]`;
  if (value && typeof value === 'object') return `{${Object.entries(value).filter(([, v]) => v !== undefined).sort(([a], [b]) => a.localeCompare(b)).map(([k, v]) => `${JSON.stringify(k)}:${definitionFingerprint(v)}`).join(',')}}`;
  return JSON.stringify(value);
}

// A failed request can have reached the server. Reuse its key for the same intent.
export function createPlaybookIntentKey() {
  let fingerprint = '';
  let key = '';
  return (intent: unknown) => {
    const next = definitionFingerprint(intent);
    if (next !== fingerprint || !key) { fingerprint = next; key = crypto.randomUUID(); }
    return key;
  };
}
