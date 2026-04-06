import { useCallback, useEffect, useMemo, useState, type FormEvent } from 'react';
import { pmWorkflowService } from '@/lib/services/pmWorkflowService';
import { automationRuleService } from '@/lib/services/automationRuleService';
import { agentService } from '@/lib/services/agentService';
import { StateTypeIcon } from '@/lib/pmConstants';
import type { WorkspaceTeam } from '@/lib/types';
import type { Agent, AutomationRule, StateType, WorkflowState, WorkflowWithStates } from '@/lib/pmTypes';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { Separator } from '@/components/ui/separator';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { ConfirmDialog } from '@/components/pm/ConfirmDialog';
import { ArrowDown02Icon, ArrowUp02Icon, BotIcon, Tick01Icon, Copy01Icon, MoreVerticalIcon, GitBranchIcon, Loading01Icon, PencilEdit01Icon, PlayIcon, PlusSignIcon, Delete01Icon, Cancel01Icon } from '@/lib/icons';
import { toast } from 'sonner';
import { ColorPicker } from '@/components/pm/ColorPicker';
import { PRESET_COLORS } from '@/components/pm/ColorPicker';
import { QuickTooltip } from '@/components/ui/quick-tooltip';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu';
import { cn } from '@/lib/utils';

const STATE_TYPE_ORDER: StateType[] = ['backlog', 'unstarted', 'started', 'done'];
const STATE_TYPE_LABEL: Record<StateType, string> = {
  backlog: 'Backlog',
  unstarted: 'Unstarted',
  started: 'Started',
  done: 'Done',
};

// ── Pipeline Rules Section (state edit dialog) ──

const ACTION_LABELS: Record<string, string> = {
  start_agent_run: 'Run agent',
  move_to_state: 'Move to state',
  merge_branch: 'Merge branch',
};

const TRIGGER_LABELS: Record<string, string> = {
  'task.state_entered': 'On state entry',
  'agent_run.approved': 'On run approved',
};

