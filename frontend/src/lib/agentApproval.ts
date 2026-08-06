import type { AgentApprovalMode } from '@/lib/pmTypes';

export const AGENT_APPROVAL_OPTIONS: ReadonlyArray<{
  value: AgentApprovalMode;
  label: string;
  description: string;
}> = [
  {
    value: 'risk_based',
    label: 'Approve high-risk actions',
    description: 'Reads and routine reversible changes run immediately. Sensitive and destructive actions require approval.',
  },
  {
    value: 'mutating_tools',
    label: 'Approve writes and actions',
    description: 'Read-only work starts immediately. A team member approves each write or other mutating action.',
  },
  {
    value: 'always',
    label: 'Approve before run and actions',
    description: 'A team member approves before the run starts and again for mutating actions.',
  },
  {
    value: 'never',
    label: 'No approval',
    description: 'Runs and mutating actions execute automatically.',
  },
  {
    value: 'preset_default',
    label: 'Use agent default',
    description: 'Use the default approval behavior for this agent profile.',
  },
];

export function agentApprovalDescription(mode: AgentApprovalMode): string {
  return AGENT_APPROVAL_OPTIONS.find((option) => option.value === mode)?.description ?? '';
}
