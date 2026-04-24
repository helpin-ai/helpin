import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react';
import { useQuery } from '@tanstack/react-query';
import {
  ArrowRight01Icon,
  Clock03Icon,
  FilterIcon,
  GitBranchIcon,
  GitPullRequestIcon,
  MoreHorizontalIcon,
  PlayIcon,
  PlusSignIcon,
  SparklesIcon,
  Tag01Icon,
  ZapIcon,
} from '@/lib/icons';
import { toast } from 'sonner';
import { RepositoryBranchPicker } from '@/components/git/RepositoryBranchPicker';
import { BASE_BRANCH_TOKEN, TASK_BRANCH_TOKEN, describeMergeInto, describeRunBranchOverrides } from '@/lib/branchLabels';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Skeleton } from '@/components/ui/skeleton';
import { Textarea } from '@/components/ui/textarea';
import { useWorkspaceAccess, usePermissions } from '@/hooks/queries';
import { useAutomationFlows, useAutomationOverview, useAgents, useWorkflows } from '@/hooks/queries';
import { useWorkspaceSettings } from '@/hooks/queries/useSettings';
import { useWorkspaceTeams } from '@/hooks/useWorkspaceTeams';
import { useTitle } from '@/hooks/useTitle';
import { automationService } from '@/lib/services/automationService';
import { gitService } from '@/lib/services/gitService';
import { pmEpicService } from '@/lib/services/pmEpicService';
import { pmTaskService } from '@/lib/services/pmTaskService';
import { queryKeys } from '@/lib/queryKeys';
import { unwrap } from '@/lib/queryUtils';
import type { AutomationInventoryItem } from '@/lib/types';
import type { AutomationRule, EpicWithStats, GitRepository, Task, WorkflowState, WorkflowWithStates } from '@/lib/pmTypes';
import { buildAutomationActivityPath } from '@/lib/automationUi';
import { cn } from '@/lib/utils';
import { useWorkspaceStore } from '@/stores/workspaceStore';

export type AutomationFlowsSearch = {
  workflow?: string;
  team?: string;
  template?: string;
  template_title?: string;
  template_description?: string;
  show_trigger?: string;
  show_trigger_title?: string;
  show_rule?: string;
  show_rule_title?: string;
  create_event_rule?: boolean;
  trigger_type?: string;
  agent_id?: string;
  repo_full_name?: string;
  branch?: string;
  base_branch?: string;
  tag_name?: string;
  conclusion?: string;
  target_mode?: 'event' | 'task' | 'epic' | 'repository' | 'workspace';
  target_id?: string;
};

type FlowDraft = {
  name: string;
  description: string;
  triggerType: string;
  workflowId: string;
  triggerStateId: string;
  actionType: 'start_agent_run' | 'move_to_state' | 'merge_branch';
  agentId: string;
  targetMode: 'event' | 'task' | 'epic' | 'repository' | 'workspace';
  targetId: string;
  targetStateId: string;
  targetBranch: string;
  runBaseBranch: string;
  runWorkingBranch: string;
  repoFullName: string;
  branch: string;
  baseBranch: string;
  tagName: string;
  conclusion: string;
  cronCategory: string;
  cronMode: 'simple' | 'advanced';
  scheduleFrequency: 'hourly' | 'daily' | 'weekly' | 'monthly';
  scheduleMinute: string;
  scheduleTime: string;
  scheduleWeekdays: number[];
  scheduleDayOfMonth: string;
};

const WORKFLOW_TRIGGER_TYPES = ['task.state_entered', 'agent_run.approved'] as const;
const TRIGGER_OPTIONS = [
  { value: 'task.state_entered', label: 'Task enters a workflow state', group: 'Workflow events' },
  { value: 'agent_run.approved', label: 'Interactive run is approved', group: 'Workflow events' },
  { value: 'github.push', label: 'GitHub push arrives', group: 'GitHub events' },
  { value: 'github.pull_request_opened', label: 'GitHub pull request opens', group: 'GitHub events' },
  { value: 'github.pull_request_merged', label: 'GitHub pull request merges', group: 'GitHub events' },
  { value: 'github.pull_request_review_requested', label: 'GitHub review is requested', group: 'GitHub events' },
  { value: 'github.release_published', label: 'GitHub release publishes', group: 'GitHub events' },
  { value: 'github.check_suite_completed', label: 'GitHub check suite completes', group: 'GitHub events' },
  { value: 'cron', label: 'Schedule ticks', group: 'Advanced' },
] as const;

const ACTION_LABELS: Record<FlowDraft['actionType'], string> = {
  start_agent_run: 'Start an agent run',
  move_to_state: 'Move the task to a state',
  merge_branch: 'Merge into',
};

const TARGET_LABELS: Record<FlowDraft['targetMode'], string> = {
  event: 'the task / repo that triggered it',
  task: 'a specific task',
  epic: 'a specific epic',
  repository: 'a specific repository',
  workspace: 'this workspace',
};

const TARGET_SHORT_LABELS: Record<FlowDraft['targetMode'], string> = {
  event: 'whatever triggered it',
  task: 'a specific task',
  epic: 'a specific epic',
  repository: 'a specific repository',
  workspace: 'this workspace',
};

const SCHEDULE_PRESET_OPTIONS = [
  { value: 'hourly', label: 'Every hour', schedule: '0 * * * *', hint: 'top of every hour (UTC)' },
  { value: 'daily', label: 'Every day', schedule: '0 0 * * *', hint: 'midnight UTC' },
  { value: 'weekly', label: 'Every week', schedule: '0 0 * * 1', hint: 'Mondays at midnight UTC' },
] as const;

const SCHEDULE_WEEKDAY_OPTIONS = [
  { value: 1, label: 'Mon' },
  { value: 2, label: 'Tue' },
  { value: 3, label: 'Wed' },
  { value: 4, label: 'Thu' },
  { value: 5, label: 'Fri' },
  { value: 6, label: 'Sat' },
  { value: 0, label: 'Sun' },
] as const;

type ParsedSimpleSchedule = {
  frequency: FlowDraft['scheduleFrequency'];
  minute: string;
  time: string;
  weekdays: number[];
  dayOfMonth: string;
};

const LEGACY_CRON_CATEGORY_TO_PRESET: Record<string, (typeof SCHEDULE_PRESET_OPTIONS)[number]['value']> = {
  workspace_hourly: 'hourly',
  workspace_daily: 'daily',
  workspace_weekly: 'weekly',
};

function scheduleExpressionFromConfig(config: Record<string, unknown> | undefined) {
  const schedule = stringValue(config?.schedule);
  if (schedule.trim()) return schedule.trim();

  const preset = stringValue(config?.preset);
  if (preset.trim()) {
    return SCHEDULE_PRESET_OPTIONS.find((option) => option.value === preset)?.schedule ?? '';
  }

  const category = stringValue(config?.category);
  const legacyPreset = LEGACY_CRON_CATEGORY_TO_PRESET[category];
  if (legacyPreset) {
    return SCHEDULE_PRESET_OPTIONS.find((option) => option.value === legacyPreset)?.schedule ?? '';
  }
  if (category.includes(' ')) return category.trim();
  return '';
}

function schedulePresetForExpression(expression: string) {
  const trimmed = expression.trim();
  return SCHEDULE_PRESET_OPTIONS.find((option) => option.schedule === trimmed) ?? null;
}

function clampScheduleNumber(value: string, min: number, max: number, fallback: number) {
  const parsed = Number.parseInt(value, 10);
  if (!Number.isFinite(parsed)) return String(fallback);
  return String(Math.min(max, Math.max(min, parsed)));
}

function normalizeScheduleTime(value: string) {
  if (!/^\d{2}:\d{2}$/.test(value)) return '09:00';
  const [hourText, minuteText] = value.split(':');
  const hour = Number.parseInt(hourText, 10);
  const minute = Number.parseInt(minuteText, 10);
  if (!Number.isFinite(hour) || !Number.isFinite(minute)) return '09:00';
  if (hour < 0 || hour > 23 || minute < 0 || minute > 59) return '09:00';
  return `${String(hour).padStart(2, '0')}:${String(minute).padStart(2, '0')}`;
}

function normalizeScheduleWeekdays(weekdays: number[]) {
  const normalized = Array.from(new Set(weekdays.filter((value) => Number.isInteger(value) && value >= 0 && value <= 6)));
  if (normalized.length === 0) return [1];
  return [...normalized].sort(
    (left, right) =>
      SCHEDULE_WEEKDAY_OPTIONS.findIndex((option) => option.value === left)
      - SCHEDULE_WEEKDAY_OPTIONS.findIndex((option) => option.value === right),
  );
}

function buildSimpleScheduleExpression(draft: Pick<
  FlowDraft,
  'scheduleFrequency' | 'scheduleMinute' | 'scheduleTime' | 'scheduleWeekdays' | 'scheduleDayOfMonth'
>) {
  const frequency = draft.scheduleFrequency;
  if (frequency === 'hourly') {
    const minute = clampScheduleNumber(draft.scheduleMinute, 0, 59, 0);
    return `${minute} * * * *`;
  }

  const [hour, minute] = normalizeScheduleTime(draft.scheduleTime).split(':');
  if (frequency === 'daily') {
    return `${Number.parseInt(minute, 10)} ${Number.parseInt(hour, 10)} * * *`;
  }
  if (frequency === 'weekly') {
    return `${Number.parseInt(minute, 10)} ${Number.parseInt(hour, 10)} * * ${normalizeScheduleWeekdays(draft.scheduleWeekdays).join(',')}`;
  }
  return `${Number.parseInt(minute, 10)} ${Number.parseInt(hour, 10)} ${clampScheduleNumber(draft.scheduleDayOfMonth, 1, 31, 1)} * *`;
}

function parseSimpleScheduleExpression(expression: string): ParsedSimpleSchedule | null {
  const trimmed = expression.trim();
  if (!trimmed) return null;
  const parts = trimmed.split(/\s+/);
  if (parts.length !== 5) return null;
  const [minuteField, hourField, dayOfMonthField, monthField, dayOfWeekField] = parts;
  if (monthField !== '*') return null;

  if (
    dayOfMonthField === '*'
    && dayOfWeekField === '*'
    && hourField === '*'
    && /^\d{1,2}$/.test(minuteField)
  ) {
    return {
      frequency: 'hourly',
      minute: clampScheduleNumber(minuteField, 0, 59, 0),
      time: '09:00',
      weekdays: [1],
      dayOfMonth: '1',
    };
  }

  if (!/^\d{1,2}$/.test(minuteField) || !/^\d{1,2}$/.test(hourField)) return null;
  const time = `${clampScheduleNumber(hourField, 0, 23, 9).padStart(2, '0')}:${clampScheduleNumber(minuteField, 0, 59, 0).padStart(2, '0')}`;

  if (dayOfMonthField === '*' && dayOfWeekField === '*') {
    return {
      frequency: 'daily',
      minute: clampScheduleNumber(minuteField, 0, 59, 0),
      time,
      weekdays: [1],
      dayOfMonth: '1',
    };
  }

  if (dayOfMonthField === '*' && /^[0-6](,[0-6])*$/.test(dayOfWeekField)) {
    return {
      frequency: 'weekly',
      minute: clampScheduleNumber(minuteField, 0, 59, 0),
      time,
      weekdays: normalizeScheduleWeekdays(dayOfWeekField.split(',').map((value) => Number.parseInt(value, 10))),
      dayOfMonth: '1',
    };
  }

  if (/^\d{1,2}$/.test(dayOfMonthField) && dayOfWeekField === '*') {
    return {
      frequency: 'monthly',
      minute: clampScheduleNumber(minuteField, 0, 59, 0),
      time,
      weekdays: [1],
      dayOfMonth: clampScheduleNumber(dayOfMonthField, 1, 31, 1),
    };
  }

  return null;
}

