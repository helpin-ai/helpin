import type { CRMPlaybookDefinition } from './crmPlaybookTypes';

export const playbookSetupSteps = [
  { id: 'purpose', label: 'Purpose & scope' },
  { id: 'milestones', label: 'Milestones' },
  { id: 'team', label: 'Team & permissions' },
  { id: 'follow_up', label: 'Monitoring' },
] as const;
export type PlaybookSetupStep = typeof playbookSetupSteps[number]['id'];
export type PlaybookSetupIssues = Record<PlaybookSetupStep, string[]>;

// Completion describes definition readiness, never execution or activation.
export function playbookSetupIssues(d: CRMPlaybookDefinition, escalationAvailable: boolean | 'loading' | 'error'): PlaybookSetupIssues {
  const issues: PlaybookSetupIssues = { purpose: [], milestones: [], team: [], follow_up: [] };
  if (!d.name.trim()) issues.purpose.push('Enter a playbook name.');
  else if (d.name.length > 160) issues.purpose.push('Keep the playbook name within 160 characters.');
  if (!d.objective.trim()) issues.purpose.push('Enter a desired outcome.');
  else if (d.objective.length > 4000) issues.purpose.push('Keep the desired outcome within 4,000 characters.');
  if (d.description.length > 2000) issues.purpose.push('Keep the description within 2,000 characters.');
  if (!d.eligibility.commercial_motions.length) issues.purpose.push('Select at least one matching stage.');
  if (!d.milestones?.length) issues.milestones.push('Add at least one milestone.');
  if ((d.milestones?.length || 0) > 20) issues.milestones.push('Keep up to 20 milestones.');
  d.milestones?.forEach((milestone, index) => {
    if (!milestone.name.trim()) issues.milestones.push(`Milestone ${index + 1}: Enter a name.`);
    else if (milestone.name.length > 160) issues.milestones.push(`Milestone ${index + 1}: Keep the name within 160 characters.`);
    if (!milestone.success_criteria.trim()) issues.milestones.push(`Milestone ${index + 1}: Add success criteria.`);
    else if (milestone.success_criteria.length > 2000) issues.milestones.push(`Milestone ${index + 1}: Keep success criteria within 2,000 characters.`);
  });
  if (!d.responsibilities.escalation_member_id) issues.team.push('Select an escalation owner.');
  else if (escalationAvailable === 'loading') issues.team.push('Checking escalation owner availability…');
  else if (escalationAvailable === 'error') issues.team.push('Could not verify the escalation owner. Reload team members.');
  else if (!escalationAvailable) issues.team.push('Select an active escalation owner.');
  if (!['signal_owner', 'account_owner', 'deal_owner', 'customer_success_owner'].includes(d.responsibilities.owner_role)) issues.team.push('Select an owner role.');
  if (!['next_action_owner', 'signal_owner'].includes(d.responsibilities.approver_role)) issues.team.push('Select who approvals go to.');
  if (![ 'approval_required', 'not_allowed' ].includes(d.policy.outbound_messages)) issues.team.push('Set permission for customer messages.');
  if (![ 'approval_required', 'not_allowed' ].includes(d.policy.crm_changes)) issues.team.push('Set permission for CRM changes.');
  if (![ 'approval_required', 'not_allowed' ].includes(d.policy.pm_tasks)) issues.team.push('Set permission for tasks.');
  if (!Number.isInteger(d.policy.check_after_hours) || d.policy.check_after_hours < 1 || d.policy.check_after_hours > 8760) issues.follow_up.push('Set Check after between 1 and 8,760 whole hours.');
  if (!Number.isInteger(d.policy.escalate_after_hours) || d.policy.escalate_after_hours < d.policy.check_after_hours || d.policy.escalate_after_hours > 8760) issues.follow_up.push('Set Escalate after at or after Check after, up to 8,760 whole hours.');
  if (!d.policy.stop_conditions.length) issues.follow_up.push('Select at least one Stop when condition.');
  return issues;
}
