import type { AgentRun } from '@/lib/pmTypes';
import type { AutomationTriggerExecutionListItem } from '@/lib/types';

export interface ActivityTargetPresentation {
  targetType: string;
  targetId: string;
  typeLabel: string;
  key: string;
  title: string;
  primary: string;
  clickable: boolean;
}

export interface ExecutionMetadataPresentation {
  subject: string;
  sourceLabel: string;
  sourceIsFlow: boolean;
  triggerLabel: string;
}

export function truncateMiddle(value: string, start = 8, end = 4) {
  if (value.length <= start + end + 3) return value;
  return `${value.slice(0, start)}...${value.slice(-end)}`;
}

export function normalizeActivityTargetType(value?: string | null) {
  const normalized = value?.trim().toLowerCase().replace(/^pm_/, '') ?? '';
  if (normalized === 'doc') return 'document';
  return normalized === 'story' ? 'task' : normalized;
}

export function activityTargetTypeLabel(targetType: string) {
  switch (normalizeActivityTargetType(targetType)) {
    case 'task':
      return 'Task';
    case 'epic':
      return 'Epic';
    case 'document':
      return 'Document';
    case 'support_conversation':
      return 'Conversation';
    case 'support_coverage_gap':
      return 'Coverage gap';
    case 'crm_contact':
      return 'Contact';
    case 'crm_deal':
      return 'Deal';
    case 'repository':
      return 'Repository';
    case 'workspace':
      return 'Workspace';
    default:
      return targetType ? titleCaseWords(targetType) : 'Target';
  }
}

export function activityTargetIsClickable(targetType: string) {
  return ['task', 'epic', 'document', 'support_conversation', 'crm_contact', 'crm_deal'].includes(normalizeActivityTargetType(targetType));
}

export function buildActivityTargetPresentation(input: {
  targetType?: string | null;
  targetId?: string | null;
  targetTitle?: string | null;
  targetKey?: string | null;
  run?: Pick<AgentRun, 'target_type' | 'target_id' | 'target_info' | 'input' | 'repo_full_name'> | null;
  referenceTitle?: string | null;
}): ActivityTargetPresentation {
  const targetType = normalizeActivityTargetType(input.targetType || input.run?.target_type || '');
  const targetId = (input.targetId || input.run?.target_id || '').trim();
  const typeLabel = activityTargetTypeLabel(targetType);
  const runInput = asRecord(input.run?.input);
  const inputTarget = asRecord(runInput?.target);
  const key = (
    input.targetKey ||
    input.run?.target_info?.task_key ||
    ''
  ).trim();
  const title = (
    input.targetTitle ||
    input.run?.target_info?.title ||
    asNonEmptyString(inputTarget?.title) ||
    asNonEmptyString(runInput?.reference_title) ||
    asNonEmptyString(runInput?.title) ||
    input.run?.repo_full_name ||
    input.referenceTitle ||
    ''
  ).trim();

  let primary: string;
  if (key && title) {
    primary = `${key} \u00b7 ${title}`;
  } else if (key) {
    primary = key;
  } else if (title) {
    primary = title;
  } else if (targetType === 'workspace') {
    primary = 'Workspace';
  } else if (targetId) {
    primary = truncateMiddle(targetId, 8, 4);
  } else {
    primary = 'Workspace';
  }

  return {
    targetType,
    targetId,
    typeLabel,
    key,
    title,
    primary,
    clickable: Boolean(targetId) && activityTargetIsClickable(targetType),
  };
}

export function buildExecutionTargetPresentation(
  item: Pick<
    AutomationTriggerExecutionListItem,
    'target_type' | 'target_id' | 'reference_title'
  > & {
    target_title?: string;
    target_key?: string;
  },
  run?: Pick<AgentRun, 'target_type' | 'target_id' | 'target_info' | 'input' | 'repo_full_name'> | null,
) {
  return buildActivityTargetPresentation({
    targetType: item.target_type,
    targetId: item.target_id,
    targetTitle: item.target_title,
    targetKey: item.target_key,
    referenceTitle: item.reference_title,
    run,
  });
}

export function buildExecutionFlowPresentation(item: Pick<
  AutomationTriggerExecutionListItem,
  'binding_kind' | 'binding_title' | 'reference_title' | 'trigger_title'
>) {
  const flowLabel = item.binding_kind === 'automation_rule'
    ? (item.reference_title?.trim() || item.binding_title?.trim() || 'Flow')
    : '';
  const triggerLabel = item.trigger_title?.trim() || item.binding_title?.trim() || activitySourceLabel(item.binding_kind);
  return {
    flowLabel,
    triggerLabel,
  };
}

export function buildExecutionMetadataPresentation(item: Pick<
  AutomationTriggerExecutionListItem,
  'agent_name' | 'binding_kind' | 'binding_title' | 'reference_title' | 'trigger_title'
>): ExecutionMetadataPresentation {
  const agentName = item.agent_name?.trim() || '';
  const subject = item.binding_kind === 'automation_rule' && (!agentName || agentName === 'Unknown agent') ? 'Flow' : agentName || 'Flow';
  const { flowLabel, triggerLabel } = buildExecutionFlowPresentation(item);
  const sourceLabel = flowLabel || activitySourceLabel(item.binding_kind);
  const normalizedSource = sourceLabel.trim().toLowerCase();
  const normalizedTrigger = triggerLabel.trim().toLowerCase();

  return {
    subject,
    sourceLabel,
    sourceIsFlow: Boolean(flowLabel),
    triggerLabel: normalizedTrigger && normalizedTrigger !== normalizedSource ? triggerLabel : '',
  };
}

export function formatAttentionWaitDuration(startIso?: string, endIso?: string) {
  if (!startIso) return '0m';
  const start = new Date(startIso).getTime();
  const end = endIso ? new Date(endIso).getTime() : Date.now();
  if (!Number.isFinite(start) || !Number.isFinite(end)) return '0m';

  const totalMins = Math.max(0, Math.round((end - start) / 60000));
  const days = Math.floor(totalMins / 1440);
  const hours = Math.floor((totalMins % 1440) / 60);
  const mins = totalMins % 60;

  if (days > 0) {
    return hours > 0 ? `${days}d ${hours}h` : `${days}d`;
  }
  if (hours > 0) {
    return `${hours}h ${mins.toString().padStart(2, '0')}m`;
  }
  return `${mins}m`;
}

function activitySourceLabel(value?: string) {
  switch (value) {
    case 'manual':
      return 'Manual';
    case 'automation_rule':
      return 'Flow';
    case 'schedule':
      return 'Scheduled';
    case 'support_widget':
      return 'Support';
    case 'command_bar':
      return 'Command bar';
    case 'agent_run':
      return 'Agent run';
    case 'task_assignment':
      return 'Assignment';
    default:
      return value ? value.replace(/_/g, ' ') : 'Manual';
  }
}

function titleCaseWords(value: string) {
  return value
    .replace(/_/g, ' ')
    .split(' ')
    .filter(Boolean)
    .map((word) => word.charAt(0).toUpperCase() + word.slice(1))
    .join(' ');
}

function asRecord(value: unknown): Record<string, unknown> | null {
  return value && typeof value === 'object' && !Array.isArray(value) ? value as Record<string, unknown> : null;
}

function asNonEmptyString(value: unknown) {
  return typeof value === 'string' && value.trim() ? value.trim() : '';
}
