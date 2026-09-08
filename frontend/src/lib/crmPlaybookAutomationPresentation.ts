import type { CRMPlaybookAutomationBinding } from './crmPlaybookTypes';

const blockers: Record<string, string> = {
  permission_changed: 'Access changed — ask an admin to review setup',
  runtime_unavailable: 'Automation is unavailable',
  daily_run_limit: 'Daily check limit reached',
  no_progress: 'Needs your attention — no progress since the last checks',
  needs_owner: 'Assign an owner to continue',
  run_failed: 'Beacon could not finish its check',
  start_uncertain: 'Checking whether Beacon started',
  launch_uncertain: 'Checking whether Beacon started',
  usage_limit: 'AI usage limit reached',
  budget_exceeded: 'AI usage limit reached',
  signal_paused: 'Paused with this signal',
  signal_closed: 'Stopped — signal closed',
  automation_disabled: 'Playbook automation is off',
  paused_by_member: 'Paused by a teammate',
  awaiting_approval: 'Waiting for approval',
  awaiting_result: 'Waiting for an action result',
  awaiting_work: 'Waiting for linked work',
  no_longer_eligible: 'Stopped — this signal no longer matches the playbook',
  contact_restricted: 'Stopped — contact cannot receive messages',
  automation_paused: 'Playbook automation is paused',
  run_start_failed: 'Beacon could not start its check',
};
export function playbookAutomationStatus(binding: CRMPlaybookAutomationBinding | null | undefined) {
  if (!binding) return 'Not started';
  if (binding.blocker) return blockers[binding.blocker] || (binding.enabled ? 'Automation needs attention' : 'Paused');
  return binding.enabled ? 'Beacon is following this signal' : 'Paused';
}
