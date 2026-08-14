import { format, parseISO } from 'date-fns';

import {
  deliveryTargetLabel,
  normalizeRunAction,
  taskUpdateAgentPresentation,
  type TaskUpdateAgentPresentation,
} from '@/components/pm/task-detail/taskUpdateEventLabel';
import { getActivityActorLabel } from '@/components/pm/ActivityTimeline';
import type { ActivityLogEntry, TaskUpdateEntry } from '@/lib/pmTypes';
import { truncateText } from '@/lib/utils';

export interface EpicActivityPresentation {
  title: string;
  detail?: string;
  agent?: TaskUpdateAgentPresentation;
  emphasizedValues: string[];
}

function metadataString(entry: ActivityLogEntry, key: string) {
  const value = entry.activity.metadata?.[key];
  return typeof value === 'string' ? value.trim() : '';
}

function readableValue(entry: ActivityLogEntry, side: 'old' | 'new') {
  return metadataString(entry, `${side}_label`) || entry.activity[`${side}_value`]?.trim() || '';
}

function quoted(value: string) {
  return `“${truncateText(value, 72)}”`;
}

function readableDate(value: string) {
  if (!value) return '';
  try {
    return format(parseISO(value), 'MMM d, yyyy');
  } catch {
    return value;
  }
}

function healthLabel(value: string) {
  const labels: Record<string, string> = {
    no_health: 'No health',
    on_track: 'On track',
    at_risk: 'At risk',
    off_track: 'Off track',
  };
  return labels[value] ?? value.replaceAll('_', ' ');
}

function valueChange(
  oldValue: string,
  newValue: string,
  copy: { set: (value: string) => string; clear: (value: string) => string; change: (oldValue: string, newValue: string) => string },
) {
  if (!oldValue && newValue) return copy.set(newValue);
  if (oldValue && !newValue) return copy.clear(oldValue);
  if (oldValue && newValue && oldValue !== newValue) return copy.change(oldValue, newValue);
  return '';
}

function activityAction(entry: ActivityLogEntry) {
  const { activity } = entry;
  const oldValue = readableValue(entry, 'old');
  const newValue = readableValue(entry, 'new');

  if (activity.action === 'created') return 'created this epic';
  if (activity.action === 'archived') return 'archived this epic';
  if (activity.action === 'restored') return 'restored this epic';
  if (activity.action === 'auto-started by automation') return 'started this epic';
  if (activity.action === 'auto-completed by automation') return 'completed this epic';
  if (activity.action === 'attachment_added') return 'attached a file';
  if (activity.action === 'attachment_removed') return 'removed an attachment';

  switch (activity.field_name) {
    case 'name':
      return valueChange(oldValue, newValue, {
        set: (value) => `renamed this epic to ${quoted(value)}`,
        clear: () => 'updated the epic title',
        change: (oldName, newName) => `renamed this epic from ${quoted(oldName)} to ${quoted(newName)}`,
      }) || 'updated the epic title';
    case 'description':
      return 'updated the description';
    case 'epic_state_id':
      return valueChange(oldValue, newValue, {
        set: (value) => `set the state to ${value}`,
        clear: (value) => `cleared the state ${value}`,
        change: (oldState, newState) => `moved this epic from ${oldState} to ${newState}`,
      }) || 'updated the state';
    case 'owner_member_id':
    case 'owner_id':
      return valueChange(oldValue, newValue, {
        set: (value) => `assigned owner ${value}`,
        clear: (value) => `removed owner ${value}`,
        change: (oldOwner, newOwner) => `changed the owner from ${oldOwner} to ${newOwner}`,
      }) || 'updated the owner';
    case 'team_id':
      return valueChange(oldValue, newValue, {
        set: (value) => `assigned this epic to team ${value}`,
        clear: (value) => `removed this epic from team ${value}`,
        change: (oldTeam, newTeam) => `moved this epic from team ${oldTeam} to ${newTeam}`,
      }) || 'updated the team';
    case 'planned_start_date': {
      const oldDate = readableDate(oldValue);
      const newDate = readableDate(newValue);
      return valueChange(oldDate, newDate, {
        set: (value) => `set the start date to ${value}`,
        clear: () => 'cleared the start date',
        change: (oldDateValue, newDateValue) => `changed the start date from ${oldDateValue} to ${newDateValue}`,
      }) || 'updated the start date';
    }
    case 'deadline': {
      const oldDate = readableDate(oldValue);
      const newDate = readableDate(newValue);
      return valueChange(oldDate, newDate, {
        set: (value) => `set the due date to ${value}`,
        clear: () => 'cleared the due date',
        change: (oldDateValue, newDateValue) => `changed the due date from ${oldDateValue} to ${newDateValue}`,
      }) || 'updated the due date';
    }
    case 'health':
      return valueChange(healthLabel(oldValue), healthLabel(newValue), {
        set: (value) => `set health to ${value}`,
        clear: () => 'cleared health',
        change: (oldHealth, newHealth) => `changed health from ${oldHealth} to ${newHealth}`,
      }) || 'updated health';
    case 'health_comment':
      return 'updated the health note';
    case 'planning_repository_id':
      return valueChange(oldValue, newValue, {
        set: (value) => `set the code repository to ${value}`,
        clear: (value) => `removed the code repository ${value}`,
        change: (oldRepo, newRepo) => `changed the code repository from ${oldRepo} to ${newRepo}`,
      }) || 'updated the code repository';
    case 'assigned_agent_id':
      return valueChange(oldValue, newValue, {
        set: (value) => `assigned planning agent ${value}`,
        clear: (value) => `removed planning agent ${value}`,
        change: (oldAgent, newAgent) => `changed the planning agent from ${oldAgent} to ${newAgent}`,
      }) || 'updated the planning agent';
    case 'color':
      return 'changed the epic color';
    case 'label': {
      const label = newValue || oldValue;
      if (activity.action === 'label_added') return label ? `added label ${label}` : 'added a label';
      if (activity.action === 'label_removed') return label ? `removed label ${label}` : 'removed a label';
      return 'updated the labels';
    }
    case 'approved_spec_version_id':
      return 'approved a new specification version';
    case 'delivery_target': {
      const target = deliveryTargetLabel(newValue);
      return target ? `changed the delivery target to ${target}` : 'changed the delivery target';
    }
  }

  return activity.action.replaceAll('_', ' ');
}

