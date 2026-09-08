import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { useQuery } from '@tanstack/react-query';
import {
  ArrowRight01Icon,
  FilterIcon,
  HelpCircleIcon,
  BookOpen01Icon,
  Cancel01Icon,
  MoreHorizontalIcon,
  PlusSignIcon,
  SourceCodeIcon,
} from '@/lib/icons';
import { toast } from 'sonner';
import { UpgradeRequiredDialog } from '@/components/billing/UpgradeRequiredDialog';
import { AutomationShell } from '@/components/automation/AutomationShell';
import {
  QuietEmptyState,
  QuietPrimaryAction,
  QuietSearchInput,
  QuietTextAction,
  quietUnderlineControlClassName,
} from '@/components/design-system/quiet';
import { RepositoryBranchPicker } from '@/components/git/RepositoryBranchPicker';
import { ToolMultiSelectPopover } from '@/components/automation/ToolMultiSelectPopover';
import { CRMRecordPicker } from '@/components/automation/CRMRecordPicker';
import { CRM_AGENT_TARGET_OPTIONS, CRM_RECORD_TARGETS, isCRMRecordTarget } from '@/lib/agentCRMTargets';
import { BASE_BRANCH_TOKEN, TASK_BRANCH_TOKEN, describeMergeInto, describeRunBranchOverrides } from '@/lib/branchLabels';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Select, SelectContent, SelectGroup, SelectItem, SelectLabel, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Skeleton } from '@/components/ui/skeleton';
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Textarea } from '@/components/ui/textarea';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from '@/components/ui/command';
import { useWorkspaceAccess, usePermissions } from '@/hooks/queries';
import { useAllDocsCollections, useAutomationActivity, useAutomationFlowTemplates, useAutomationFlows, useAutomationOverview, useAutomationSkillCatalog, useAutomationToolCatalog, useAgents, useDocsSpaces, useInstallAutomationFlowTemplate, useUninstallAutomationFlowTemplate, useWorkflows } from '@/hooks/queries';
import { useWorkspaceSettings } from '@/hooks/queries/useSettings';
import { useWorkspaceTeams } from '@/hooks/useWorkspaceTeams';
import { useTitle } from '@/hooks/useTitle';
import { automationService } from '@/lib/services/automationService';
import { gitService } from '@/lib/services/gitService';
import { pmEpicService } from '@/lib/services/pmEpicService';
import { pmTaskService } from '@/lib/services/pmTaskService';
import { queryKeys } from '@/lib/queryKeys';
import { unwrap } from '@/lib/queryUtils';
import type { AutomationInventoryItem, Workspace, WorkspaceTeam } from '@/lib/types';
import type { Agent, AgentApprovalMode, AgentRuntimeKind, AgentSkillRef, AgentTargetType, AutomationRule, EpicWithStats, FlowTemplateInput, FlowTemplateManifest, GitRepository, SkillCatalogEntry, Task, ToolCatalogEntry, WorkflowState, WorkflowWithStates } from '@/lib/pmTypes';
import type { DocsCollection, DocsSpace } from '@/lib/docsTypes';
import { buildAutomationActivityPath, type FlowTargetMode } from '@/lib/automationUi';
import { getAgentTeamIds, isAgentVisibleToActor } from '@/lib/agentAccess';
import { getUpgradeRequiredReason, type UpgradeRequiredReason } from '@/lib/upgradeRequired';
import { cn } from '@/lib/utils';
import { AGENT_APPROVAL_OPTIONS, agentApprovalDescription } from '@/lib/agentApproval';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useAuthStore } from '@/stores/authStore';

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
  trigger_type?: string;
  agent_id?: string;
  repo_full_name?: string;
  branch?: string;
  base_branch?: string;
  tag_name?: string;
  conclusion?: string;
  target_mode?: FlowTargetMode;
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
  targetMode: FlowTargetMode;
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
  { value: 'github.pull_request_closed', label: 'GitHub pull request closes without merging', group: 'GitHub events' },
  { value: 'github.pull_request_review_requested', label: 'GitHub review is requested', group: 'GitHub events' },
  { value: 'github.release_published', label: 'GitHub release publishes', group: 'GitHub events' },
  { value: 'github.check_suite_completed', label: 'GitHub check suite completes', group: 'GitHub events' },
  { value: 'gitlab.push', label: 'GitLab push arrives', group: 'GitLab events' },
  { value: 'gitlab.merge_request_opened', label: 'GitLab merge request opens', group: 'GitLab events' },
  { value: 'gitlab.merge_request_merged', label: 'GitLab merge request merges', group: 'GitLab events' },
  { value: 'gitlab.merge_request_closed', label: 'GitLab merge request closes without merging', group: 'GitLab events' },
  { value: 'gitlab.release_published', label: 'GitLab release publishes', group: 'GitLab events' },
  { value: 'gitlab.pipeline_completed', label: 'GitLab pipeline completes', group: 'GitLab events' },
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
  crm_deal: 'a specific deal',
  crm_contact: 'a specific contact',
  crm_company: 'a specific company',
};

