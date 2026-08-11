import type { AgentPresetKey, AgentRun, TaskUpdateEntry } from '@/lib/pmTypes';
import { getAgentRunDisplayStatus } from '@/components/pm/agentRunConstants';

export interface TaskUpdateAgentPresentation {
  agentName: string;
  agentPresetKey?: AgentPresetKey;
  runId?: string;
  title: string;
  detail?: string;
  automated: boolean;
}

function sentenceCase(value: string) {
  return value ? `${value[0].toUpperCase()}${value.slice(1)}` : value;
}

function readableActorName(entry: TaskUpdateEntry) {
  return entry.actor?.full_name?.trim() || entry.actor?.email?.trim() || '';
}

function metadataString(entry: TaskUpdateEntry, key: string) {
  const value = entry.activity?.metadata?.[key];
  return typeof value === 'string' ? value.trim() : '';
}

function readableAgentName(entry: TaskUpdateEntry) {
  return metadataString(entry, 'agent_name') || entry.agent_name?.trim() || 'Agent';
}

function readableAgentPresetKey(entry: TaskUpdateEntry) {
  const value = metadataString(entry, 'agent_preset_key');
  return value ? value as AgentPresetKey : undefined;
}

function runTriggerSource(run?: AgentRun) {
  const trigger = run?.input?.trigger;
  if (!trigger || typeof trigger !== 'object' || Array.isArray(trigger)) return '';
  const source = (trigger as Record<string, unknown>).source;
  return typeof source === 'string' ? source : '';
}

function normalizeRunAction(value: string) {
  const normalized = value.trim().toLowerCase().replace(/[ -]+/g, '_');
  const aliases: Record<string, string> = {
    approve: 'approved',
    cancel: 'cancelled',
    canceled: 'cancelled',
    request_changes: 'changes_requested',
    requested_changes: 'changes_requested',
    add_note: 'note_added',
    note: 'note_added',
    resume: 'resumed',
  };
  return aliases[normalized] ?? normalized;
}

function namedRun(agentName: string) {
  return agentName === 'Agent' ? 'the agent run' : `${agentName}’s run`;
}

function lifecycleTitle(run: AgentRun, agentName: string, automated: boolean) {
  const status = getAgentRunDisplayStatus(run);
  if (automated) {
    switch (status) {
      case 'queued': return `${agentName} automated run queued`;
      case 'running': return `${agentName} is running via automation`;
      case 'completed': return `${agentName} completed an automated run`;
      case 'failed': return `${agentName} automated run failed`;
      case 'cancelled': return `${agentName} automated run was cancelled`;
      case 'awaiting_input': return `${agentName} automated run is waiting for input`;
      case 'awaiting_reply': return `${agentName} automated run is waiting for a reply`;
      case 'awaiting_approval': return `${agentName} automated run is waiting for approval`;
      case 'awaiting_auth': return `${agentName} automated run is waiting for sign-in`;
      default: return `${agentName} automated run ${status.replace(/_/g, ' ')}`;
    }
  }

  const agentSuffix = agentName === 'Agent' ? '' : ` · ${agentName}`;
  switch (status) {
    case 'queued': return `Agent run queued${agentSuffix}`;
    case 'running': return `Agent run started${agentSuffix}`;
    case 'completed': return `Agent run completed${agentSuffix}`;
    case 'failed': return `Agent run failed${agentSuffix}`;
    case 'cancelled': return `Agent run cancelled${agentSuffix}`;
    case 'awaiting_input': return `Agent run waiting for input${agentSuffix}`;
    case 'awaiting_reply': return `Agent run waiting for a reply${agentSuffix}`;
    case 'awaiting_approval': return `Agent run waiting for approval${agentSuffix}`;
    case 'awaiting_auth': return `Agent run waiting for sign-in${agentSuffix}`;
    default: return `Agent run ${status.replace(/_/g, ' ')}${agentSuffix}`;
  }
}

function automatedActivityTitle(action: string, agentName: string) {
  const run = namedRun(agentName);
  switch (action) {
    case 'started': return `${agentName} started a run via automation`;
    case 'completed': return `${agentName} completed an automated run`;
    case 'failed': return `${agentName} automated run failed`;
    case 'cancelled': return `${run} was cancelled via automation`;
    case 'approved': return `${run} was approved via automation`;
    case 'changes_requested': return `${run} received an automated change request`;
    case 'note_added': return `${run} received an automated note`;
    case 'paused': return `${run} was paused by automation`;
    case 'resumed': return `${agentName} resumed a run via automation`;
    default: return `${run} ${action.replace(/_/g, ' ')} via automation`;
  }
}