function applyScheduleExpressionToDraft(draft: FlowDraft, expression: string) {
  const parsed = parseSimpleScheduleExpression(expression);
  draft.cronCategory = expression || '0 * * * *';
  if (parsed) {
    draft.cronMode = 'simple';
    draft.scheduleFrequency = parsed.frequency;
    draft.scheduleMinute = parsed.minute;
    draft.scheduleTime = parsed.time;
    draft.scheduleWeekdays = parsed.weekdays;
    draft.scheduleDayOfMonth = parsed.dayOfMonth;
    return;
  }
  draft.cronMode = 'advanced';
}

function scheduleExpressionForDraft(draft: FlowDraft) {
  if (draft.triggerType !== 'cron') return draft.cronCategory.trim();
  if (draft.cronMode === 'advanced') return draft.cronCategory.trim();
  return buildSimpleScheduleExpression(draft).trim();
}

function describeSimpleSchedule(schedule: ParsedSimpleSchedule) {
  if (schedule.frequency === 'hourly') {
    return `Every hour at :${schedule.minute.padStart(2, '0')} UTC`;
  }
  if (schedule.frequency === 'daily') {
    return `Every day at ${schedule.time} UTC`;
  }
  if (schedule.frequency === 'weekly') {
    const days = schedule.weekdays
      .map((weekday) => SCHEDULE_WEEKDAY_OPTIONS.find((option) => option.value === weekday)?.label)
      .filter(Boolean)
      .join(', ');
    return `Every week on ${days} at ${schedule.time} UTC`;
  }
  return `Every month on day ${schedule.dayOfMonth} at ${schedule.time} UTC`;
}

function describeScheduleExpression(expression: string) {
  const parsed = parseSimpleScheduleExpression(expression);
  if (parsed) {
    return describeSimpleSchedule(parsed);
  }
  const preset = schedulePresetForExpression(expression);
  if (preset) {
    return `${preset.label} (${preset.hint})`;
  }
  return expression.trim();
}

function serializeScheduleConfig(expression: string) {
  const schedule = expression.trim();
  const preset = schedulePresetForExpression(schedule);
  const config: Record<string, string> = { schedule };
  if (preset) {
    config.preset = preset.value;
  }
  return config;
}

const FLOW_EMPTY_STATE_CARDS: Array<{
  icon: typeof PlayIcon;
  title: string;
  desc: string;
}> = [
  {
    icon: ZapIcon,
    title: 'Event-driven',
    desc: 'Fires when a task changes state, a PR is merged, a tag ships, or on a schedule.',
  },
  {
    icon: SparklesIcon,
    title: 'Runs an agent',
    desc: 'Pair a trigger with any agent — Lens reviews, Forge builds, Scribe drafts, etc.',
  },
  {
    icon: Clock03Icon,
    title: 'Safe by default',
    desc: 'Flows start in review-first mode; flip them to autonomous once you trust the output.',
  },
];

type FlowTemplate = {
  id: string;
  title: string;
  description: string;
  icon: typeof PlayIcon;
  tone: 'emerald' | 'blue' | 'purple' | 'amber' | 'rose' | 'slate';
  apply: (base: FlowDraft) => FlowDraft;
};

const FLOW_TEMPLATES: FlowTemplate[] = [
  {
    id: 'review-merged-prs',
    title: 'Review merged PRs',
    description: 'When a PR merges, start an agent to review the diff.',
    icon: GitPullRequestIcon,
    tone: 'emerald',
    apply: (base) => ({ ...base, name: 'Review merged PRs', triggerType: 'github.pull_request_merged', baseBranch: 'main', actionType: 'start_agent_run' }),
  },
  {
    id: 'run-on-check-failure',
    title: 'Fix failing checks',
    description: 'When CI check suite completes, start an agent if it failed.',
    icon: ZapIcon,
    tone: 'amber',
    apply: (base) => ({ ...base, name: 'Fix failing checks', triggerType: 'github.check_suite_completed', conclusion: 'failure', actionType: 'start_agent_run' }),
  },
  {
    id: 'approve-advances',
    title: 'Advance on approval',
    description: 'When an interactive agent run is approved, move the task to the next state.',
    icon: SparklesIcon,
    tone: 'blue',
    apply: (base) => ({ ...base, name: 'Advance on approval', triggerType: 'agent_run.approved', actionType: 'move_to_state' }),
  },
  {
    id: 'merge-on-done',
    title: 'Merge when done',
    description: 'When a task enters Done, merge its branch into main.',
    icon: GitBranchIcon,
    tone: 'purple',
    apply: (base) => ({ ...base, name: 'Merge when done', triggerType: 'task.state_entered', actionType: 'merge_branch', targetBranch: 'main' }),
  },
  {
    id: 'release-tag',
    title: 'Run on release',
    description: 'When a release is published, kick off a release agent.',
    icon: Tag01Icon,
    tone: 'rose',
    apply: (base) => ({ ...base, name: 'Run on release', triggerType: 'github.release_published', actionType: 'start_agent_run' }),
  },
  {
    id: 'hourly-tick',
    title: 'Hourly digest',
    description: 'On every hour, run an agent across the workspace or against a fixed target.',
    icon: Clock03Icon,
    tone: 'slate',
    apply: (base) => ({ ...base, name: 'Hourly digest', triggerType: 'cron', cronCategory: '0 * * * *', actionType: 'start_agent_run', targetMode: 'workspace' }),
  },
];

const TEMPLATE_TONE: Record<FlowTemplate['tone'], string> = {
  emerald: 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400',
  blue: 'bg-blue-500/10 text-blue-600 dark:text-blue-400',
  purple: 'bg-purple-500/10 text-purple-600 dark:text-purple-400',
  amber: 'bg-amber-500/10 text-amber-600 dark:text-amber-400',
  rose: 'bg-rose-500/10 text-rose-600 dark:text-rose-400',
  slate: 'bg-muted text-foreground/70',
};

function defaultDraft(): FlowDraft {
  return {
    name: '',
    description: '',
    triggerType: 'github.pull_request_merged',
    workflowId: '',
    triggerStateId: '',
    actionType: 'start_agent_run',
    agentId: '',
    targetMode: 'event',
    targetId: '',
    targetStateId: '',
    targetBranch: '',
    runBaseBranch: '',
    runWorkingBranch: '',
    repoFullName: '',
    branch: '',
    baseBranch: 'main',
    tagName: '',
    conclusion: '',
    cronCategory: '0 * * * *',
    cronMode: 'simple',
    scheduleFrequency: 'hourly',
    scheduleMinute: '0',
    scheduleTime: '09:00',
    scheduleWeekdays: [1],
    scheduleDayOfMonth: '1',
  };
}

function isWorkflowTrigger(triggerType: string) {
  return WORKFLOW_TRIGGER_TYPES.includes(triggerType as (typeof WORKFLOW_TRIGGER_TYPES)[number]);
}

function allowedActions(triggerType: string): FlowDraft['actionType'][] {
  if (isWorkflowTrigger(triggerType)) {
    return ['start_agent_run', 'move_to_state', 'merge_branch'];
  }
  return ['start_agent_run'];
}

function triggerLabel(triggerType: string) {
  return TRIGGER_OPTIONS.find((option) => option.value === triggerType)?.label ?? triggerType;
}

function buildStateIndex(workflows: WorkflowWithStates[]) {
  const statesById = new Map<string, WorkflowState>();
  for (const workflow of workflows) {
    for (const state of workflow.states) {
      statesById.set(state.id, state);
    }
  }
  return statesById;
}

function stringValue(value: unknown) {
  return typeof value === 'string' ? value : '';
}

function firstStateIdForWorkflow(workflows: WorkflowWithStates[], workflowId: string) {
  return workflows.find((workflow) => workflow.workflow.id === workflowId)?.states[0]?.id ?? '';
}

function applyTriggerDefaults(next: FlowDraft, workflows: WorkflowWithStates[]) {
  const actions = allowedActions(next.triggerType);
  if (!actions.includes(next.actionType)) {
    next.actionType = actions[0];
  }
  if (!isWorkflowTrigger(next.triggerType)) {
    next.workflowId = '';
    next.triggerStateId = '';
    if (next.actionType !== 'start_agent_run') {
      next.actionType = 'start_agent_run';
    }
  } else if (!next.workflowId && workflows[0]) {
    next.workflowId = workflows[0].workflow.id;
    next.triggerStateId = firstStateIdForWorkflow(workflows, next.workflowId);
  }
  if (next.actionType !== 'start_agent_run') {
    next.targetMode = 'event';
    next.targetId = '';
    next.runBaseBranch = '';
    next.runWorkingBranch = '';
  }
  if (next.actionType !== 'move_to_state') {
    next.targetStateId = '';
  }
  if (next.actionType !== 'merge_branch') {
    next.targetBranch = '';
  }
}

function draftFromSearch(search: AutomationFlowsSearch, workflows: WorkflowWithStates[]): FlowDraft | null {
  const requestedTrigger = search.trigger_type || search.template;
  if (!requestedTrigger) {
    return null;
  }
  const draft = defaultDraft();
  draft.triggerType = requestedTrigger;
  draft.agentId = search.agent_id ?? '';
  draft.repoFullName = search.repo_full_name ?? '';
  draft.branch = search.branch ?? '';
  draft.baseBranch = search.base_branch ?? 'main';
  draft.tagName = search.tag_name ?? '';
  draft.conclusion = search.conclusion ?? '';
  draft.targetMode = search.target_mode ?? 'event';
  draft.targetId = search.target_id ?? '';
  if (search.template_title?.trim()) {
    draft.name = search.template_title.replace(/\s+template$/i, '').trim();
  }
  applyTriggerDefaults(draft, workflows);
  if (isWorkflowTrigger(draft.triggerType) && search.workflow) {
    draft.workflowId = search.workflow;
    draft.triggerStateId = firstStateIdForWorkflow(workflows, search.workflow);
  }
  return draft;
}