function activityValueEmphasis(entry: ActivityLogEntry) {
  const oldLabel = metadataString(entry, 'old_label');
  const newLabel = metadataString(entry, 'new_label');
  const oldValue = entry.activity.old_value?.trim() || '';
  const newValue = entry.activity.new_value?.trim() || '';

  switch (entry.activity.field_name) {
    case 'name':
      return [oldValue, newValue];
    case 'epic_state_id':
    case 'owner_member_id':
    case 'owner_id':
    case 'team_id':
    case 'planning_repository_id':
    case 'assigned_agent_id':
    case 'label':
      return [oldLabel, newLabel];
    case 'planned_start_date':
    case 'deadline':
      return [readableDate(oldValue), readableDate(newValue)];
    case 'health':
      return [healthLabel(oldValue), healthLabel(newValue)];
    case 'delivery_target':
      return [deliveryTargetLabel(oldValue), deliveryTargetLabel(newValue)];
    default:
      return [];
  }
}

function asTaskUpdateEntry(entry: ActivityLogEntry): TaskUpdateEntry {
  const actor = entry.actor ? {
    ...entry.actor,
    avatar_url: entry.actor.avatar_url ?? undefined,
    avatar_style: entry.actor.avatar_style ?? undefined,
    avatar_seed: entry.actor.avatar_seed ?? undefined,
    avatar_background_mode: entry.actor.avatar_background_mode ?? undefined,
    avatar_background_color: entry.actor.avatar_background_color ?? undefined,
  } : undefined;
  return {
    id: `activity:${entry.activity.id}`,
    kind: 'change',
    occurred_at: entry.activity.created_at,
    actor,
    activity: entry.activity,
  };
}

export function epicActivityPresentation(entry: ActivityLogEntry): EpicActivityPresentation {
  const agent = taskUpdateAgentPresentation(asTaskUpdateEntry(entry));
  const humanActorName = entry.actor?.full_name?.trim() || entry.actor?.email?.trim() || '';
  if (agent) {
    return {
      title: agent.title,
      detail: agent.detail,
      agent,
      emphasizedValues: [humanActorName, agent.agentName],
    };
  }

  const actorName = getActivityActorLabel(entry.activity.action, entry.actor);
  return {
    title: `${actorName} ${activityAction(entry)}`,
    emphasizedValues: [humanActorName, ...activityValueEmphasis(entry)],
  };
}

function hasMetadata(entry: ActivityLogEntry) {
  return !!entry.activity.metadata && Object.keys(entry.activity.metadata).length > 0;
}

export function isInformationFreeEpicActivity(entry: ActivityLogEntry) {
  const { activity } = entry;
  return activity.action === 'updated'
    && !activity.field_name
    && !activity.old_value
    && !activity.new_value
    && !hasMetadata(entry);
}

export function filterRedundantEpicActivity(entries: ActivityLogEntry[]) {
  const seenTerminalRunActions = new Set<string>();
  return entries.filter((entry) => {
    if (isInformationFreeEpicActivity(entry)) return false;
    if (entry.activity.field_name !== 'agent_run') return true;

    const runId = metadataString(entry, 'run_id');
    const action = normalizeRunAction(entry.activity.new_value ?? '');
    if (!runId || !['completed', 'failed', 'cancelled'].includes(action)) return true;

    const key = `${runId}:${action}`;
    if (seenTerminalRunActions.has(key)) return false;
    seenTerminalRunActions.add(key);
    return true;
  });
}
