import type { CRMPlaybookDefinition } from './crmPlaybookTypes';

export const playbookSetupSteps = [
  { id: 'purpose', label: 'Purpose & scope' },
  { id: 'milestones', label: 'Milestones' },
  { id: 'team', label: 'Team & permissions' },
  { id: 'follow_up', label: 'Monitoring' },
  { id: 'review', label: 'Review & automation' },
] as const;
export type PlaybookSetupStep = typeof playbookSetupSteps[number]['id'];

// Completion describes definition readiness, never execution or activation.
export function playbookSetupIssues(d: CRMPlaybookDefinition, escalationAvailable: boolean) {
  const issues: Record<Exclude<PlaybookSetupStep, 'review'>, string[]> = { purpose: [], milestones: [], team: [], follow_up: [] };
  if (!d.name.trim() || d.name.length > 160) issues.purpose.push('Give the playbook a name, up to 160 characters.');
  if (!d.objective.trim() || d.objective.length > 4000) issues.purpose.push('Describe the customer outcome, up to 4,000 characters.');
  if (d.description.length > 2000) issues.purpose.push('Keep the description within 2,000 characters.');
  if (!d.eligibility.commercial_motions.length) issues.purpose.push('Choose at least one sales or success stage.');
  if (!d.milestones?.length) issues.milestones.push('Add at least one milestone.');
  if ((d.milestones?.length || 0) > 20 || d.milestones?.some((m) => !m.name.trim() || m.name.length > 160 || !m.success_criteria.trim() || m.success_criteria.length > 2000)) issues.milestones.push('Give every milestone a name and success criteria within the field limits.');
  if (!d.responsibilities.escalation_member_id || !escalationAvailable) issues.team.push('Choose an active escalation owner.');
  if (!['signal_owner', 'account_owner', 'deal_owner', 'customer_success_owner'].includes(d.responsibilities.owner_role) || !['next_action_owner', 'signal_owner'].includes(d.responsibilities.approver_role)) issues.team.push('Choose who owns the work and receives approvals.');
  if ([d.policy.outbound_messages, d.policy.crm_changes, d.policy.pm_tasks].some((value) => !['approval_required', 'not_allowed'].includes(value))) issues.team.push('Review the action permissions.');
  if (!Number.isInteger(d.policy.check_after_hours) || d.policy.check_after_hours < 1 || d.policy.check_after_hours > 8760) issues.follow_up.push('Set the check interval between 1 and 8,760 whole hours.');
  if (!Number.isInteger(d.policy.escalate_after_hours) || d.policy.escalate_after_hours < d.policy.check_after_hours || d.policy.escalate_after_hours > 8760) issues.follow_up.push('Set escalation at or after the check interval, up to 8,760 whole hours.');
  if (!d.policy.stop_conditions.length) issues.follow_up.push('Choose at least one stop condition.');
  return issues;
}