function draftFromRule(rule: AutomationRule, workflows: WorkflowWithStates[]): FlowDraft {
  const draft = defaultDraft();
  draft.name = rule.name;
  draft.description = rule.description ?? '';
  draft.triggerType = rule.trigger_type;
  draft.workflowId = rule.workflow_id ?? '';
  draft.triggerStateId = stringValue(rule.trigger_config?.state_id);
  draft.actionType = (rule.action_type as FlowDraft['actionType']) || 'start_agent_run';
  draft.agentId = stringValue(rule.action_config?.agent_id);
  draft.targetMode = (stringValue(rule.action_config?.target_type) as FlowDraft['targetMode']) || 'event';
  draft.targetId = stringValue(rule.action_config?.target_id);
  draft.targetStateId = stringValue(rule.action_config?.target_state_id);
  draft.targetBranch = stringValue(rule.action_config?.target_branch);
  draft.runBaseBranch = stringValue(rule.action_config?.base_branch);
  draft.runWorkingBranch = stringValue(rule.action_config?.working_branch);
  draft.repoFullName = stringValue(rule.trigger_config?.repo_full_name);
  draft.branch = stringValue(rule.trigger_config?.branch);
  draft.baseBranch = stringValue(rule.trigger_config?.base_branch) || 'main';
  draft.tagName = stringValue(rule.trigger_config?.tag_name);
  draft.conclusion = stringValue(rule.trigger_config?.conclusion);
  applyScheduleExpressionToDraft(draft, scheduleExpressionFromConfig(rule.trigger_config) || '0 * * * *');
  applyTriggerDefaults(draft, workflows);
  return draft;
}

function serializeDraft(draft: FlowDraft, workspaceId: string) {
  let triggerConfig: Record<string, string> = {};
  if (draft.triggerType === 'task.state_entered') {
    triggerConfig = { state_id: draft.triggerStateId };
  } else if (draft.triggerType === 'agent_run.approved') {
    triggerConfig = { state_id: draft.triggerStateId };
  } else if (draft.triggerType === 'github.push') {
    triggerConfig = { repo_full_name: draft.repoFullName.trim(), branch: draft.branch.trim() };
  } else if (
    draft.triggerType === 'github.pull_request_opened'
    || draft.triggerType === 'github.pull_request_merged'
    || draft.triggerType === 'github.pull_request_review_requested'
  ) {
    triggerConfig = { repo_full_name: draft.repoFullName.trim(), base_branch: draft.baseBranch.trim() };
  } else if (draft.triggerType === 'github.release_published') {
    triggerConfig = { repo_full_name: draft.repoFullName.trim(), tag_name: draft.tagName.trim() };
  } else if (draft.triggerType === 'github.check_suite_completed') {
    triggerConfig = { repo_full_name: draft.repoFullName.trim(), branch: draft.branch.trim(), conclusion: draft.conclusion.trim() };
  } else if (draft.triggerType === 'cron') {
    triggerConfig = serializeScheduleConfig(scheduleExpressionForDraft(draft));
  }

  let actionConfig: Record<string, unknown> = {};
  if (draft.actionType === 'start_agent_run') {
    actionConfig = { agent_id: draft.agentId };
    if (draft.targetMode === 'workspace') {
      actionConfig.target_type = 'workspace';
      actionConfig.target_id = workspaceId;
    } else if (draft.targetMode !== 'event' && draft.targetId) {
      actionConfig.target_type = draft.targetMode;
      actionConfig.target_id = draft.targetId;
    }
    if (draft.runBaseBranch.trim()) {
      actionConfig.base_branch = draft.runBaseBranch.trim();
    }
    if (draft.runWorkingBranch.trim()) {
      actionConfig.working_branch = draft.runWorkingBranch.trim();
    }
  } else if (draft.actionType === 'move_to_state') {
    actionConfig = { target_state_id: draft.targetStateId };
  } else if (draft.actionType === 'merge_branch') {
    actionConfig = { target_branch: draft.targetBranch.trim() };
  }

  return {
    name: draft.name.trim(),
    description: draft.description.trim() || undefined,
    workflow_id: isWorkflowTrigger(draft.triggerType) ? draft.workflowId : undefined,
    trigger_type: draft.triggerType,
    trigger_config: triggerConfig,
    action_type: draft.actionType,
    action_config: actionConfig,
  };
}

function validateDraft(draft: FlowDraft) {
  if (!draft.name.trim()) return 'Give the flow a name';
  if (isWorkflowTrigger(draft.triggerType)) {
    if (!draft.workflowId) return 'Choose a workflow';
    if (!draft.triggerStateId) return 'Choose the state that starts this flow';
  }
  if (draft.triggerType === 'github.push' && !draft.repoFullName.trim() && !draft.branch.trim()) {
    return 'Add a repository or branch filter';
  }
  if (
    (draft.triggerType === 'github.pull_request_opened'
      || draft.triggerType === 'github.pull_request_merged'
      || draft.triggerType === 'github.pull_request_review_requested')
    && !draft.repoFullName.trim()
    && !draft.baseBranch.trim()
  ) {
    return 'Add a repository or base branch filter';
  }
  if (draft.triggerType === 'github.release_published' && !draft.repoFullName.trim() && !draft.tagName.trim()) {
    return 'Add a repository or tag filter';
  }
  if (draft.triggerType === 'github.check_suite_completed' && !draft.repoFullName.trim() && !draft.branch.trim() && !draft.conclusion.trim()) {
    return 'Add a repository, branch, or conclusion filter';
  }
  if (draft.triggerType === 'cron' && !scheduleExpressionForDraft(draft)) {
    return 'Add a cron schedule';
  }
  if (draft.actionType === 'start_agent_run') {
    if (!draft.agentId) return 'Choose an agent';
    if (draft.targetMode !== 'event' && draft.targetMode !== 'workspace' && !draft.targetId) {
      return 'Choose a target';
    }
  }
  if (draft.actionType === 'move_to_state' && !draft.targetStateId) {
    return 'Choose the destination state';
  }
  if (draft.actionType === 'merge_branch' && !draft.targetBranch.trim()) {
    return 'Enter the branch to merge into';
  }
  return null;
}

function describeFilters(rule: AutomationRule, statesById: Map<string, WorkflowState>) {
  const filters: string[] = [];
  const triggerStateId = stringValue(rule.trigger_config?.state_id);
  if (triggerStateId) {
    const stateName = statesById.get(triggerStateId)?.name;
    if (stateName) filters.push(`state = ${stateName}`);
  }
  const repoFullName = stringValue(rule.trigger_config?.repo_full_name);
  const branch = stringValue(rule.trigger_config?.branch);
  const baseBranch = stringValue(rule.trigger_config?.base_branch);
  const tagName = stringValue(rule.trigger_config?.tag_name);
  const conclusion = stringValue(rule.trigger_config?.conclusion);
  const schedule = scheduleExpressionFromConfig(rule.trigger_config);
  if (repoFullName) filters.push(`repo = ${repoFullName}`);
  if (branch) filters.push(`branch = ${branch}`);
  if (baseBranch) filters.push(`base branch = ${baseBranch}`);
  if (tagName) filters.push(`tag = ${tagName}`);
  if (conclusion) filters.push(`conclusion = ${conclusion}`);
  if (schedule) filters.push(`schedule = ${describeScheduleExpression(schedule)}`);
  return filters.length ? filters.join(' · ') : 'No additional filters';
}

function describeThen(rule: AutomationRule, statesById: Map<string, WorkflowState>) {
  if (rule.action_type === 'start_agent_run') {
    const parts = ['Start an agent run'];
    const baseBranch = stringValue(rule.action_config?.base_branch);
    const workingBranch = stringValue(rule.action_config?.working_branch);
    const overrides = describeRunBranchOverrides(baseBranch, workingBranch);
    if (overrides) parts.push(overrides);
    return parts.join(' ');
  }
  if (rule.action_type === 'move_to_state') {
    const stateId = stringValue(rule.action_config?.target_state_id);
    return stateId ? `Move the task to ${statesById.get(stateId)?.name ?? 'another state'}` : 'Move the task to another state';
  }
  if (rule.action_type === 'merge_branch') {
    const branch = stringValue(rule.action_config?.target_branch);
    return describeMergeInto(branch || BASE_BRANCH_TOKEN);
  }
  return rule.action_type.replaceAll('_', ' ');
}

function relativeTime(value?: string) {
  if (!value) return 'No recent activity';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return 'No recent activity';
  const diff = Date.now() - date.getTime();
  if (diff < 60_000) return 'just now';
  if (diff < 3_600_000) return `${Math.floor(diff / 60_000)}m ago`;
  if (diff < 86_400_000) return `${Math.floor(diff / 3_600_000)}h ago`;
  return `${Math.floor(diff / 86_400_000)}d ago`;
}

function draftSentence(draft: FlowDraft, workflows: WorkflowWithStates[], statesById: Map<string, WorkflowState>, agents: Map<string, string>) {
  const workflowName = workflows.find((workflow) => workflow.workflow.id === draft.workflowId)?.workflow.name;
  const triggerStateName = statesById.get(draft.triggerStateId)?.name;
  const destinationStateName = statesById.get(draft.targetStateId)?.name;
  const when = triggerLabel(draft.triggerType);
  const conditions = (() => {
    if (isWorkflowTrigger(draft.triggerType)) {
      const parts = [];
      if (workflowName) parts.push(`workflow = ${workflowName}`);
      if (triggerStateName) parts.push(`state = ${triggerStateName}`);
      return parts.join(' · ') || 'No additional filters';
    }
    const parts = [];
    if (draft.repoFullName.trim()) parts.push(`repo = ${draft.repoFullName.trim()}`);
    if (draft.branch.trim()) parts.push(`branch = ${draft.branch.trim()}`);
    if (draft.baseBranch.trim()) parts.push(`base branch = ${draft.baseBranch.trim()}`);
    if (draft.tagName.trim()) parts.push(`tag = ${draft.tagName.trim()}`);
    if (draft.conclusion.trim()) parts.push(`conclusion = ${draft.conclusion.trim()}`);
    if (scheduleExpressionForDraft(draft) && draft.triggerType === 'cron') parts.push(`schedule = ${describeScheduleExpression(scheduleExpressionForDraft(draft))}`);
    return parts.join(' · ') || 'No additional filters';
  })();
  const then = draft.actionType === 'move_to_state'
    ? `Move the task to ${destinationStateName || 'another state'}`
    : draft.actionType === 'merge_branch'
      ? describeMergeInto(draft.targetBranch.trim() || BASE_BRANCH_TOKEN)
      : [
          ACTION_LABELS[draft.actionType],
          describeRunBranchOverrides(draft.runBaseBranch.trim(), draft.runWorkingBranch.trim()),
        ].filter(Boolean).join(' ');
  const using = draft.actionType === 'start_agent_run' ? (agents.get(draft.agentId) ?? 'Choose an agent') : 'No agent';
  const on = draft.actionType === 'start_agent_run' ? TARGET_LABELS[draft.targetMode] : 'Current task context';
  return { when, conditions, then, using, on };
}

