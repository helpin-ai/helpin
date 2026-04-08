import { useCallback, useEffect, useMemo, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { ArrowRight01Icon, MoreHorizontalIcon, PlayIcon } from '@/lib/icons';
import { toast } from 'sonner';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
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

type AutomationFlowsSearch = {
  workflow?: string;
  team?: string;
  template?: string;
  template_title?: string;
  template_description?: string;
  show_trigger?: string;
  show_trigger_title?: string;
  create_event_rule?: boolean;
  trigger_type?: string;
  agent_id?: string;
  repo_full_name?: string;
  branch?: string;
  base_branch?: string;
  tag_name?: string;
  conclusion?: string;
  target_mode?: 'event' | 'task' | 'epic' | 'repository';
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
  targetMode: 'event' | 'task' | 'epic' | 'repository';
  targetId: string;
  targetStateId: string;
  targetBranch: string;
  repoFullName: string;
  branch: string;
  baseBranch: string;
  tagName: string;
  conclusion: string;
  cronCategory: string;
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
  merge_branch: 'Merge into a branch',
};

const TARGET_LABELS: Record<FlowDraft['targetMode'], string> = {
  event: 'the event target',
  task: 'a fixed task',
  epic: 'a fixed epic',
  repository: 'a fixed repository',
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
    repoFullName: '',
    branch: '',
    baseBranch: 'main',
    tagName: '',
    conclusion: '',
    cronCategory: 'workspace_hourly',
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
  draft.repoFullName = stringValue(rule.trigger_config?.repo_full_name);
  draft.branch = stringValue(rule.trigger_config?.branch);
  draft.baseBranch = stringValue(rule.trigger_config?.base_branch) || 'main';
  draft.tagName = stringValue(rule.trigger_config?.tag_name);
  draft.conclusion = stringValue(rule.trigger_config?.conclusion);
  draft.cronCategory = stringValue(rule.trigger_config?.category) || 'workspace_hourly';
  applyTriggerDefaults(draft, workflows);
  return draft;
}

function serializeDraft(draft: FlowDraft) {
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
    triggerConfig = { category: draft.cronCategory.trim() };
  }

  let actionConfig: Record<string, unknown> = {};
  if (draft.actionType === 'start_agent_run') {
    actionConfig = { agent_id: draft.agentId };
    if (draft.targetMode !== 'event' && draft.targetId) {
      actionConfig.target_type = draft.targetMode;
      actionConfig.target_id = draft.targetId;
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
  if (draft.triggerType === 'cron' && !draft.cronCategory.trim()) {
    return 'Add a schedule category';
  }
  if (draft.actionType === 'start_agent_run') {
    if (!draft.agentId) return 'Choose an agent';
    if (draft.triggerType === 'cron' && (draft.targetMode === 'event' || !draft.targetId)) {
      return 'Scheduled flows need a fixed target';
    }
    if (draft.targetMode !== 'event' && !draft.targetId) {
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
  const category = stringValue(rule.trigger_config?.category);
  if (repoFullName) filters.push(`repo = ${repoFullName}`);
  if (branch) filters.push(`branch = ${branch}`);
  if (baseBranch) filters.push(`base = ${baseBranch}`);
  if (tagName) filters.push(`tag = ${tagName}`);
  if (conclusion) filters.push(`conclusion = ${conclusion}`);
  if (category) filters.push(`category = ${category}`);
  return filters.length ? filters.join(' · ') : 'No additional filters';
}

function describeThen(rule: AutomationRule, statesById: Map<string, WorkflowState>) {
  if (rule.action_type === 'start_agent_run') return 'Start an agent run';
  if (rule.action_type === 'move_to_state') {
    const stateId = stringValue(rule.action_config?.target_state_id);
    return stateId ? `Move the task to ${statesById.get(stateId)?.name ?? 'another state'}` : 'Move the task to another state';
  }
  if (rule.action_type === 'merge_branch') {
    const branch = stringValue(rule.action_config?.target_branch);
    return branch ? `Merge into ${branch}` : 'Merge to a branch';
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
    if (draft.baseBranch.trim()) parts.push(`base = ${draft.baseBranch.trim()}`);
    if (draft.tagName.trim()) parts.push(`tag = ${draft.tagName.trim()}`);
    if (draft.conclusion.trim()) parts.push(`conclusion = ${draft.conclusion.trim()}`);
    if (draft.cronCategory.trim() && draft.triggerType === 'cron') parts.push(`category = ${draft.cronCategory.trim()}`);
    return parts.join(' · ') || 'No additional filters';
  })();
  const then = draft.actionType === 'move_to_state'
    ? `Move the task to ${destinationStateName || 'another state'}`
    : draft.actionType === 'merge_branch'
      ? `Merge into ${draft.targetBranch.trim() || 'a branch'}`
      : ACTION_LABELS[draft.actionType];
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
    triggerPart = `PR ${action}${baseBranch ? ` to ${baseBranch}` : ''}`;
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
    actionPart = agentName ? `Run ${agentName}` : 'Start agent';
  } else if (rule.action_type === 'move_to_state') {
    const targetStateId = stringValue(rule.action_config?.target_state_id);
    const targetStateName = statesById.get(targetStateId)?.name;
    actionPart = targetStateName ? `Move to ${targetStateName}` : 'Move task state';
  } else if (rule.action_type === 'merge_branch') {
    const branch = stringValue(rule.action_config?.target_branch);
    actionPart = branch ? `Merge into ${branch}` : 'Merge branch';
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
  const hasHealthError = Boolean(healthItem?.health.last_error_at);
  const hasSuccess = Boolean(healthItem?.health.last_success_at);
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

  return (
    <div
      className={cn(
        'rounded-lg border bg-card px-4 py-3 transition-colors',
        hasError
          ? 'border-destructive/50 bg-destructive/5'
          : 'border-border/60 hover:border-border',
      )}
    >
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0 space-y-2">
          {/* Title row */}
          <div className="flex items-center gap-2">
            <span
              className={cn(
                'mt-0.5 h-2 w-2 shrink-0 rounded-full',
                hasError || hasHealthError
                  ? 'bg-destructive'
                  : hasSuccess
                    ? 'bg-emerald-500'
                    : 'bg-muted-foreground/30',
              )}
            />
            <p className="text-sm font-medium text-foreground/90">{title}</p>
            {agentMissing && (
              <>
                <Badge variant="destructive" className="text-xs">Missing agent</Badge>
                <Badge variant="destructive" className="text-xs">Unknown agent</Badge>
              </>
            )}
          </div>

          {/* Badges row */}
          <div className="flex flex-wrap items-center gap-1.5 pl-4">
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
        </div>

        {/* Right side: last run + menu */}
        <div className="flex shrink-0 items-center gap-2 pt-0.5">
          <span className="whitespace-nowrap text-xs text-muted-foreground">{lastRunLabel}</span>
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
      </div>
    </div>
  );
}

function FlowComposer({
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
  onDraftChange,
  onSave,
}: {
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

  const updateDraft = (mutate: (current: FlowDraft) => FlowDraft) => onDraftChange((current) => {
    const next = mutate(current);
    const normalized = { ...next };
    applyTriggerDefaults(normalized, workflows);
    return normalized;
  });

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-4xl">
        <DialogHeader>
          <DialogTitle>{mode === 'create' ? 'Create flow' : 'Edit flow'}</DialogTitle>
        </DialogHeader>

        <div className="space-y-5">
          <div className="grid gap-3 md:grid-cols-2">
            <div className="space-y-2">
              <Label htmlFor="flow-name">Flow name</Label>
              <Input id="flow-name" value={draft.name} onChange={(event) => updateDraft((current) => ({ ...current, name: event.target.value }))} placeholder="Review merged PRs" />
            </div>
            <div className="space-y-2">
              <Label htmlFor="flow-trigger">When this happens</Label>
              <Select value={draft.triggerType} onValueChange={(value) => updateDraft((current) => ({ ...current, triggerType: value }))}>
                <SelectTrigger id="flow-trigger">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {Array.from(new Set(TRIGGER_OPTIONS.map((option) => option.group))).map((group) => (
                    <div key={group}>
                      <div className="px-2 py-1 text-[11px] font-medium uppercase tracking-wide text-muted-foreground">{group}</div>
                      {TRIGGER_OPTIONS.filter((option) => option.group === group).map((option) => (
                        <SelectItem key={option.value} value={option.value}>{option.label}</SelectItem>
                      ))}
                    </div>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>

          <div className="space-y-2">
            <Label htmlFor="flow-description">Description</Label>
            <Textarea id="flow-description" rows={2} value={draft.description} onChange={(event) => updateDraft((current) => ({ ...current, description: event.target.value }))} placeholder="Explain what this flow is for and how the team should use it." />
          </div>

          <div className="space-y-4">
            <div className="flex items-center gap-3">
              <p className="text-sm font-medium text-muted-foreground">Conditions</p>
              <div className="flex-1 border-t border-border/40" />
            </div>
            {isWorkflowTrigger(draft.triggerType) ? (
              <div className="grid gap-3 md:grid-cols-2">
                <div className="space-y-2">
                  <Label>Workflow</Label>
                  <Select value={draft.workflowId} onValueChange={(value) => updateDraft((current) => ({ ...current, workflowId: value, triggerStateId: firstStateIdForWorkflow(workflows, value), targetStateId: '' }))}>
                    <SelectTrigger>
                      <SelectValue placeholder="Select workflow..." />
                    </SelectTrigger>
                    <SelectContent>
                      {workflows.map((workflow) => (
                        <SelectItem key={workflow.workflow.id} value={workflow.workflow.id}>{workflow.workflow.name}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <div className="space-y-2">
                  <Label>State</Label>
                  <Select value={draft.triggerStateId} onValueChange={(value) => updateDraft((current) => ({ ...current, triggerStateId: value }))}>
                    <SelectTrigger>
                      <SelectValue placeholder="Select state..." />
                    </SelectTrigger>
                    <SelectContent>
                      {workflowStates.map((state) => (
                        <SelectItem key={state.id} value={state.id}>{state.name}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
              </div>
            ) : (
              <div className="grid gap-3 md:grid-cols-2">
                <div className="space-y-2">
                  <Label>Repository</Label>
                  <Select value={draft.repoFullName || '__custom__'} onValueChange={(value) => updateDraft((current) => ({ ...current, repoFullName: value === '__custom__' ? '' : value }))}>
                    <SelectTrigger>
                      <SelectValue placeholder="Any repository" />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="__custom__">Custom / any repository</SelectItem>
                      {repositoryOptions.map((repo) => (
                        <SelectItem key={repo.value} value={repo.value}>{repo.label}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  <Input value={draft.repoFullName} onChange={(event) => updateDraft((current) => ({ ...current, repoFullName: event.target.value }))} placeholder="owner/repo" />
                </div>

                {(draft.triggerType === 'github.push' || draft.triggerType === 'github.check_suite_completed') && (
                  <div className="space-y-2">
                    <Label>Branch</Label>
                    <Input value={draft.branch} onChange={(event) => updateDraft((current) => ({ ...current, branch: event.target.value }))} placeholder="main" />
                  </div>
                )}

                {(draft.triggerType === 'github.pull_request_opened' || draft.triggerType === 'github.pull_request_merged' || draft.triggerType === 'github.pull_request_review_requested') && (
                  <div className="space-y-2">
                    <Label>Base branch</Label>
                    <Input value={draft.baseBranch} onChange={(event) => updateDraft((current) => ({ ...current, baseBranch: event.target.value }))} placeholder="main" />
                  </div>
                )}

                {draft.triggerType === 'github.release_published' && (
                  <div className="space-y-2">
                    <Label>Tag</Label>
                    <Input value={draft.tagName} onChange={(event) => updateDraft((current) => ({ ...current, tagName: event.target.value }))} placeholder="v1.0.0" />
                  </div>
                )}

                {draft.triggerType === 'github.check_suite_completed' && (
                  <div className="space-y-2">
                    <Label>Conclusion</Label>
                    <Input value={draft.conclusion} onChange={(event) => updateDraft((current) => ({ ...current, conclusion: event.target.value }))} placeholder="success" />
                  </div>
                )}

                {draft.triggerType === 'cron' && (
                  <div className="space-y-2">
                    <Label>Schedule category</Label>
                    <Input value={draft.cronCategory} onChange={(event) => updateDraft((current) => ({ ...current, cronCategory: event.target.value }))} placeholder="workspace_hourly" />
                  </div>
                )}
              </div>
            )}
          </div>

          <div className="space-y-4">
            <div className="flex items-center gap-3">
              <p className="text-sm font-medium text-muted-foreground">Action</p>
              <div className="flex-1 border-t border-border/40" />
            </div>
            <div className="grid gap-3 md:grid-cols-2">
              <div className="space-y-2">
                <Label>Action</Label>
                <Select value={draft.actionType} onValueChange={(value: FlowDraft['actionType']) => updateDraft((current) => ({ ...current, actionType: value }))}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {actionOptions.map((action) => (
                      <SelectItem key={action} value={action}>{ACTION_LABELS[action]}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              {draft.actionType === 'start_agent_run' && (
                <div className="space-y-2">
                  <Label>Agent</Label>
                  <Select value={draft.agentId} onValueChange={(value) => updateDraft((current) => ({ ...current, agentId: value }))}>
                    <SelectTrigger>
                      <SelectValue placeholder="Select agent..." />
                    </SelectTrigger>
                    <SelectContent>
                      {Array.from(agents.entries()).map(([id, name]) => (
                        <SelectItem key={id} value={id}>{name}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
              )}
            </div>

            {draft.actionType === 'start_agent_run' && (
              <div className="grid gap-3 md:grid-cols-[180px_minmax(0,1fr)]">
                <div className="space-y-2">
                  <Label>Target</Label>
                  <Select value={draft.targetMode} onValueChange={(value: FlowDraft['targetMode']) => updateDraft((current) => ({ ...current, targetMode: value, targetId: '' }))}>
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="event">Use event target</SelectItem>
                      <SelectItem value="task">Fixed task</SelectItem>
                      <SelectItem value="epic">Fixed epic</SelectItem>
                      <SelectItem value="repository">Fixed repository</SelectItem>
                    </SelectContent>
                  </Select>
                </div>

                {draft.targetMode !== 'event' && (
                  <div className="space-y-2">
                    <Label>Fixed target</Label>
                    <Select value={draft.targetId} onValueChange={(value) => updateDraft((current) => ({ ...current, targetId: value }))}>
                      <SelectTrigger>
                        <SelectValue placeholder="Select target..." />
                      </SelectTrigger>
                      <SelectContent>
                        {(draft.targetMode === 'task' ? targetTaskOptions : draft.targetMode === 'epic' ? targetEpicOptions : targetRepositoryOptions).map((option) => (
                          <SelectItem key={option.value} value={option.value}>{option.label}</SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                    <p className="text-[11px] text-muted-foreground">
                      Always run this flow against the same target, regardless of the event.
                    </p>
                  </div>
                )}
              </div>
            )}

            {draft.actionType === 'move_to_state' && (
              <div className="space-y-2">
                <Label>Move to</Label>
                <Select value={draft.targetStateId} onValueChange={(value) => updateDraft((current) => ({ ...current, targetStateId: value }))}>
                  <SelectTrigger>
                    <SelectValue placeholder="Select destination state..." />
                  </SelectTrigger>
                  <SelectContent>
                    {workflowStates.map((state) => (
                      <SelectItem key={state.id} value={state.id}>{state.name}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            )}

            {draft.actionType === 'merge_branch' && (
              <div className="space-y-2">
                <Label>Merge into branch</Label>
                <Input value={draft.targetBranch} onChange={(event) => updateDraft((current) => ({ ...current, targetBranch: event.target.value }))} placeholder="main" />
              </div>
            )}
          </div>

          <div className="flex flex-wrap items-center gap-1.5 rounded-lg border border-border/50 bg-muted/30 px-4 py-2.5">
            <span className="inline-flex items-center rounded border border-border/60 bg-background px-2 py-0.5 text-xs font-medium">
              {sentence.when}
            </span>
            {sentence.conditions !== 'No additional filters' && (
              <>
                <ArrowRight01Icon className="h-3 w-3 shrink-0 text-muted-foreground/40" />
                <span className="text-xs text-muted-foreground">{sentence.conditions}</span>
              </>
            )}
            <ArrowRight01Icon className="h-3 w-3 shrink-0 text-muted-foreground/40" />
            <span className="text-xs">{sentence.then}</span>
            {draft.actionType === 'start_agent_run' && (
              <>
                <span className="text-muted-foreground/40">·</span>
                <span className="text-xs text-muted-foreground">{sentence.using}</span>
              </>
            )}
          </div>
        </div>

        <DialogFooter className="gap-2">
          <Button type="button" variant="ghost" onClick={() => onOpenChange(false)}>Cancel</Button>
          <Button type="button" onClick={() => void onSave()} disabled={saving || !canEdit}>
            {saving ? 'Saving...' : mode === 'create' ? 'Create flow' : 'Save changes'}
          </Button>
        </DialogFooter>
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
    () => search.show_trigger ? authoredFlows.filter((rule) => rule.trigger_type === search.show_trigger) : authoredFlows,
    [authoredFlows, search.show_trigger],
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

  const loading = settingsQuery.isLoading || inventoryQuery.isLoading || rulesQuery.isLoading || tasksQuery.isLoading || epicsQuery.isLoading || repositoriesQuery.isLoading;

  const refreshAll = useCallback(async () => {
    await Promise.all([
      rulesQuery.refetch(),
      inventoryQuery.refetch(),
    ]);
  }, [inventoryQuery, rulesQuery]);

  const resetComposerSearch = useCallback(() => {
    if (search.template || search.trigger_type || search.create_event_rule || search.workflow || search.show_trigger || search.show_trigger_title || search.template_title || search.template_description || search.agent_id || search.repo_full_name || search.branch || search.base_branch || search.tag_name || search.conclusion || search.target_mode || search.target_id) {
      onSearchChange({
        workflow: undefined,
        template: undefined,
        template_title: undefined,
        template_description: undefined,
        show_trigger: search.show_trigger,
        show_trigger_title: search.show_trigger_title,
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
    setComposerOpen(true);
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
    const payload = serializeDraft(draft);
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
      <FlowComposer
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
      ) : (
        <>
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
                <p className="text-sm font-medium">No automation flows yet</p>
                <p className="mt-1 text-sm text-muted-foreground">
                  Create a flow to connect events to agents and workflow actions.
                </p>
              </div>
            )}
          </div>
        </>
      )}
    </div>
  );
}