function activityTitle(action: string, agentName: string, actorName: string) {
  const run = namedRun(agentName);
  if (actorName) {
    switch (action) {
      case 'started': return `${actorName} started ${run}`;
      case 'completed': return `${actorName} completed ${run}`;
      case 'failed': return `${actorName} marked ${run} as failed`;
      case 'cancelled': return `${actorName} cancelled ${run}`;
      case 'approved': return `${actorName} approved ${run}`;
      case 'changes_requested': return `${actorName} requested changes on ${run}`;
      case 'note_added': return `${actorName} added a note to ${run}`;
      case 'paused': return `${actorName} paused ${run}`;
      case 'resumed': return `${actorName} resumed ${run}`;
      default: return `${actorName} updated ${run} · ${action.replace(/_/g, ' ')}`;
    }
  }

  switch (action) {
    case 'started': return `${agentName} started a run`;
    case 'completed': return `${agentName} completed a run`;
    case 'failed': return `${agentName} run failed`;
    case 'cancelled': return `${run} was cancelled`;
    case 'approved': return `${run} was approved`;
    case 'changes_requested': return `${run} received a change request`;
    case 'note_added': return `${run} received a note`;
    case 'paused': return `${run} was paused`;
    case 'resumed': return `${agentName} resumed a run`;
    default: return `${run} ${action.replace(/_/g, ' ')}`;
  }
}

export function taskUpdateAgentPresentation(
  entry: TaskUpdateEntry,
  linkedRun?: AgentRun,
): TaskUpdateAgentPresentation | null {
  const isRunEntry = entry.kind === 'agent_run' && !!entry.agent_run;
  const isRunActivity = entry.kind === 'change' && entry.activity?.field_name === 'agent_run';
  if (!isRunEntry && !isRunActivity) return null;

  const run = entry.agent_run ?? linkedRun;
  const agentName = readableAgentName(entry);
  const automated = runTriggerSource(run) === 'automation_rule';
  const runId = entry.agent_run?.id || metadataString(entry, 'run_id') || undefined;
  const detail = isRunActivity && normalizeRunAction(entry.activity?.new_value ?? '') === 'note_added'
    ? metadataString(entry, 'note_snippet') || undefined
    : entry.agent_run?.error_message?.trim() || undefined;

  return {
    agentName,
    agentPresetKey: readableAgentPresetKey(entry),
    runId,
    title: isRunEntry
      ? lifecycleTitle(entry.agent_run!, agentName, automated)
      : automated
        ? automatedActivityTitle(normalizeRunAction(entry.activity?.new_value ?? 'updated'), agentName)
        : activityTitle(normalizeRunAction(entry.activity?.new_value ?? 'updated'), agentName, readableActorName(entry)),
    detail,
    automated,
  };
}

export function taskUpdateEventLabel(entry: TaskUpdateEntry, linkedRun?: AgentRun) {
  const agentPresentation = taskUpdateAgentPresentation(entry, linkedRun);
  if (agentPresentation) {
    if (agentPresentation.automated) return agentPresentation.title;
    const action = normalizeRunAction(entry.activity?.new_value ?? '');
    if (entry.kind === 'change') {
      return activityTitle(action || 'updated', agentPresentation.agentName, readableActorName(entry));
    }
    return agentPresentation.title;
  }
  if (entry.kind === 'git' && entry.git_link) {
    if (entry.git_link.pr_number) return `Pull request #${entry.git_link.pr_number} · ${entry.git_link.pr_status ?? 'linked'}`;
    if (entry.git_link.commit_sha) return `Commit ${entry.git_link.commit_sha.slice(0, 7)} linked`;
    return `Branch ${entry.git_link.branch ?? ''} linked`;
  }

  const activity = entry.activity;
  if (!activity) return 'Task updated';
  if (activity.field_name && activity.new_value) {
    return sentenceCase(`${activity.field_name.replace(/_/g, ' ')} changed${activity.old_value ? ` · ${activity.old_value} → ${activity.new_value}` : ` to ${activity.new_value}`}`);
  }
  return sentenceCase(activity.action.replace(/_/g, ' '));
}