function describeFlowTitle(rule: AutomationRule, statesById: Map<string, WorkflowState>, agentNames: Map<string, string>) {
  const triggerStateId = stringValue(rule.trigger_config?.state_id);
  const stateName = statesById.get(triggerStateId)?.name;

  let triggerPart = '';
  if (rule.trigger_type === 'task.state_entered' && stateName) {
    triggerPart = `Story enters ${stateName}`;
  } else if (rule.trigger_type === 'agent_run.approved' && stateName) {
    triggerPart = `Approved in ${stateName}`;
  } else if (rule.trigger_type.startsWith('github.pull_request')) {
    const action = rule.trigger_type === 'github.pull_request_merged' ? 'merged'
      : rule.trigger_type === 'github.pull_request_opened' ? 'opened'
      : 'review requested';
    const baseBranch = stringValue(rule.trigger_config?.base_branch);
    triggerPart = `PR ${action}${baseBranch ? ` to base branch ${baseBranch}` : ''}`;
  } else if (rule.trigger_type === 'github.push') {
    triggerPart = 'Push arrives';
  } else if (rule.trigger_type === 'github.release_published') {
    triggerPart = 'Release published';
  } else if (rule.trigger_type === 'github.check_suite_completed') {
    triggerPart = 'Check suite completes';
  } else if (rule.trigger_type === 'cron') {
    triggerPart = 'Schedule ticks';
  } else {
    triggerPart = triggerLabel(rule.trigger_type);
  }

  let actionPart = '';
  if (rule.action_type === 'start_agent_run') {
    const agentId = stringValue(rule.action_config?.agent_id);
    const agentName = agentNames.get(agentId);
    const overrides = describeRunBranchOverrides(
      stringValue(rule.action_config?.base_branch),
      stringValue(rule.action_config?.working_branch),
    );
    actionPart = agentName ? `Run ${agentName}${overrides}` : 'Start agent';
  } else if (rule.action_type === 'move_to_state') {
    const targetStateId = stringValue(rule.action_config?.target_state_id);
    const targetStateName = statesById.get(targetStateId)?.name;
    actionPart = targetStateName ? `Move to ${targetStateName}` : 'Move task state';
  } else if (rule.action_type === 'merge_branch') {
    const branch = stringValue(rule.action_config?.target_branch);
    actionPart = describeMergeInto(branch || BASE_BRANCH_TOKEN);
  } else {
    actionPart = rule.action_type.replaceAll('_', ' ');
  }

  return `${triggerPart} \u2192 ${actionPart}`;
}

function flowHasError(rule: AutomationRule, agentNames: Map<string, string>) {
  if (rule.action_type !== 'start_agent_run') return false;
  const agentId = stringValue(rule.action_config?.agent_id);
  return !agentId || !agentNames.has(agentId);
}

type FlowState = 'running' | 'paused' | 'error' | 'draft';

function deriveFlowState(rule: AutomationRule, healthItem?: AutomationInventoryItem): FlowState {
  if (!rule.enabled) return 'paused';
  if (healthItem?.health.last_error_at) return 'error';
  if (healthItem?.health.last_success_at || healthItem?.health.last_seen_at) return 'running';
  return 'draft';
}

const FLOW_STATE_STYLES: Record<FlowState, { pill: string; dot: string; label: string }> = {
  running: {
    pill: 'border-sky-500/30 bg-sky-500/10 text-sky-700 dark:text-sky-400',
    dot: 'bg-current',
    label: 'Running',
  },
  paused: {
    pill: 'border-amber-500/30 bg-amber-500/10 text-amber-700 dark:text-amber-400',
    dot: 'bg-amber-500',
    label: 'Paused',
  },
  error: {
    pill: 'border-rose-500/40 bg-rose-500/10 text-rose-600 dark:text-rose-400',
    dot: 'bg-rose-500',
    label: 'Error',
  },
  draft: {
    pill: 'border-border/60 bg-muted/40 text-muted-foreground',
    dot: 'bg-muted-foreground/40',
    label: 'Draft',
  },
};

function FlowStatePill({ state }: { state: FlowState }) {
  const style = FLOW_STATE_STYLES[state];
  return (
    <Badge variant="outline" className={cn('gap-1.5 rounded-full px-2.5 py-0.5 text-[11px] font-medium', style.pill)}>
      <span className={cn('h-1.5 w-1.5 rounded-full', style.dot)} />
      {style.label}
    </Badge>
  );
}

function readMetricNumber(metrics: Record<string, unknown> | undefined, keys: string[]): number | undefined {
  if (!metrics) return undefined;
  for (const key of keys) {
    const value = metrics[key];
    if (typeof value === 'number' && Number.isFinite(value)) return value;
    if (typeof value === 'string' && value.trim() && !Number.isNaN(Number(value))) return Number(value);
  }
  return undefined;
}

function FlowStat({ value, label, sub, tone = 'neutral' }: { value: number | undefined; label: string; sub?: string; tone?: 'neutral' | 'warn' }) {
  return (
    <div>
      <div className={cn('text-xl font-semibold leading-none tracking-tight', tone === 'warn' && value ? 'text-amber-600 dark:text-amber-400' : 'text-foreground')}>
        {value === undefined ? <span className="text-muted-foreground">—</span> : value}
      </div>
      <div className="mt-1.5 text-[11px] text-muted-foreground">{label}</div>
      {sub && <div className="font-mono text-[10px] text-muted-foreground/70">{sub}</div>}
    </div>
  );
}

function FlowFilterBar({
  value,
  count,
  onClear,
}: {
  value: string;
  count: number;
  onClear: () => void;
}) {
  return (
    <div className="flex flex-col gap-2 rounded-lg border border-border/70 bg-muted/30 px-3 py-3 sm:flex-row sm:items-center">
      <div className="flex items-center gap-2 text-xs font-medium uppercase tracking-[0.14em] text-muted-foreground">
        <FilterIcon className="h-3.5 w-3.5" />
        Flow filter
      </div>
      <Input
        value={value}
        readOnly
        className="h-8 flex-1 bg-background text-xs"
        aria-label="Active flow filter"
      />
      <span className="text-xs text-muted-foreground">
        {count} {count === 1 ? 'match' : 'matches'}
      </span>
      <Button type="button" variant="ghost" size="sm" className="h-8 px-2 text-xs" onClick={onClear}>
        Clear
      </Button>
    </div>
  );
}

function FlowRow({
  rule,
  statesById,
  agentNames,
  teamName,
  healthItem,
  workspaceSlug,
  canEdit,
  onEdit,
  onToggle,
  onDelete,
}: {
  rule: AutomationRule;
  statesById: Map<string, WorkflowState>;
  agentNames: Map<string, string>;
  teamName?: string;
  healthItem?: AutomationInventoryItem;
  workspaceSlug?: string;
  canEdit: boolean;
  onEdit: (rule: AutomationRule) => void;
  onToggle: (rule: AutomationRule) => void;
  onDelete: (rule: AutomationRule) => void;
}) {
  const hasError = flowHasError(rule, agentNames);
  const title = describeFlowTitle(rule, statesById, agentNames);
  const filters = describeFilters(rule, statesById);
  const hasFilters = filters !== 'No additional filters';
  const agentId = stringValue(rule.action_config?.agent_id);
  const agentName = agentNames.get(agentId);
  const agentMissing = rule.action_type === 'start_agent_run' && (!agentId || !agentNames.has(agentId));

  const lastRunLabel = (() => {
    if (healthItem?.health.last_success_at) return `Last run ${relativeTime(healthItem.health.last_success_at)}`;
    if (healthItem?.health.last_seen_at) return `Last run ${relativeTime(healthItem.health.last_seen_at)}`;
    return 'Never run';
  })();

  const flowState = deriveFlowState(rule, healthItem);
  const metrics = (healthItem?.health.metrics ?? undefined) as Record<string, unknown> | undefined;
  const handled = readMetricNumber(metrics, ['handled_week', 'runs_week', 'runs_7d', 'handled', 'handled_count']);
  const flagged = readMetricNumber(metrics, ['flagged_week', 'flagged', 'needs_review', 'flagged_count']);
  const lastErrorMessage = healthItem?.health.last_error_message?.trim();

  return (
    <div
      className={cn(
        'rounded-lg border bg-card transition-colors',
        hasError || flowState === 'error'
          ? 'border-destructive/40 bg-destructive/[0.03]'
          : 'border-border/60 hover:border-border',
      )}
    >
      <div className="grid gap-4 px-4 py-3 md:grid-cols-[minmax(0,1fr)_280px]">
        {/* LEFT — title / trigger sentence */}
        <div className="min-w-0 space-y-2">
          <div className="flex items-center gap-2">
            <FlowStatePill state={flowState} />
            <p className="truncate text-sm font-medium text-foreground/90">{title}</p>
            {agentMissing && (
              <Badge variant="destructive" className="text-xs">Missing agent</Badge>
            )}
            <span className="flex-1" />
            {canEdit && (
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button variant="ghost" size="sm" className="h-7 w-7 p-0">
                    <MoreHorizontalIcon className="h-4 w-4" />
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end">
                  <DropdownMenuItem onClick={() => onEdit(rule)}>Edit</DropdownMenuItem>
                  <DropdownMenuItem onClick={() => onToggle(rule)}>
                    {rule.enabled ? 'Disable' : 'Enable'}
                  </DropdownMenuItem>
                  {workspaceSlug && (
                    <DropdownMenuItem asChild>
                      <a href={buildAutomationActivityPath(workspaceSlug, { page: 1, source: 'automation_rule', reference_id: rule.id }, 'trigger-executions')}>
                        Activity
                      </a>
                    </DropdownMenuItem>
                  )}
                  <DropdownMenuItem
                    className="text-destructive focus:text-destructive"
                    onClick={() => onDelete(rule)}
                  >
                    Delete
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
            )}
          </div>

          <div className="flex flex-wrap items-center gap-1.5">
            <Badge variant="secondary" className="text-xs font-normal">
              {rule.trigger_type.replace('github.', '').replaceAll('_', '.')}
              {hasFilters && (
                <span className="ml-1 text-muted-foreground">| {filters}</span>
              )}
            </Badge>
            <ArrowRight01Icon className="h-3.5 w-3.5 shrink-0 text-muted-foreground/50" />
            {rule.action_type === 'start_agent_run' && agentName ? (
              <Badge variant="outline" className="border-emerald-500/30 bg-emerald-500/10 text-xs font-normal text-emerald-600 dark:text-emerald-400">
                {agentName}
              </Badge>
            ) : rule.action_type === 'move_to_state' ? (
              <Badge variant="outline" className="border-blue-500/30 bg-blue-500/10 text-xs font-normal text-blue-600 dark:text-blue-400">
                {describeThen(rule, statesById)}
              </Badge>
            ) : rule.action_type === 'merge_branch' ? (
              <Badge variant="outline" className="border-purple-500/30 bg-purple-500/10 text-xs font-normal text-purple-600 dark:text-purple-400">
                {describeThen(rule, statesById)}
              </Badge>
            ) : (
              <Badge variant="outline" className="text-xs font-normal">
                {describeThen(rule, statesById)}
              </Badge>
            )}
            {teamName && (
              <Badge variant="outline" className="text-xs font-normal">{teamName}</Badge>
            )}
          </div>

          {flowState === 'error' && lastErrorMessage && (
            <div className="rounded-md border border-destructive/30 bg-destructive/5 px-2.5 py-1.5 font-mono text-[11px] text-destructive">
              ⚠ {lastErrorMessage}
            </div>
          )}
        </div>

        {/* RIGHT — outcome strip */}
        <div className="grid grid-cols-2 gap-3 border-t border-border/60 pt-3 md:border-l md:border-t-0 md:pl-4 md:pt-0">
          <FlowStat value={handled} label="Handled" sub="last 7 days" />
          <FlowStat value={flagged} label="Flagged" sub="for review" tone="warn" />
          <div className="col-span-2 flex items-center justify-between border-t border-border/60 pt-2 text-[11px] text-muted-foreground">
            <span className="uppercase tracking-[0.1em]">Last run</span>
            <span className="font-mono text-foreground/80">{lastRunLabel.replace('Last run ', '')}</span>
          </div>
        </div>
      </div>
    </div>
  );
}