const TARGET_SHORT_LABELS: Record<FlowDraft['targetMode'], string> = {
  event: 'whatever triggered it',
  task: 'a specific task',
  epic: 'a specific epic',
  repository: 'a specific repository',
  workspace: 'this workspace',
  crm_deal: 'a specific deal',
  crm_contact: 'a specific contact',
  crm_company: 'a specific company',
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

const SCHEDULE_HOUR_OPTIONS = Array.from({ length: 24 }, (_, hour) => ({
  value: String(hour).padStart(2, '0'),
  label: String(hour).padStart(2, '0'),
}));

const SCHEDULE_MINUTE_OPTIONS = ['00', '15', '30', '45'].map((minute) => ({
  value: minute,
  label: minute,
}));

const SCHEDULE_MONTH_DAY_OPTIONS = Array.from({ length: 31 }, (_, idx) => {
  const day = String(idx + 1);
  return { value: day, label: day };
});

type ParsedSimpleSchedule = {
  frequency: FlowDraft['scheduleFrequency'];
  minute: string;
  time: string;
  weekdays: number[];
  dayOfMonth: string;
};

type TemplateAgentSetup = {
  name: string;
  system_prompt: string;
  approval_mode: AgentApprovalMode;
  allowed_targets: AgentTargetType[];
  allowed_tools: string[];
  skills: AgentSkillRef[];
  max_concurrent_runs: string;
};

type TemplateFlowSetup = {
  name: string;
  description: string;
};

type FlowTemplateInstallStep = 'inputs' | 'review';

type TemplateSelectOption = {
  value: string;
  label: string;
  description?: string;
  indent?: number;
};

export const NO_REPOSITORY_VALUE = '__no_repository__';

function isOptionalRepositoryInput(input: FlowTemplateInput) {
  return input.type === 'repository' && !input.required;
}

export function templateSelectOptions(
  input: FlowTemplateInput,
  repositories: Pick<GitRepository, 'id' | 'full_name'>[],
): TemplateSelectOption[] {
  const options = repositories.map((repo) => ({ value: repo.id, label: repo.full_name }));
  if (!isOptionalRepositoryInput(input)) return options;
  return [
    { value: NO_REPOSITORY_VALUE, label: 'No repository', description: 'Skip repository context' },
    ...options,
  ];
}

export function templateSelectValue(input: FlowTemplateInput, value: unknown) {
  if (isOptionalRepositoryInput(input) && !stringValue(value)) return NO_REPOSITORY_VALUE;
  return stringValue(value);
}

export function templateSelectChangeValue(input: FlowTemplateInput, value: string) {
  if (isOptionalRepositoryInput(input) && value === NO_REPOSITORY_VALUE) return '';
  return value;
}

type FlowLogicRow = {
  connector: string;
  text: string;
  tone?: 'muted' | 'strong' | 'warning';
};

type FlowComposerMode = 'create' | 'edit';

const LEGACY_CRON_CATEGORY_TO_PRESET: Record<string, (typeof SCHEDULE_PRESET_OPTIONS)[number]['value']> = {
  workspace_hourly: 'hourly',
  workspace_daily: 'daily',
  workspace_weekly: 'weekly',
};

const TEMPLATE_TARGET_OPTIONS: Array<{ value: AgentTargetType; label: string }> = [
  { value: 'task', label: 'Task' },
  { value: 'epic', label: 'Epic' },
  { value: 'repository', label: 'Repository' },
  { value: 'workspace', label: 'Workspace' },
  ...CRM_AGENT_TARGET_OPTIONS.map((target) => ({ value: target.value, label: `CRM ${CRM_RECORD_TARGETS[target.value].singular.toLowerCase()}` })),
  { value: 'document', label: 'Document' },
  { value: 'support_conversation', label: 'Support conversation' },
  { value: 'support_coverage_gap', label: 'Support coverage gap' },
];

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

function applyScheduleExpressionToDraft(draft: FlowDraft, expression: string, timezone?: string) {
  const parsedUTC = parseSimpleScheduleExpression(expression);
  const parsed = parsedUTC && timezone ? utcScheduleToLocal(parsedUTC, timezone) : parsedUTC;
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

function scheduleExpressionForDraftUTC(draft: FlowDraft, timezone: string) {
  if (draft.triggerType !== 'cron') return draft.cronCategory.trim();
  if (draft.cronMode === 'advanced') return draft.cronCategory.trim();
  const localSchedule = parseSimpleScheduleExpression(buildSimpleScheduleExpression(draft));
  if (!localSchedule) return '';
  return scheduleExpressionFromParsed(localScheduleToUTC(localSchedule, timezone));
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

function describeScheduleExpressionInTimeZone(expression: string, timezone: string) {
  const resolvedTimeZone = normalizeTimeZone(timezone);
  const parsedUTC = parseSimpleScheduleExpression(expression);
  if (!parsedUTC) return describeScheduleExpression(expression);
  return describeSimpleSchedule(utcScheduleToLocal(parsedUTC, resolvedTimeZone)).replace('UTC', resolvedTimeZone);
}

function daysInMonth(year: number, monthIndex: number) {
  return new Date(Date.UTC(year, monthIndex + 1, 0)).getUTCDate();
}

function nextCronRunDate(schedule: ParsedSimpleSchedule, now = new Date()) {
  const after = new Date(now.getTime() + 1000);
  const minute = Number.parseInt(schedule.minute, 10);
  const [hourText, minuteText] = normalizeScheduleTime(schedule.time).split(':');
  const hour = Number.parseInt(hourText, 10);
  const timeMinute = Number.parseInt(minuteText, 10);

  if (schedule.frequency === 'hourly') {
    const candidate = new Date(after);
    candidate.setUTCSeconds(0, 0);
    candidate.setUTCMinutes(minute);
    if (candidate <= after) candidate.setUTCHours(candidate.getUTCHours() + 1);
    return candidate;
  }

  if (schedule.frequency === 'daily') {
    const candidate = new Date(Date.UTC(after.getUTCFullYear(), after.getUTCMonth(), after.getUTCDate(), hour, timeMinute));
    if (candidate <= after) candidate.setUTCDate(candidate.getUTCDate() + 1);
    return candidate;
  }

  if (schedule.frequency === 'weekly') {
    const weekdays = normalizeScheduleWeekdays(schedule.weekdays);
    let best: Date | null = null;
    for (const weekday of weekdays) {
      const daysUntil = (weekday - after.getUTCDay() + 7) % 7;
      const candidate = new Date(Date.UTC(after.getUTCFullYear(), after.getUTCMonth(), after.getUTCDate() + daysUntil, hour, timeMinute));
      if (candidate <= after) candidate.setUTCDate(candidate.getUTCDate() + 7);
      if (!best || candidate < best) best = candidate;
    }
    return best;
  }

  const dayOfMonth = Number.parseInt(schedule.dayOfMonth, 10);
  for (let monthOffset = 0; monthOffset <= 13; monthOffset += 1) {
    const year = after.getUTCFullYear();
    const month = after.getUTCMonth() + monthOffset;
    const candidateYear = year + Math.floor(month / 12);
    const candidateMonth = ((month % 12) + 12) % 12;
    const day = Math.min(dayOfMonth, daysInMonth(candidateYear, candidateMonth));
    const candidate = new Date(Date.UTC(candidateYear, candidateMonth, day, hour, timeMinute));
    if (candidate > after) return candidate;
  }

  return null;
}

function normalizeTimeZone(timezone?: string) {
  const fallback = Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC';
  const candidate = timezone?.trim() || fallback;
  try {
    new Intl.DateTimeFormat('en-US', { timeZone: candidate }).format(new Date());
    return candidate;
  } catch {
    return 'UTC';
  }
}

function timeZoneOffsetMinutes(timezone: string, date = new Date()) {
  try {
    const parts = new Intl.DateTimeFormat('en-US', {
      timeZone: timezone,
      timeZoneName: 'shortOffset',
      hour: '2-digit',
    }).formatToParts(date);
    const offset = parts.find((part) => part.type === 'timeZoneName')?.value ?? 'GMT';
    const match = offset.match(/^GMT(?:(?<sign>[+-])(?<hour>\d{1,2})(?::(?<minute>\d{2}))?)?$/);
    if (!match?.groups?.sign) return 0;
    const sign = match.groups.sign === '+' ? 1 : -1;
    const hour = Number.parseInt(match.groups.hour ?? '0', 10);
    const minute = Number.parseInt(match.groups.minute ?? '0', 10);
    return sign * (hour * 60 + minute);
  } catch {
    return 0;
  }
}

function shiftWeekday(weekday: number, dayShift: number) {
  return ((weekday + dayShift) % 7 + 7) % 7;
}

function localScheduleToUTC(schedule: ParsedSimpleSchedule, timezone: string): ParsedSimpleSchedule {
  const offset = timeZoneOffsetMinutes(timezone);
  if (schedule.frequency === 'hourly') return schedule;
  const [localHour, localMinute] = normalizeScheduleTime(schedule.time).split(':').map((part) => Number.parseInt(part, 10));
  const utcTotal = localHour * 60 + localMinute - offset;
  const dayShift = Math.floor(utcTotal / 1440);
  const normalizedTotal = ((utcTotal % 1440) + 1440) % 1440;
  const utcHour = Math.floor(normalizedTotal / 60);
  const utcMinute = normalizedTotal % 60;
  return {
    ...schedule,
    time: `${String(utcHour).padStart(2, '0')}:${String(utcMinute).padStart(2, '0')}`,
    minute: String(utcMinute),
    weekdays: normalizeScheduleWeekdays(schedule.weekdays.map((weekday) => shiftWeekday(weekday, dayShift))),
    dayOfMonth: clampScheduleNumber(String(Number.parseInt(schedule.dayOfMonth, 10) + dayShift), 1, 31, 1),
  };
}

function utcScheduleToLocal(schedule: ParsedSimpleSchedule, timezone: string): ParsedSimpleSchedule {
  const offset = timeZoneOffsetMinutes(timezone);
  if (schedule.frequency === 'hourly') return schedule;
  const [utcHour, utcMinute] = normalizeScheduleTime(schedule.time).split(':').map((part) => Number.parseInt(part, 10));
  const localTotal = utcHour * 60 + utcMinute + offset;
  const dayShift = Math.floor(localTotal / 1440);
  const normalizedTotal = ((localTotal % 1440) + 1440) % 1440;
  const localHour = Math.floor(normalizedTotal / 60);
  const localMinute = normalizedTotal % 60;
  return {
    ...schedule,
    time: `${String(localHour).padStart(2, '0')}:${String(localMinute).padStart(2, '0')}`,
    minute: String(localMinute),
    weekdays: normalizeScheduleWeekdays(schedule.weekdays.map((weekday) => shiftWeekday(weekday, dayShift))),
    dayOfMonth: clampScheduleNumber(String(Number.parseInt(schedule.dayOfMonth, 10) + dayShift), 1, 31, 1),
  };
}

function scheduleExpressionFromParsed(schedule: ParsedSimpleSchedule) {
  return buildSimpleScheduleExpression({
    scheduleFrequency: schedule.frequency,
    scheduleMinute: schedule.minute,
    scheduleTime: schedule.time,
    scheduleWeekdays: schedule.weekdays,
    scheduleDayOfMonth: schedule.dayOfMonth,
  });
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

const TEMPLATE_CATEGORY_LABELS: Record<string, string> = {
  engineering: 'Engineering',
  sales: 'Sales',
  support: 'Support',
  marketing: 'Marketing',
  docs: 'Docs',
  workflow: 'Workflow',
};

const TEMPLATE_TONE_CATEGORY_BY_KEY: Record<string, string> = {
  merge_when_done: 'workflow',
  release_notes_writer: 'docs',
};

const TEMPLATE_DISPLAY_ORDER: Record<string, number> = {
  review_merged_prs: 10,
  triage_failing_checks: 20,
  release_notes_writer: 30,
  engineering_security_triage: 40,
  engineering_dependency_auditor: 50,
  docs_freshness_sweep: 60,
  public_help_freshness_sweep: 70,
  api_docs_freshness_sweep: 80,
  competitors_changelog_tracking_report: 90,
  buying_signal_to_task: 100,
  stale_task_escalation: 110,
  merge_when_done: 120,
  advance_on_approval: 130,
  run_on_release: 140,
  run_on_a_schedule: 150,
};

function templatePrimaryCategory(template: FlowTemplateManifest) {
  return TEMPLATE_TONE_CATEGORY_BY_KEY[template.key] ?? template.categories?.[0] ?? 'workflow';
}

function compareTemplatesForDisplay(a: FlowTemplateManifest, b: FlowTemplateManifest) {
  const orderA = TEMPLATE_DISPLAY_ORDER[a.key] ?? 10_000;
  const orderB = TEMPLATE_DISPLAY_ORDER[b.key] ?? 10_000;
  if (orderA !== orderB) return orderA - orderB;
  return a.name.localeCompare(b.name);
}

export function templateMatchesSearch(template: FlowTemplateManifest, search: string) {
  const query = search.trim().toLowerCase();
  if (!query) return true;
  const categories = template.categories ?? [];

  const searchableParts = [
    template.key,
    template.name,
    template.short_description,
    template.description_ref,
    template.trigger.type,
    template.trigger.event,
    ...categories,
    ...categories.map((category) => TEMPLATE_CATEGORY_LABELS[category]),
  ];

  return searchableParts.some((part) => String(part ?? '').toLowerCase().includes(query));
}

function normalizeToolList(tools: string[]) {
  const seen = new Set<string>();
  return tools.reduce<string[]>((result, tool) => {
    const normalized = tool.trim();
    if (!normalized || seen.has(normalized)) return result;
    seen.add(normalized);
    result.push(normalized);
    return result;
  }, []);
}

function normalizeTargetList(targets: AgentTargetType[]) {
  const seen = new Set<AgentTargetType>();
  return targets.reduce<AgentTargetType[]>((result, target) => {
    if (seen.has(target)) return result;
    seen.add(target);
    result.push(target);
    return result;
  }, []);
}

export function defaultTemplateInputs(
  template: FlowTemplateManifest,
  workspace?: Pick<Workspace, 'name' | 'website_url'> | null,
  repositories: Pick<GitRepository, 'id'>[] = [],
) {
  const inputs = Object.fromEntries(template.inputs.map((input) => [
    input.key,
    input.default ?? (
      input.type === 'bool'
        ? false
        : input.type === 'cron'
          ? '0 9 * * 1'
          : input.type === 'multi_select' || input.type === 'string_list'
            ? []
            : ''
    ),
  ]));
  if (repositories.length === 1) {
    for (const input of template.inputs) {
      if (input.type === 'repository' && !String(inputs[input.key] ?? '').trim()) {
        inputs[input.key] = repositories[0].id;
      }
    }
  }
  if (template.key === 'competitors_changelog_tracking_report') {
    if (!String(inputs.target_company ?? '').trim() && workspace?.name?.trim()) {
      inputs.target_company = workspace.name.trim();
    }
    if (!String(inputs.target_domain ?? '').trim() && workspace?.website_url?.trim()) {
      inputs.target_domain = workspace.website_url.trim();
    }
  }
  return inputs;
}

function templateAgentName(template: FlowTemplateManifest) {
  return (template.agent.create?.name_template || '{{template_name}} agent').replaceAll('{{template_name}}', template.name);
}

function defaultTemplateAgentInstructions(template: FlowTemplateManifest, inputs: Record<string, unknown>) {
  if (template.agent.create?.system_prompt?.trim()) {
    return renderTemplateText(template.agent.create.system_prompt, inputs);
  }
  const configuredInputs = JSON.stringify(inputs, null, 2);
  return [
    `You are running the ${template.name} flow template.`,
    'Use the configured flow inputs as resolved product context. Do not ask the user to provide these values again.',
    '',
    'Configured inputs:',
    '```json',
    configuredInputs,
    '```',
  ].join('\n');
}

function defaultTemplateAgentSetup(template: FlowTemplateManifest, inputs: Record<string, unknown>): TemplateAgentSetup | null {
  if (!template.agent.create) return null;
  return {
    name: templateAgentName(template),
    system_prompt: defaultTemplateAgentInstructions(template, inputs),
    approval_mode: (template.agent.create.approval_mode as AgentApprovalMode | undefined) ?? 'mutating_tools',
    allowed_targets: (template.agent.create.allowed_targets ?? ['task']) as AgentTargetType[],
    allowed_tools: normalizeToolList(template.agent.create.allowed_tools ?? []),
    skills: (template.agent.create.skills ?? []).map((key) => ({ key })),
    max_concurrent_runs: '1',
  };
}

function buildTemplateRenderInputs(
  template: FlowTemplateManifest,
  inputs: Record<string, unknown>,
  context: {
    agents: Agent[];
    workflows: WorkflowWithStates[];
    repositories: GitRepository[];
    spaces: DocsSpace[];
    collections: DocsCollection[];
    teams: WorkspaceTeam[];
    timezone: string;
  },
  agentSetup?: TemplateAgentSetup | null,
) {
  const renderInputs: Record<string, unknown> = { ...inputs, template_name: template.name };
  for (const input of template.inputs) {
    renderInputs[`${input.key}_label`] = formatTemplateReviewValue(input, inputs[input.key], context);
  }
  const repositoryID = String(inputs.repository_id ?? inputs.source_repository_id ?? '').trim();
  const repository = context.repositories.find((repo) => repo.id === repositoryID);
  if (repository?.full_name) {
    renderInputs.repo_full_name = repository.full_name;
    renderInputs.repository_name = repository.full_name;
  }
  const pickedAgentID = String(inputs.agent_id ?? '').trim();
  const pickedAgentName = context.agents.find((agent) => agent.id === pickedAgentID)?.name;
  renderInputs.agent_name = agentSetup?.name?.trim() || pickedAgentName || 'the selected agent';
  return renderInputs;
}

function defaultTemplateFlowSetup(
  template: FlowTemplateManifest,
  inputs: Record<string, unknown>,
  context: {
    agents: Agent[];
    workflows: WorkflowWithStates[];
    repositories: GitRepository[];
    spaces: DocsSpace[];
    collections: DocsCollection[];
    teams: WorkspaceTeam[];
    timezone: string;
  },
  agentSetup?: TemplateAgentSetup | null,
): TemplateFlowSetup {
  const renderInputs = buildTemplateRenderInputs(template, inputs, context, agentSetup);
  const nameFromTemplate = template.flow.name_template?.trim()
    ? renderTemplateText(template.flow.name_template, renderInputs)
    : '';
  const targetLabel = String(renderInputs.repo_full_name ?? '').trim();
  const name = nameFromTemplate || (targetLabel ? `${template.name} - ${targetLabel}` : template.name);
  const descriptionFromTemplate = template.flow.description_template?.trim()
    ? renderTemplateText(template.flow.description_template, renderInputs)
    : '';
  const description = descriptionFromTemplate || template.short_description;
  return { name, description };
}

function defaultTemplateRunContext(template: FlowTemplateManifest, inputs: Record<string, unknown>) {
  const context = template.flow.additional_context?.trim();
  if (context) return renderTemplateText(context, inputs);
  return defaultTemplateAgentInstructions(template, inputs);
}

function templateNeedsReviewStep(template: FlowTemplateManifest) {
  return Boolean(template.agent.create || template.agent.pick_existing || template.agent.reuse_system || template.flow.name_template || template.flow.description_template);
}

function normalizeTemplateMaxRuns(value: string) {
  const parsed = Number.parseInt(value, 10);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : 1;
}

function formatTemplateInputValue(input: FlowTemplateInput, value: unknown) {
  if (input.type === 'bool') return value === true ? 'Yes' : 'No';
  if (input.type === 'cron') return describeScheduleExpression(String(value ?? ''));
  if (Array.isArray(value)) return value.length > 0 ? value.join(', ') : 'Not set';
  const text = String(value ?? '').trim();
  if (!text) return 'Not set';
  return text;
}

function formatTemplateReviewValue(
  input: FlowTemplateInput,
  value: unknown,
  context: {
    agents: Agent[];
    workflows: WorkflowWithStates[];
    repositories: GitRepository[];
    spaces: DocsSpace[];
    collections: DocsCollection[];
    teams: WorkspaceTeam[];
    timezone: string;
  },
) {
  const raw = String(value ?? '').trim();
  if (!raw) return formatTemplateInputValue(input, value);
  switch (input.type) {
    case 'agent':
      return context.agents.find((agent) => agent.id === raw)?.name ?? raw;
    case 'repository':
      return context.repositories.find((repo) => repo.id === raw)?.full_name ?? raw;
    case 'space':
      return context.spaces.find((space) => space.id === raw)?.name ?? raw;
    case 'collection': {
      const collection = context.collections.find((item) => item.id === raw);
      if (!collection) return raw;
      const spaceName = context.spaces.find((space) => space.id === collection.space_id)?.name;
      return spaceName ? `${collection.name} · ${spaceName}` : collection.name;
    }
    case 'team':
      return context.teams.find((team) => team.id === raw)?.name ?? raw;
    case 'workflow':
      return context.workflows.find((workflow) => workflow.workflow.id === raw)?.workflow.name ?? raw;
    case 'workflow_state':
      return context.workflows.flatMap((workflow) => workflow.states).find((state) => state.id === raw)?.name ?? raw;
    case 'cron': {
      const parsedUTC = parseSimpleScheduleExpression(raw);
      const local = parsedUTC ? utcScheduleToLocal(parsedUTC, normalizeTimeZone(context.timezone)) : null;
      return local ? describeSimpleSchedule(local).replace('UTC', normalizeTimeZone(context.timezone)) : describeScheduleExpression(raw);
    }
    default:
      return formatTemplateInputValue(input, value);
  }
}

function templateInputDisplayValue(
  template: FlowTemplateManifest,
  key: string,
  values: Record<string, unknown>,
  context: {
    agents: Agent[];
    workflows: WorkflowWithStates[];
    repositories: GitRepository[];
    spaces: DocsSpace[];
    collections: DocsCollection[];
    teams: WorkspaceTeam[];
    timezone: string;
  },
) {
  const input = template.inputs.find((item) => item.key === key);
  if (!input) return 'Not set';
  return formatTemplateReviewValue(input, values[key], context);
}

function templateAgentDisplayName(template: FlowTemplateManifest, values: Record<string, unknown>, agents: Agent[]) {
  const selectedAgentId = String(values.agent_id ?? '').trim();
  if (selectedAgentId) return agents.find((agent) => agent.id === selectedAgentId)?.name ?? 'selected agent';
  if (template.agent.create) return 'Custom agent';
  const preset = template.agent.reuse_system || '';
  if (preset === 'documentation_agent') return 'Quill';
  if (preset === 'crm_agent') return 'Beacon';
  return 'the agent';
}

function compactList(parts: Array<string | false | null | undefined>) {
  return parts.filter((part): part is string => Boolean(part && part.trim())).join(' · ');
}

function compactRows<T>(rows: Array<T | false | null | undefined>) {
  return rows.filter((row): row is T => Boolean(row));
}

function templateLogicRows(
  template: FlowTemplateManifest,
  values: Record<string, unknown>,
  context: {
    agents: Agent[];
    workflows: WorkflowWithStates[];
    repositories: GitRepository[];
    spaces: DocsSpace[];
    collections: DocsCollection[];
    teams: WorkspaceTeam[];
    timezone: string;
  },
) {
  const v = (key: string) => templateInputDisplayValue(template, key, values, context);
  const agentName = templateAgentDisplayName(template, values, context.agents);
  const rows: Array<{ connector: 'When' | 'If' | 'Then' | 'On'; text: string; tone?: 'strong' | 'muted' }> = [];
  const addFilterRow = (text: string) => {
    if (text) rows.push({ connector: 'If', text });
  };

  const schedule = v('schedule');
  if (template.trigger.type === 'cron' || schedule !== 'Not set') {
    rows.push({ connector: 'When', text: schedule !== 'Not set' ? schedule : 'the schedule ticks', tone: 'strong' });
  } else if (template.trigger.event === 'github.pull_request_merged') {
    rows.push({ connector: 'When', text: 'a pull request is merged', tone: 'strong' });
  } else if (template.trigger.event === 'github.release_published') {
    rows.push({ connector: 'When', text: 'a GitHub release is published', tone: 'strong' });
  } else if (template.trigger.event === 'github.check_suite_completed') {
    rows.push({ connector: 'When', text: 'a GitHub check suite completes', tone: 'strong' });
  } else if (template.trigger.event === 'task.state_entered') {
    rows.push({ connector: 'When', text: 'a task enters a workflow state', tone: 'strong' });
  } else if (template.trigger.event === 'agent_run.approved') {
    rows.push({ connector: 'When', text: 'an agent run is approved', tone: 'strong' });
  } else {
    rows.push({ connector: 'When', text: triggerLabel(template.trigger.event || template.trigger.type), tone: 'strong' });
  }

  const conditions = compactList([
    v('repository_id') !== 'Not set' && `repository is ${v('repository_id')}`,
    v('base_branch') !== 'Not set' && `branch is ${v('base_branch')}`,
    v('branch') !== 'Not set' && `branch is ${v('branch')}`,
    v('conclusion') !== 'Not set' && `result is ${v('conclusion')}`,
    v('workflow_id') !== 'Not set' && `workflow is ${v('workflow_id')}`,
    v('from_state_id') !== 'Not set' && `state is ${v('from_state_id')}`,
    'include_prerelease' in values && (values.include_prerelease === true ? 'prereleases are included' : 'normal releases only'),
  ]);
  addFilterRow(conditions);

  if (template.flow.action === 'start_agent_run') {
    rows.push({ connector: 'Then', text: `Run ${agentName}` });
    const targetInput = String(template.flow.target?.from_input ?? '').trim();
    if (targetInput) {
      const targetValue = v(targetInput);
      if (targetValue !== 'Not set') rows.push({ connector: 'On', text: targetValue });
    }
  } else if (template.flow.action === 'move_to_state') {
    rows.push({ connector: 'Then', text: `move the task to ${v('to_state_id')}` });
  } else if (template.flow.action === 'merge_branch') {
    rows.push({ connector: 'Then', text: describeMergeInto(String(values.target_branch ?? '').trim() || BASE_BRANCH_TOKEN) });
  } else {
    rows.push({ connector: 'Then', text: template.flow.action.replaceAll('_', ' ') });
  }

  return rows;
}

function TemplateLogicSummary({
  template,
  values,
  context,
}: {
  template: FlowTemplateManifest;
  values: Record<string, unknown>;
  context: {
    agents: Agent[];
    workflows: WorkflowWithStates[];
    repositories: GitRepository[];
    spaces: DocsSpace[];
    collections: DocsCollection[];
    teams: WorkspaceTeam[];
    timezone: string;
  };
}) {
  const rows = templateLogicRows(template, values, context);
  return (
    <FlowLogicSummary
      rows={rows}
      description="Review the automation that will be installed."
    />
  );
}

function FlowLogicSummary({
  rows,
  description,
}: {
  rows: FlowLogicRow[];
  description: string;
}) {
  return (
    <section className="space-y-3">
      <div>
        <h3 className="text-sm font-medium">Flow logic</h3>
        <p className="text-xs text-muted-foreground">{description}</p>
      </div>
      <div className="space-y-2 rounded-xl border border-border/60 bg-muted/15 p-3">
        {rows.map((row, index) => (
          <SentenceRow key={`${row.connector}-${index}`} connector={row.connector} tone={row.tone}>
            <span
              className={cn(
                'text-sm',
                row.tone === 'warning' ? 'text-destructive' : 'text-foreground',
              )}
            >
              {row.text}
            </span>
          </SentenceRow>
        ))}
      </div>
    </section>
  );
}

function FlowSummaryParagraph({ rows }: { rows: FlowLogicRow[] }) {
  return (
    <section className="space-y-2">
      <h3 className="text-sm font-medium">Flow summary</h3>
      <div className="rounded-xl border border-border/60 bg-muted/15 px-3 py-2 text-sm leading-relaxed">
        {rows.map((row, index) => (
          <span key={`${row.connector}-${index}`} className={cn(index > 0 && 'ml-1.5')}>
            <span className="font-medium text-muted-foreground">{row.connector}</span>{' '}
            <span className={cn(row.tone === 'warning' ? 'text-destructive' : 'text-foreground')}>
              {row.text}
            </span>
            {index < rows.length - 1 && <span className="text-muted-foreground">,</span>}
          </span>
        ))}
        <span className="text-muted-foreground">.</span>
      </div>
    </section>
  );
}

function skillRefIdentity(skill: Pick<SkillCatalogEntry, 'id' | 'key'> | AgentSkillRef) {
  if ('id' in skill && skill.id) return skill.id;
  if ('skill_id' in skill && skill.skill_id) return skill.skill_id;
  return skill.key;
}

function skillRefDisplayName(skill: Pick<SkillCatalogEntry, 'title' | 'key'> | undefined, fallbackKey: string) {
  return skill?.title?.trim() || fallbackKey;
}

function TemplateSkillPicker({
  open,
  onOpenChange,
  skills,
  selectedSkills,
  onAddSkill,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  skills: SkillCatalogEntry[];
  selectedSkills: AgentSkillRef[];
  onAddSkill: (skill: SkillCatalogEntry) => void;
}) {
  const selected = new Set(selectedSkills.map(skillRefIdentity));
  const available = skills.filter((skill) => !selected.has(skillRefIdentity(skill)));
  return (
    <Popover open={open} onOpenChange={onOpenChange}>
      <PopoverTrigger asChild>
        <Button type="button" variant="outline" size="sm" className="h-8 gap-1.5 px-2 text-[11px]" disabled={available.length === 0}>
          <PlusSignIcon className="h-3.5 w-3.5" />
          Add skill
        </Button>
      </PopoverTrigger>
      <PopoverContent align="end" className="w-[28rem] overflow-hidden p-0" onWheelCapture={(event) => event.stopPropagation()}>
        <Command>
          <CommandInput placeholder="Search skills..." />
          <CommandList className="max-h-72 overscroll-contain">
            <CommandEmpty>No more skills available.</CommandEmpty>
            <CommandGroup heading={`${available.length} available`}>
              {available.map((skill) => (
                <CommandItem
                  key={skillRefIdentity(skill)}
                  value={`${skill.key} ${skill.title} ${skill.description}`}
                  onSelect={() => onAddSkill(skill)}
                  className="cursor-pointer items-start py-2"
                >
                  <div className="min-w-0 flex-1 space-y-0.5">
                    <div className="flex items-center gap-2">
                      <span className="text-xs font-medium text-foreground">{skillRefDisplayName(skill, skill.key)}</span>
                      <span className="font-mono text-[10px] text-muted-foreground">{skill.key}</span>
                      <Badge variant="outline" className="text-[10px]">{skill.source_kind === 'built_in' ? 'built-in' : skill.source_kind}</Badge>
                    </div>
                    <p className="text-xs leading-relaxed text-muted-foreground">{skill.description}</p>
                  </div>
                </CommandItem>
              ))}
            </CommandGroup>
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  );
}

function isTemplateInputVisible(input: FlowTemplateInput, values: Record<string, unknown>) {
  const controller = input.show_if?.trim();
  if (!controller) return true;
  if (controller.includes('=')) {
    const [key, expected] = controller.split('=', 2).map((part) => part.trim());
    return expected.split('|').map((part) => part.trim()).includes(String(values[key] ?? ''));
  }
  return values[controller] === true;
}

function isTemplateInputControlledBy(input: FlowTemplateInput, key: string) {
  const controller = input.show_if?.trim();
  if (!controller) return false;
  if (controller.includes('=')) {
    return controller.split('=', 1)[0].trim() === key;
  }
  return controller === key;
}

function templateInputAllowsSpace(input: FlowTemplateInput, space: DocsSpace) {
  const requested = input.space_type?.trim() || 'internal';
  if (requested === 'any') return true;
  return space.type === requested;
}

function templateSpaceSelectLabel(input: FlowTemplateInput) {
  const requested = input.space_type?.trim() || 'internal';
  if (requested === 'external_capable') return 'Select public help space';
  if (requested === 'any') return 'Select docs space';
  return 'Select internal docs space';
}

function groupTemplateInputs(inputs: FlowTemplateInput[], values: Record<string, unknown>) {
  const groups: Array<{ title: string; inputs: FlowTemplateInput[] }> = [];
  for (const input of inputs) {
    if (!isTemplateInputVisible(input, values)) continue;
    const title = input.section?.trim() || 'Template inputs';
    const existing = groups.find((group) => group.title === title);
    if (existing) existing.inputs.push(input);
    else groups.push({ title, inputs: [input] });
  }
  return groups;
}

function isFollowUpTaskSection(title: string) {
  return title === 'Should it create follow-up tasks?';
}

function TemplateInputLabel({ input }: { input: FlowTemplateInput }) {
  return (
    <div className="flex items-center gap-1.5">
      <Label className="text-sm font-medium text-foreground">
        {input.label}{input.required ? ' *' : ''}
      </Label>
      {input.help_text?.trim() && (
        <Tooltip>
          <TooltipTrigger asChild>
            <button
              type="button"
              tabIndex={-1}
              aria-label={`${input.label} help`}
              className="inline-flex text-muted-foreground/60 transition-colors hover:text-muted-foreground"
            >
              <HelpCircleIcon className="h-3.5 w-3.5" />
            </button>
          </TooltipTrigger>
          <TooltipContent side="right" className="max-w-64 text-xs leading-relaxed">
            {input.help_text}
          </TooltipContent>
        </Tooltip>
      )}
    </div>
  );
}

function TooltipIfDisabled({ message, children }: { message?: string | null; children: ReactNode }) {
  if (!message) return <>{children}</>;
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span className="inline-flex">{children}</span>
      </TooltipTrigger>
      <TooltipContent side="top" className="max-w-64 text-xs leading-relaxed">
        {message}
      </TooltipContent>
    </Tooltip>
  );
}

function renderTemplateText(template: string, inputs: Record<string, unknown>) {
  const rawConfig = JSON.stringify(inputs, null, 2);
  let rendered = template.trim().replaceAll('{{raw_configuration_json}}', `\`\`\`json\n${rawConfig}\n\`\`\``);
  for (const [key, value] of Object.entries(inputs)) {
    rendered = rendered.replaceAll(`{{${key}}}`, templateInputLabelValue(value));
  }
  return rendered;
}

function templateInputLabelValue(value: unknown) {
  if (Array.isArray(value)) return value.length > 0 ? value.join(', ') : 'none configured';
  if (typeof value === 'boolean') return value ? 'true' : 'false';
  const text = String(value ?? '').trim();
  return text || 'not configured';
}

function splitStringList(value: string) {
  return value
    .split(/\r?\n|,/)
    .map((item) => item.trim())
    .filter(Boolean);
}

function TemplateStringListInput({
  input,
  value,
  onChange,
}: {
  input: FlowTemplateInput;
  value: unknown;
  onChange: (value: unknown) => void;
}) {
  const [text, setText] = useState(() => (Array.isArray(value) ? value.join('\n') : String(value ?? '')));
  const normalizedValue = Array.isArray(value) ? value.join('\n') : String(value ?? '');

  useEffect(() => {
    if (splitStringList(text).join('\n') !== splitStringList(normalizedValue).join('\n')) {
      setText(normalizedValue);
    }
  }, [normalizedValue, text]);

  return (
    <div className="space-y-1.5 sm:col-span-2">
      <TemplateInputLabel input={input} />
      <Textarea
        rows={3}
        value={text}
        placeholder={input.placeholder || 'One item per line'}
        className="min-h-24 resize-y rounded-lg border-border bg-background"
        onChange={(event) => {
          const next = event.target.value;
          setText(next);
          onChange(splitStringList(next));
        }}
      />
    </div>
  );
}

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

function requiresExplicitAgentTarget(triggerType: string) {
  return isGitProviderTrigger(triggerType) && !isReleaseTrigger(triggerType);
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
  const requestedTrigger = search.trigger_type;
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

export function filterAutomationFlowsForSearch(rules: AutomationRule[], search: Pick<AutomationFlowsSearch, 'show_rule' | 'show_trigger' | 'agent_id' | 'workflow' | 'team'>) {
  const selectedAgentId = String(search.agent_id ?? '').trim();
  return rules.filter((rule) => {
    if (search.show_rule && rule.id !== search.show_rule) return false;
    if (search.show_trigger && rule.trigger_type !== search.show_trigger) return false;
    if (selectedAgentId && stringValue(rule.action_config?.agent_id) !== selectedAgentId) return false;
    if (search.workflow && rule.workflow_id !== search.workflow) return false;
    if (search.team && rule.team_id !== search.team) return false;
    return true;
  });
}

export function draftFromRule(rule: AutomationRule, workflows: WorkflowWithStates[], timezone: string): FlowDraft {
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
  applyScheduleExpressionToDraft(draft, scheduleExpressionFromConfig(rule.trigger_config) || '0 * * * *', timezone);
  applyTriggerDefaults(draft, workflows);
  return draft;
}

export function serializeDraft(draft: FlowDraft, workspaceId: string, timezone: string) {
  let triggerConfig: Record<string, string> = {};
  if (draft.triggerType === 'task.state_entered') {
    triggerConfig = { state_id: draft.triggerStateId };
  } else if (draft.triggerType === 'agent_run.approved') {
    triggerConfig = { state_id: draft.triggerStateId };
  } else if (isPushTrigger(draft.triggerType)) {
    triggerConfig = { repo_full_name: draft.repoFullName.trim(), branch: draft.branch.trim() };
  } else if (isPullRequestTrigger(draft.triggerType)) {
    triggerConfig = { repo_full_name: draft.repoFullName.trim(), base_branch: draft.baseBranch.trim() };
  } else if (isReleaseTrigger(draft.triggerType)) {
    triggerConfig = { repo_full_name: draft.repoFullName.trim(), tag_name: draft.tagName.trim() };
  } else if (isPipelineTrigger(draft.triggerType)) {
    triggerConfig = { repo_full_name: draft.repoFullName.trim(), branch: draft.branch.trim(), conclusion: draft.conclusion.trim() };
  } else if (draft.triggerType === 'cron') {
    triggerConfig = serializeScheduleConfig(scheduleExpressionForDraftUTC(draft, timezone));
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

export function validateDraft(draft: FlowDraft) {
  if (!draft.name.trim()) return 'Give the flow a name';
  if (isWorkflowTrigger(draft.triggerType)) {
    if (!draft.workflowId) return 'Choose a workflow';
    if (!draft.triggerStateId) return 'Choose the state that starts this flow';
  }
  if (isPushTrigger(draft.triggerType) && !draft.repoFullName.trim() && !draft.branch.trim()) {
    return 'Add a repository or branch filter';
  }
  if (
    isPullRequestTrigger(draft.triggerType)
    && !draft.repoFullName.trim()
    && !draft.baseBranch.trim()
  ) {
    return 'Add a repository or base branch filter';
  }
  if (isReleaseTrigger(draft.triggerType) && !draft.repoFullName.trim() && !draft.tagName.trim()) {
    return 'Add a repository or tag filter';
  }
  if (isPipelineTrigger(draft.triggerType) && !draft.repoFullName.trim() && !draft.branch.trim() && !draft.conclusion.trim()) {
    return 'Add a repository, branch, or conclusion filter';
  }
  if (draft.triggerType === 'cron' && !scheduleExpressionForDraft(draft)) {
    return 'Add a cron schedule';
  }
  if (draft.actionType === 'start_agent_run') {
    if (!draft.agentId) return 'Choose an agent';
    if (requiresExplicitAgentTarget(draft.triggerType) && draft.targetMode === 'event') {
      return 'Choose where the agent should run';
    }
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

export function flowMetadataPills(rule: AutomationRule, teamName?: string, _timezone = 'UTC') {
  const pills: Array<{ key: string; label: string; tone?: 'default' | 'info' | 'team' }> = [];
  const repoFullName = stringValue(rule.trigger_config?.repo_full_name);
  const branch = stringValue(rule.trigger_config?.branch);
  const baseBranch = stringValue(rule.trigger_config?.base_branch);
  const tagName = stringValue(rule.trigger_config?.tag_name);
  const conclusion = stringValue(rule.trigger_config?.conclusion);
  const targetType = stringValue(rule.action_config?.target_type);
  const branchOverrides = describeRunBranchOverrides(
    stringValue(rule.action_config?.base_branch),
    stringValue(rule.action_config?.working_branch),
  );

  if (repoFullName) pills.push({ key: 'repo', label: `Repo: ${repoFullName}` });
  if (branch) pills.push({ key: 'branch', label: `Branch: ${branch}` });
  if (baseBranch) pills.push({ key: 'base', label: `Base: ${baseBranch}` });
  if (tagName) pills.push({ key: 'tag', label: `Tag: ${tagName}` });
  if (conclusion) pills.push({ key: 'conclusion', label: `Conclusion: ${conclusion}` });
  if (targetType && targetType !== 'event') pills.push({ key: 'target', label: `Target: ${TARGET_SHORT_LABELS[targetType as FlowDraft['targetMode']] ?? targetType.replaceAll('_', ' ')}` });
  if (branchOverrides) pills.push({ key: 'branches', label: branchOverrides.replace(/[()]/g, ''), tone: 'info' });
  if (teamName) pills.push({ key: 'team', label: teamName, tone: 'team' });

  return pills;
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

function futureRelativeTime(date: Date, now = new Date()) {
  const diff = date.getTime() - now.getTime();
  if (diff <= 60_000) return 'in <1m';
  if (diff < 3_600_000) return `in ${Math.ceil(diff / 60_000)}m`;
  if (diff < 86_400_000) return `in ${Math.ceil(diff / 3_600_000)}h`;
  return `in ${Math.ceil(diff / 86_400_000)}d`;
}

function draftLogicRows(draft: FlowDraft, workflows: WorkflowWithStates[], statesById: Map<string, WorkflowState>, agents: Map<string, string>): FlowLogicRow[] {
  const workflowName = workflows.find((workflow) => workflow.workflow.id === draft.workflowId)?.workflow.name;
  const triggerStateName = statesById.get(draft.triggerStateId)?.name;
  const destinationStateName = statesById.get(draft.targetStateId)?.name;
  const rows: FlowLogicRow[] = [{ connector: 'When', text: triggerLabel(draft.triggerType), tone: 'strong' }];
  const conditions = compactList((() => {
    if (isWorkflowTrigger(draft.triggerType)) {
      const parts = [];
      if (workflowName) parts.push(`workflow = ${workflowName}`);
      if (triggerStateName) parts.push(`state = ${triggerStateName}`);
      return parts;
    }
    const parts = [];
    if (draft.repoFullName.trim()) parts.push(`repo = ${draft.repoFullName.trim()}`);
    if (draft.branch.trim()) parts.push(`branch = ${draft.branch.trim()}`);
    if (draft.baseBranch.trim()) parts.push(`base branch = ${draft.baseBranch.trim()}`);
    if (draft.tagName.trim()) parts.push(`tag = ${draft.tagName.trim()}`);
    if (draft.conclusion.trim()) parts.push(`conclusion = ${draft.conclusion.trim()}`);
    if (scheduleExpressionForDraft(draft) && draft.triggerType === 'cron') parts.push(`schedule = ${describeScheduleExpression(scheduleExpressionForDraft(draft))}`);
    return parts;
  })());
  if (conditions.length > 0) rows.push({ connector: 'If', text: conditions });

  if (draft.actionType === 'move_to_state') {
    rows.push({ connector: 'Then', text: `Move the task to ${destinationStateName || 'another state'}`, tone: destinationStateName ? undefined : 'warning' });
  } else if (draft.actionType === 'merge_branch') {
    rows.push({ connector: 'Then', text: describeMergeInto(draft.targetBranch.trim() || BASE_BRANCH_TOKEN) });
  } else {
    const agentName = agents.get(draft.agentId);
    rows.push({ connector: 'Then', text: agentName ? `Run ${agentName}` : 'Choose an agent', tone: agentName ? undefined : 'warning' });
    rows.push({ connector: 'On', text: TARGET_LABELS[draft.targetMode] });
    const branchOverrides = describeRunBranchOverrides(draft.runBaseBranch.trim(), draft.runWorkingBranch.trim());
    if (branchOverrides) rows.push({ connector: 'And', text: branchOverrides });
  }

  return rows;
}

function triggerTargetType(triggerType: string): AgentTargetType | null {
  if (isWorkflowTrigger(triggerType)) return 'task';
  if (isGitProviderTrigger(triggerType)) return 'repository';
  return null;
}

function draftAgentTargetType(draft: FlowDraft): AgentTargetType | null {
  if (draft.targetMode === 'event') return triggerTargetType(draft.triggerType);
  return draft.targetMode;
}

function draftTargetTeamId(draft: FlowDraft, tasks: Task[], epics: EpicWithStats[]): string | null {
  if (draft.targetMode === 'task') {
    return tasks.find((task) => task.id === draft.targetId)?.team_id ?? null;
  }
  if (draft.targetMode === 'epic') {
    return epics.find((epic) => epic.epic.id === draft.targetId)?.epic.team_id ?? null;
  }
  return null;
}

const AGENT_TARGET_LABELS: Record<string, string> = {
  task: 'tasks',
  epic: 'epics',
  repository: 'repositories',
  document: 'docs',
  workspace: 'workspace',
  support_conversation: 'support conversations',
  support_coverage_gap: 'support gaps',
  crm_deal: 'CRM deals',
  crm_contact: 'CRM contacts',
  crm_company: 'CRM companies',
};

function describeAgentTargets(targets: string[] | undefined) {
  const labels = (targets ?? []).map((target) => AGENT_TARGET_LABELS[target] ?? target);
  if (labels.length === 0) return 'no configured areas';
  if (labels.length <= 3) return labels.join(', ');
  return `${labels.slice(0, 3).join(', ')} +${labels.length - 3}`;
}

function agentUnavailableReasonForFlow(agent: Agent, targetType: AgentTargetType | null, targetTeamId: string | null) {
  if (!targetType) return null;
  if (!agent.allowed_targets?.includes(targetType)) {
    return `Works on ${describeAgentTargets(agent.allowed_targets)}`;
  }
  const agentTeamIds = getAgentTeamIds(agent);
  if (agentTeamIds.length === 0) return null;
  if (!targetTeamId?.trim()) return 'Needs a team-specific target';
  if (!agentTeamIds.includes(targetTeamId)) return 'Limited to another team';
  return null;
}

type FlowSummary = {
  label: string;
  value?: string;
  verb?: string;
};

export function flowTriggerSummary(rule: AutomationRule, statesById: Map<string, WorkflowState>, timezone = 'UTC'): FlowSummary {
  if (rule.trigger_type === 'crm.playbook.work_due') return { label: 'Customer updates or a playbook check', value: 'CRM' };
  const triggerStateId = stringValue(rule.trigger_config?.state_id);
  const stateName = statesById.get(triggerStateId)?.name;

  let label = triggerLabel(rule.trigger_type);
  let triggerValue = '';
  if (rule.trigger_type === 'task.state_entered' && stateName) {
    label = 'Task enters';
    triggerValue = stateName;
  } else if (rule.trigger_type === 'agent_run.approved' && stateName) {
    label = 'Approved in';
    triggerValue = stateName;
  } else if (isPullRequestTrigger(rule.trigger_type)) {
    label = rule.trigger_type === 'github.pull_request_merged' || rule.trigger_type === 'gitlab.merge_request_merged' ? 'PR merged'
      : rule.trigger_type === 'github.pull_request_opened' || rule.trigger_type === 'gitlab.merge_request_opened' ? 'PR opened'
      : rule.trigger_type === 'github.pull_request_closed' || rule.trigger_type === 'gitlab.merge_request_closed' ? 'PR closed'
      : 'PR review requested';
    const baseBranch = stringValue(rule.trigger_config?.base_branch);
    triggerValue = baseBranch ? `base branch ${baseBranch}` : '';
  } else if (isPushTrigger(rule.trigger_type)) {
    label = 'Push arrives';
  } else if (isReleaseTrigger(rule.trigger_type)) {
    label = 'Release published';
  } else if (isPipelineTrigger(rule.trigger_type)) {
    label = rule.trigger_type === 'gitlab.pipeline_completed' ? 'Pipeline completes' : 'Check suite completes';
  } else if (rule.trigger_type === 'cron') {
    const schedule = scheduleExpressionFromConfig(rule.trigger_config);
    label = schedule ? describeScheduleExpressionInTimeZone(schedule, timezone) : 'Schedule ticks';
  }

  return { label, value: triggerValue };
}

export function flowActionSummary(rule: AutomationRule, statesById: Map<string, WorkflowState>, agentNames: Map<string, string>): FlowSummary {
  const targetStateId = stringValue(rule.action_config?.target_state_id);
  const targetStateName = statesById.get(targetStateId)?.name;
  const agentId = stringValue(rule.action_config?.agent_id);
  const agentName = agentNames.get(agentId);

  let actionVerb = rule.action_type.replaceAll('_', ' ');
  let actionValue = '';
  if (rule.action_type === 'start_agent_run') {
    const overrides = describeRunBranchOverrides(
      stringValue(rule.action_config?.base_branch),
      stringValue(rule.action_config?.working_branch),
    );
    actionVerb = 'Run';
    actionValue = agentName ? `${agentName}${overrides}` : 'agent';
  } else if (rule.action_type === 'move_to_state') {
    actionVerb = 'Move to';
    actionValue = targetStateName || 'task state';
  } else if (rule.action_type === 'merge_branch') {
    actionVerb = 'Merge into';
    actionValue = stringValue(rule.action_config?.target_branch) || BASE_BRANCH_TOKEN;
  }

  return {
    label: actionValue ? `${actionVerb} ${actionValue}` : actionVerb,
    verb: actionVerb,
    value: actionValue,
  };
}

function flowIsIncomplete(rule: AutomationRule, agentNames: Map<string, string>) {
  if (rule.trigger_type === 'crm.playbook.work_due') return !stringValue(rule.trigger_config?.playbook_id);
  if (isWorkflowTrigger(rule.trigger_type)) {
    if (!rule.workflow_id || !stringValue(rule.trigger_config?.state_id)) return true;
  }
  if (isPushTrigger(rule.trigger_type) && !stringValue(rule.trigger_config?.repo_full_name) && !stringValue(rule.trigger_config?.branch)) {
    return true;
  }
  if (isPullRequestTrigger(rule.trigger_type) && !stringValue(rule.trigger_config?.repo_full_name) && !stringValue(rule.trigger_config?.base_branch)) {
    return true;
  }
  if (isReleaseTrigger(rule.trigger_type) && !stringValue(rule.trigger_config?.repo_full_name) && !stringValue(rule.trigger_config?.tag_name)) {
    return true;
  }
  if (
    isPipelineTrigger(rule.trigger_type)
    && !stringValue(rule.trigger_config?.repo_full_name)
    && !stringValue(rule.trigger_config?.branch)
    && !stringValue(rule.trigger_config?.conclusion)
  ) {
    return true;
  }
  if (rule.trigger_type === 'cron' && !scheduleExpressionFromConfig(rule.trigger_config)) {
    return true;
  }

  if (rule.action_type === 'start_agent_run') {
    const agentId = stringValue(rule.action_config?.agent_id);
    const targetType = stringValue(rule.action_config?.target_type) || 'event';
    if (!agentId || !agentNames.has(agentId)) return true;
    if (requiresExplicitAgentTarget(rule.trigger_type) && targetType === 'event') return true;
    if (targetType !== 'event' && targetType !== 'workspace' && !stringValue(rule.action_config?.target_id)) return true;
  }
  if (rule.action_type === 'move_to_state' && !stringValue(rule.action_config?.target_state_id)) {
    return true;
  }
  if (rule.action_type === 'merge_branch' && !stringValue(rule.action_config?.target_branch)) {
    return true;
  }

  return false;
}

function flowRunNowBlocker(rule: AutomationRule, agentNames: Map<string, string>) {
  if (rule.trigger_type !== 'cron') return 'Run now is only available for scheduled flows. Event flows run when their trigger happens.';
  if (rule.action_type !== 'start_agent_run') return 'Run now is only supported for agent flows.';
  const agentId = stringValue(rule.action_config?.agent_id);
  if (!agentId || !agentNames.has(agentId)) return 'Complete the flow setup before running it.';
  const targetType = stringValue(rule.action_config?.target_type) || 'event';
  const targetId = stringValue(rule.action_config?.target_id);
  if (rule.trigger_type === 'cron' && targetType === 'event') return '';
  if (targetType === 'workspace') return '';
  if (targetType !== 'event' && targetId) return '';
  return 'This flow needs an event to run.';
}

function flowRunNowRuntimeBlocker(lastRunStatus?: string) {
  const status = lastRunStatus?.trim().toLowerCase();
  if (status === 'queued' || status === 'running') return 'Flow is already running.';
  return '';
}

type FlowState = 'active' | 'paused' | 'error' | 'needs_review' | 'incomplete' | 'playbook';

function flowNeedsReview(healthItem?: AutomationInventoryItem) {
  const metrics = (healthItem?.health.metrics ?? undefined) as Record<string, unknown> | undefined;
  if ((readMetricNumber(metrics, ['pending_review_count', 'pending_approval_count', 'flagged_count']) ?? 0) > 0) {
    return true;
  }
  return readMetricBoolean(metrics, ['needs_review', 'pending_review', 'requires_review', 'flagged']);
}

function deriveFlowState(rule: AutomationRule, agentNames: Map<string, string>, healthItem?: AutomationInventoryItem): FlowState {
  if (rule.trigger_type === 'crm.playbook.work_due') return 'playbook';
  if (flowIsIncomplete(rule, agentNames)) return 'incomplete';
  if (healthItem?.health.last_error_at) return 'error';
  if (flowNeedsReview(healthItem)) return 'needs_review';
  if (!rule.enabled) return 'paused';
  return 'active';
}

const FLOW_STATE_STYLES: Record<FlowState, { pill: string; dot: string; label: string; color: string; edge?: string }> = {
  playbook: { pill: 'border-border bg-muted text-muted-foreground', dot: 'bg-current', label: 'Playbook managed', color: '#787774' },
  active: {
    pill: 'border-emerald-500/30 bg-emerald-500/10 text-emerald-700 dark:text-emerald-400',
    dot: 'bg-current',
    label: 'Active',
    color: '#1E9E6A',
  },
  paused: {
    pill: 'border-amber-500/30 bg-amber-500/10 text-amber-700 dark:text-amber-400',
    dot: 'bg-amber-500',
    label: 'Paused',
    color: '#B4B0A7',
  },
  error: {
    pill: 'border-rose-500/40 bg-rose-500/10 text-rose-600 dark:text-rose-400',
    dot: 'bg-rose-500',
    label: 'Errored',
    color: '#C0483C',
    edge: '#C0483C',
  },
  needs_review: {
    pill: 'border-violet-500/30 bg-violet-500/10 text-violet-700 dark:text-violet-400',
    dot: 'bg-violet-500',
    label: 'Needs review',
    color: '#C08A2E',
    edge: '#C08A2E',
  },
  incomplete: {
    pill: 'border-border/60 bg-muted/40 text-muted-foreground',
    dot: 'bg-muted-foreground/40',
    label: 'Incomplete',
    color: '#C08A2E',
    edge: '#C08A2E',
  },
};
function readMetricNumber(metrics: Record<string, unknown> | undefined, keys: string[]): number | undefined {
  if (!metrics) return undefined;
  for (const key of keys) {
    const value = metrics[key];
    if (typeof value === 'number' && Number.isFinite(value)) return value;
    if (typeof value === 'string' && value.trim() && !Number.isNaN(Number(value))) return Number(value);
  }
  return undefined;
}

function readMetricString(metrics: Record<string, unknown> | undefined, keys: string[]): string | undefined {
  if (!metrics) return undefined;
  for (const key of keys) {
    const value = metrics[key];
    if (typeof value === 'string' && value.trim()) return value.trim();
  }
  return undefined;
}

function readMetricBoolean(metrics: Record<string, unknown> | undefined, keys: string[]) {
  if (!metrics) return false;
  for (const key of keys) {
    const value = metrics[key];
    if (typeof value === 'boolean') return value;
    if (typeof value === 'number' && Number.isFinite(value)) return value > 0;
    if (typeof value === 'string') {
      const normalized = value.trim().toLowerCase();
      if (['true', 'yes', '1'].includes(normalized)) return true;
      if (['false', 'no', '0', ''].includes(normalized)) return false;
    }
  }
  return false;
}

type FlowDetailsSection = {
  title: string;
  rows: Array<{ label: string; value: string }>;
};

function flowLastRunLabel(healthItem?: AutomationInventoryItem) {
  if (healthItem?.health.last_success_at) return relativeTime(healthItem.health.last_success_at);
  if (healthItem?.health.last_error_at) return relativeTime(healthItem.health.last_error_at);
  if (healthItem?.health.last_seen_at) return relativeTime(healthItem.health.last_seen_at);
  return 'never';
}

function flowHasRunActivity(healthItem?: AutomationInventoryItem) {
  return Boolean(healthItem?.health.last_success_at || healthItem?.health.last_seen_at || healthItem?.health.last_error_at);
}

export function flowNextRunLabel(rule: AutomationRule, now = new Date()) {
  if (rule.trigger_type !== 'cron') return null;
  const schedule = scheduleExpressionFromConfig(rule.trigger_config);
  if (!schedule) return 'Not scheduled';
  const parsed = parseSimpleScheduleExpression(schedule);
  if (!parsed) return 'Custom schedule';
  const nextRun = nextCronRunDate(parsed, now);
  return nextRun ? futureRelativeTime(nextRun, now) : 'Not scheduled';
}

function flowTriggerRunLabel(rule: AutomationRule) {
  return rule.trigger_type === 'cron' ? null : 'Runs when triggered';
}

function flowMetricValues(healthItem?: AutomationInventoryItem) {
  const metrics = (healthItem?.health.metrics ?? undefined) as Record<string, unknown> | undefined;
  return {
    totalRuns: readMetricNumber(metrics, ['total_runs', 'runs_total', 'handled_total']),
    errorRuns: readMetricNumber(metrics, ['error_runs', 'failed_runs', 'errors_total']),
    lastRunId: readMetricString(metrics, ['last_run_id', 'run_id']),
    lastExecutionId: readMetricString(metrics, ['last_execution_id', 'execution_id']),
    lastRunStatus: readMetricString(metrics, ['last_run_status', 'run_status', 'status']),
    nextRunAt: readMetricString(metrics, ['next_run_at']),
  };
}

function flowCurrentRunLabel(lastRunStatus?: string) {
  switch (lastRunStatus?.trim().toLowerCase()) {
    case 'running':
      return 'Running now';
    case 'queued':
      return 'Run queued';
    default:
      return null;
  }
}

function flowActivityBlockerLabel(flowState: FlowState) {
  if (flowState === 'paused') return 'Flow paused';
  if (flowState === 'incomplete') return 'Setup incomplete';
  return null;
}

export function flowDetailsSections({
  rule,
  workflows = [],
  statesById,
  agentNames,
  teamName,
  healthItem,
  timezone = 'UTC',
}: {
  rule: AutomationRule;
  workflows?: WorkflowWithStates[];
  statesById: Map<string, WorkflowState>;
  agentNames: Map<string, string>;
  teamName?: string;
  healthItem?: AutomationInventoryItem;
  timezone?: string;
}): FlowDetailsSection[] {
  const flowState = deriveFlowState(rule, agentNames, healthItem);
  const draft = draftFromRule(rule, workflows, timezone);
  const workflowName = workflows.find((workflow) => workflow.workflow.id === draft.workflowId)?.workflow.name;
  const triggerStateName = statesById.get(draft.triggerStateId)?.name;
  const destinationStateName = statesById.get(draft.targetStateId)?.name;
  const agentName = agentNames.get(draft.agentId);
  const additionalContext = stringValue(rule.action_config?.additional_context).trim();
  const nextRunLabel = flowNextRunLabel(rule);
  const triggerRunLabel = flowTriggerRunLabel(rule);
  const lastError = healthItem?.health.last_error_message?.trim();
  const storedSchedule = scheduleExpressionFromConfig(rule.trigger_config);
  const branchOverrides = describeRunBranchOverrides(draft.runBaseBranch.trim(), draft.runWorkingBranch.trim());

  const sections: FlowDetailsSection[] = [
    {
      title: 'Flow setup',
      rows: compactRows([
        { label: 'Name', value: draft.name },
        draft.description.trim() ? { label: 'Internal description', value: draft.description.trim() } : null,
        teamName ? { label: 'Team', value: teamName } : null,
        { label: 'Status', value: FLOW_STATE_STYLES[flowState].label },
      ]),
    },
    {
      title: 'Trigger setup',
      rows: compactRows([
        { label: 'Trigger', value: triggerLabel(draft.triggerType) },
        workflowName ? { label: 'Workflow', value: workflowName } : null,
        triggerStateName ? { label: 'State', value: triggerStateName } : null,
        draft.repoFullName.trim() ? { label: 'Repository', value: draft.repoFullName.trim() } : null,
        draft.branch.trim() ? { label: 'Branch', value: draft.branch.trim() } : null,
        draft.baseBranch.trim() && isPullRequestTrigger(draft.triggerType) ? { label: 'Base branch', value: draft.baseBranch.trim() } : null,
        draft.tagName.trim() ? { label: 'Tag', value: draft.tagName.trim() } : null,
        draft.conclusion.trim() ? { label: 'Conclusion', value: draft.conclusion.trim() } : null,
        draft.triggerType === 'cron' && storedSchedule ? { label: 'Schedule', value: describeScheduleExpressionInTimeZone(storedSchedule, timezone) } : null,
        nextRunLabel ? { label: 'Next run', value: nextRunLabel } : null,
        triggerRunLabel ? { label: 'Run timing', value: triggerRunLabel } : null,
      ]),
    },
    {
      title: 'Action setup',
      rows: compactRows([
        { label: 'Action', value: ACTION_LABELS[draft.actionType] ?? draft.actionType.replaceAll('_', ' ') },
        agentName ? { label: 'Agent', value: agentName } : null,
        draft.actionType === 'start_agent_run' ? { label: 'Run on', value: TARGET_LABELS[draft.targetMode] ?? draft.targetMode } : null,
        draft.targetId.trim() ? { label: 'Target ID', value: draft.targetId.trim() } : null,
        destinationStateName ? { label: 'Destination state', value: destinationStateName } : null,
        draft.targetBranch.trim() ? { label: 'Merge target', value: describeMergeInto(draft.targetBranch.trim()) } : null,
        branchOverrides ? { label: 'Branch overrides', value: branchOverrides.replace(/[()]/g, '') } : null,
      ]),
    },
  ];

  if (additionalContext) {
    sections.push({
      title: 'Instructions',
      rows: [{ label: 'Run context', value: additionalContext }],
    });
  }

  sections.push({
    title: 'Diagnostics',
    rows: compactRows([
      { label: 'Last run', value: flowLastRunLabel(healthItem) },
      lastError ? { label: 'Last error', value: lastError } : null,
    ]),
  });

  return sections;
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

function TruncatedTextWithTooltip({
  children,
  className,
  testId,
}: {
  children: string;
  className?: string;
  testId: string;
}) {
  const textRef = useRef<HTMLParagraphElement | null>(null);
  const [isTruncated, setIsTruncated] = useState(false);
  const measure = useCallback(() => {
    const element = textRef.current;
    if (!element) return;
    setIsTruncated(element.scrollWidth > element.clientWidth || element.scrollHeight > element.clientHeight);
  }, []);

  useEffect(() => {
    measure();
    const element = textRef.current;
    if (!element || typeof ResizeObserver === 'undefined') return;
    const observer = new ResizeObserver(measure);
    observer.observe(element);
    return () => observer.disconnect();
  }, [children, measure]);

  const text = (
    <p
      ref={textRef}
      className={className}
      data-testid={testId}
      data-tooltip-enabled={isTruncated ? 'true' : 'false'}
    >
      {children}
    </p>
  );

  if (!isTruncated) return text;

  return (
    <TooltipProvider>
      <Tooltip>
        <TooltipTrigger asChild>{text}</TooltipTrigger>
        <TooltipContent side="top" align="start" className="max-w-md whitespace-pre-wrap break-words text-xs leading-relaxed">
          {children}
        </TooltipContent>
      </Tooltip>
    </TooltipProvider>
  );
}

export function FlowRow({
  rule,
  statesById,
  agentNames,
  healthItem,
  workspaceSlug,
  timezone = 'UTC',
  canEdit,
  canRunNowAction,
  onEdit,
  onToggle,
  onRunNow,
  onDelete,
  onOpen,
  runningNow,
}: {
  rule: AutomationRule;
  statesById: Map<string, WorkflowState>;
  agentNames: Map<string, string>;
  teamName?: string;
  healthItem?: AutomationInventoryItem;
  workspaceSlug?: string;
  timezone?: string;
  canEdit: boolean;
  canRunNowAction: boolean;
  onEdit: (rule: AutomationRule) => void;
  onToggle: (rule: AutomationRule) => void;
  onRunNow: (rule: AutomationRule) => void;
  onDelete: (rule: AutomationRule) => void;
  onOpen?: (rule: AutomationRule) => void;
  runningNow?: boolean;
}) {
  const menuTriggerRef = useRef<HTMLButtonElement | null>(null);
  const isIncomplete = flowIsIncomplete(rule, agentNames);
  const agentId = stringValue(rule.action_config?.agent_id);
  const agentName = agentNames.get(agentId);
  const agentMissing = rule.action_type === 'start_agent_run' && (!agentId || !agentNames.has(agentId));
  const flowState = deriveFlowState(rule, agentNames, healthItem);
  const { totalRuns, lastRunStatus, nextRunAt } = flowMetricValues(healthItem);
  const trigger = flowTriggerSummary(rule, statesById, timezone);
  const action = flowActionSummary(rule, statesById, agentNames);
  const lastRunLabel = flowLastRunLabel(healthItem);
  const hasRunActivity = flowHasRunActivity(healthItem);
  const nextRunLabel = rule.enabled && nextRunAt
    ? futureRelativeTime(new Date(nextRunAt))
    : flowNextRunLabel(rule);
  const triggerRunLabel = flowTriggerRunLabel(rule);
  const lastErrorMessage = healthItem?.health.last_error_message?.trim();
  const currentRunLabel = flowCurrentRunLabel(lastRunStatus);
  const blockerLabel = flowActivityBlockerLabel(flowState);
  const activitySearch = { page: 1, source: 'automation_rule', reference_id: rule.id };
  const runNowBlocker = flowRunNowRuntimeBlocker(lastRunStatus) || flowRunNowBlocker(rule, agentNames);
  const managed = rule.trigger_type === 'crm.playbook.work_due';
  const playbookPath = managed && workspaceSlug && stringValue(rule.trigger_config?.playbook_id) ? `/w/${encodeURIComponent(workspaceSlug)}/crm/playbooks/${encodeURIComponent(stringValue(rule.trigger_config?.playbook_id))}` : undefined;
  const supportsRunNow = rule.trigger_type === 'cron';
  const showRunNow = canRunNowAction;
  const canRunNow = showRunNow && supportsRunNow && !runNowBlocker;
  const stateStyle = FLOW_STATE_STYLES[flowState];
  const triggerKind = managed ? 'CRM' : rule.trigger_type === 'cron'
    ? 'Schedule'
    : rule.trigger_type.startsWith('github.')
      ? 'GitHub'
      : rule.trigger_type.startsWith('gitlab.')
        ? 'GitLab'
        : rule.trigger_type.startsWith('agent_run.')
          ? 'Approval'
          : 'Task';
  const actionKind = rule.action_type === 'start_agent_run'
    ? 'Agent'
    : rule.action_type === 'merge_branch'
      ? 'Branch'
      : 'Task';
  const desktopNextLabel = managed ? 'Managed in CRM' : blockerLabel ?? (rule.enabled ? nextRunLabel ?? triggerRunLabel ?? 'On trigger' : 'Paused');

  const openRow = () => onOpen?.(rule);

  return (
    <div
      role={onOpen ? 'button' : undefined}
      tabIndex={onOpen ? 0 : undefined}
      aria-label={onOpen ? `Open ${rule.name}` : undefined}
      onClick={openRow}
      onKeyDown={(event) => {
        if (!onOpen || (event.key !== 'Enter' && event.key !== ' ')) return;
        event.preventDefault();
        openRow();
      }}
      className="group relative grid min-w-0 cursor-pointer grid-cols-[minmax(0,1fr)_28px] items-center gap-x-3 gap-y-2 border-b border-border/60 px-[14px] py-[13px] outline-none transition-colors duration-100 hover:bg-muted/50 focus-visible:bg-muted/50 focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring md:grid-cols-[minmax(0,1fr)_96px_116px_120px_28px] md:gap-4"
      style={stateStyle.edge ? { boxShadow: `inset 2px 0 0 ${stateStyle.edge}` } : undefined}
    >
      <div className="min-w-0" data-testid="flow-row-title-area">
        <div className="flex min-w-0 items-center gap-2">
          <span className="h-[7px] w-[7px] shrink-0 rounded-full" style={{ backgroundColor: stateStyle.color }} aria-hidden="true" />
          <span className="sr-only">{stateStyle.label}</span>
          <TruncatedTextWithTooltip testId="flow-row-name-text" className="min-w-0 truncate text-sm font-medium text-foreground">
            {rule.name}
          </TruncatedTextWithTooltip>
          {agentMissing ? (
            <span className="shrink-0 rounded-md bg-amber-500/10 px-1.5 py-0.5 text-[10px] font-medium text-amber-700 dark:text-amber-300">Missing agent</span>
          ) : isIncomplete ? (
            <span className="shrink-0 rounded-md bg-amber-500/10 px-1.5 py-0.5 text-[10px] font-medium text-amber-700 dark:text-amber-300">Setup incomplete</span>
          ) : null}
        </div>
        <div className="mt-1.5 flex min-w-0 items-center gap-1.5 pl-[15px] text-xs text-foreground/80">
          <span className="shrink-0 text-muted-foreground">When</span>
          <span className="inline-flex min-w-0 items-center gap-1.5 rounded-md border border-border bg-background px-2 py-0.5">
            <span className="shrink-0 text-[10px] font-medium uppercase tracking-wide text-muted-foreground">{triggerKind}</span>
            <span className="truncate">{trigger.value ? `${trigger.label} ${trigger.value}` : trigger.label}</span>
          </span>
          <ArrowRight01Icon className="h-3 w-3 shrink-0 text-muted-foreground/50" />
          <span className="inline-flex min-w-0 items-center gap-1.5 rounded-md border border-border bg-background px-2 py-0.5" data-testid={agentName ? 'flow-row-action-agent' : undefined}>
            <span className="shrink-0 text-[10px] font-medium uppercase tracking-wide text-muted-foreground" data-testid={agentName ? 'flow-row-action-verb' : undefined}>{actionKind}</span>
            <span className="truncate" data-testid={agentName ? 'flow-row-action-agent-name' : undefined}>{action.label}</span>
            {agentName ? <span className="sr-only" data-testid="flow-row-action-agent-avatar">Agent avatar</span> : null}
          </span>
        </div>
        <div className="mt-2 flex flex-wrap gap-x-4 gap-y-1 pl-[15px] text-xs text-muted-foreground md:hidden">
          <span>{totalRuns ?? 0} runs</span>
          <span>{currentRunLabel ?? (hasRunActivity ? lastRunLabel : 'Never run')}</span>
          <span>{desktopNextLabel}</span>
        </div>
        {flowState === 'error' && lastErrorMessage ? (
          <p className="mt-2 truncate pl-[15px] text-xs text-destructive">{lastErrorMessage}</p>
        ) : null}
      </div>

      <div className="hidden text-right text-sm tabular-nums text-foreground md:block">{totalRuns ?? 0}</div>
      <div className="hidden text-xs text-muted-foreground md:block">{currentRunLabel ?? (hasRunActivity ? lastRunLabel : 'Never')}</div>
      <div className={cn('hidden text-xs md:block', rule.enabled && nextRunLabel ? 'text-foreground' : 'text-muted-foreground')}>{desktopNextLabel}</div>

      <div onClick={(event) => event.stopPropagation()} onKeyDown={(event) => event.stopPropagation()}>
        <DropdownMenu onOpenChange={(open) => {
          if (!open) window.requestAnimationFrame(() => menuTriggerRef.current?.blur());
        }}>
          <DropdownMenuTrigger asChild>
            <Button ref={menuTriggerRef} variant="ghost" size="sm" className="h-[26px] w-[26px] p-0 text-muted-foreground hover:bg-muted hover:text-foreground" aria-label={`Actions for ${rule.name}`}>
              <MoreHorizontalIcon className="h-4 w-4" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="w-44 rounded-lg p-1.5">
            {workspaceSlug ? <DropdownMenuItem asChild><a href={buildAutomationActivityPath(workspaceSlug, activitySearch, 'trigger-executions')}>View run history</a></DropdownMenuItem> : null}
            {supportsRunNow && showRunNow ? <DropdownMenuItem disabled={!canRunNow || runningNow} onClick={() => onRunNow(rule)}>{runningNow ? 'Running…' : 'Run now'}</DropdownMenuItem> : null}
            {playbookPath ? <DropdownMenuItem asChild><a href={playbookPath}>Open playbook</a></DropdownMenuItem> : null}
            {canEdit && !managed ? <DropdownMenuItem onClick={() => onEdit(rule)}>Edit flow</DropdownMenuItem> : null}
            {canEdit && !managed ? <DropdownMenuItem onClick={() => onToggle(rule)}>{rule.enabled ? 'Disable' : 'Enable'}</DropdownMenuItem> : null}
            {canEdit && !managed ? <DropdownMenuItem className="text-destructive focus:text-destructive" onClick={() => onDelete(rule)}>Delete</DropdownMenuItem> : null}
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
    </div>
  );
}

function flowTargetLabel(rule: AutomationRule, workspaceName?: string) {
  const targetType = stringValue(rule.action_config?.target_type);
  const targetID = stringValue(rule.action_config?.target_id);
  const repo = stringValue(rule.trigger_config?.repo_full_name);
  if (targetType === 'workspace') return workspaceName || 'Workspace';
  if (targetType === 'event' || (!targetType && rule.trigger_type !== 'cron')) {
    return repo || 'Event target';
  }
  if (repo && targetType === 'repository') return repo;
  if (targetType && targetID) return `${targetType.replaceAll('_', ' ')} · ${targetID}`;
  if (repo) return repo;
  return 'Workspace';
}

function executionStatusMeta(status: string) {
  switch (status.trim().toLowerCase()) {
    case 'completed':
      return { color: '#1E9E6A', label: 'Completed' };
    case 'running':
    case 'queued':
      return { color: '#3F6A9E', label: status === 'queued' ? 'Queued' : 'Running' };
    case 'failed':
      return { color: '#C0483C', label: 'Failed' };
    case 'paused':
      return { color: '#C08A2E', label: 'Needs attention' };
    case 'cancelled':
      return { color: '#B4B0A7', label: 'Cancelled' };
    default:
      return { color: '#B4B0A7', label: status || 'Unknown' };
  }
}

function FlowDetailDrawer({
  rule,
  workspaceId,
  workspaceSlug,
  workspaceName,
  statesById,
  agentNames,
  healthItem,
  timezone,
  canEdit,
  canRunNowAction,
  runningNow,
  onOpenChange,
  onEdit,
  onToggle,
  onRunNow,
  onDelete,
}: {
  rule: AutomationRule | null;
  workspaceId: string;
  workspaceSlug?: string;
  workspaceName?: string;
  statesById: Map<string, WorkflowState>;
  agentNames: Map<string, string>;
  healthItem?: AutomationInventoryItem;
  timezone: string;
  canEdit: boolean;
  canRunNowAction: boolean;
  runningNow: boolean;
  onOpenChange: (open: boolean) => void;
  onEdit: (rule: AutomationRule) => void;
  onToggle: (rule: AutomationRule) => void;
  onRunNow: (rule: AutomationRule) => void;
  onDelete: (rule: AutomationRule) => void;
}) {
  const recentRuns = useAutomationActivity(workspaceId, {
    source: 'automation_rule',
    reference_id: rule?.id,
    page: 1,
    per_page: 4,
  }, !!rule);

  const trigger = rule ? flowTriggerSummary(rule, statesById, timezone) : null;
  const action = rule ? flowActionSummary(rule, statesById, agentNames) : null;
  const agentId = rule ? stringValue(rule.action_config?.agent_id) : '';
  const agentName = agentNames.get(agentId);
  const flowState = rule ? deriveFlowState(rule, agentNames, healthItem) : 'active';
  const stateStyle = FLOW_STATE_STYLES[flowState];
  const metrics = flowMetricValues(healthItem);
  const lastRun = flowLastRunLabel(healthItem);
  const managed = rule?.trigger_type === 'crm.playbook.work_due';
  const nextRun = managed ? 'Managed in the CRM playbook' : rule
    ? !rule.enabled
      ? 'Paused'
      : metrics.nextRunAt
        ? futureRelativeTime(new Date(metrics.nextRunAt))
        : flowNextRunLabel(rule) ?? 'On trigger'
    : '—';
  const runNowBlocker = rule
    ? flowRunNowRuntimeBlocker(metrics.lastRunStatus) || flowRunNowBlocker(rule, agentNames)
    : '';
  const canRunNow = !!rule && rule.trigger_type === 'cron' && canRunNowAction && !runNowBlocker && !runningNow;

  return (
    <Sheet open={!!rule} onOpenChange={onOpenChange}>
      <SheetContent
        className="w-full overflow-hidden border-l border-border bg-popover p-0 shadow-none duration-150 sm:max-w-[460px]"
        overlayClassName="bg-[rgba(26,25,23,0.14)] backdrop-blur-none duration-150"
        showCloseButton
      >
        {rule ? (
          <>
            <SheetHeader className="sticky top-0 z-10 border-b border-border bg-popover px-[22px] py-[18px] pr-14">
              <div className="flex min-w-0 items-center gap-2.5">
                <span className="h-[7px] w-[7px] shrink-0 rounded-full" style={{ backgroundColor: stateStyle.color }} aria-hidden="true" />
                <SheetTitle className="truncate text-sm font-semibold">{rule.name}</SheetTitle>
              </div>
              <SheetDescription className="sr-only">Details and actions for {rule.name}</SheetDescription>
            </SheetHeader>

            <div className="flex min-h-0 flex-1 flex-col overflow-y-auto px-[22px] py-[22px]">
              <div className="space-y-[26px]">
                <p className="text-pretty text-sm leading-relaxed text-muted-foreground">
                  {rule.description?.trim() || 'This flow connects a trigger to an automated action.'}
                </p>

                <section className="space-y-2.5">
                  <h3 className="text-xs font-medium uppercase tracking-wide text-muted-foreground">Logic</h3>
                  <div className="overflow-hidden rounded-lg border border-border">
                    {[
                      ['When', trigger?.value ? `${trigger.label} ${trigger.value}` : trigger?.label || '—'],
                      ['Then', action?.label || '—'],
                      ['Using', agentName || (rule.action_type === 'start_agent_run' ? 'Not connected' : 'Built-in action')],
                      ['On', flowTargetLabel(rule, workspaceName)],
                    ].map(([label, value], index) => (
                      <div key={label} className={cn('flex gap-3 px-[14px] py-[13px]', index > 0 && 'border-t border-border')}>
                        <span className="w-[52px] shrink-0 text-xs text-muted-foreground">{label}</span>
                        <span className="min-w-0 break-words text-sm text-foreground">{value}</span>
                      </div>
                    ))}
                  </div>
                </section>

                <section className="grid grid-cols-2 gap-px overflow-hidden rounded-lg border border-border bg-border">
                  {[
                    ['Total runs', String(metrics.totalRuns ?? 0), false],
                    ['Errors', String(metrics.errorRuns ?? 0), (metrics.errorRuns ?? 0) > 0],
                    ['Last run', lastRun, false],
                    ['Next run', nextRun, false],
                  ].map(([label, value, danger]) => (
                    <div key={String(label)} className="flex min-h-[64px] flex-col gap-1 bg-popover px-[14px] py-[13px]">
                      <span className="text-xs text-muted-foreground">{label}</span>
                      <span className={cn(label === 'Total runs' || label === 'Errors' ? 'text-base tabular-nums' : 'text-sm', danger && 'text-destructive')}>{String(value)}</span>
                    </div>
                  ))}
                </section>

                <section className="space-y-2.5">
                  <div className="flex items-center justify-between gap-3">
                    <h3 className="text-xs font-medium uppercase tracking-wide text-muted-foreground">Recent runs</h3>
                    {workspaceSlug ? <a className="text-xs text-muted-foreground hover:text-foreground" href={buildAutomationActivityPath(workspaceSlug, { source: 'automation_rule', reference_id: rule.id }, 'trigger-executions')}>View all</a> : null}
                  </div>
                  <div>
                    {recentRuns.isLoading ? Array.from({ length: 3 }).map((_, index) => <Skeleton key={index} className="mb-2 h-8 w-full rounded-md" />) : null}
                    {recentRuns.isError ? <p className="py-3 text-xs text-destructive">Recent runs could not be loaded.</p> : null}
                    {!recentRuns.isLoading && !recentRuns.isError && recentRuns.data?.data.length === 0 ? <p className="py-3 text-xs text-muted-foreground">No runs yet.</p> : null}
                    {recentRuns.data?.data.map((run) => {
                      const status = executionStatusMeta(run.status);
                      const path = workspaceSlug
                        ? buildAutomationActivityPath(workspaceSlug, {
                            source: 'automation_rule',
                            reference_id: rule.id,
                            run_id: run.run_id,
                            execution_id: run.run_id ? undefined : run.execution_id,
                          }, 'trigger-executions')
                        : undefined;
                      const content = (
                        <>
                          <span className="h-[7px] w-[7px] shrink-0 rounded-full" style={{ backgroundColor: status.color }} aria-hidden="true" />
                          <span className="min-w-0 flex-1 truncate text-xs text-foreground">{status.label}{run.target_title ? ` · ${run.target_title}` : ''}</span>
                          <span className="shrink-0 text-xs text-muted-foreground">{relativeTime(run.fired_at)}</span>
                        </>
                      );
                      return path ? <a key={run.execution_id} href={path} className="flex items-center gap-2.5 border-b border-border/60 px-0.5 py-2.5 hover:bg-muted/50">{content}</a> : <div key={run.execution_id} className="flex items-center gap-2.5 border-b border-border/60 px-0.5 py-2.5">{content}</div>;
                    })}
                  </div>
                </section>

                <div className="flex flex-wrap gap-2 pb-2">
                  {managed && workspaceSlug ? <Button variant="outline" size="sm" asChild><a href={`/w/${encodeURIComponent(workspaceSlug)}/crm/playbooks/${encodeURIComponent(stringValue(rule.trigger_config?.playbook_id))}`}>Manage in Playbook Setup</a></Button> : null}
                  {canEdit && !managed ? <Button variant="outline" size="sm" onClick={() => onToggle(rule)}>{rule.enabled ? 'Pause flow' : 'Enable flow'}</Button> : null}
                  {canEdit && !managed ? <Button variant="outline" size="sm" onClick={() => onEdit(rule)}>Edit flow</Button> : null}
                  {rule.trigger_type === 'cron' && canRunNowAction ? (
                    <TooltipProvider><Tooltip><TooltipTrigger asChild><span><Button variant="outline" size="sm" disabled={!canRunNow} onClick={() => onRunNow(rule)}>{runningNow ? 'Running…' : 'Run now'}</Button></span></TooltipTrigger>{runNowBlocker ? <TooltipContent>{runNowBlocker}</TooltipContent> : null}</Tooltip></TooltipProvider>
                  ) : null}
                  {canEdit && !managed ? <Button variant="destructive" size="sm" onClick={() => onDelete(rule)}>Delete</Button> : null}
                </div>
              </div>
            </div>
          </>
        ) : null}
      </SheetContent>
    </Sheet>
  );
}

function isGitProviderTrigger(triggerType: string) {
  return triggerType.startsWith('github.') || triggerType.startsWith('gitlab.');
}
function showRepoField(triggerType: string) {
  return isGitProviderTrigger(triggerType);
}
function showBranchField(triggerType: string) {
  return isPushTrigger(triggerType) || isPipelineTrigger(triggerType);
}
function showBaseBranchField(triggerType: string) {
  return isPullRequestTrigger(triggerType);
}
function showTagField(triggerType: string) {
  return isReleaseTrigger(triggerType);
}
function showConclusionField(triggerType: string) {
  return isPipelineTrigger(triggerType);
}
function showBranchOverrideFields(triggerType: string, actionType: string) {
  return actionType === 'start_agent_run' && isGitProviderTrigger(triggerType);
}

function isPushTrigger(triggerType: string) {
  return triggerType === 'github.push' || triggerType === 'gitlab.push';
}

function isPullRequestTrigger(triggerType: string) {
  return triggerType === 'github.pull_request_opened'
    || triggerType === 'github.pull_request_merged'
    || triggerType === 'github.pull_request_closed'
    || triggerType === 'github.pull_request_review_requested'
    || triggerType === 'gitlab.merge_request_opened'
    || triggerType === 'gitlab.merge_request_merged'
    || triggerType === 'gitlab.merge_request_closed';
}

function isReleaseTrigger(triggerType: string) {
  return triggerType === 'github.release_published' || triggerType === 'gitlab.release_published';
}

function isPipelineTrigger(triggerType: string) {
  return triggerType === 'github.check_suite_completed' || triggerType === 'gitlab.pipeline_completed';
}

function SentenceRow({ connector, tone, children }: { connector: string; tone?: FlowLogicRow['tone']; children: ReactNode }) {
  return (
    <div className="flex flex-wrap items-center gap-2">
      <span
        className={cn(
          'mt-0.5 w-10 shrink-0 text-[10px] font-semibold uppercase tracking-[0.08em]',
          tone === 'strong' ? 'text-foreground/70' : tone === 'warning' ? 'text-destructive/80' : 'text-muted-foreground/80',
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

export function FlowComposer({
  workspaceId,
  open,
  mode,
  draft,
  workflows,
  statesById,
  agents,
  accessibleTeamIds,
  canSeeAllAgents,
  tasks,
  epics,
  repositories,
  timezone,
  saving,
  canEdit,
  onOpenChange,
  onBack,
  onDraftChange,
  onSave,
}: {
  workspaceId: string;
  open: boolean;
  mode: FlowComposerMode;
  draft: FlowDraft;
  workflows: WorkflowWithStates[];
  statesById: Map<string, WorkflowState>;
  agents: Agent[];
  accessibleTeamIds: Set<string>;
  canSeeAllAgents: boolean;
  tasks: Task[];
  epics: EpicWithStats[];
  repositories: GitRepository[];
  timezone: string;
  saving: boolean;
  canEdit: boolean;
  onOpenChange: (open: boolean) => void;
  onBack?: () => void;
  onDraftChange: (updater: (current: FlowDraft) => FlowDraft) => void;
  onSave: () => Promise<void>;
}) {
  const workflowStates = workflows.find((workflow) => workflow.workflow.id === draft.workflowId)?.states ?? [];
  const actionOptions = allowedActions(draft.triggerType);
  const agentNames = useMemo(() => new Map(agents.map((agent) => [agent.id, agent.name])), [agents]);
  const targetType = draftAgentTargetType(draft);
  const targetTeamId = draftTargetTeamId(draft, tasks, epics);
  const visibleAgents = useMemo(
    () => agents.filter((agent) => isAgentVisibleToActor(agent, accessibleTeamIds, canSeeAllAgents)),
    [accessibleTeamIds, agents, canSeeAllAgents],
  );
  const agentOptions = useMemo(
    () => visibleAgents.map((agent) => ({
      agent,
      disabledReason: agentUnavailableReasonForFlow(agent, targetType, targetTeamId),
    })),
    [targetTeamId, targetType, visibleAgents],
  );
  const availableAgents = useMemo(
    () => agentOptions.filter((option) => !option.disabledReason).map((option) => option.agent),
    [agentOptions],
  );
  const flowLogicRows = useMemo(
    () => draftLogicRows(draft, workflows, statesById, agentNames),
    [agentNames, draft, statesById, workflows],
  );
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
    if (!draft.agentId) return;
    if (availableAgents.some((agent) => agent.id === draft.agentId)) return;
    onDraftChange((current) => current.agentId === draft.agentId ? { ...current, agentId: '' } : current);
  }, [availableAgents, draft.agentId, onDraftChange]);

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
    if (normalized.actionType === 'start_agent_run' && normalized.triggerType === 'cron' && normalized.targetMode === 'event') {
      normalized.targetMode = 'workspace';
      normalized.targetId = workspaceId;
    }
    return normalized;
  });

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        className="max-h-[90vh] overflow-y-auto sm:max-w-3xl"
        onOpenAutoFocus={(event) => {
          if (mode !== 'create') event.preventDefault();
        }}
      >
        <DialogHeader>
          <DialogTitle>{mode === 'create' ? 'Create flow' : 'Edit flow'}</DialogTitle>
          <DialogDescription className="text-xs">
            Reads top to bottom as a sentence. Only fields relevant to your trigger and action appear.
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-5">
          <div className="space-y-2">
            <div className="space-y-1.5">
              <Label htmlFor="flow-name">Flow name</Label>
              <Input
                id="flow-name"
                value={draft.name}
                onChange={(event) => updateDraft((current) => ({ ...current, name: event.target.value }))}
                placeholder="Review merged PRs"
                className="h-10 text-base font-medium"
                autoFocus={mode === 'create'}
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="flow-description">Internal description</Label>
              <Textarea
                id="flow-description"
                rows={2}
                value={draft.description}
                onChange={(event) => updateDraft((current) => ({ ...current, description: event.target.value }))}
                placeholder="What this flow is for, who owns it, or what it should do."
                className="resize-none text-sm"
              />
            </div>
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
                    onValueChange={(value) => updateDraft((current) => {
                      const repo = repositories.find((entry) => entry.full_name === value);
                      return {
                        ...current,
                        repoFullName: value === '__custom__' ? '' : value,
                        targetMode: repo && requiresExplicitAgentTarget(current.triggerType) ? 'repository' : current.targetMode,
                        targetId: repo && requiresExplicitAgentTarget(current.triggerType) ? repo.id : current.targetId,
                      };
                    })}
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
                            <Select value={draft.scheduleMinute.padStart(2, '0')} onValueChange={(value) => updateDraft((current) => ({ ...current, scheduleMinute: String(Number.parseInt(value, 10)) }))}>
                              <SelectTrigger size="sm" className="w-24">
                                <SelectValue />
                              </SelectTrigger>
                              <SelectContent>
                                {SCHEDULE_MINUTE_OPTIONS.map((option) => (
                                  <SelectItem key={option.value} value={option.value}>{option.label}</SelectItem>
                                ))}
                              </SelectContent>
                            </Select>
                          </>
                        ) : (
                          <>
                            <PillGlue>at</PillGlue>
                            <Select value={draft.scheduleTime.slice(0, 2)} onValueChange={(hour) => updateDraft((current) => ({ ...current, scheduleTime: `${hour}:${normalizeScheduleTime(current.scheduleTime).slice(3, 5)}` }))}>
                              <SelectTrigger size="sm" className="w-24">
                                <SelectValue />
                              </SelectTrigger>
                              <SelectContent>
                                {SCHEDULE_HOUR_OPTIONS.map((option) => (
                                  <SelectItem key={option.value} value={option.value}>{option.label}</SelectItem>
                                ))}
                              </SelectContent>
                            </Select>
                            <PillGlue>:</PillGlue>
                            <Select value={draft.scheduleTime.slice(3, 5)} onValueChange={(minute) => updateDraft((current) => ({ ...current, scheduleTime: `${normalizeScheduleTime(current.scheduleTime).slice(0, 2)}:${minute}` }))}>
                              <SelectTrigger size="sm" className="w-24">
                                <SelectValue />
                              </SelectTrigger>
                              <SelectContent>
                                {SCHEDULE_MINUTE_OPTIONS.map((option) => (
                                  <SelectItem key={option.value} value={option.value}>{option.label}</SelectItem>
                                ))}
                              </SelectContent>
                            </Select>
                            <PillGlue>{timezone}</PillGlue>
                          </>
                        )}

                        {draft.scheduleFrequency === 'monthly' && (
                          <>
                            <PillGlue>on day</PillGlue>
                            <Select value={draft.scheduleDayOfMonth} onValueChange={(dayOfMonth) => updateDraft((current) => ({ ...current, scheduleDayOfMonth: dayOfMonth }))}>
                              <SelectTrigger size="sm" className="w-24">
                                <SelectValue />
                              </SelectTrigger>
                              <SelectContent>
                                {SCHEDULE_MONTH_DAY_OPTIONS.map((option) => (
                                  <SelectItem key={option.value} value={option.value}>{option.label}</SelectItem>
                                ))}
                              </SelectContent>
                            </Select>
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
                          {simpleSchedulePreview ? describeSimpleSchedule(simpleSchedulePreview).replace('UTC', timezone) : 'Choose a supported schedule'}
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
                      {availableAgents.length === 0 ? (
                        <SelectItem value="__none_available__" disabled>
                          No agents available for this target
                        </SelectItem>
                      ) : null}
                      {agentOptions.map(({ agent, disabledReason }) => (
                        <SelectItem
                          key={agent.id}
                          value={agent.id}
                          disabled={!!disabledReason}
                          className={disabledReason ? 'data-disabled:pointer-events-auto' : undefined}
                        >
                          {disabledReason ? (
                            <Tooltip>
                              <TooltipTrigger asChild>
                                <span className="block max-w-64 truncate" title={agent.name}>{agent.name}</span>
                              </TooltipTrigger>
                              <TooltipContent side="right" className="max-w-56 text-xs">
                                {disabledReason}
                              </TooltipContent>
                            </Tooltip>
                          ) : (
                            <span className="block max-w-64 truncate" title={agent.name}>{agent.name}</span>
                          )}
                        </SelectItem>
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
                    <SelectGroup>
                      <SelectLabel>CRM</SelectLabel>
                      {CRM_AGENT_TARGET_OPTIONS.map((target) => (
                        <SelectItem key={target.value} value={target.value}>{TARGET_SHORT_LABELS[target.value]}</SelectItem>
                      ))}
                    </SelectGroup>
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
                {isCRMRecordTarget(draft.targetMode) && (
                  <CRMRecordPicker
                    workspaceId={workspaceId}
                    targetType={draft.targetMode}
                    value={draft.targetId}
                    onChange={(value) => updateDraft((current) => ({ ...current, targetId: value }))}
                    disabled={!canEdit || saving}
                  />
                )}
                {draft.targetMode !== 'event' && draft.targetMode !== 'workspace' && !isCRMRecordTarget(draft.targetMode) && (
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

          <FlowSummaryParagraph rows={flowLogicRows} />
        </div>

        <DialogFooter className="gap-2 sm:justify-between">
          <div>
            {mode === 'create' && onBack && (
              <Button type="button" variant="ghost" size="sm" className="text-muted-foreground hover:text-foreground" onClick={onBack}>
                ← Back to templates
              </Button>
            )}
          </div>
          <div className="flex gap-2">
            <Button type="button" variant="ghost" onClick={() => onOpenChange(false)}>Cancel</Button>
            <TooltipIfDisabled message={saving ? null : (!canEdit ? 'You do not have permission to save flows.' : validation)}>
              <Button type="button" onClick={() => void onSave()} disabled={saving || !canEdit || !!validation}>
                {saving ? 'Saving…' : mode === 'create' ? 'Create flow' : 'Save changes'}
              </Button>
            </TooltipIfDisabled>
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
  templates,
  loading,
  error,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onPick: (template: FlowTemplateManifest | null) => void;
  templates: FlowTemplateManifest[];
  loading: boolean;
  error: string | null;
}) {
  const [category, setCategory] = useState('all');
  const [search, setSearch] = useState('');
  const trimmedSearch = search.trim();
  const categories = useMemo(() => {
    const keys = new Set<string>();
    for (const template of templates) {
      for (const item of template.categories ?? []) keys.add(item);
    }
    return Array.from(keys).sort((a, b) => (TEMPLATE_CATEGORY_LABELS[a] ?? a).localeCompare(TEMPLATE_CATEGORY_LABELS[b] ?? b));
  }, [templates]);
  const visibleTemplates = useMemo(
    () => templates
      .filter((template) => category === 'all' || template.categories?.includes(category))
      .filter((template) => templateMatchesSearch(template, trimmedSearch))
      .slice()
      .sort(compareTemplatesForDisplay),
    [category, templates, trimmedSearch],
  );

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="flex max-h-[86vh] flex-col gap-4 p-6 sm:max-w-[720px]">
        <DialogHeader>
          <DialogTitle>New flow</DialogTitle>
          <DialogDescription>Pick a starting point, or build one from an empty trigger.</DialogDescription>
        </DialogHeader>

        <QuietSearchInput
          aria-label="Search flow templates"
          value={search}
          onChange={(event) => setSearch(event.target.value)}
          placeholder="Search templates..."
        />

        <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <Tabs value={category} onValueChange={setCategory}>
            <TabsList variant="line" className="max-w-full flex-wrap justify-start">
            {['all', ...categories].map((item) => {
              return (
                <TabsTrigger
                  key={item}
                  value={item}
                >
                  {item === 'all' ? 'All' : TEMPLATE_CATEGORY_LABELS[item] ?? item}
                </TabsTrigger>
              );
            })}
            </TabsList>
          </Tabs>
        </div>

        <div className="grid h-[52vh] min-h-[360px] auto-rows-min content-start gap-2.5 overflow-y-auto pr-1 sm:grid-cols-2">
          {loading && Array.from({ length: 6 }).map((_, index) => (
            <div key={index} className="rounded-lg border border-border/60 p-4">
              <Skeleton className="h-8 w-8 rounded-lg" />
              <Skeleton className="mt-3 h-4 w-28" />
              <Skeleton className="mt-2 h-10 w-full" />
            </div>
          ))}
          {!loading && error && (
            <div className="rounded-lg border border-destructive/30 bg-destructive/[0.03] p-4 text-sm sm:col-span-2">
              <p className="font-medium text-destructive">Templates could not be loaded</p>
              <p className="mt-1 text-muted-foreground">{error}</p>
            </div>
          )}
          {!loading && !error && visibleTemplates.length === 0 && (
            <div className="rounded-lg border border-border/60 bg-muted/20 p-4 text-sm sm:col-span-2">
              <p className="font-medium">No templates available</p>
              <p className="mt-1 text-muted-foreground">
                {trimmedSearch ? 'No templates match this search.' : category === 'all' ? 'No flow templates are available in this workspace yet.' : 'No templates match this category.'}
              </p>
            </div>
          )}
          {!loading && !error && visibleTemplates.map((template) => {
            const kind = template.trigger.type === 'cron'
              ? 'Schedule'
              : (template.trigger.event?.split('.')[0] || templatePrimaryCategory(template)).replaceAll('_', ' ');
            return (
              <button
                key={template.key}
                type="button"
                onClick={() => onPick(template)}
                className={cn(
                  'group relative flex min-h-[112px] flex-col items-start gap-2 overflow-hidden rounded-lg border border-border bg-popover p-[14px] text-left transition-colors duration-100 hover:bg-muted/50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
                )}
              >
                <span className="text-[10px] font-medium uppercase tracking-wide text-muted-foreground">{kind}</span>
                <p className="text-sm font-medium leading-tight">{template.name}</p>
                <p className="text-pretty text-xs leading-relaxed text-muted-foreground">{template.short_description}</p>
              </button>
            );
          })}
        </div>
        <Button variant="outline" className="w-full border-dashed" onClick={() => onPick(null)}>Build a custom flow</Button>
      </DialogContent>
    </Dialog>
  );
}

function FlowTemplateInstallDialog({
  open,
  onOpenChange,
  template,
  values,
  flowSetup,
  agentSetup,
  additionalInstructions,
  agents,
  workflows,
  repositories,
  spaces,
  collections,
  teams,
  toolCatalog,
  skillCatalog,
  timezone,
  saving,
  canEdit,
  onBack,
  onValueChange,
  onFlowSetupChange,
  onAgentSetupChange,
  onAgentInstructionsEdited,
  onAdditionalInstructionsChange,
  onInstall,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  template: FlowTemplateManifest | null;
  values: Record<string, unknown>;
  flowSetup: TemplateFlowSetup | null;
  agentSetup: TemplateAgentSetup | null;
  additionalInstructions: string;
  agents: Agent[];
  workflows: WorkflowWithStates[];
  repositories: GitRepository[];
  spaces: DocsSpace[];
  collections: DocsCollection[];
  teams: WorkspaceTeam[];
  toolCatalog: ToolCatalogEntry[];
  skillCatalog: SkillCatalogEntry[];
  timezone: string;
  saving: boolean;
  canEdit: boolean;
  onBack: () => void;
  onValueChange: (key: string, value: unknown) => void;
  onFlowSetupChange: (setup: TemplateFlowSetup | null) => void;
  onAgentSetupChange: (setup: TemplateAgentSetup | null) => void;
  onAgentInstructionsEdited: () => void;
  onAdditionalInstructionsChange: (value: string) => void;
  onInstall: () => void;
}) {
  const [step, setStep] = useState<FlowTemplateInstallStep>('inputs');
  const [toolPickerOpen, setToolPickerOpen] = useState(false);
  const [skillPickerOpen, setSkillPickerOpen] = useState(false);

  useEffect(() => {
    if (open) setStep('inputs');
  }, [open, template?.key]);

  const inputValidation = useMemo(() => {
    if (!template) return null;
    for (const input of template.inputs) {
      if (!isTemplateInputVisible(input, values)) continue;
      if (input.required && !String(values[input.key] ?? '').trim()) {
        return `${input.label} is required`;
      }
    }
    return null;
  }, [template, values]);

  const reviewValidation = useMemo(() => {
    if (!template) return null;
    if (!flowSetup?.name.trim()) return 'Flow name is required';
    if (template.agent.create) {
      if (!agentSetup?.name.trim()) return 'Agent name is required';
      if (!agentSetup.allowed_targets.length) return 'Choose at least one place this agent can run';
    }
    return null;
  }, [agentSetup, flowSetup, template]);
  const validation = step === 'inputs' ? inputValidation : reviewValidation;

  if (!template) return null;
  const needsReviewStep = templateNeedsReviewStep(template);
  const reviewInputs = { ...values, additional_instructions: additionalInstructions.trim() };
  const runContext = defaultTemplateRunContext(template, reviewInputs);
  const inputGroups = groupTemplateInputs(template.inputs, values);
  const permissionDisabledMessage = 'You do not have permission to install flow templates.';
  const reviewSetupDisabledMessage = !canEdit ? permissionDisabledMessage : inputValidation;
  const installDisabledMessage = !canEdit ? permissionDisabledMessage : validation;

  const setAgentSetup = (updates: Partial<TemplateAgentSetup>) => {
    if (!agentSetup) return;
    onAgentSetupChange({ ...agentSetup, ...updates });
  };

  const setFlowSetup = (updates: Partial<TemplateFlowSetup>) => {
    if (!flowSetup) return;
    onFlowSetupChange({ ...flowSetup, ...updates });
  };

  const toggleTarget = (target: AgentTargetType) => {
    if (!agentSetup) return;
    const hasTarget = agentSetup.allowed_targets.includes(target);
    if (hasTarget && agentSetup.allowed_targets.length === 1) return;
    setAgentSetup({
      allowed_targets: hasTarget
        ? agentSetup.allowed_targets.filter((value) => value !== target)
        : normalizeTargetList([...agentSetup.allowed_targets, target]),
    });
  };

  const toggleTool = (toolName: string) => {
    if (!agentSetup) return;
    const selected = agentSetup.allowed_tools.includes(toolName);
    setAgentSetup({
      allowed_tools: selected
        ? agentSetup.allowed_tools.filter((tool) => tool !== toolName)
        : normalizeToolList([...agentSetup.allowed_tools, toolName]),
    });
  };

  const removeTool = (toolName: string) => {
    if (!agentSetup) return;
    setAgentSetup({ allowed_tools: agentSetup.allowed_tools.filter((tool) => tool !== toolName) });
  };

  const addSkill = (entry: SkillCatalogEntry) => {
    if (!agentSetup) return;
    if (agentSetup.skills.some((skill) => skillRefIdentity(skill) === skillRefIdentity(entry))) return;
    const ref: AgentSkillRef = { key: entry.key };
    if (entry.id) ref.skill_id = entry.id;
    const missingTools = (entry.required_tools ?? []).filter((tool) => !agentSetup.allowed_tools.includes(tool));
    if (missingTools.length > 0) {
      toast.warning(`Skill "${entry.key}" requires tools not yet allowed: ${missingTools.join(', ')}`);
    }
    setAgentSetup({ skills: [...agentSetup.skills, ref] });
    setSkillPickerOpen(false);
  };

  const removeSkill = (identity: string) => {
    if (!agentSetup) return;
    setAgentSetup({ skills: agentSetup.skills.filter((skill) => skillRefIdentity(skill) !== identity) });
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="grid max-h-[88vh] grid-rows-[auto_minmax(0,1fr)_auto] gap-0 overflow-hidden p-0 sm:max-w-3xl">
        <DialogHeader className="border-b border-border/60 px-6 py-5 text-left">
          <DialogTitle>{template.name}</DialogTitle>
          <DialogDescription>{template.short_description}</DialogDescription>
        </DialogHeader>

        <div className="min-h-0 space-y-6 overflow-y-auto px-6 pb-16 pt-5">
          {step === 'inputs' ? (
            <div className="space-y-6">
              {inputGroups.map((group, index) => (
                <section key={group.title} className={cn('space-y-3', index > 0 && 'pt-1')}>
                  {group.title !== 'Template inputs' && !isFollowUpTaskSection(group.title) && (
                    <h3 className="text-[0.8rem] font-semibold leading-none text-muted-foreground">
                      {group.title}
                    </h3>
                  )}
                  <div className="grid gap-x-4 gap-y-5 sm:grid-cols-2">
                    {group.inputs.map((input) => (
                      <TemplateInputControl
                        key={input.key}
                        input={input}
                        template={template}
                        hideLabel={input.type === 'cron' && group.inputs.length === 1 && group.title !== 'Template inputs'}
                        compactBool={isFollowUpTaskSection(group.title) && input.key === 'create_follow_up_tasks'}
                        value={values[input.key]}
                        values={values}
                        agents={agents}
                        workflows={workflows}
                        repositories={repositories}
                        spaces={spaces}
                        collections={collections}
                        teams={teams}
                        timezone={timezone}
                        onChange={(value) => onValueChange(input.key, value)}
                      />
                    ))}
                  </div>
                </section>
              ))}
            </div>
          ) : (
            <div className="space-y-5">
              {flowSetup && (
                <section className="space-y-3">
                  <div>
                    <h3 className="text-sm font-medium">Flow details</h3>
                    <p className="text-xs text-muted-foreground">
                      This is how the installed flow will appear in the automation list.
                    </p>
                  </div>
                  <div className="space-y-1.5">
                    <Label htmlFor="template-flow-name">Flow name</Label>
                    <Input
                      id="template-flow-name"
                      className="rounded-lg border-border bg-background"
                      value={flowSetup.name}
                      onChange={(event) => setFlowSetup({ name: event.target.value })}
                    />
                  </div>
                  <div className="space-y-1.5">
                    <Label htmlFor="template-flow-description">Internal description</Label>
                    <Textarea
                      id="template-flow-description"
                      value={flowSetup.description}
                      rows={3}
                      placeholder="What this flow is for, who owns it, or what it should do."
                      className="min-h-24 resize-y rounded-lg border-border bg-background"
                      onChange={(event) => setFlowSetup({ description: event.target.value })}
                    />
                  </div>
                </section>
              )}
              <TemplateLogicSummary
                template={template}
                values={values}
                context={{
                  agents,
                  workflows,
                  repositories,
                  spaces,
                  collections,
                  teams,
                  timezone,
                }}
              />
            </div>
          )}

          {step === 'review' && agentSetup && template.agent.create && (
            <section className="space-y-3 border-t border-border/60 pt-5">
              <div>
                <h3 className="text-sm font-medium">Agent setup</h3>
                <p className="text-xs text-muted-foreground">
                  Review the agent this template will create. The instructions are drafted from the template inputs and can be edited before install.
                </p>
              </div>
              <div className="grid gap-4 sm:grid-cols-[1fr_220px]">
                <div className="space-y-1.5">
                  <Label htmlFor="template-agent-name">Agent name</Label>
                  <Input id="template-agent-name" className="rounded-lg border-border bg-background" value={agentSetup.name} onChange={(event) => setAgentSetup({ name: event.target.value })} />
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="template-agent-approval">Run approval</Label>
                  <Select value={agentSetup.approval_mode} onValueChange={(value) => setAgentSetup({ approval_mode: value as AgentApprovalMode })}>
                    <SelectTrigger id="template-agent-approval" className="w-full rounded-lg border-border bg-background">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {AGENT_APPROVAL_OPTIONS.map((option) => (
                        <SelectItem key={option.value} value={option.value}>
                          {option.label}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  <p className="text-xs leading-relaxed text-muted-foreground">
                    {agentApprovalDescription(agentSetup.approval_mode)}
                  </p>
                </div>
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="template-agent-instructions">Instructions</Label>
                <Textarea
                  id="template-agent-instructions"
                  value={agentSetup.system_prompt}
                  rows={8}
                  className="min-h-44 resize-y rounded-lg border-border bg-background font-mono text-xs leading-relaxed"
                  onChange={(event) => {
                    onAgentInstructionsEdited();
                    setAgentSetup({ system_prompt: event.target.value });
                  }}
                />
              </div>
              <details open className="rounded-lg border border-border/60 bg-muted/10 p-3">
                <summary className="cursor-pointer text-sm font-medium">Capabilities</summary>
                <div className="mt-3 space-y-4">
                  <div className="space-y-2">
                    <p className="text-sm font-medium text-foreground">Working areas</p>
                    <div className="flex flex-wrap gap-2">
                      {TEMPLATE_TARGET_OPTIONS.map((target) => (
                        <button
                          key={target.value}
                          type="button"
                          onClick={() => toggleTarget(target.value)}
                          className={cn(
                            'rounded-md border px-2.5 py-1 text-xs transition-colors',
                            agentSetup.allowed_targets.includes(target.value)
                              ? 'border-primary/40 bg-primary/10 text-primary'
                              : 'border-border bg-background text-muted-foreground hover:bg-muted/50',
                          )}
                        >
                          {target.label}
                        </button>
                      ))}
                    </div>
                  </div>
                  <div className="space-y-2">
                    <div className="flex items-center justify-between gap-2">
                      <p className="text-sm font-medium text-foreground">Tools</p>
                      <ToolMultiSelectPopover
                        open={toolPickerOpen}
                        onOpenChange={setToolPickerOpen}
                        tools={toolCatalog}
                        selectedTools={agentSetup.allowed_tools}
                        onToggleTool={toggleTool}
                      />
                    </div>
                    <div className="flex flex-wrap gap-2">
                      {agentSetup.allowed_tools.length > 0 ? (
                        agentSetup.allowed_tools.map((tool) => (
                          <Badge key={tool} variant="secondary" className="gap-1 pr-1 font-mono text-[11px]">
                            <SourceCodeIcon className="h-3 w-3 text-muted-foreground" />
                            {tool}
                            <button
                              type="button"
                              className="rounded-sm p-0.5 text-muted-foreground transition-colors hover:bg-background hover:text-foreground"
                              onClick={() => removeTool(tool)}
                              aria-label={`Remove ${tool}`}
                            >
                              <Cancel01Icon className="h-3 w-3" />
                            </button>
                          </Badge>
                        ))
                      ) : (
                        <p className="text-xs text-muted-foreground">No tools selected.</p>
                      )}
                    </div>
                  </div>
                  <div className="space-y-2">
                    <div className="flex items-center justify-between gap-2">
                      <p className="text-sm font-medium text-foreground">Skills</p>
                      <TemplateSkillPicker
                        open={skillPickerOpen}
                        onOpenChange={setSkillPickerOpen}
                        skills={skillCatalog}
                        selectedSkills={agentSetup.skills}
                        onAddSkill={addSkill}
                      />
                    </div>
                    <div className="flex flex-wrap gap-2">
                      {agentSetup.skills.length > 0 ? (
                        agentSetup.skills.map((ref) => {
                          const entry = skillCatalog.find((skill) => skillRefIdentity(skill) === skillRefIdentity(ref));
                          return (
                            <Badge key={skillRefIdentity(ref)} variant="secondary" className="gap-1 pr-1 text-[11px]">
                              <BookOpen01Icon className="h-3 w-3 text-muted-foreground" />
                              <span>{skillRefDisplayName(entry, ref.key)}</span>
                              <span className="font-mono text-[9px] text-muted-foreground/70">{ref.key}</span>
                              <button
                                type="button"
                                className="rounded-sm p-0.5 text-muted-foreground transition-colors hover:bg-background hover:text-foreground"
                                onClick={() => removeSkill(skillRefIdentity(ref))}
                                aria-label={`Remove ${skillRefDisplayName(entry, ref.key)}`}
                              >
                                <Cancel01Icon className="h-3 w-3" />
                              </button>
                            </Badge>
                          );
                        })
                      ) : (
                        <p className="text-xs text-muted-foreground">No skills selected.</p>
                      )}
                    </div>
                  </div>
                </div>
              </details>
            </section>
          )}

          {step === 'review' && template.agent.reuse_system && !template.agent.create && (
            <section className="space-y-4 border-t border-border/60 pt-5">
              <div>
                <h3 className="text-sm font-medium">Run instructions</h3>
                <p className="text-xs text-muted-foreground">
                  This template reuses Quill. Review the context that will be sent when the schedule starts a run.
                </p>
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="template-prompt-context">Prompt context</Label>
                <Textarea
                  id="template-prompt-context"
                  value={runContext}
                  readOnly
                  rows={8}
                  className="min-h-44 resize-y rounded-lg border-border bg-muted/20 font-mono text-xs leading-relaxed"
                />
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="template-additional-instructions">Additional instructions</Label>
                <Textarea
                  id="template-additional-instructions"
                  value={additionalInstructions}
                  rows={3}
                  placeholder="Optional. Add anything Quill should pay special attention to in this sweep."
                  className="min-h-24 resize-y rounded-lg border-border bg-background"
                  onChange={(event) => onAdditionalInstructionsChange(event.target.value)}
                />
              </div>
            </section>
          )}

        </div>

        <DialogFooter className="shrink-0 border-t border-border/60 px-6 py-4 gap-2 sm:justify-between">
          <Button
            type="button"
            variant="ghost"
            size="sm"
            className="text-muted-foreground hover:text-foreground"
            onClick={step === 'inputs' ? onBack : () => setStep('inputs')}
          >
            {step === 'inputs' ? '← Back to templates' : '← Back to inputs'}
          </Button>
          <div className="flex gap-2">
            <Button type="button" variant="ghost" onClick={() => onOpenChange(false)}>Cancel</Button>
            {step === 'inputs' && needsReviewStep ? (
              <TooltipIfDisabled message={reviewSetupDisabledMessage}>
                <Button type="button" onClick={() => setStep('review')} disabled={!canEdit || !!inputValidation}>
                  Review setup
                </Button>
              </TooltipIfDisabled>
            ) : (
              <TooltipIfDisabled message={saving ? null : installDisabledMessage}>
                <Button type="button" onClick={onInstall} disabled={saving || !canEdit || !!validation}>
                  {saving ? 'Installing…' : 'Install'}
                </Button>
              </TooltipIfDisabled>
            )}
          </div>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function DeleteFlowDialog({
  rule,
  open,
  onOpenChange,
  agentReferencedElsewhere,
  deleteCreatedAgent,
  onDeleteCreatedAgentChange,
  saving,
  onConfirm,
}: {
  rule: AutomationRule | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  agentReferencedElsewhere: boolean;
  deleteCreatedAgent: boolean;
  onDeleteCreatedAgentChange: (value: boolean) => void;
  saving: boolean;
  onConfirm: () => void;
}) {
  if (!rule) return null;
  const hasTemplateCreatedAgent = !!rule.template_instance_id && !!rule.action_config?.agent_id;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Delete flow?</DialogTitle>
          <DialogDescription>
            <span className="font-medium text-foreground">{rule.name}</span> will be removed. This cannot be undone.
          </DialogDescription>
        </DialogHeader>

        {hasTemplateCreatedAgent && (
          <div className="space-y-3 rounded-lg border border-border/60 p-3 text-sm">
            {agentReferencedElsewhere ? (
              <p className="text-muted-foreground">The agent created with this flow is used by another flow, so it will be kept.</p>
            ) : (
              <>
                <label className="flex items-start gap-3">
                  <Checkbox checked={!deleteCreatedAgent} onCheckedChange={() => onDeleteCreatedAgentChange(false)} />
                  <span>
                    <span className="block font-medium">Delete flow only</span>
                    <span className="text-muted-foreground">Keep the agent as a regular custom agent.</span>
                  </span>
                </label>
                <label className="flex items-start gap-3">
                  <Checkbox checked={deleteCreatedAgent} onCheckedChange={() => onDeleteCreatedAgentChange(true)} />
                  <span>
                    <span className="block font-medium">Delete the agent too</span>
                    <span className="text-muted-foreground">Use this only when the agent was created just for this flow.</span>
                  </span>
                </label>
              </>
            )}
          </div>
        )}

        <DialogFooter>
          <Button type="button" variant="ghost" onClick={() => onOpenChange(false)}>Cancel</Button>
          <Button type="button" variant="destructive" onClick={onConfirm} disabled={saving}>
            {saving ? 'Deleting…' : 'Delete flow'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function TemplateInputControl({
  input,
  template,
  value,
  values,
  agents,
  workflows,
  repositories,
  spaces,
  collections,
  teams,
  timezone,
  hideLabel = false,
  compactBool = false,
  onChange,
}: {
  input: FlowTemplateInput;
  template: FlowTemplateManifest;
  value: unknown;
  values: Record<string, unknown>;
  agents: Agent[];
  workflows: WorkflowWithStates[];
  repositories: GitRepository[];
  spaces: DocsSpace[];
  collections: DocsCollection[];
  teams: WorkspaceTeam[];
  timezone: string;
  hideLabel?: boolean;
  compactBool?: boolean;
  onChange: (value: unknown) => void;
}) {
  const label = <TemplateInputLabel input={input} />;
  const selectedWorkflowId = stringValue(values.workflow_id);
  const dependentValue = input.depends_on ? stringValue(values[input.depends_on]) : '';
  const workflowStates = (() => {
    if (input.type !== 'workflow_state') return [];
    if (input.depends_on === 'workflow_id') {
      return dependentValue ? workflows.find((workflow) => workflow.workflow.id === dependentValue)?.states ?? [] : [];
    }
    if (input.depends_on && input.depends_on.includes('team')) {
      return dependentValue
        ? workflows.filter((workflow) => workflow.workflow.team_id === dependentValue).flatMap((workflow) => workflow.states)
        : [];
    }
    return selectedWorkflowId
      ? workflows.find((workflow) => workflow.workflow.id === selectedWorkflowId)?.states ?? []
      : workflows.flatMap((workflow) => workflow.states);
  })();
  const collectionSpaceId = input.type === 'collection' && input.depends_on ? dependentValue : '';
  const spaceNames = useMemo(() => new Map(spaces.map((space) => [space.id, space.name])), [spaces]);

  if (input.type === 'cron') {
    return (
      <div className="space-y-1.5 sm:col-span-2">
        {!hideLabel && label}
        <TemplateScheduleInput value={stringValue(value) || '0 9 * * 1'} timezone={timezone} onChange={onChange} />
      </div>
    );
  }

  if (input.type === 'bool') {
    return (
      <label className={cn(
        'flex items-start gap-3 rounded-lg border border-border/60 px-3 text-sm transition-colors sm:col-span-2',
        compactBool ? 'bg-background py-2.5 hover:bg-muted/10' : 'bg-muted/10 py-2.5 hover:bg-muted/20',
      )}>
        <Checkbox className="mt-0.5" checked={value === true} onCheckedChange={(checked) => onChange(checked === true)} />
        <span className="min-w-0">
          <span className="flex items-center gap-1.5 font-medium text-foreground">
            {input.label}
            {input.help_text?.trim() && (
              <Tooltip>
                <TooltipTrigger asChild>
                  <button
                    type="button"
                    tabIndex={-1}
                    aria-label={`${input.label} help`}
                    className="inline-flex text-muted-foreground/60 transition-colors hover:text-muted-foreground"
                  >
                    <HelpCircleIcon className="h-3.5 w-3.5" />
                  </button>
                </TooltipTrigger>
                <TooltipContent side="right" className="max-w-64 text-xs leading-relaxed">
                  {input.help_text}
                </TooltipContent>
              </Tooltip>
            )}
          </span>
        </span>
      </label>
    );
  }

  if (input.type === 'string_list') {
    return <TemplateStringListInput input={input} value={value} onChange={onChange} />;
  }

  if (input.type === 'multi_select') {
    const selected = new Set(Array.isArray(value) ? value.map(String) : []);
    const options = input.options ?? [];
    return (
      <div className="space-y-1.5 sm:col-span-2">
        {label}
        <div className="flex flex-wrap gap-2 rounded-lg border border-border/60 bg-background p-2">
          {options.map((option) => (
            <button
              key={option.value}
              type="button"
              onClick={() => {
                const next = new Set(selected);
                if (next.has(option.value)) next.delete(option.value);
                else next.add(option.value);
                onChange(Array.from(next));
              }}
              className={cn(
                'rounded-md border px-2.5 py-1 text-xs transition-colors',
                selected.has(option.value)
                  ? 'border-primary/40 bg-primary/10 text-primary'
                  : 'border-border bg-background text-muted-foreground hover:bg-muted/50',
              )}
            >
              {option.label}
            </button>
          ))}
        </div>
      </div>
    );
  }

  const selectOptions: TemplateSelectOption[] | null = (() => {
    switch (input.type) {
      case 'agent':
        return agents
          .filter((agent) => {
            const targets = template.agent.pick_existing?.constraints?.targets ?? [];
            if (targets.length > 0 && !targets.some((target) => agent.allowed_targets?.includes(target))) return false;
            const presets = template.agent.pick_existing?.constraints?.presets ?? [];
            if (presets.length > 0 && (!agent.preset_key || !presets.includes(agent.preset_key))) return false;
            return true;
          })
          .map((agent) => ({ value: agent.id, label: agent.name }));
      case 'repository':
        return templateSelectOptions(input, repositories);
      case 'space':
        return spaces.filter((space) => templateInputAllowsSpace(input, space)).map((space) => ({
          value: space.id,
          label: space.name,
        }));
      case 'collection':
        return collections
          .filter((collection) => collectionSpaceId && collection.space_id === collectionSpaceId)
          .map((collection) => ({
            value: collection.id,
            label: collection.name,
            description: spaceNames.get(collection.space_id) ?? 'Docs collection',
            indent: collection.depth,
          }));
      case 'team':
        return teams.map((team) => ({ value: team.id, label: team.name }));
      case 'workflow':
        return workflows.map((workflow) => ({ value: workflow.workflow.id, label: workflow.workflow.name }));
      case 'workflow_state':
        return workflowStates.map((state) => ({ value: state.id, label: state.name }));
      default:
        if (input.options?.length) {
          return input.options;
        }
        if (input.type.startsWith('enum<') && input.type.endsWith('>')) {
          return input.type.slice(5, -1).split(',').map((option) => ({ value: option.trim(), label: option.trim() }));
        }
        return null;
    }
  })();

  if (selectOptions) {
    const selectValue = templateSelectValue(input, value);
    const selectedOption = selectOptions.find((option) => option.value === selectValue);
    const richDestinationSelect = input.type === 'space' || input.type === 'collection';
    const disabledUntilDependencySelected = (input.type === 'collection' || input.type === 'workflow_state') && Boolean(input.depends_on) && !dependentValue;
    const dependencyPlaceholder = input.type === 'collection'
      ? 'Select a docs space first'
      : input.type === 'workflow_state'
        ? input.depends_on === 'workflow_id' ? 'Select a workflow first' : 'Select a team first'
        : `Select ${input.label.toLowerCase()}`;
    return (
      <div className="space-y-1.5">
        {label}
        <Select value={selectValue} onValueChange={(nextValue) => onChange(templateSelectChangeValue(input, nextValue))} disabled={disabledUntilDependencySelected}>
          <SelectTrigger className="w-full justify-between rounded-lg border-border bg-background">
            <SelectValue placeholder={disabledUntilDependencySelected ? dependencyPlaceholder : `Select ${input.label.toLowerCase()}`}>
              {selectedOption?.label}
            </SelectValue>
          </SelectTrigger>
          <SelectContent className={cn(richDestinationSelect && 'min-w-[20rem]')}>
            <SelectGroup>
              {richDestinationSelect && (
                <SelectLabel>{input.type === 'space' ? templateSpaceSelectLabel(input) : 'Select collection'}</SelectLabel>
              )}
              {selectOptions.map((option) => (
                <SelectItem key={option.value} value={option.value} textValue={option.label}>
                  {richDestinationSelect || option.description ? (
                    <span className="flex min-w-0 flex-col items-start gap-0.5 py-0.5" style={{ paddingLeft: `${(option.indent ?? 0) * 12}px` }}>
                      <span className="max-w-[18rem] truncate text-sm font-medium">{option.label}</span>
                      {option.description && <span className="max-w-[18rem] truncate text-xs text-muted-foreground">{option.description}</span>}
                    </span>
                  ) : (
                    option.label
                  )}
                </SelectItem>
              ))}
              {selectOptions.length === 0 && (
                <SelectItem value="__empty__" disabled>
                  {input.type === 'collection'
                    ? (collectionSpaceId ? 'No collections in this space' : 'Select a docs space first')
                    : input.type === 'workflow_state'
                      ? input.depends_on === 'workflow_id'
                        ? (dependentValue ? 'No stages for this workflow' : 'Select a workflow first')
                        : (dependentValue ? 'No stages for this team' : 'Select a team first')
                    : 'No options available'}
                </SelectItem>
              )}
            </SelectGroup>
          </SelectContent>
        </Select>
      </div>
    );
  }

  return (
    <div className="space-y-1.5">
      {label}
      <Input
        className="rounded-lg border-border bg-background"
        type={input.type === 'int' ? 'number' : 'text'}
        value={String(value ?? '')}
        placeholder={input.placeholder || (input.type === 'cron' ? '0 9 * * 1' : undefined)}
        onChange={(event) => onChange(input.type === 'int' ? Number(event.target.value) : event.target.value)}
      />
    </div>
  );
}

function TemplateScheduleInput({
  value,
  timezone,
  onChange,
}: {
  value: string;
  timezone: string;
  onChange: (value: string) => void;
}) {
  const resolvedTimeZone = normalizeTimeZone(timezone);
  const parsedUTC = parseSimpleScheduleExpression(value) ?? {
    frequency: 'weekly',
    minute: '0',
    time: '09:00',
    weekdays: [1],
    dayOfMonth: '1',
  };
  const parsed = utcScheduleToLocal(parsedUTC, resolvedTimeZone);
  const update = (updates: Partial<ParsedSimpleSchedule>) => {
    const next = { ...parsed, ...updates };
    onChange(scheduleExpressionFromParsed(localScheduleToUTC(next, resolvedTimeZone)));
  };
  const [hour, minute] = normalizeScheduleTime(parsed.time).split(':');
  return (
    <div className="space-y-3 rounded-lg border border-border/60 p-3">
      <div className="grid gap-3 sm:grid-cols-[160px_1fr]">
        <Select value={parsed.frequency} onValueChange={(frequency) => update({ frequency: frequency as ParsedSimpleSchedule['frequency'] })}>
          <SelectTrigger>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="hourly">Hourly</SelectItem>
            <SelectItem value="daily">Daily</SelectItem>
            <SelectItem value="weekly">Weekly</SelectItem>
            <SelectItem value="monthly">Monthly</SelectItem>
          </SelectContent>
        </Select>
        {parsed.frequency === 'hourly' ? (
          <div className="flex items-center gap-2">
            <span className="text-sm text-muted-foreground">At minute</span>
            <Select value={parsed.minute.padStart(2, '0')} onValueChange={(nextMinute) => update({ minute: String(Number.parseInt(nextMinute, 10)) })}>
              <SelectTrigger className="w-24">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {SCHEDULE_MINUTE_OPTIONS.map((option) => (
                  <SelectItem key={option.value} value={option.value}>{option.label}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        ) : (
          <div className="flex flex-wrap items-center gap-2">
            <span className="text-sm text-muted-foreground">At</span>
            <Select value={hour} onValueChange={(nextHour) => update({ time: `${nextHour}:${minute}` })}>
              <SelectTrigger className="w-24">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {SCHEDULE_HOUR_OPTIONS.map((option) => (
                  <SelectItem key={option.value} value={option.value}>{option.label}</SelectItem>
                ))}
              </SelectContent>
            </Select>
            <span className="text-sm text-muted-foreground">:</span>
            <Select value={minute} onValueChange={(nextMinute) => update({ time: `${hour}:${nextMinute}` })}>
              <SelectTrigger className="w-24">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {SCHEDULE_MINUTE_OPTIONS.map((option) => (
                  <SelectItem key={option.value} value={option.value}>{option.label}</SelectItem>
                ))}
              </SelectContent>
            </Select>
            <span className="text-sm text-muted-foreground">{resolvedTimeZone}</span>
          </div>
        )}
      </div>
      {parsed.frequency === 'weekly' && (
        <div className="flex flex-wrap gap-2">
          {SCHEDULE_WEEKDAY_OPTIONS.map((option) => {
            const checked = parsed.weekdays.includes(option.value);
            return (
              <button
                key={option.value}
                type="button"
                onClick={() => update({
                  weekdays: normalizeScheduleWeekdays(
                    checked ? parsed.weekdays.filter((weekday) => weekday !== option.value) : [...parsed.weekdays, option.value],
                  ),
                })}
                className={cn(
                  'rounded-md border px-2.5 py-1 text-xs transition-colors',
                  checked ? 'border-primary/40 bg-primary/10 text-primary' : 'border-border bg-background text-muted-foreground hover:bg-muted/50',
                )}
              >
                {option.label}
              </button>
            );
          })}
        </div>
      )}
      {parsed.frequency === 'monthly' && (
        <div className="flex items-center gap-2">
          <span className="text-sm text-muted-foreground">On day</span>
          <Select value={parsed.dayOfMonth} onValueChange={(dayOfMonth) => update({ dayOfMonth })}>
            <SelectTrigger className="w-24">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {SCHEDULE_MONTH_DAY_OPTIONS.map((option) => (
                <SelectItem key={option.value} value={option.value}>{option.label}</SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      )}
      <p className="text-xs text-muted-foreground">
        Runs {describeSimpleSchedule(parsed).replace('UTC', resolvedTimeZone)}.
      </p>
    </div>
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
  const currentUser = useAuthStore((state) => state.user);
  const workspaceId = workspace?.id ?? '';
  const workspaceSlug = workspace?.slug;
  const scheduleTimezone = normalizeTimeZone(workspace?.timezone);
  const { data: access } = useWorkspaceAccess(workspaceId);
  const permissions = usePermissions(access);
  const accessibleTeamIds = useMemo(
    () => new Set((access?.team_memberships ?? []).map((team) => team.team_id)),
    [access?.team_memberships],
  );
  const settingsQuery = useWorkspaceSettings(workspaceId);
  const inventoryQuery = useAutomationOverview(workspaceId);
  const flowTemplatesQuery = useAutomationFlowTemplates(workspaceId);
  const toolCatalogQuery = useAutomationToolCatalog(workspaceId);
  const skillCatalogQuery = useAutomationSkillCatalog(workspaceId);
  const installFlowTemplate = useInstallAutomationFlowTemplate(workspaceId);
  const uninstallFlowTemplate = useUninstallAutomationFlowTemplate(workspaceId);
  const { data: agents = [] } = useAgents(workspaceId);
  const { data: workflows = [] } = useWorkflows(workspaceId);
  const rulesQuery = useAutomationFlows(workspaceId);
  const { teams } = useWorkspaceTeams(workspaceId);
  const { data: docsCollections = [] } = useAllDocsCollections(workspaceId);
  const { data: docsSpaces = [] } = useDocsSpaces(workspaceId);
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
  const [selectedTemplate, setSelectedTemplate] = useState<FlowTemplateManifest | null>(null);
  const [templateInputs, setTemplateInputs] = useState<Record<string, unknown>>({});
  const [templateFlowSetup, setTemplateFlowSetup] = useState<TemplateFlowSetup | null>(null);
  const [templateFlowSetupEdited, setTemplateFlowSetupEdited] = useState(false);
  const [templateAgentSetup, setTemplateAgentSetup] = useState<TemplateAgentSetup | null>(null);
  const [templateAgentInstructionsEdited, setTemplateAgentInstructionsEdited] = useState(false);
  const [templateAdditionalInstructions, setTemplateAdditionalInstructions] = useState('');
  const [deleteRule, setDeleteRule] = useState<AutomationRule | null>(null);
  const [deleteCreatedAgent, setDeleteCreatedAgent] = useState(false);
  const [deleteAgentReferencedElsewhere, setDeleteAgentReferencedElsewhere] = useState(false);
  const [deletingFlow, setDeletingFlow] = useState(false);
  const [editingRuleId, setEditingRuleId] = useState<string | null>(null);
  const [composerMode, setComposerMode] = useState<FlowComposerMode>('create');
  const [draft, setDraft] = useState<FlowDraft>(defaultDraft());
  const [saving, setSaving] = useState(false);
  const [runningFlowId, setRunningFlowId] = useState<string | null>(null);
  const [selectedFlowId, setSelectedFlowId] = useState<string | null>(null);
  const [statusFilter, setStatusFilter] = useState<'all' | 'active' | 'paused' | 'attention'>('all');
  const [flowQuery, setFlowQuery] = useState('');
  const [scopeFilter, setScopeFilter] = useState<'workspace' | 'team' | 'mine'>('workspace');
  const [upgradeDialogReason, setUpgradeDialogReason] = useState<UpgradeRequiredReason | null>(null);
  const searchSignature = useMemo(() => JSON.stringify(search), [search]);
  const [appliedSearchSignature, setAppliedSearchSignature] = useState('');

  const showUpgradeDialogForError = useCallback((error: unknown) => {
    const reason = getUpgradeRequiredReason(error);
    if (!reason) return false;
    setUpgradeDialogReason(reason);
    return true;
  }, []);

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
    () => filterAutomationFlowsForSearch(authoredFlows, search),
    [authoredFlows, search],
  );
  const teamNamesById = useMemo(() => new Map(teams.map((t) => [t.id, t.name])), [teams]);
  const scopedAndSearchedFlows = useMemo(() => {
    const query = flowQuery.trim().toLowerCase();
    return highlightedFlows.filter((rule) => {
      if (scopeFilter === 'team' && (!rule.team_id || !accessibleTeamIds.has(rule.team_id))) return false;
      if (scopeFilter === 'mine' && (!currentUser?.id || rule.created_by !== currentUser.id)) return false;
      if (!query) return true;
      const trigger = flowTriggerSummary(rule, statesById, scheduleTimezone);
      const action = flowActionSummary(rule, statesById, agentNames);
      const target = flowTargetLabel(rule, workspace?.name);
      return [rule.name, rule.description, trigger.label, trigger.value, action.label, target]
        .filter(Boolean)
        .join(' ')
        .toLowerCase()
        .includes(query);
    });
  }, [accessibleTeamIds, agentNames, currentUser?.id, flowQuery, highlightedFlows, scheduleTimezone, scopeFilter, statesById, workspace?.name]);
  const statusCounts = useMemo(() => {
    let active = 0;
    let paused = 0;
    let attention = 0;
    for (const rule of scopedAndSearchedFlows) {
      const state = deriveFlowState(rule, agentNames, flowHealth.get(rule.id));
      if (state === 'active') active++;
      else if (state === 'paused') paused++;
      else attention++;
    }
    return { all: scopedAndSearchedFlows.length, active, paused, attention };
  }, [agentNames, flowHealth, scopedAndSearchedFlows]);
  const filteredFlows = useMemo(() => scopedAndSearchedFlows.filter((rule) => {
    if (statusFilter === 'all') return true;
    const state = deriveFlowState(rule, agentNames, flowHealth.get(rule.id));
    if (statusFilter === 'attention') return state === 'error' || state === 'incomplete' || state === 'needs_review';
    return state === statusFilter;
  }), [agentNames, flowHealth, scopedAndSearchedFlows, statusFilter]);
  const selectedRule = useMemo(
    () => authoredFlows.find((rule) => rule.id === selectedFlowId) ?? null,
    [authoredFlows, selectedFlowId],
  );
  const activeFlowFilterLabel = useMemo(() => {
    if (search.show_rule) {
      return search.show_rule_title
        || authoredFlows.find((rule) => rule.id === search.show_rule)?.name
        || search.show_rule;
    }
    if (search.show_trigger) {
      return search.show_trigger_title || triggerLabel(search.show_trigger);
    }
    if (search.agent_id) {
      return agentNames.get(search.agent_id) ?? search.agent_id;
    }
    if (search.workflow) {
      return workflows.find((item) => item.workflow.id === search.workflow)?.workflow.name ?? search.workflow;
    }
    if (search.team) {
      return teamNamesById.get(search.team) ?? search.team;
    }
    return '';
  }, [agentNames, authoredFlows, search.agent_id, search.show_rule, search.show_rule_title, search.show_trigger, search.show_trigger_title, search.team, search.workflow, teamNamesById, workflows]);
  const hasFlowFilter = Boolean(search.show_rule || search.show_trigger || search.agent_id || search.workflow || search.team);

  const loading = settingsQuery.isLoading || inventoryQuery.isLoading || rulesQuery.isLoading || tasksQuery.isLoading || epicsQuery.isLoading || repositoriesQuery.isLoading;

  const refreshAll = useCallback(async () => {
    await Promise.all([
      rulesQuery.refetch(),
      inventoryQuery.refetch(),
    ]);
  }, [inventoryQuery, rulesQuery]);

  const resetComposerSearch = useCallback(() => {
    if (search.template || search.trigger_type || search.workflow || search.show_trigger || search.show_trigger_title || search.show_rule || search.show_rule_title || search.template_title || search.template_description || search.agent_id || search.repo_full_name || search.branch || search.base_branch || search.tag_name || search.conclusion || search.target_mode || search.target_id) {
      onSearchChange({
        workflow: undefined,
        template: undefined,
        template_title: undefined,
        template_description: undefined,
        show_trigger: search.show_trigger,
        show_trigger_title: search.show_trigger_title,
        show_rule: search.show_rule,
        show_rule_title: search.show_rule_title,
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
      agent_id: undefined,
      workflow: undefined,
      team: undefined,
    });
  }, [onSearchChange]);

  useEffect(() => {
    if (loading || appliedSearchSignature === searchSignature) return;
    if (search.template) {
      const template = flowTemplatesQuery.data?.find((item) => item.key === search.template);
      if (!template) {
        if (flowTemplatesQuery.isLoading) return;
        setGalleryOpen(true);
        setAppliedSearchSignature(searchSignature);
        return;
      }
      const inputs = defaultTemplateInputs(template, workspace, repositories);
      const agentSetup = defaultTemplateAgentSetup(template, inputs);
      const templateContext = { agents, workflows, repositories, spaces: docsSpaces, collections: docsCollections, teams, timezone: scheduleTimezone };
      setSelectedTemplate(template);
      setTemplateInputs(inputs);
      setTemplateAgentSetup(agentSetup);
      setTemplateFlowSetup(defaultTemplateFlowSetup(template, inputs, templateContext, agentSetup));
      setTemplateFlowSetupEdited(false);
      setTemplateAgentInstructionsEdited(false);
      setTemplateAdditionalInstructions('');
      setAppliedSearchSignature(searchSignature);
      return;
    }
    const prefilledDraft = draftFromSearch(search, workflows);
    if (!prefilledDraft) return;
    setDraft(prefilledDraft);
    setEditingRuleId(null);
    setComposerMode('create');
    setComposerOpen(true);
    setAppliedSearchSignature(searchSignature);
  }, [agents, appliedSearchSignature, docsCollections, docsSpaces, flowTemplatesQuery.data, flowTemplatesQuery.isLoading, loading, repositories, scheduleTimezone, search, searchSignature, teams, workflows, workspace]);

  useEffect(() => {
    if (!search.show_rule || !authoredFlows.some((rule) => rule.id === search.show_rule)) return;
    setSelectedFlowId(search.show_rule);
  }, [authoredFlows, search.show_rule]);

  const openCreateComposer = () => {
    setEditingRuleId(null);
    setComposerMode('create');
    setDraft(defaultDraft());
    setGalleryOpen(true);
  };

  const handleTemplatePick = (template: FlowTemplateManifest | null) => {
    setEditingRuleId(null);
    setGalleryOpen(false);
    if (!template) {
      const next = defaultDraft();
      applyTriggerDefaults(next, workflows);
      setDraft(next);
      setComposerMode('create');
      setComposerOpen(true);
      return;
    }
    setSelectedTemplate(template);
    const inputs = defaultTemplateInputs(template, workspace, repositories);
    const agentSetup = defaultTemplateAgentSetup(template, inputs);
    const templateContext = { agents, workflows, repositories, spaces: docsSpaces, collections: docsCollections, teams, timezone: scheduleTimezone };
    setTemplateInputs(inputs);
    setTemplateAgentSetup(agentSetup);
    setTemplateFlowSetup(defaultTemplateFlowSetup(template, inputs, templateContext, agentSetup));
    setTemplateFlowSetupEdited(false);
    setTemplateAgentInstructionsEdited(false);
    setTemplateAdditionalInstructions('');
  };

  const backToGallery = () => {
    setComposerOpen(false);
    setSelectedTemplate(null);
    setTemplateFlowSetup(null);
    setTemplateFlowSetupEdited(false);
    setTemplateAgentSetup(null);
    setTemplateAgentInstructionsEdited(false);
    setTemplateAdditionalInstructions('');
    setGalleryOpen(true);
  };

  const updateTemplateInput = (key: string, value: unknown) => {
    if (!selectedTemplate) return;
    const nextInputs = { ...templateInputs, [key]: value };
    for (const input of selectedTemplate.inputs) {
      if (input.depends_on === key || isTemplateInputControlledBy(input, key)) {
        nextInputs[input.key] = input.default ?? '';
      }
    }
    setTemplateInputs(nextInputs);
    if (!templateFlowSetupEdited) {
      const nextAgentSetup = !templateAgentInstructionsEdited && selectedTemplate.agent.create
        ? defaultTemplateAgentSetup(selectedTemplate, nextInputs)
        : templateAgentSetup;
      setTemplateFlowSetup(defaultTemplateFlowSetup(
        selectedTemplate,
        nextInputs,
        { agents, workflows, repositories, spaces: docsSpaces, collections: docsCollections, teams, timezone: scheduleTimezone },
        nextAgentSetup,
      ));
    }
    if (!templateAgentInstructionsEdited && selectedTemplate.agent.create) {
      setTemplateAgentSetup((current) => current
        ? { ...current, system_prompt: defaultTemplateAgentInstructions(selectedTemplate, nextInputs) }
        : defaultTemplateAgentSetup(selectedTemplate, nextInputs));
    }
  };

  const openEditComposer = (rule: AutomationRule) => {
    if (rule.trigger_type === 'crm.playbook.work_due') { toast.info('Manage this Flow from its CRM Playbook.'); return; }
    setEditingRuleId(rule.id);
    setComposerMode('edit');
    setDraft(draftFromRule(rule, workflows, scheduleTimezone));
    setComposerOpen(true);
  };

  const handleToggle = async (rule: AutomationRule) => {
    if (rule.trigger_type === 'crm.playbook.work_due') { toast.info('Manage this Flow from its CRM Playbook.'); return; }
    const res = await automationService.updateFlow(workspaceId, rule.id, { enabled: !rule.enabled });
    if (res.error) {
      if (showUpgradeDialogForError(res.error)) return;
      toast.error(res.error);
      return;
    }
    await refreshAll();
  };

  const handleRunNow = async (rule: AutomationRule) => {
    const blocker = flowRunNowBlocker(rule, agentNames);
    if (blocker) {
      toast.error(blocker);
      return;
    }
    setRunningFlowId(rule.id);
    const res = await automationService.runFlowNow(workspaceId, rule.id);
    setRunningFlowId(null);
    if (res.error) {
      if (showUpgradeDialogForError(res.error)) return;
      toast.error(res.error);
      return;
    }
    toast.success('Flow run started');
    await refreshAll();
  };

  const openDeleteFlow = (rule: AutomationRule) => {
    const agentId = stringValue(rule.action_config?.agent_id);
    setDeleteRule(rule);
    setDeleteCreatedAgent(false);
    setDeleteAgentReferencedElsewhere(
      !!agentId
      && authoredFlows.some((other) => other.id !== rule.id && stringValue(other.action_config?.agent_id) === agentId),
    );
  };

  const handleSave = async () => {
    const validationError = validateDraft(draft);
    if (validationError) {
      toast.error(validationError);
      return;
    }
    setSaving(true);
    const payload = serializeDraft(draft, workspaceId, scheduleTimezone);
    const res = editingRuleId && composerMode === 'edit'
      ? await automationService.updateFlow(workspaceId, editingRuleId, payload)
      : await automationService.createFlow(workspaceId, { workspace_id: workspaceId, ...payload, position: authoredFlows.length });
    setSaving(false);
    if (res.error) {
      if (showUpgradeDialogForError(res.error)) return;
      toast.error(res.error);
      return;
    }
    toast.success(editingRuleId && composerMode === 'edit' ? 'Flow updated' : 'Flow created');
    setComposerOpen(false);
    setEditingRuleId(null);
    setComposerMode('create');
    await refreshAll();
    resetComposerSearch();
  };

  const handleInstallTemplate = async () => {
    if (!selectedTemplate) return;
    const installInputs = {
      ...templateInputs,
      ...(templateAdditionalInstructions.trim()
        ? { additional_instructions: templateAdditionalInstructions.trim() }
        : {}),
    };
    try {
      const result = await installFlowTemplate.mutateAsync({
        templateKey: selectedTemplate.key,
        payload: {
          name: templateFlowSetup?.name.trim() || selectedTemplate.name,
          description: templateFlowSetup?.description.trim() || undefined,
          agent_name: templateAgentSetup?.name.trim() || undefined,
          inputs: installInputs,
          agent_overrides: templateAgentSetup && selectedTemplate.agent.create
            ? {
                role: selectedTemplate.name,
                runtime_kind: (selectedTemplate.agent.create.runtime_kind || 'native_sdk') as AgentRuntimeKind,
                system_prompt: templateAgentSetup.system_prompt.trim() || undefined,
                allowed_tools: templateAgentSetup.allowed_tools,
                allowed_targets: templateAgentSetup.allowed_targets,
                skills: templateAgentSetup.skills,
                approval_mode: templateAgentSetup.approval_mode,
                max_concurrent_runs: normalizeTemplateMaxRuns(templateAgentSetup.max_concurrent_runs),
                default_invocation_mode: 'interactive',
              }
            : undefined,
        },
      });
      toast.success(`${result.template.name} installed`);
      setSelectedTemplate(null);
      setTemplateFlowSetup(null);
      setTemplateFlowSetupEdited(false);
      setTemplateAgentSetup(null);
      setTemplateAgentInstructionsEdited(false);
      setTemplateAdditionalInstructions('');
      onSearchChange({
        show_rule: result.rule.id,
        show_rule_title: result.rule.name,
        show_trigger: undefined,
        show_trigger_title: undefined,
      });
      await refreshAll();
    } catch (error) {
      if (showUpgradeDialogForError(error)) return;
      toast.error(error instanceof Error ? error.message : 'Failed to install template');
    }
  };

  const handleConfirmDeleteFlow = async () => {
    if (!deleteRule) return;
    if (deleteRule.template_instance_id) {
      try {
        const result = await uninstallFlowTemplate.mutateAsync({
          instanceId: deleteRule.template_instance_id,
          payload: { delete_created_agent: deleteCreatedAgent && !deleteAgentReferencedElsewhere },
        });
        const suffix = result.agent_action === 'deleted'
          ? ' and deleted the agent'
          : result.agent_action === 'kept'
            ? ' and kept the agent'
            : '';
        toast.success(`Flow deleted${suffix}`);
        setSelectedFlowId(null);
        setDeleteRule(null);
        await refreshAll();
      } catch (error) {
        toast.error(error instanceof Error ? error.message : 'Failed to delete flow');
      }
      return;
    }

    setDeletingFlow(true);
    const res = await automationService.deleteFlow(workspaceId, deleteRule.id);
    setDeletingFlow(false);
    if (res.error) {
      toast.error(res.error);
      return;
    }
    toast.success('Flow deleted');
    setSelectedFlowId(null);
    setDeleteRule(null);
    await refreshAll();
  };

  if (!workspaceId) {
    return <p className="text-sm text-muted-foreground">Workspace not found.</p>;
  }

  return (
    <div className="h-full text-foreground">
      <FlowTemplateGallery
        open={galleryOpen}
        onOpenChange={(open) => {
          setGalleryOpen(open);
          if (!open) resetComposerSearch();
        }}
        onPick={handleTemplatePick}
        templates={flowTemplatesQuery.data ?? []}
        loading={flowTemplatesQuery.isLoading}
        error={flowTemplatesQuery.error instanceof Error ? flowTemplatesQuery.error.message : flowTemplatesQuery.isError ? 'Request failed' : null}
      />
      <FlowTemplateInstallDialog
        open={!!selectedTemplate}
        onOpenChange={(open) => {
          if (!open) {
            setSelectedTemplate(null);
            setTemplateFlowSetup(null);
            setTemplateFlowSetupEdited(false);
            setTemplateAgentSetup(null);
            setTemplateAgentInstructionsEdited(false);
            setTemplateAdditionalInstructions('');
          }
        }}
        template={selectedTemplate}
        values={templateInputs}
        flowSetup={templateFlowSetup}
        agentSetup={templateAgentSetup}
        additionalInstructions={templateAdditionalInstructions}
        agents={agents}
        workflows={workflows}
        repositories={repositories}
        spaces={docsSpaces}
        collections={docsCollections}
        teams={teams}
        toolCatalog={toolCatalogQuery.data?.tools ?? []}
        skillCatalog={skillCatalogQuery.data?.skills ?? []}
        timezone={scheduleTimezone}
        saving={installFlowTemplate.isPending}
        canEdit={permissions.canAdminAutomations}
        onBack={() => {
          setSelectedTemplate(null);
          setTemplateFlowSetup(null);
          setTemplateFlowSetupEdited(false);
          setTemplateAgentSetup(null);
          setTemplateAgentInstructionsEdited(false);
          setTemplateAdditionalInstructions('');
          setGalleryOpen(true);
        }}
        onValueChange={updateTemplateInput}
        onFlowSetupChange={(setup) => {
          setTemplateFlowSetup(setup);
          setTemplateFlowSetupEdited(true);
        }}
        onAgentSetupChange={setTemplateAgentSetup}
        onAgentInstructionsEdited={() => setTemplateAgentInstructionsEdited(true)}
        onAdditionalInstructionsChange={setTemplateAdditionalInstructions}
        onInstall={() => void handleInstallTemplate()}
      />
      <DeleteFlowDialog
        rule={deleteRule}
        open={!!deleteRule}
        onOpenChange={(open) => {
          if (!open) {
            setDeleteRule(null);
            setDeleteCreatedAgent(false);
            setDeleteAgentReferencedElsewhere(false);
          }
        }}
        agentReferencedElsewhere={deleteAgentReferencedElsewhere}
        deleteCreatedAgent={deleteCreatedAgent}
        onDeleteCreatedAgentChange={setDeleteCreatedAgent}
        saving={uninstallFlowTemplate.isPending || deletingFlow}
        onConfirm={() => void handleConfirmDeleteFlow()}
      />
      <FlowComposer
        workspaceId={workspaceId}
        open={composerOpen}
        mode={composerMode}
        draft={draft}
        workflows={workflows}
        statesById={statesById}
        agents={agents}
        accessibleTeamIds={accessibleTeamIds}
        canSeeAllAgents={permissions.isAdmin}
        tasks={tasks}
        epics={epics}
        repositories={repositories}
        timezone={scheduleTimezone}
        saving={saving}
        canEdit={permissions.canAdminAutomations}
        onOpenChange={(open) => {
          setComposerOpen(open);
          if (!open) {
            resetComposerSearch();
          }
        }}
        onBack={composerMode === 'create' ? backToGallery : undefined}
        onDraftChange={setDraft}
        onSave={handleSave}
      />

      <FlowDetailDrawer
        rule={selectedRule}
        workspaceId={workspaceId}
        workspaceSlug={workspaceSlug}
        workspaceName={workspace?.name}
        statesById={statesById}
        agentNames={agentNames}
        healthItem={selectedRule ? flowHealth.get(selectedRule.id) : undefined}
        timezone={scheduleTimezone}
        canEdit={permissions.canAdminAutomations}
        canRunNowAction={permissions.canEdit}
        runningNow={runningFlowId === selectedRule?.id}
        onOpenChange={(open) => { if (!open) setSelectedFlowId(null); }}
        onEdit={(rule) => { setSelectedFlowId(null); openEditComposer(rule); }}
        onToggle={handleToggle}
        onRunNow={handleRunNow}
        onDelete={openDeleteFlow}
      />

      <AutomationShell
        title="Flows"
        description="When something happens, do something. Flows keep agents working without anyone prompting them."
        actions={permissions.canAdminAutomations ? (
          <QuietPrimaryAction className="shrink-0 gap-1.5" onClick={openCreateComposer}>
            <PlusSignIcon className="h-4 w-4" />
            New flow
          </QuietPrimaryAction>
        ) : undefined}
      >

      {loading ? (
        <div className="space-y-2">
          <Skeleton className="mb-5 h-8 w-full rounded-lg" />
          {Array.from({ length: 5 }).map((_, index) => <Skeleton key={index} className="h-[68px] w-full rounded-[9px]" />)}
        </div>
      ) : rulesQuery.isError ? (
        <QuietEmptyState
          title="Flows could not be loaded"
          description="The flow list is temporarily unavailable. Your existing flows have not been changed."
          action={<QuietTextAction className="border-b border-quiet-field pb-0.5" onClick={() => void rulesQuery.refetch()}>Try again</QuietTextAction>}
        />
      ) : authoredFlows.length === 0 ? (
        <QuietEmptyState
          title="Put routine work on autopilot"
          description="Start with a proven template or connect a trigger to an agent and build your own flow."
          action={permissions.canAdminAutomations ? (
            <div className="flex flex-wrap items-center gap-4">
              <QuietPrimaryAction onClick={openCreateComposer}>Choose a template</QuietPrimaryAction>
              <QuietTextAction className="border-b border-quiet-field pb-0.5" onClick={() => handleTemplatePick(null)}>Build custom flow</QuietTextAction>
            </div>
          ) : undefined}
        />
      ) : (
        <>
          {hasFlowFilter && (
            <FlowFilterBar
              value={activeFlowFilterLabel}
              count={highlightedFlows.length}
              onClear={clearFlowFilter}
            />
          )}

          <div className="mb-5 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <Tabs value={statusFilter} onValueChange={(value) => setStatusFilter(value as typeof statusFilter)} className="gap-0">
              <TabsList variant="quiet" aria-label="Flow status" className="max-w-full flex-wrap justify-start border-b-0">
              {([
                ['all', 'All'],
                ['active', 'Active'],
                ['paused', 'Paused'],
                ['attention', 'Needs attention'],
              ] as const).map(([value, label]) => (
                <TabsTrigger
                  key={value}
                  value={value}
                >
                  {label}
                  <span className="text-[12px] font-normal tabular-nums text-quiet-muted">{statusCounts[value]}</span>
                </TabsTrigger>
              ))}
              </TabsList>
            </Tabs>
            <div className="flex min-w-0 flex-col gap-2 xs:flex-row sm:justify-end">
              <QuietSearchInput
                containerClassName="min-w-0 sm:w-52"
                aria-label="Search flows"
                value={flowQuery}
                onChange={(event) => setFlowQuery(event.target.value)}
                placeholder="Search flows"
              />
              <Select value={scopeFilter} onValueChange={(value) => setScopeFilter(value as typeof scopeFilter)}>
                <SelectTrigger aria-label="Flow scope" className={cn(quietUnderlineControlClassName, 'w-full justify-between xs:w-36')}><SelectValue /></SelectTrigger>
                <SelectContent><SelectItem value="workspace">Workspace-wide</SelectItem><SelectItem value="team">My team</SelectItem><SelectItem value="mine">Created by me</SelectItem></SelectContent>
              </Select>
            </div>
          </div>

          <div className="hidden grid-cols-[minmax(0,1fr)_96px_116px_120px_28px] gap-4 border-b border-border px-[14px] pb-[9px] text-xs font-medium uppercase tracking-wide text-muted-foreground md:grid">
            <span>Flow</span><span className="text-right">Runs</span><span>Last run</span><span>Next run</span><span />
          </div>
          <div>
            {filteredFlows.length > 0 ? (
              <div>
                {filteredFlows.map((rule) => (
                  <FlowRow
                    key={rule.id}
                    rule={rule}
                    statesById={statesById}
                    agentNames={agentNames}
                    teamName={rule.team_id ? teamNamesById.get(rule.team_id) : undefined}
                    healthItem={flowHealth.get(rule.id)}
                    workspaceSlug={workspaceSlug}
                    timezone={scheduleTimezone}
                    canEdit={permissions.canAdminAutomations}
                    canRunNowAction={permissions.canEdit}
                    onEdit={openEditComposer}
                    onToggle={handleToggle}
                    onRunNow={handleRunNow}
                    onDelete={openDeleteFlow}
                    onOpen={(selected) => setSelectedFlowId(selected.id)}
                    runningNow={runningFlowId === rule.id}
                  />
                ))}
              </div>
            ) : (
              <QuietEmptyState title="No flows match this view" description="Clear a filter or try a broader search to see more flows." />
            )}
          </div>

          {permissions.canAdminAutomations ? (
            <div className="mt-10 flex flex-col items-start justify-between gap-3 border-t border-border pt-5 sm:flex-row sm:items-center">
              <span className="text-sm text-muted-foreground">Start from a template instead of building from scratch.</span>
              <QuietTextAction className="border-b border-quiet-field pb-0.5" onClick={() => setGalleryOpen(true)}>Browse templates</QuietTextAction>
            </div>
          ) : null}
        </>
      )}
      </AutomationShell>
      <UpgradeRequiredDialog
        open={upgradeDialogReason !== null}
        onOpenChange={(open) => {
          if (!open) setUpgradeDialogReason(null);
        }}
        onUpgrade={() => {
          setComposerOpen(false);
          setGalleryOpen(false);
          setSelectedTemplate(null);
          setTemplateFlowSetup(null);
          setTemplateFlowSetupEdited(false);
          setTemplateAgentSetup(null);
          setTemplateAgentInstructionsEdited(false);
          setTemplateAdditionalInstructions('');
        }}
        reason={upgradeDialogReason}
      />
    </div>
  );
}