function PipelineRulesSection({
  workspaceId,
  workflowId,
  stateId,
  stateName,
  rules,
  agents,
  states,
  onChanged,
}: {
  workspaceId: string;
  workflowId: string;
  stateId: string;
  stateName: string;
  rules: AutomationRule[];
  agents: Agent[];
  states: WorkflowState[];
  onChanged: () => void;
}) {
  const [adding, setAdding] = useState(false);
  const [newTrigger, setNewTrigger] = useState<string>('task.state_entered');
  const [newAction, setNewAction] = useState<string>('start_agent_run');
  const [newAgentId, setNewAgentId] = useState<string>('');
  const [newTargetStateId, setNewTargetStateId] = useState<string>('');
  const [newTargetBranch, setNewTargetBranch] = useState<string>('');
  const [saving, setSaving] = useState(false);

  const handleAdd = async () => {
    if (!newAction) return;
    setSaving(true);

    let actionConfig: Record<string, unknown> = {};
    if (newAction === 'start_agent_run') {
      if (!newAgentId) { toast.error('Select an agent'); setSaving(false); return; }
      actionConfig = { agent_id: newAgentId };
    } else if (newAction === 'move_to_state') {
      if (!newTargetStateId) { toast.error('Select a target state'); setSaving(false); return; }
      actionConfig = { target_state_id: newTargetStateId };
    } else if (newAction === 'merge_branch') {
      if (!newTargetBranch.trim()) { toast.error('Enter a target branch'); setSaving(false); return; }
      actionConfig = { target_branch: newTargetBranch.trim() };
    }

    const res = await automationRuleService.create(workspaceId, {
      workspace_id: workspaceId,
      name: `${ACTION_LABELS[newAction] ?? newAction} on ${stateName}`,
      workflow_id: workflowId,
      trigger_type: newTrigger,
      trigger_config: { state_id: stateId },
      action_type: newAction,
      action_config: actionConfig,
      position: rules.length,
    });
    setSaving(false);
    if (res.error) { toast.error(res.error); return; }
    toast.success('Automation rule added');
    setAdding(false);
    setNewAgentId('');
    setNewTargetStateId('');
    setNewTargetBranch('');
    onChanged();
  };

  const handleDelete = async (ruleId: string) => {
    const res = await automationRuleService.remove(workspaceId, ruleId);
    if (res.error) { toast.error(res.error); return; }
    toast.success('Rule removed');
    onChanged();
  };

  const handleToggle = async (rule: AutomationRule) => {
    const res = await automationRuleService.update(workspaceId, rule.id, { enabled: !rule.enabled });
    if (res.error) { toast.error(res.error); return; }
    onChanged();
  };

  const agentName = (id: string) => agents.find((a) => a.id === id)?.name ?? 'Unknown agent';
  const stateFn = (id: string) => states.find((s) => s.id === id)?.name ?? 'Unknown state';

  const ruleDescription = (rule: AutomationRule) => {
    const trigger = TRIGGER_LABELS[rule.trigger_type] ?? rule.trigger_type;
    if (rule.action_type === 'start_agent_run') return `${trigger} → Run ${agentName(rule.action_config?.agent_id as string)}`;
    if (rule.action_type === 'move_to_state') return `${trigger} → Move to ${stateFn(rule.action_config?.target_state_id as string)}`;
    if (rule.action_type === 'merge_branch') return `${trigger} → Merge to ${rule.action_config?.target_branch as string}`;
    return `${trigger} → ${rule.action_type}`;
  };

  return (
    <div className="space-y-2">
      <Label className="flex items-center gap-1.5">
        <BotIcon className="h-3.5 w-3.5 text-violet-500" />
        Pipeline Rules
      </Label>
      <p className="text-xs text-muted-foreground">Automation rules triggered when tasks enter or are approved in this state.</p>

      {rules.length > 0 && (
        <div className="space-y-1">
          {rules.map((rule) => (
            <div key={rule.id} className="flex items-center gap-2 rounded-md border border-border px-2.5 py-1.5 text-xs">
              <PlayIcon className="h-3 w-3 shrink-0 text-violet-500" />
              <span className={cn('flex-1 truncate', !rule.enabled && 'opacity-50 line-through')}>
                {ruleDescription(rule)}
              </span>
              <button type="button" className="shrink-0 text-muted-foreground hover:text-foreground" onClick={() => handleToggle(rule)}>
                {rule.enabled ? 'On' : 'Off'}
              </button>
              <button type="button" className="shrink-0 text-muted-foreground hover:text-destructive" onClick={() => handleDelete(rule.id)}>
                <Cancel01Icon className="h-3 w-3" />
              </button>
            </div>
          ))}
        </div>
      )}

      {adding ? (
        <div className="space-y-2 rounded-md border border-border p-3">
          <div className="flex gap-2">
            <Select value={newTrigger} onValueChange={setNewTrigger}>
              <SelectTrigger className="h-7 flex-1 text-xs">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="task.state_entered">On state entry</SelectItem>
                <SelectItem value="agent_run.approved">On run approved</SelectItem>
              </SelectContent>
            </Select>
              <Select value={newAction} onValueChange={setNewAction}>
                <SelectTrigger className="h-7 flex-1 text-xs">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                <SelectItem value="start_agent_run">Run agent</SelectItem>
                <SelectItem value="move_to_state">Move to state</SelectItem>
                <SelectItem value="merge_branch">Merge branch</SelectItem>
                </SelectContent>
              </Select>
          </div>

          {newAction === 'start_agent_run' && (
            <Select value={newAgentId} onValueChange={setNewAgentId}>
              <SelectTrigger className="h-7 text-xs">
                <SelectValue placeholder="Select agent..." />
              </SelectTrigger>
              <SelectContent>
                {agents.map((agent) => (
                  <SelectItem key={agent.id} value={agent.id}>{agent.name}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          )}

          {newAction === 'move_to_state' && (
            <Select value={newTargetStateId} onValueChange={setNewTargetStateId}>
              <SelectTrigger className="h-7 text-xs">
                <SelectValue placeholder="Select target state..." />
              </SelectTrigger>
              <SelectContent>
                {states.filter((s) => s.id !== stateId).map((s) => (
                  <SelectItem key={s.id} value={s.id}>{s.name}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          )}

          {newAction === 'merge_branch' && (
            <Input
              className="h-7 text-xs"
              value={newTargetBranch}
              onChange={(e) => setNewTargetBranch(e.target.value)}
              placeholder="e.g. develop"
            />
          )}

          <div className="flex gap-2">
            <Button type="button" size="sm" className="h-7 text-xs" onClick={handleAdd} disabled={saving}>
              {saving ? 'Adding...' : 'Add rule'}
            </Button>
            <Button type="button" size="sm" variant="ghost" className="h-7 text-xs" onClick={() => setAdding(false)}>
              Cancel
            </Button>
          </div>
        </div>
      ) : (
        <Button type="button" variant="outline" size="sm" className="h-7 text-xs gap-1" onClick={() => setAdding(true)}>
          <PlusSignIcon className="h-3 w-3" /> Add rule
        </Button>
      )}
    </div>
  );
}

// PipelineBuilder is in its own file
import { PipelineBuilder } from './PipelineBuilder';

// ── Main component ──

interface WorkflowManagerProps {
  workspaceId: string;
  teams: WorkspaceTeam[];
  editable: boolean;
  initialWorkflowId?: string;
  initialTeamId?: string;
}

export function WorkflowManager({ workspaceId, teams, editable, initialWorkflowId, initialTeamId }: WorkflowManagerProps) {
  const [workflows, setWorkflows] = useState<WorkflowWithStates[]>([]);
  const [selectedId, setSelectedId] = useState<string>('');
  const [loading, setLoading] = useState(true);

  // Workflow dialog state
  const [workflowDialogOpen, setWorkflowDialogOpen] = useState(false);
  const [editWorkflow, setEditWorkflow] = useState<WorkflowWithStates | null>(null);
  const [wfName, setWfName] = useState('');
  const [wfDescription, setWfDescription] = useState('');
  const [wfTeamID, setWfTeamID] = useState<string>('none');
  const [wfAutoAssign, setWfAutoAssign] = useState(false);
  const [wfSaving, setWfSaving] = useState(false);
  const [duplicatingId, setDuplicatingId] = useState<string | null>(null);
  const [deleteWorkflowConfirm, setDeleteWorkflowConfirm] = useState<string | null>(null);

  // State dialog state
  const [stateDialogOpen, setStateDialogOpen] = useState(false);
  const [editState, setEditState] = useState<WorkflowState | null>(null);
  const [newStateType, setNewStateType] = useState<StateType>('unstarted');
  const [stateName, setStateName] = useState('');
  const [stateDescription, setStateDescription] = useState('');
  const [stateColor, setStateColor] = useState('');
  const [deleteStateConfirm, setDeleteStateConfirm] = useState(false);
  const [stateSaving, setStateSaving] = useState(false);

  // Automation rules state
  const [automationRules, setAutomationRules] = useState<AutomationRule[]>([]);
  const [agents, setAgents] = useState<Agent[]>([]);

  // --- Data loading ---

  const loadWorkflows = async () => {
    setLoading(true);
    const { data, error } = await pmWorkflowService.list(workspaceId);
    if (error) {
      toast.error(error);
      setWorkflows([]);
      setLoading(false);
      return;
    }
    const next = data ?? [];
    setWorkflows(next);
    setSelectedId((prev) => {
      if (initialWorkflowId && next.some((w) => w.workflow.id === initialWorkflowId)) return initialWorkflowId;
      if (initialTeamId) {
        const matchingWorkflow = next.find((w) => w.workflow.team_id === initialTeamId);
        if (matchingWorkflow) return matchingWorkflow.workflow.id;
      }
      if (prev && next.some((w) => w.workflow.id === prev)) return prev;
      return next[0]?.workflow.id ?? '';
    });
    setLoading(false);
  };

  useEffect(() => { loadWorkflows(); }, [workspaceId, initialWorkflowId, initialTeamId]);

  // Load automation rules + agents for the selected workflow
  const loadAutomationRules = useCallback(async () => {
    if (!selectedId) { setAutomationRules([]); return; }
    const res = await automationRuleService.listByWorkflow(workspaceId, selectedId);
    if (res.data) setAutomationRules(res.data);
  }, [workspaceId, selectedId]);

  useEffect(() => { loadAutomationRules(); }, [loadAutomationRules]);
  useEffect(() => {
    agentService.list(workspaceId).then((res) => { if (res.data) setAgents(res.data); });
  }, [workspaceId]);

  const llmAgents = useMemo(() => agents, [agents]);

  // Rules grouped by state_id for quick lookup
  const rulesByStateId = useMemo(() => {
    const map = new Map<string, AutomationRule[]>();
    for (const rule of automationRules) {
      const stateId = rule.trigger_config?.state_id;
      if (!stateId) continue;
      const list = map.get(stateId) ?? [];
      list.push(rule);
      map.set(stateId, list);
    }
    return map;
  }, [automationRules]);

  // --- Derived data ---

  const selected = workflows.find((w) => w.workflow.id === selectedId) ?? null;

  const sortedStates = selected
    ? [...selected.states].sort((a, b) => a.position - b.position)
    : [];

  const statesByType: Record<StateType, WorkflowState[]> = {
    backlog: sortedStates.filter((s) => s.state_type === 'backlog'),
    unstarted: sortedStates.filter((s) => s.state_type === 'unstarted'),
    started: sortedStates.filter((s) => s.state_type === 'started'),
    done: sortedStates.filter((s) => s.state_type === 'done'),
  };

  const findTeamName = (id?: string) => {
    if (!id) return 'All teams';
    return teams.find((t) => t.id === id)?.name ?? 'Unknown Team';
  };

  // --- Workflow CRUD ---

  const openCreateWorkflow = () => {
    setEditWorkflow(null);
    setWfName('');
    setWfDescription('');
    setWfTeamID(initialTeamId ?? 'none');
    setWfAutoAssign(false);
    setWorkflowDialogOpen(true);
  };

  const openEditWorkflow = (entry?: WorkflowWithStates) => {
    const target = entry ?? selected;
    if (!target) return;
    setEditWorkflow(target);
    setWfName(target.workflow.name);
    setWfDescription(target.workflow.description ?? '');
    setWfTeamID(target.workflow.team_id ?? 'none');
    setWfAutoAssign(target.workflow.auto_assign_owner);
    setWorkflowDialogOpen(true);
  };

  const handleSaveWorkflow = async (e: FormEvent) => {
    e.preventDefault();
    if (!wfName.trim()) return;
    setWfSaving(true);
    const normalizedTeamID = wfTeamID === 'none' ? undefined : wfTeamID;

    if (editWorkflow) {
      const { error } = await pmWorkflowService.update(workspaceId, editWorkflow.workflow.id, {
        name: wfName.trim(),
        description: wfDescription.trim() || undefined,
        team_id: normalizedTeamID,
        auto_assign_owner: wfAutoAssign,
      });
      if (error) toast.error(error);
      else {
        toast.success('Workflow updated');
        setWorkflowDialogOpen(false);
        await loadWorkflows();
      }
    } else {
      const { data, error } = await pmWorkflowService.create({
        workspace_id: workspaceId,
        name: wfName.trim(),
        description: wfDescription.trim() || undefined,
        team_id: normalizedTeamID,
        auto_assign_owner: wfAutoAssign,
      });
      if (error) toast.error(error);
      else {
        toast.success('Workflow created');
        setWorkflowDialogOpen(false);
        if (data) setSelectedId(data.workflow.id);
        await loadWorkflows();
      }
    }
    setWfSaving(false);
  };

  const handleDuplicateWorkflow = async (entry: WorkflowWithStates) => {
    setDuplicatingId(entry.workflow.id);
    const { data, error } = await pmWorkflowService.create({
      workspace_id: workspaceId,
      name: `${entry.workflow.name} (copy)`,
      description: entry.workflow.description ?? undefined,
      team_id: entry.workflow.team_id ?? undefined,
      auto_assign_owner: entry.workflow.auto_assign_owner,
    });
    if (error) { toast.error(error); setDuplicatingId(null); return; }
    if (data) {
      for (const state of entry.states) {
        await pmWorkflowService.createState(workspaceId, data.workflow.id, {
          name: state.name,
          state_type: state.state_type,
          description: state.description ?? undefined,
          color: state.color ?? undefined,
          is_default: state.is_default,
        });
      }
      setSelectedId(data.workflow.id);
    }
    toast.success('Workflow duplicated');
    await loadWorkflows();
    setDuplicatingId(null);
  };

  const handleDeleteWorkflow = async (workflowId: string) => {
    const { error } = await pmWorkflowService.remove(workspaceId, workflowId);
    if (error) toast.error(error);
    else {
      toast.success('Workflow deleted');
      if (selectedId === workflowId) setSelectedId('');
      await loadWorkflows();
    }
  };

  // --- State CRUD ---

  const orderedStateIDs = (workflow: WorkflowWithStates) =>
    STATE_TYPE_ORDER.flatMap((type) =>
      [...workflow.states]
        .filter((s) => s.state_type === type)
        .sort((a, b) => a.position - b.position)
        .map((s) => s.id)
    );

  const normalizeOrdering = async (workflow: WorkflowWithStates) => {
    const targetIDs = orderedStateIDs(workflow);
    const currentIDs = [...workflow.states].sort((a, b) => a.position - b.position).map((s) => s.id);
    if (targetIDs.length === currentIDs.length && targetIDs.every((id, idx) => currentIDs[idx] === id)) return;
    await pmWorkflowService.reorderStates(workspaceId, workflow.workflow.id, targetIDs);
  };

  const openCreateState = (type: StateType) => {
    setEditState(null);
    setNewStateType(type);
    setStateName('');
    setStateDescription('');
    setStateColor('');
    setStateDialogOpen(true);
  };

  const openEditState = (state: WorkflowState) => {
    setEditState(state);
    setNewStateType(state.state_type);
    setStateName(state.name);
    setStateDescription(state.description ?? '');
    setStateColor(state.color ?? '');
    setStateDialogOpen(true);
  };

  const handleSaveState = async (e: FormEvent) => {
    e.preventDefault();
    if (!selected || !stateName.trim()) return;
    setStateSaving(true);
    const payload = {
      name: stateName.trim(),
      state_type: newStateType,
      description: stateDescription.trim() || undefined,
      color: stateColor.trim() || undefined,
    };

    if (editState) {
      const { error } = await pmWorkflowService.updateState(workspaceId, selected.workflow.id, editState.id, payload);
      if (error) toast.error(error);
      else toast.success('State updated');
    } else {
      const { error } = await pmWorkflowService.createState(workspaceId, selected.workflow.id, payload);
      if (error) toast.error(error);
      else {
        toast.success('State created');
        const refreshed = await pmWorkflowService.get(workspaceId, selected.workflow.id);
        if (refreshed.data) await normalizeOrdering(refreshed.data);
      }
    }
    await loadWorkflows();
    setStateDialogOpen(false);
    setStateSaving(false);
  };

  const handleDeleteState = async () => {
    if (!selected || !editState) return;
    const { error } = await pmWorkflowService.removeState(workspaceId, selected.workflow.id, editState.id);
    if (error) toast.error(error);
    else {
      toast.success('State deleted');
      setStateDialogOpen(false);
      await loadWorkflows();
    }
  };

  const handleMoveWithinType = async (type: StateType, stateID: string, direction: 'up' | 'down') => {
    if (!selected) return;
    const typed = [...statesByType[type]];
    const idx = typed.findIndex((s) => s.id === stateID);
    if (idx < 0) return;
    const swapIdx = direction === 'up' ? idx - 1 : idx + 1;
    if (swapIdx < 0 || swapIdx >= typed.length) return;
    const copy = [...typed];
    const [current] = copy.splice(idx, 1);
    copy.splice(swapIdx, 0, current);

    const idsByType: Record<StateType, string[]> = {
      backlog: statesByType.backlog.map((s) => s.id),
      unstarted: statesByType.unstarted.map((s) => s.id),
      started: statesByType.started.map((s) => s.id),
      done: statesByType.done.map((s) => s.id),
    };
    idsByType[type] = copy.map((s) => s.id);
    const nextIDs = STATE_TYPE_ORDER.flatMap((t) => idsByType[t]);

    const { error } = await pmWorkflowService.reorderStates(workspaceId, selected.workflow.id, nextIDs);
    if (error) toast.error(error);
    else await loadWorkflows();
  };

  const handleSetDefault = async (stateId: string) => {
    if (!selected) return;
    const { error } = await pmWorkflowService.updateState(workspaceId, selected.workflow.id, stateId, { is_default: true });
    if (error) toast.error(error);
    else {
      toast.success('Default state updated');
      await loadWorkflows();
    }
  };

  // --- Render ---

  if (loading) {
    return <p className="text-sm text-muted-foreground text-center py-6">Loading workflows...</p>;
  }

  return (
    <div className="flex gap-6 min-h-[500px]">
      {/* Left panel — Workflow list */}
      <div className="w-[260px] shrink-0 space-y-3">
        <div className="flex items-center justify-between">
          <span className="text-sm font-medium text-muted-foreground">Workflows</span>
          {editable && (
            <Button variant="ghost" size="sm" className="h-7 gap-1 text-xs" onClick={openCreateWorkflow}>
              <PlusSignIcon className="h-3.5 w-3.5" /> New
            </Button>
          )}
        </div>

        <div className="space-y-1">
          {workflows.length === 0 ? (
            <p className="text-sm text-muted-foreground py-4 text-center">No workflows found.</p>
          ) : (
            workflows.map((entry) => (
              <div
                key={entry.workflow.id}
                onClick={() => setSelectedId(entry.workflow.id)}
                className={cn(
                  'group/card flex items-center gap-2 w-full rounded-md border px-3 py-2.5 text-left transition-colors cursor-pointer',
                  entry.workflow.id === selectedId
                    ? 'border-primary/30 bg-primary/5'
                    : 'border-transparent hover:bg-muted/50'
                )}
              >
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2">
                    {duplicatingId === entry.workflow.id ? (
                      <Loading01Icon className="h-3.5 w-3.5 shrink-0 text-muted-foreground animate-spin" />
                    ) : (
                      <GitBranchIcon className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                    )}
                    <span className="text-sm font-medium truncate">{entry.workflow.name}</span>
                  </div>
                  <div className="mt-0.5 ml-5.5 flex items-center gap-1.5 text-xs text-muted-foreground">
                    <span>{entry.states.length} states</span>
                    <span>&middot;</span>
                    <span className="truncate">{findTeamName(entry.workflow.team_id)}</span>
                  </div>
                </div>
                {editable && (
                  <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-6 w-6 shrink-0 opacity-0 group-hover/card:opacity-100 transition-opacity focus-visible:ring-0 focus-visible:ring-offset-0"
                        onClick={(e) => e.stopPropagation()}
                      >
                        <MoreVerticalIcon className="h-3.5 w-3.5" />
                      </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="end">
                      <DropdownMenuItem onClick={() => { setSelectedId(entry.workflow.id); openEditWorkflow(entry); }}>
                        <PencilEdit01Icon className="h-3.5 w-3.5 mr-2" /> Edit
                      </DropdownMenuItem>
                      <DropdownMenuItem onClick={() => handleDuplicateWorkflow(entry)}>
                        <Copy01Icon className="h-3.5 w-3.5 mr-2" /> Duplicate
                      </DropdownMenuItem>
                      {workflows.length <= 1 || (!entry.workflow.team_id && workflows.filter((w) => !w.workflow.team_id).length <= 1) ? (
                        <QuickTooltip label={!entry.workflow.team_id ? 'Must keep at least one default workflow' : 'You must have at least one workflow'} side="left">
                          <DropdownMenuItem
                            variant="destructive"
                            className="opacity-40 pointer-events-auto cursor-not-allowed"
                            onSelect={(e) => e.preventDefault()}
                          >
                            <Delete01Icon className="h-3.5 w-3.5 mr-2" /> Delete
                          </DropdownMenuItem>
                        </QuickTooltip>
                      ) : (
                        <DropdownMenuItem
                          variant="destructive"
                          onClick={() => setDeleteWorkflowConfirm(entry.workflow.id)}
                        >
                          <Delete01Icon className="h-3.5 w-3.5 mr-2" /> Delete
                        </DropdownMenuItem>
                      )}
                    </DropdownMenuContent>
                  </DropdownMenu>
                )}
              </div>
            ))
          )}
        </div>
      </div>

      {/* Right panel — Selected workflow detail */}
      <div className="flex-1 min-w-0">
        {!selected ? (
          <div className="flex items-center justify-center h-full text-muted-foreground text-sm">
            {workflows.length === 0 ? (
              <div className="text-center space-y-3">
                <p>No workflows yet.</p>
                {editable && (
                  <Button size="sm" onClick={openCreateWorkflow}>
                    <PlusSignIcon className="h-4 w-4 mr-1" /> Create your first workflow
                  </Button>
                )}
              </div>
            ) : (
              <p>Select a workflow from the list.</p>
            )}
          </div>
        ) : (
          <div className="space-y-6">
            {/* Pipeline builder */}
            <PipelineBuilder
              workspaceId={workspaceId}
              workflowId={selected.workflow.id}
              states={sortedStates}
              agents={llmAgents}
              rules={automationRules}
              editable={editable}
              onChanged={loadAutomationRules}
            />

            <Separator />

            {/* Breadcrumb header */}
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-1.5 text-sm min-w-0">
                <span className="text-muted-foreground truncate">{selected.workflow.name}</span>
                <span className="text-muted-foreground">/</span>
                <span className="font-semibold">States</span>
              </div>
            </div>

            {/* States */}
            <div className="space-y-0">
              {STATE_TYPE_ORDER.map((type, typeIdx) => (
                <section key={type} className={cn('space-y-2 pb-5', typeIdx > 0 && 'pt-3')}>
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-1.5 text-sm font-semibold tracking-tight">
                      <StateTypeIcon stateType={type} className="h-4 w-4" />
                      {STATE_TYPE_LABEL[type]}
                    </div>
                    {editable && (
                      <Button variant="ghost" size="sm" className="h-7 gap-1 text-xs" onClick={() => openCreateState(type)}>
                        <PlusSignIcon className="h-3.5 w-3.5" /> Add
                      </Button>
                    )}
                  </div>
                  {statesByType[type].length === 0 ? (
                    <div className="rounded-md border border-dashed border-border px-3 py-3 text-sm text-muted-foreground">
                      No states in this group.
                    </div>
                  ) : (
                    <div className="space-y-1">
                      {statesByType[type].map((state, idx) => (
                        <div key={state.id} className="group flex items-center justify-between gap-3 rounded-md border border-border px-3 py-2">
                          <div className="flex items-center gap-2 min-w-0">
                            {state.color && (
                              <span
                                className="h-3 w-3 rounded-full shrink-0"
                                style={{ backgroundColor: state.color }}
                              />
                            )}
                            {!state.color && (
                              <StateTypeIcon stateType={state.state_type} className="h-3.5 w-3.5 shrink-0" />
                            )}
                            <span className="text-sm font-medium truncate">{state.name}</span>
                            {state.is_default && (
                              <QuickTooltip label="New tasks are created in this state">
                                <Badge variant="secondary" className="text-xs gap-1 shrink-0 cursor-default">
                                  <Tick01Icon className="h-3 w-3" /> Default
                                </Badge>
                              </QuickTooltip>
                            )}
                            {(rulesByStateId.get(state.id)?.length ?? 0) > 0 && (
                              <QuickTooltip label={`${rulesByStateId.get(state.id)!.length} automation rule(s)`}>
                                <Badge variant="outline" className="text-xs gap-1 shrink-0 cursor-default border-violet-300 bg-violet-50 text-violet-600 dark:border-violet-800 dark:bg-violet-950/50 dark:text-violet-400">
                                  <BotIcon className="h-3 w-3" /> {rulesByStateId.get(state.id)!.length}
                                </Badge>
                              </QuickTooltip>
                            )}
                          </div>
                          <div className="flex items-center gap-0.5 shrink-0 opacity-0 group-hover:opacity-100 transition-opacity">
                            {!state.is_default && editable && (
                              <Button
                                variant="ghost"
                                size="sm"
                                className="h-6 text-xs text-muted-foreground"
                                onClick={() => handleSetDefault(state.id)}
                              >
                                Set as default
                              </Button>
                            )}
                            {editable && (
                              <>
                                <Button
                                  size="icon"
                                  variant="ghost"
                                  className="h-6 w-6"
                                  disabled={idx === 0}
                                  onClick={() => handleMoveWithinType(type, state.id, 'up')}
                                >
                                  <ArrowUp02Icon className="h-3 w-3" />
                                </Button>
                                <Button
                                  size="icon"
                                  variant="ghost"
                                  className="h-6 w-6"
                                  disabled={idx === statesByType[type].length - 1}
                                  onClick={() => handleMoveWithinType(type, state.id, 'down')}
                                >
                                  <ArrowDown02Icon className="h-3 w-3" />
                                </Button>
                                <Button size="icon" variant="ghost" className="h-6 w-6" onClick={() => openEditState(state)}>
                                  <PencilEdit01Icon className="h-3 w-3" />
                                </Button>
                              </>
                            )}
                          </div>
                        </div>
                      ))}
                    </div>
                  )}
                </section>
              ))}
            </div>
          </div>
        )}
      </div>

      {/* Workflow create/edit dialog */}
      <Dialog open={workflowDialogOpen} onOpenChange={setWorkflowDialogOpen}>
        <DialogContent>
          <form onSubmit={handleSaveWorkflow}>
            <DialogHeader>
              <DialogTitle>{editWorkflow ? 'Edit Workflow' : 'Create Workflow'}</DialogTitle>
            </DialogHeader>
            <div className="space-y-4 py-4">
              <div className="space-y-2">
                <Label>Name</Label>
                <Input value={wfName} onChange={(e) => setWfName(e.target.value)} placeholder="e.g. Engineering Team Workflow" required />
              </div>
              <div className="space-y-2">
                <Label>Description <span className="text-muted-foreground font-normal">(optional)</span></Label>
                <Input value={wfDescription} onChange={(e) => setWfDescription(e.target.value)} placeholder="e.g. Standard workflow for the engineering team" />
              </div>
              <div className="grid gap-2">
                <Label>Team</Label>
                <div className="grid grid-cols-2 gap-2">
                  <button
                    type="button"
                    onClick={() => setWfTeamID('none')}
                    className={`rounded-lg border p-3 text-left transition-colors ${
                      wfTeamID === 'none'
                        ? 'border-primary bg-primary/5 ring-1 ring-primary/20'
                        : 'border-border/60 hover:border-border hover:bg-muted/30'
                    }`}
                  >
                    <span className="text-sm font-medium">All teams</span>
                    <p className="mt-0.5 text-[11px] text-muted-foreground leading-snug">Available to everyone</p>
                  </button>
                  <button
                    type="button"
                    onClick={() => setWfTeamID(teams[0]?.id ?? 'none')}
                    className={`rounded-lg border p-3 text-left transition-colors ${
                      wfTeamID !== 'none'
                        ? 'border-primary bg-primary/5 ring-1 ring-primary/20'
                        : 'border-border/60 hover:border-border hover:bg-muted/30'
                    }`}
                  >
                    <span className="text-sm font-medium">Specific team</span>
                    <p className="mt-0.5 text-[11px] text-muted-foreground leading-snug">Only for a selected team</p>
                  </button>
                </div>
                {wfTeamID !== 'none' && (
                  teams.length === 0 ? (
                    <p className="text-xs text-muted-foreground italic">No teams created yet.</p>
                  ) : (
                    <div className="flex flex-wrap gap-1.5 mt-1">
                      {teams.map((team) => (
                        <button
                          key={team.id}
                          type="button"
                          onClick={() => setWfTeamID(team.id)}
                          className={`rounded-md border px-2.5 py-1 text-xs transition-colors ${
                            wfTeamID === team.id
                              ? 'border-primary bg-primary/10 text-primary font-medium'
                              : 'border-border text-muted-foreground hover:border-primary/50 hover:text-foreground'
                          }`}
                        >
                          {team.name}
                        </button>
                      ))}
                    </div>
                  )
                )}
              </div>
              <div className="flex items-center justify-between rounded-md border border-border px-3 py-2">
                <div>
                  <Label>Auto assign owner when moved to started state</Label>
                  <p className="text-xs text-muted-foreground">Assign current user when task enters a started state and has no owner.</p>
                </div>
                <Switch checked={wfAutoAssign} onCheckedChange={setWfAutoAssign} />
              </div>
            </div>
            <DialogFooter>
              <Button type="submit" disabled={wfSaving}>{wfSaving ? 'Saving...' : 'Save'}</Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      {/* State create/edit dialog */}
      <Dialog open={stateDialogOpen} onOpenChange={setStateDialogOpen}>
        <DialogContent>
          <form onSubmit={handleSaveState}>
            <DialogHeader>
              <DialogTitle>{editState ? 'Edit State' : 'Add State'}</DialogTitle>
            </DialogHeader>
            <div className="space-y-4 py-4">
              <div className="space-y-2">
                <Label>State Type</Label>
                <Input value={STATE_TYPE_LABEL[newStateType]} disabled />
              </div>
              <div className="space-y-2">
                <Label>Name</Label>
                <Input value={stateName} onChange={(e) => setStateName(e.target.value)} placeholder="e.g. In Review" required />
              </div>
              <div className="space-y-2">
                <Label>Description <span className="text-muted-foreground font-normal">(optional)</span></Label>
                <Input value={stateDescription} onChange={(e) => setStateDescription(e.target.value)} placeholder="e.g. Waiting for peer code review" />
                <p className="text-xs text-muted-foreground">Shown as a tooltip on the board column header.</p>
              </div>
              <div className="space-y-2">
                <Label>Color <span className="text-muted-foreground font-normal">(optional)</span></Label>
                <ColorPicker value={stateColor || PRESET_COLORS[0]} onChange={setStateColor} />
              </div>

              {/* Pipeline Automation Rules */}
              {editState && editable && (
                <PipelineRulesSection
                  workspaceId={workspaceId}
                  workflowId={selected?.workflow.id ?? ''}
                  stateId={editState.id}
                  stateName={editState.name}
                  rules={rulesByStateId.get(editState.id) ?? []}
                  agents={llmAgents}
                  states={sortedStates}
                  onChanged={loadAutomationRules}
                />
              )}
            </div>
            <DialogFooter className="justify-between">
              <div>
                {editState && editable && (
                  <Button type="button" variant="ghost" className="text-destructive" onClick={() => setDeleteStateConfirm(true)}>
                    <Delete01Icon className="h-3.5 w-3.5 mr-1" /> Delete State
                  </Button>
                )}
              </div>
              <Button type="submit" disabled={stateSaving || !editable}>{stateSaving ? 'Saving...' : 'Save'}</Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      {/* Delete workflow confirm */}
      <ConfirmDialog
        open={deleteWorkflowConfirm !== null}
        onOpenChange={(open) => { if (!open) setDeleteWorkflowConfirm(null); }}
        title="Delete workflow"
        description="This will permanently delete the workflow and all its states. Tasks using this workflow will need to be reassigned. This action cannot be undone."
        confirmLabel="Delete"
        variant="destructive"
        onConfirm={() => { if (deleteWorkflowConfirm) handleDeleteWorkflow(deleteWorkflowConfirm); setDeleteWorkflowConfirm(null); }}
      />

      {/* Delete state confirm */}
      <ConfirmDialog
        open={deleteStateConfirm}
        onOpenChange={setDeleteStateConfirm}
        title="Delete workflow state"
        description="This will permanently delete this state. Tasks in this state will need to be moved to another state. This action cannot be undone."
        confirmLabel="Delete"
        variant="destructive"
        onConfirm={() => { handleDeleteState(); setDeleteStateConfirm(false); }}
      />
    </div>
  );
}