function isGithubTrigger(triggerType: string) {
  return triggerType.startsWith('github.');
}
function showRepoField(triggerType: string) {
  return isGithubTrigger(triggerType);
}
function showBranchField(triggerType: string) {
  return triggerType === 'github.push' || triggerType === 'github.check_suite_completed';
}
function showBaseBranchField(triggerType: string) {
  return triggerType.startsWith('github.pull_request_');
}
function showTagField(triggerType: string) {
  return triggerType === 'github.release_published';
}
function showConclusionField(triggerType: string) {
  return triggerType === 'github.check_suite_completed';
}
function showBranchOverrideFields(triggerType: string, actionType: string) {
  return actionType === 'start_agent_run' && isGithubTrigger(triggerType);
}

function SentenceRow({ connector, tone, children }: { connector: string; tone?: 'muted' | 'strong'; children: ReactNode }) {
  return (
    <div className="flex flex-wrap items-center gap-2">
      <span
        className={cn(
          'mt-0.5 w-10 shrink-0 text-[10px] font-semibold uppercase tracking-[0.08em]',
          tone === 'strong' ? 'text-foreground/70' : 'text-muted-foreground/80',
        )}
      >
        {connector}
      </span>
      <div className="flex flex-1 flex-wrap items-center gap-1.5">{children}</div>
    </div>
  );
}

function PillGlue({ children }: { children: ReactNode }) {
  return <span className="text-xs text-muted-foreground">{children}</span>;
}

function PillInput({
  value,
  onChange,
  placeholder,
  className,
  width = 'auto',
  disabled,
  invalid,
}: {
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  className?: string;
  width?: 'auto' | 'sm' | 'md' | 'lg';
  disabled?: boolean;
  invalid?: boolean;
}) {
  const widthClass = width === 'sm' ? 'w-28' : width === 'md' ? 'w-40' : width === 'lg' ? 'w-56' : 'w-auto';
  return (
    <input
      type="text"
      value={value}
      onChange={(event) => onChange(event.target.value)}
      placeholder={placeholder}
      disabled={disabled}
      aria-invalid={invalid || undefined}
      className={cn(
        'inline-flex h-7 items-center rounded-3xl border border-transparent bg-input/50 px-3 text-xs outline-none transition-[color,box-shadow,background-color] placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/30 aria-invalid:border-destructive aria-invalid:ring-3 aria-invalid:ring-destructive/20 disabled:cursor-not-allowed disabled:opacity-50',
        widthClass,
        className,
      )}
    />
  );
}

function FlowComposer({
  workspaceId,
  open,
  mode,
  draft,
  workflows,
  statesById,
  agents,
  tasks,
  epics,
  repositories,
  saving,
  canEdit,
  onOpenChange,
  onBack,
  onDraftChange,
  onSave,
}: {
  workspaceId: string;
  open: boolean;
  mode: 'create' | 'edit';
  draft: FlowDraft;
  workflows: WorkflowWithStates[];
  statesById: Map<string, WorkflowState>;
  agents: Map<string, string>;
  tasks: Task[];
  epics: EpicWithStats[];
  repositories: GitRepository[];
  saving: boolean;
  canEdit: boolean;
  onOpenChange: (open: boolean) => void;
  onBack?: () => void;
  onDraftChange: (updater: (current: FlowDraft) => FlowDraft) => void;
  onSave: () => Promise<void>;
}) {
  const workflowStates = workflows.find((workflow) => workflow.workflow.id === draft.workflowId)?.states ?? [];
  const actionOptions = allowedActions(draft.triggerType);
  const sentence = draftSentence(draft, workflows, statesById, agents);
  const repositoryOptions = repositories.map((repo) => ({ value: repo.full_name, label: repo.full_name }));
  const targetTaskOptions = tasks.map((task) => ({ value: task.id, label: `${task.task_key} · ${task.name}` }));
  const targetEpicOptions = epics.map((epic) => ({ value: epic.epic.id, label: epic.epic.name }));
  const targetRepositoryOptions = repositories.map((repo) => ({ value: repo.id, label: repo.full_name }));
  const selectedRepoId = repositories.find((repo) => repo.full_name === draft.repoFullName)?.id;
  const isCronTrigger = draft.triggerType === 'cron';
  const resolvedScheduleExpression = scheduleExpressionForDraft(draft);
  const simpleSchedulePreview = parseSimpleScheduleExpression(resolvedScheduleExpression);
  const validation = validateDraft(draft);

  useEffect(() => {
    if (draft.targetMode !== 'workspace' || draft.targetId === workspaceId) return;
    onDraftChange((current) => current.targetMode === 'workspace'
      ? { ...current, targetId: workspaceId }
      : current);
  }, [draft.targetId, draft.targetMode, onDraftChange, workspaceId]);

  const updateDraft = (mutate: (current: FlowDraft) => FlowDraft) => onDraftChange((current) => {
    const next = mutate(current);
    const normalized = { ...next };
    normalized.scheduleMinute = clampScheduleNumber(normalized.scheduleMinute, 0, 59, 0);
    normalized.scheduleTime = normalizeScheduleTime(normalized.scheduleTime);
    normalized.scheduleWeekdays = normalizeScheduleWeekdays(normalized.scheduleWeekdays);
    normalized.scheduleDayOfMonth = clampScheduleNumber(normalized.scheduleDayOfMonth, 1, 31, 1);
    applyTriggerDefaults(normalized, workflows);
    return normalized;
  });

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-3xl">
        <DialogHeader>
          <DialogTitle>{mode === 'create' ? 'Create flow' : 'Edit flow'}</DialogTitle>
          <DialogDescription className="text-xs">
            Reads top to bottom as a sentence. Only fields relevant to your trigger and action appear.
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-5">
          <div className="space-y-2">
            <Input
              value={draft.name}
              onChange={(event) => updateDraft((current) => ({ ...current, name: event.target.value }))}
              placeholder="Flow name (e.g. Review merged PRs)"
              className="h-10 text-base font-medium"
              autoFocus={mode === 'create'}
            />
            <Textarea
              rows={2}
              value={draft.description}
              onChange={(event) => updateDraft((current) => ({ ...current, description: event.target.value }))}
              placeholder="Optional description — what is this flow for?"
              className="resize-none text-sm"
            />
          </div>

          <div className="space-y-3 rounded-xl border border-border/60 bg-muted/20 p-4">
            <SentenceRow connector="When" tone="strong">
              <Select
                value={draft.triggerType}
                onValueChange={(value) => updateDraft((current) => ({ ...current, triggerType: value }))}
              >
                <SelectTrigger size="sm" className="min-w-[12rem]">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {Array.from(new Set(TRIGGER_OPTIONS.map((option) => option.group))).map((group) => (
                    <div key={group}>
                      <div className="px-2 py-1 text-[10px] font-semibold uppercase tracking-wide text-muted-foreground">{group}</div>
                      {TRIGGER_OPTIONS.filter((option) => option.group === group).map((option) => (
                        <SelectItem key={option.value} value={option.value}>{option.label}</SelectItem>
                      ))}
                    </div>
                  ))}
                </SelectContent>
              </Select>
            </SentenceRow>

            {isWorkflowTrigger(draft.triggerType) && (
              <SentenceRow connector="if">
                <PillGlue>workflow is</PillGlue>
                <Select
                  value={draft.workflowId}
                  onValueChange={(value) => updateDraft((current) => ({
                    ...current,
                    workflowId: value,
                    triggerStateId: firstStateIdForWorkflow(workflows, value),
                    targetStateId: '',
                  }))}
                >
                  <SelectTrigger size="sm" className="min-w-[10rem]">
                    <SelectValue placeholder="select workflow…" />
                  </SelectTrigger>
                  <SelectContent>
                    {workflows.map((workflow) => (
                      <SelectItem key={workflow.workflow.id} value={workflow.workflow.id}>{workflow.workflow.name}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <PillGlue>and state is</PillGlue>
                <Select
                  value={draft.triggerStateId}
                  onValueChange={(value) => updateDraft((current) => ({ ...current, triggerStateId: value }))}
                >
                  <SelectTrigger size="sm" className="min-w-[9rem]">
                    <SelectValue placeholder="select state…" />
                  </SelectTrigger>
                  <SelectContent>
                    {workflowStates.map((state) => (
                      <SelectItem key={state.id} value={state.id}>{state.name}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </SentenceRow>
            )}

            {showRepoField(draft.triggerType) && (() => {
              const isKnownRepo = repositoryOptions.some((option) => option.value === draft.repoFullName);
              const repoSelectValue = draft.repoFullName && isKnownRepo ? draft.repoFullName : '__custom__';
              return (
                <SentenceRow connector="if">
                  <PillGlue>repository is</PillGlue>
                  <Select
                    value={repoSelectValue}
                    onValueChange={(value) => updateDraft((current) => ({
                      ...current,
                      repoFullName: value === '__custom__' ? '' : value,
                    }))}
                  >
                    <SelectTrigger size="sm" className="min-w-[10rem]">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="__custom__">any / custom…</SelectItem>
                      {repositoryOptions.map((repo) => (
                        <SelectItem key={repo.value} value={repo.value}>{repo.label}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  {repoSelectValue === '__custom__' && (
                    <PillInput
                      value={draft.repoFullName}
                      onChange={(value) => updateDraft((current) => ({ ...current, repoFullName: value }))}
                      placeholder="owner/repo (optional)"
                      width="md"
                    />
                  )}
                </SentenceRow>
              );
            })()}

            {showBranchField(draft.triggerType) && (
              <SentenceRow connector="and">
                <PillGlue>branch is</PillGlue>
                <div className="inline-flex">
                  <RepositoryBranchPicker
                    workspaceId={workspaceId}
                    repositoryId={selectedRepoId}
                    value={draft.branch}
                    onChange={(value) => updateDraft((current) => ({ ...current, branch: value }))}
                    placeholder="main"
                    emptyLabel="any branch"
                    disabled={saving}
                  />
                </div>
              </SentenceRow>
            )}

            {showBaseBranchField(draft.triggerType) && (
              <SentenceRow connector="and">
                <PillGlue>base branch is</PillGlue>
                <div className="inline-flex">
                  <RepositoryBranchPicker
                    workspaceId={workspaceId}
                    repositoryId={selectedRepoId}
                    value={draft.baseBranch}
                    onChange={(value) => updateDraft((current) => ({ ...current, baseBranch: value }))}
                    placeholder="main"
                    emptyLabel="any base branch"
                    disabled={saving}
                  />
                </div>
              </SentenceRow>
            )}

            {showTagField(draft.triggerType) && (
              <SentenceRow connector="and">
                <PillGlue>tag matches</PillGlue>
                <PillInput
                  value={draft.tagName}
                  onChange={(value) => updateDraft((current) => ({ ...current, tagName: value }))}
                  placeholder="v1.0.0"
                  width="sm"
                />
              </SentenceRow>
            )}

            {showConclusionField(draft.triggerType) && (
              <SentenceRow connector="and">
                <PillGlue>conclusion is</PillGlue>
                <Select
                  value={draft.conclusion || '__any__'}
                  onValueChange={(value) => updateDraft((current) => ({ ...current, conclusion: value === '__any__' ? '' : value }))}
                >
                  <SelectTrigger size="sm" className="min-w-[8rem]">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="__any__">any</SelectItem>
                    <SelectItem value="success">success</SelectItem>
                    <SelectItem value="failure">failure</SelectItem>
                    <SelectItem value="cancelled">cancelled</SelectItem>
                    <SelectItem value="timed_out">timed out</SelectItem>
                  </SelectContent>
                </Select>
              </SentenceRow>
            )}

            {isCronTrigger && (
              <>
                <SentenceRow connector="at">
                  <PillGlue>schedule</PillGlue>
                </SentenceRow>
                <div className="ml-12 space-y-3 rounded-xl border border-border/60 bg-background/80 p-3">
                  <div className="flex flex-wrap items-center gap-2">
                    <Button
                      type="button"
                      variant={draft.cronMode === 'simple' ? 'secondary' : 'ghost'}
                      size="sm"
                      onClick={() => updateDraft((current) => {
                        const parsed = parseSimpleScheduleExpression(scheduleExpressionForDraft(current));
                        return {
                          ...current,
                          cronMode: 'simple',
                          scheduleFrequency: parsed?.frequency ?? current.scheduleFrequency,
                          scheduleMinute: parsed?.minute ?? current.scheduleMinute,
                          scheduleTime: parsed?.time ?? current.scheduleTime,
                          scheduleWeekdays: parsed?.weekdays ?? current.scheduleWeekdays,
                          scheduleDayOfMonth: parsed?.dayOfMonth ?? current.scheduleDayOfMonth,
                        };
                      })}
                    >
                      Simple builder
                    </Button>
                    <Button
                      type="button"
                      variant={draft.cronMode === 'advanced' ? 'secondary' : 'ghost'}
                      size="sm"
                      onClick={() => updateDraft((current) => ({
                        ...current,
                        cronMode: 'advanced',
                        cronCategory: scheduleExpressionForDraft(current),
                      }))}
                    >
                      Advanced cron
                    </Button>
                    <span className="text-[11px] text-muted-foreground">
                      Saved as cron under the hood.
                    </span>
                  </div>

                  {draft.cronMode === 'simple' ? (
                    <div className="space-y-3">
                      <div className="flex flex-wrap items-center gap-2">
                        <PillGlue>Runs</PillGlue>
                        <Select
                          value={draft.scheduleFrequency}
                          onValueChange={(value: FlowDraft['scheduleFrequency']) => updateDraft((current) => ({
                            ...current,
                            scheduleFrequency: value,
                          }))}
                        >
                          <SelectTrigger size="sm" className="min-w-[10rem]">
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem value="hourly">Every hour</SelectItem>
                            <SelectItem value="daily">Every day</SelectItem>
                            <SelectItem value="weekly">Every week</SelectItem>
                            <SelectItem value="monthly">Every month</SelectItem>
                          </SelectContent>
                        </Select>

                        {draft.scheduleFrequency === 'hourly' ? (
                          <>
                            <PillGlue>at minute</PillGlue>
                            <Input
                              type="number"
                              min={0}
                              max={59}
                              value={draft.scheduleMinute}
                              onChange={(event) => updateDraft((current) => ({ ...current, scheduleMinute: event.target.value }))}
                              className="h-8 w-24"
                            />
                          </>
                        ) : (
                          <>
                            <PillGlue>at</PillGlue>
                            <Input
                              type="time"
                              value={draft.scheduleTime}
                              onChange={(event) => updateDraft((current) => ({ ...current, scheduleTime: event.target.value }))}
                              className="h-8 w-32"
                            />
                          </>
                        )}

                        {draft.scheduleFrequency === 'monthly' && (
                          <>
                            <PillGlue>on day</PillGlue>
                            <Input
                              type="number"
                              min={1}
                              max={31}
                              value={draft.scheduleDayOfMonth}
                              onChange={(event) => updateDraft((current) => ({ ...current, scheduleDayOfMonth: event.target.value }))}
                              className="h-8 w-24"
                            />
                          </>
                        )}
                      </div>

                      {draft.scheduleFrequency === 'weekly' && (
                        <div className="space-y-1.5">
                          <PillGlue>On days</PillGlue>
                          <div className="flex flex-wrap gap-1.5">
                            {SCHEDULE_WEEKDAY_OPTIONS.map((option) => {
                              const selected = draft.scheduleWeekdays.includes(option.value);
                              return (
                                <button
                                  key={option.value}
                                  type="button"
                                  className={cn(
                                    'rounded-md border px-2.5 py-1.5 text-xs font-medium transition-colors',
                                    selected
                                      ? 'border-primary/40 bg-primary/10 text-primary'
                                      : 'border-border text-muted-foreground hover:bg-accent hover:text-foreground',
                                  )}
                                  onClick={() => updateDraft((current) => ({
                                    ...current,
                                    scheduleWeekdays: selected
                                      ? (current.scheduleWeekdays.length === 1
                                        ? current.scheduleWeekdays
                                        : current.scheduleWeekdays.filter((value) => value !== option.value))
                                      : [...current.scheduleWeekdays, option.value],
                                  }))}
                                >
                                  {option.label}
                                </button>
                              );
                            })}
                          </div>
                        </div>
                      )}

                      <div className="rounded-lg border border-border/60 bg-muted/30 px-3 py-2 text-xs text-muted-foreground">
                        <div className="font-medium text-foreground/80">
                          {simpleSchedulePreview ? describeSimpleSchedule(simpleSchedulePreview) : 'Choose a supported schedule'}
                        </div>
                        <div className="mt-1 font-mono text-[11px] text-muted-foreground/80">
                          {resolvedScheduleExpression || '—'}
                        </div>
                      </div>
                    </div>
                  ) : (
                    <div className="space-y-2">
                      <Input
                        value={draft.cronCategory}
                        onChange={(event) => updateDraft((current) => ({ ...current, cronCategory: event.target.value }))}
                        placeholder="0 * * * *"
                        className="font-mono"
                      />
                      <p className="text-[11px] text-muted-foreground">
                        Use standard 5-field cron. If it matches a simple hourly, daily, weekly, or monthly pattern, you can switch back to the builder.
                      </p>
                    </div>
                  )}
                </div>
              </>
            )}

            <div className="my-1 h-px bg-border/40" />

            <SentenceRow connector="then" tone="strong">
              <Select
                value={draft.actionType}
                onValueChange={(value: FlowDraft['actionType']) => updateDraft((current) => ({ ...current, actionType: value }))}
              >
                <SelectTrigger size="sm" className="min-w-[12rem]">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {actionOptions.map((action) => (
                    <SelectItem key={action} value={action}>{ACTION_LABELS[action]}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
              {draft.actionType === 'start_agent_run' && (
                <>
                  <PillGlue>using</PillGlue>
                  <Select
                    value={draft.agentId}
                    onValueChange={(value) => updateDraft((current) => ({ ...current, agentId: value }))}
                  >
                    <SelectTrigger
                      size="sm"
                      className={cn('min-w-[10rem]', !draft.agentId && 'border-destructive/50 bg-destructive/5')}
                    >
                      <SelectValue placeholder="choose an agent…" />
                    </SelectTrigger>
                    <SelectContent>
                      {Array.from(agents.entries()).map(([id, name]) => (
                        <SelectItem key={id} value={id}>{name}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </>
              )}
              {draft.actionType === 'move_to_state' && (
                <>
                  <PillGlue>to</PillGlue>
                  <Select
                    value={draft.targetStateId}
                    onValueChange={(value) => updateDraft((current) => ({ ...current, targetStateId: value }))}
                  >
                    <SelectTrigger size="sm" className="min-w-[10rem]">
                      <SelectValue placeholder="choose state…" />
                    </SelectTrigger>
                    <SelectContent>
                      {workflowStates.map((state) => (
                        <SelectItem key={state.id} value={state.id}>{state.name}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </>
              )}
              {draft.actionType === 'merge_branch' && (
                <>
                  <PillGlue>into branch</PillGlue>
                  <PillInput
                    value={draft.targetBranch}
                    onChange={(value) => updateDraft((current) => ({ ...current, targetBranch: value }))}
                    placeholder="main"
                    width="md"
                  />
                </>
              )}
            </SentenceRow>

            {draft.actionType === 'start_agent_run' && (
              <SentenceRow connector="on">
                <Select
                  value={draft.targetMode}
                  onValueChange={(value: FlowDraft['targetMode']) => updateDraft((current) => ({
                    ...current,
                    targetMode: value,
                    targetId: value === 'workspace' ? workspaceId : value === 'event' ? '' : '',
                  }))}
                >
                  <SelectTrigger size="sm" className="min-w-[14rem]">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="event" disabled={isCronTrigger}>{TARGET_SHORT_LABELS.event}</SelectItem>
                    <SelectItem value="task">{TARGET_SHORT_LABELS.task}</SelectItem>
                    <SelectItem value="epic">{TARGET_SHORT_LABELS.epic}</SelectItem>
                    <SelectItem value="repository">{TARGET_SHORT_LABELS.repository}</SelectItem>
                    <SelectItem value="workspace">{TARGET_SHORT_LABELS.workspace}</SelectItem>
                  </SelectContent>
                </Select>
                {draft.targetMode === 'workspace' && (
                  <>
                    <PillGlue>—</PillGlue>
                    <Badge variant="secondary" className="rounded-full px-3 py-1 text-xs font-normal">
                      current workspace
                    </Badge>
                  </>
                )}
                {draft.targetMode !== 'event' && draft.targetMode !== 'workspace' && (
                  <>
                    <PillGlue>—</PillGlue>
                    <Select
                      value={draft.targetId}
                      onValueChange={(value) => updateDraft((current) => ({ ...current, targetId: value }))}
                    >
                      <SelectTrigger size="sm" className="min-w-[12rem]">
                        <SelectValue placeholder="pick one…" />
                      </SelectTrigger>
                      <SelectContent>
                        {(draft.targetMode === 'task' ? targetTaskOptions : draft.targetMode === 'epic' ? targetEpicOptions : targetRepositoryOptions).map((option) => (
                          <SelectItem key={option.value} value={option.value}>{option.label}</SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </>
                )}
              </SentenceRow>
            )}

            {showBranchOverrideFields(draft.triggerType, draft.actionType) && (
              <details className="ml-12 group">
                <summary className="cursor-pointer list-none text-[11px] text-muted-foreground hover:text-foreground/80">
                  <span className="group-open:hidden">+ branch overrides</span>
                  <span className="hidden group-open:inline">− branch overrides</span>
                </summary>
                <div className="mt-2 space-y-2">
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="w-20 shrink-0 text-[10px] font-semibold uppercase tracking-[0.08em] text-muted-foreground/80">base</span>
                    <PillGlue>agent checks out</PillGlue>
                    <PillInput
                      value={draft.runBaseBranch}
                      onChange={(value) => updateDraft((current) => ({ ...current, runBaseBranch: value }))}
                      placeholder={`${BASE_BRANCH_TOKEN} or release/2026.04`}
                      width="lg"
                    />
                  </div>
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="w-20 shrink-0 text-[10px] font-semibold uppercase tracking-[0.08em] text-muted-foreground/80">task</span>
                    <PillGlue>branch is</PillGlue>
                    <PillInput
                      value={draft.runWorkingBranch}
                      onChange={(value) => updateDraft((current) => ({ ...current, runWorkingBranch: value }))}
                      placeholder={TASK_BRANCH_TOKEN}
                      width="lg"
                    />
                  </div>
                  <p className="text-[11px] leading-relaxed text-muted-foreground">
                    Use <code className="rounded bg-muted px-1 py-0.5 text-[10px]">{TASK_BRANCH_TOKEN}</code> for the agent's working branch, and <code className="rounded bg-muted px-1 py-0.5 text-[10px]">{BASE_BRANCH_TOKEN}</code> for the default base branch.
                  </p>
                </div>
              </details>
            )}
          </div>

          <div className="flex flex-wrap items-center gap-1.5 rounded-lg border border-border/50 bg-background px-3 py-2 text-xs">
            <span className="inline-flex items-center rounded border border-border/60 bg-muted/40 px-1.5 py-0.5 text-[10px] font-medium uppercase tracking-wide text-muted-foreground">Preview</span>
            <span className="font-medium">{sentence.when}</span>
            {sentence.conditions !== 'No additional filters' && (
              <>
                <ArrowRight01Icon className="h-3 w-3 shrink-0 text-muted-foreground/50" />
                <span className="text-muted-foreground">{sentence.conditions}</span>
              </>
            )}
            <ArrowRight01Icon className="h-3 w-3 shrink-0 text-muted-foreground/50" />
            <span>{sentence.then}</span>
            {draft.actionType === 'start_agent_run' && (
              <>
                <span className="text-muted-foreground/40">·</span>
                <span className="text-muted-foreground">{sentence.using}</span>
              </>
            )}
          </div>

          {validation && (
            <p className="flex items-center gap-2 text-xs text-destructive">
              <span className="h-1.5 w-1.5 rounded-full bg-destructive" />
              {validation}
            </p>
          )}
        </div>

        <DialogFooter className="gap-2 sm:justify-between">
          <div>
            {mode === 'create' && onBack && (
              <Button type="button" variant="ghost" size="sm" onClick={onBack}>
                ← Back to templates
              </Button>
            )}
          </div>
          <div className="flex gap-2">
            <Button type="button" variant="ghost" onClick={() => onOpenChange(false)}>Cancel</Button>
            <Button type="button" onClick={() => void onSave()} disabled={saving || !canEdit || !!validation}>
              {saving ? 'Saving…' : mode === 'create' ? 'Create flow' : 'Save changes'}
            </Button>
          </div>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function FlowTemplateGallery({
  open,
  onOpenChange,
  onPick,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onPick: (template: FlowTemplate | null) => void;
}) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-3xl">
        <DialogHeader>
          <DialogTitle>Create a flow</DialogTitle>
          <DialogDescription>Start from a template, or build from scratch.</DialogDescription>
        </DialogHeader>

        <div className="grid gap-3 sm:grid-cols-2 md:grid-cols-3">
          {FLOW_TEMPLATES.map((template) => {
            const Icon = template.icon;
            return (
              <button
                key={template.id}
                type="button"
                onClick={() => onPick(template)}
                className="group flex flex-col items-start gap-2 rounded-xl border border-border/60 bg-card p-4 text-left transition-colors hover:border-border hover:bg-muted/40 focus-visible:border-ring focus-visible:outline-none focus-visible:ring-3 focus-visible:ring-ring/30"
              >
                <span className={cn('flex h-8 w-8 items-center justify-center rounded-lg', TEMPLATE_TONE[template.tone])}>
                  <Icon className="h-4 w-4" />
                </span>
                <p className="text-sm font-medium leading-tight">{template.title}</p>
                <p className="text-xs leading-relaxed text-muted-foreground">{template.description}</p>
              </button>
            );
          })}
          <button
            type="button"
            onClick={() => onPick(null)}
            className="group flex flex-col items-start gap-2 rounded-xl border border-dashed border-border/60 bg-transparent p-4 text-left transition-colors hover:border-border hover:bg-muted/30 focus-visible:border-ring focus-visible:outline-none focus-visible:ring-3 focus-visible:ring-ring/30"
          >
            <span className="flex h-8 w-8 items-center justify-center rounded-lg bg-muted text-foreground/70">
              <PlusSignIcon className="h-4 w-4" />
            </span>
            <p className="text-sm font-medium leading-tight">Start from scratch</p>
            <p className="text-xs leading-relaxed text-muted-foreground">Build a custom flow from the ground up.</p>
          </button>
        </div>
      </DialogContent>
    </Dialog>
  );
}

export function AutomationFlowsPage({
  search,
  onSearchChange,
}: {
  search: AutomationFlowsSearch;
  onSearchChange: (updates: Partial<AutomationFlowsSearch>) => void;
}) {
  useTitle('Automation Flows');
  const workspace = useWorkspaceStore((state) => state.currentWorkspace);
  const workspaceId = workspace?.id ?? '';
  const workspaceSlug = workspace?.slug;
  const { data: access } = useWorkspaceAccess(workspaceId);
  const permissions = usePermissions(access);
  const settingsQuery = useWorkspaceSettings(workspaceId);
  const inventoryQuery = useAutomationOverview(workspaceId);
  const { data: agents = [] } = useAgents(workspaceId);
  const { data: workflows = [] } = useWorkflows(workspaceId);
  const rulesQuery = useAutomationFlows(workspaceId);
  const { teams } = useWorkspaceTeams(workspaceId);
  const tasksQuery = useQuery({
    queryKey: ['pm', workspaceId, 'flow-composer', 'tasks'],
    queryFn: async () => unwrap(await pmTaskService.list(workspaceId, { archived: false, per_page: 100 })),
    enabled: !!workspaceId,
    staleTime: 60_000,
  });
  const epicsQuery = useQuery({
    queryKey: ['pm', workspaceId, 'flow-composer', 'epics'],
    queryFn: async () => unwrap(await pmEpicService.list(workspaceId, { archived: false })),
    enabled: !!workspaceId,
    staleTime: 60_000,
  });
  const repositoriesQuery = useQuery({
    queryKey: queryKeys.git.repositories(workspaceId),
    queryFn: async () => unwrap(await gitService.listRepositories(workspaceId)),
    enabled: !!workspaceId,
    staleTime: 60_000,
  });

  const [composerOpen, setComposerOpen] = useState(false);
  const [galleryOpen, setGalleryOpen] = useState(false);
  const [editingRuleId, setEditingRuleId] = useState<string | null>(null);
  const [draft, setDraft] = useState<FlowDraft>(defaultDraft());
  const [saving, setSaving] = useState(false);
  const searchSignature = useMemo(() => JSON.stringify(search), [search]);
  const [appliedSearchSignature, setAppliedSearchSignature] = useState('');

  const authoredFlows = rulesQuery.data ?? [];
  const tasks = tasksQuery.data?.data ?? [];
  const epics = epicsQuery.data ?? [];
  const repositories = repositoriesQuery.data ?? [];
  const statesById = useMemo(() => buildStateIndex(workflows), [workflows]);
  const agentNames = useMemo(() => new Map(agents.map((agent) => [agent.id, agent.name])), [agents]);
  const flowHealth = useMemo(() => {
    const map = new Map<string, AutomationInventoryItem>();
    for (const item of inventoryQuery.data?.items ?? []) {
      if (item.kind !== 'automation_rule') continue;
      const ruleId = item.inventory_id.replace('automation_rule:rule:', '');
      map.set(ruleId, item);
    }
    return map;
  }, [inventoryQuery.data?.items]);
  const highlightedFlows = useMemo(
    () => {
      if (search.show_rule) return authoredFlows.filter((rule) => rule.id === search.show_rule);
      if (search.show_trigger) return authoredFlows.filter((rule) => rule.trigger_type === search.show_trigger);
      return authoredFlows;
    },
    [authoredFlows, search.show_rule, search.show_trigger],
  );
  const teamNamesById = useMemo(() => new Map(teams.map((t) => [t.id, t.name])), [teams]);
  const [activeTab, setActiveTab] = useState<string>('all');
  const teamTabs = useMemo(() => {
    const counts = new Map<string, number>();
    let uncategorized = 0;
    for (const rule of highlightedFlows) {
      if (rule.team_id && teamNamesById.has(rule.team_id)) {
        counts.set(rule.team_id, (counts.get(rule.team_id) ?? 0) + 1);
      } else {
        uncategorized++;
      }
    }
    const tabs: { id: string; label: string; count: number }[] = [];
    for (const [teamId, count] of counts) {
      tabs.push({ id: teamId, label: teamNamesById.get(teamId) ?? teamId, count });
    }
    tabs.sort((a, b) => a.label.localeCompare(b.label));
    if (uncategorized > 0) {
      tabs.push({ id: '__uncategorized__', label: 'Uncategorized', count: uncategorized });
    }
    return tabs;
  }, [highlightedFlows, teamNamesById]);
  const filteredFlows = useMemo(() => {
    if (activeTab === 'all') return highlightedFlows;
    if (activeTab === '__uncategorized__') return highlightedFlows.filter((r) => !r.team_id || !teamNamesById.has(r.team_id));
    return highlightedFlows.filter((r) => r.team_id === activeTab);
  }, [activeTab, highlightedFlows, teamNamesById]);
  const activeFlowFilterLabel = useMemo(() => {
    if (search.show_rule) {
      return search.show_rule_title
        || authoredFlows.find((rule) => rule.id === search.show_rule)?.name
        || search.show_rule;
    }
    if (search.show_trigger) {
      return search.show_trigger_title || triggerLabel(search.show_trigger);
    }
    return '';
  }, [authoredFlows, search.show_rule, search.show_rule_title, search.show_trigger, search.show_trigger_title]);
  const hasFlowFilter = Boolean(search.show_rule || search.show_trigger);

  const loading = settingsQuery.isLoading || inventoryQuery.isLoading || rulesQuery.isLoading || tasksQuery.isLoading || epicsQuery.isLoading || repositoriesQuery.isLoading;

  const refreshAll = useCallback(async () => {
    await Promise.all([
      rulesQuery.refetch(),
      inventoryQuery.refetch(),
    ]);
  }, [inventoryQuery, rulesQuery]);

  const resetComposerSearch = useCallback(() => {
    if (search.template || search.trigger_type || search.create_event_rule || search.workflow || search.show_trigger || search.show_trigger_title || search.show_rule || search.show_rule_title || search.template_title || search.template_description || search.agent_id || search.repo_full_name || search.branch || search.base_branch || search.tag_name || search.conclusion || search.target_mode || search.target_id) {
      onSearchChange({
        workflow: undefined,
        template: undefined,
        template_title: undefined,
        template_description: undefined,
        show_trigger: search.show_trigger,
        show_trigger_title: search.show_trigger_title,
        show_rule: search.show_rule,
        show_rule_title: search.show_rule_title,
        create_event_rule: undefined,
        trigger_type: undefined,
        agent_id: undefined,
        repo_full_name: undefined,
        branch: undefined,
        base_branch: undefined,
        tag_name: undefined,
        conclusion: undefined,
        target_mode: undefined,
        target_id: undefined,
      });
    }
  }, [onSearchChange, search]);

  const clearFlowFilter = useCallback(() => {
    onSearchChange({
      show_rule: undefined,
      show_rule_title: undefined,
      show_trigger: undefined,
      show_trigger_title: undefined,
    });
  }, [onSearchChange]);

  useEffect(() => {
    if (loading || appliedSearchSignature === searchSignature) return;
    const prefilledDraft = draftFromSearch(search, workflows);
    if (!prefilledDraft) return;
    setDraft(prefilledDraft);
    setEditingRuleId(null);
    setComposerOpen(true);
    setAppliedSearchSignature(searchSignature);
  }, [appliedSearchSignature, loading, search, searchSignature, workflows]);

  const openCreateComposer = () => {
    setEditingRuleId(null);
    setDraft(defaultDraft());
    setGalleryOpen(true);
  };

  const handleTemplatePick = (template: FlowTemplate | null) => {
    const base = defaultDraft();
    const next = template ? template.apply(base) : base;
    applyTriggerDefaults(next, workflows);
    setDraft(next);
    setEditingRuleId(null);
    setGalleryOpen(false);
    setComposerOpen(true);
  };

  const backToGallery = () => {
    setComposerOpen(false);
    setGalleryOpen(true);
  };

  const openEditComposer = (rule: AutomationRule) => {
    setEditingRuleId(rule.id);
    setDraft(draftFromRule(rule, workflows));
    setComposerOpen(true);
  };

  const handleToggle = async (rule: AutomationRule) => {
    const res = await automationService.updateFlow(workspaceId, rule.id, { enabled: !rule.enabled });
    if (res.error) {
      toast.error(res.error);
      return;
    }
    await refreshAll();
  };

  const handleDelete = async (rule: AutomationRule) => {
    const res = await automationService.deleteFlow(workspaceId, rule.id);
    if (res.error) {
      toast.error(res.error);
      return;
    }
    toast.success('Flow removed');
    await refreshAll();
  };

  const handleSave = async () => {
    const validationError = validateDraft(draft);
    if (validationError) {
      toast.error(validationError);
      return;
    }
    setSaving(true);
    const payload = serializeDraft(draft, workspaceId);
    const res = editingRuleId
      ? await automationService.updateFlow(workspaceId, editingRuleId, payload)
      : await automationService.createFlow(workspaceId, { workspace_id: workspaceId, ...payload, position: authoredFlows.length });
    setSaving(false);
    if (res.error) {
      toast.error(res.error);
      return;
    }
    toast.success(editingRuleId ? 'Flow updated' : 'Flow created');
    setComposerOpen(false);
    setEditingRuleId(null);
    await refreshAll();
    resetComposerSearch();
  };

  if (!workspaceId) {
    return <p className="text-sm text-muted-foreground">Workspace not found.</p>;
  }

  return (
    <div className="mx-auto max-w-6xl space-y-6">
      <FlowTemplateGallery
        open={galleryOpen}
        onOpenChange={(open) => {
          setGalleryOpen(open);
          if (!open) resetComposerSearch();
        }}
        onPick={handleTemplatePick}
      />
      <FlowComposer
        workspaceId={workspaceId}
        open={composerOpen}
        mode={editingRuleId ? 'edit' : 'create'}
        draft={draft}
        workflows={workflows}
        statesById={statesById}
        agents={agentNames}
        tasks={tasks}
        epics={epics}
        repositories={repositories}
        saving={saving}
        canEdit={permissions.canManageSettings}
        onOpenChange={(open) => {
          setComposerOpen(open);
          if (!open) {
            resetComposerSearch();
          }
        }}
        onBack={editingRuleId ? undefined : backToGallery}
        onDraftChange={setDraft}
        onSave={handleSave}
      />

      {/* Page header */}
      <div className="flex items-start justify-between gap-4">
        <div className="space-y-1">
          <h1 className="text-2xl font-semibold tracking-tight">Automation flows</h1>
          <p className="text-sm text-muted-foreground">Event-driven automations that trigger agents and workflow actions.</p>
        </div>
        {permissions.canManageSettings && (
          <Button size="sm" variant="outline" onClick={openCreateComposer}>
            + New flow
          </Button>
        )}
      </div>

      {loading ? (
        <div className="space-y-3">
          <Skeleton className="h-10 w-80 rounded-lg" />
          <Skeleton className="h-20 w-full rounded-lg" />
          <Skeleton className="h-20 w-full rounded-lg" />
          <Skeleton className="h-20 w-full rounded-lg" />
        </div>
      ) : authoredFlows.length === 0 ? (
        <div className="flex flex-col items-center justify-center px-4 py-16">
          <div className="mb-5 flex h-14 w-14 items-center justify-center rounded-full bg-violet-500/10">
            <ZapIcon className="h-7 w-7 text-violet-500" />
          </div>
          <h3 className="mb-1.5 text-lg font-semibold">Put an agent on autopilot</h3>
          <p className="mb-6 max-w-md text-center text-sm text-muted-foreground">
            Flows are event-driven automations — they watch for a trigger (a task changing state, a PR
            merging, a tag shipping, a schedule) and run an agent to act on it.
          </p>
          {permissions.canManageSettings && (
            <Button className="mb-8 gap-2" onClick={openCreateComposer}>
              <PlusSignIcon className="h-4 w-4" />
              New flow
            </Button>
          )}
          <div className="grid w-full max-w-4xl grid-cols-1 gap-4 sm:grid-cols-3">
            {FLOW_EMPTY_STATE_CARDS.map((card) => (
              <div
                key={card.title}
                className="flex flex-col items-center rounded-lg border border-border/50 bg-muted/30 p-6 text-center"
              >
                <card.icon className="mb-3 h-5 w-5 text-muted-foreground" />
                <p className="mb-1 text-sm font-medium">{card.title}</p>
                <p className="text-sm leading-relaxed text-muted-foreground">{card.desc}</p>
              </div>
            ))}
          </div>
        </div>
      ) : (
        <>
          {hasFlowFilter && (
            <FlowFilterBar
              value={activeFlowFilterLabel}
              count={highlightedFlows.length}
              onClear={clearFlowFilter}
            />
          )}

          {/* Category tabs */}
          <div className="flex items-center gap-1">
            <button
              type="button"
              onClick={() => setActiveTab('all')}
              className={cn(
                'rounded-full px-3 py-1 text-sm font-medium transition-colors',
                activeTab === 'all'
                  ? 'bg-foreground text-background'
                  : 'text-muted-foreground hover:bg-muted hover:text-foreground',
              )}
            >
              All ({highlightedFlows.length})
            </button>
            {teamTabs.map((tab) => (
              <button
                key={tab.id}
                type="button"
                onClick={() => setActiveTab(tab.id)}
                className={cn(
                  'rounded-full px-3 py-1 text-sm font-medium transition-colors',
                  activeTab === tab.id
                    ? 'bg-foreground text-background'
                    : 'text-muted-foreground hover:bg-muted hover:text-foreground',
                )}
              >
                {tab.label} ({tab.count})
              </button>
            ))}
          </div>

          {/* Flow list */}
          <div className="space-y-2">
            {filteredFlows.length > 0 ? filteredFlows.map((rule) => (
              <FlowRow
                key={rule.id}
                rule={rule}
                statesById={statesById}
                agentNames={agentNames}
                teamName={rule.team_id ? teamNamesById.get(rule.team_id) : undefined}
                healthItem={flowHealth.get(rule.id)}
                workspaceSlug={workspaceSlug}
                canEdit={permissions.canManageSettings}
                onEdit={openEditComposer}
                onToggle={handleToggle}
                onDelete={handleDelete}
              />
            )) : (
              <div className="rounded-lg border border-dashed border-border/70 px-6 py-12 text-center">
                <PlayIcon className="mx-auto mb-3 h-6 w-6 text-muted-foreground" />
                <p className="text-sm font-medium">{hasFlowFilter ? 'No flows match this filter' : 'No automation flows yet'}</p>
                <p className="mt-1 text-sm text-muted-foreground">
                  {hasFlowFilter
                    ? 'Clear the filter to return to all automation flows.'
                    : 'Create a flow to connect events to agents and workflow actions.'}
                </p>
              </div>
            )}
          </div>
        </>
      )}
    </div>
  );
}
