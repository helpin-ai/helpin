import { useMemo, useState, type ReactNode } from 'react';
import { toast } from 'sonner';
import { BASE_BRANCH_TOKEN, describeMergeDestination } from '@/lib/branchLabels';
import { automationRuleService } from '@/lib/services/automationRuleService';
import { pmWorkflowService } from '@/lib/services/pmWorkflowService';
import { StateTypeIcon } from '@/lib/pmConstants';
import type { Agent, AutomationRule, StateType, WorkflowState, WorkflowWithStates } from '@/lib/pmTypes';
import { ColorPicker, PRESET_COLORS } from '@/components/pm/ColorPicker';
import { ConfirmDialog } from '@/components/pm/ConfirmDialog';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Switch } from '@/components/ui/switch';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { BotIcon, GitBranchIcon, Cancel01Icon, CheckmarkCircle02Icon, Loading01Icon, ZapIcon, PlusSignIcon, Delete01Icon, PencilEdit01Icon, Tick01Icon } from '@/lib/icons';
import { cn } from '@/lib/utils';

const STATE_TYPE_ORDER: StateType[] = ['backlog', 'unstarted', 'started', 'done'];
const STATE_TYPE_LABEL: Record<StateType, string> = {
  backlog: 'Backlog',
  unstarted: 'Not started',
  started: 'Started',
  done: 'Done',
};

type PipelineBuilderProps = {
  workspaceId: string;
  workflowId?: string;
  states?: WorkflowState[];
  workflow?: WorkflowWithStates;
  agents: Agent[];
  rules: AutomationRule[];
  editable: boolean;
  onChanged: () => void;
  onWorkflowUpdate?: (updated: WorkflowWithStates) => void;
};

export function PipelineBuilder(props: PipelineBuilderProps) {
  const { workspaceId, agents, rules, editable, onChanged, workflow, onWorkflowUpdate } = props;
  const workflowId = workflow?.workflow.id ?? props.workflowId;
  const states = useMemo(
    () => (workflow?.states ?? props.states ?? []).slice().sort((a, b) => a.position - b.position),
    [props.states, workflow?.states],
  );
  const [saving, setSaving] = useState<string | null>(null);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editName, setEditName] = useState('');
  const [addingAfterId, setAddingAfterId] = useState<string | null>(null);
  const [newName, setNewName] = useState('');
  const [newColor, setNewColor] = useState(PRESET_COLORS[0]);
  const [newStateType, setNewStateType] = useState<StateType>('unstarted');
  const [deleteConfirm, setDeleteConfirm] = useState<string | null>(null);
  const canManageStates = !!workflow && !!onWorkflowUpdate && !!workflowId;

  const stateRuleMap = useMemo(() => {
    const map = new Map<string, { runRule?: AutomationRule; advanceRule?: AutomationRule; mergeRule?: AutomationRule }>();
    for (const state of states) {
      map.set(state.id, {});
    }
    for (const rule of rules) {
      const stateId = rule.trigger_config?.state_id;
      if (!stateId || !map.has(stateId)) continue;
      const entry = map.get(stateId)!;
      if (rule.trigger_type === 'task.state_entered' && rule.action_type === 'start_agent_run') {
        entry.runRule = rule;
      } else if ((rule.trigger_type === 'agent_run.completed' || rule.trigger_type === 'agent_run.approved') && rule.action_type === 'move_to_state') {
        entry.advanceRule = rule;
      } else if (rule.trigger_type === 'task.state_entered' && rule.action_type === 'merge_branch') {
        entry.mergeRule = rule;
      }
    }
    return map;
  }, [states, rules]);

  const configuredCount = useMemo(() => {
    let count = 0;
    for (const entry of stateRuleMap.values()) {
      if (entry.runRule || entry.advanceRule || entry.mergeRule) count += 1;
    }
    return count;
  }, [stateRuleMap]);

  const handleAgentChange = async (stateId: string, stateName: string, agentId: string) => {
    if (!workflowId) return;
    setSaving(stateId);
    const entry = stateRuleMap.get(stateId);
    const existing = entry?.runRule;
    const existingAdvance = entry?.advanceRule;

    try {
      if (!agentId) {
        if (existing) {
          const { error } = await automationRuleService.remove(workspaceId, existing.id);
          if (error) {
            toast.error(error);
            return;
          }
        }
        if (existingAdvance) {
          const { error } = await automationRuleService.remove(workspaceId, existingAdvance.id);
          if (error) {
            toast.error(error);
            return;
          }
        }
      } else if (existing) {
        const { error } = await automationRuleService.update(workspaceId, existing.id, {
          action_type: 'start_agent_run',
          action_config: { agent_id: agentId },
        });
        if (error) {
          toast.error(error);
          return;
        }
      } else {
        const { error } = await automationRuleService.create(workspaceId, {
          workspace_id: workspaceId,
          name: `Run agent on ${stateName}`,
          workflow_id: workflowId,
          trigger_type: 'task.state_entered',
          trigger_config: { state_id: stateId },
          action_type: 'start_agent_run',
          action_config: { agent_id: agentId },
        });
        if (error) {
          toast.error(error);
          return;
        }
      }
      onChanged();
    } finally {
      setSaving(null);
    }
  };

  const handleMoveOnCompletionChange = async (stateId: string, stateName: string, targetStateId: string | null) => {
    if (!workflowId) return;
    setSaving(stateId);
    const existing = stateRuleMap.get(stateId)?.advanceRule;

    try {
      if (!targetStateId && existing) {
        const { error } = await automationRuleService.remove(workspaceId, existing.id);
        if (error) {
          toast.error(error);
          return;
        }
      } else if (targetStateId && existing) {
        const { error } = await automationRuleService.update(workspaceId, existing.id, {
          trigger_type: 'agent_run.completed',
          trigger_config: { state_id: stateId },
          action_type: 'move_to_state',
          action_config: { target_state_id: targetStateId },
        });
        if (error) {
          toast.error(error);
          return;
        }
      } else if (targetStateId) {
        const { error } = await automationRuleService.create(workspaceId, {
          workspace_id: workspaceId,
          name: `Move task after agent completes in ${stateName}`,
          workflow_id: workflowId,
          trigger_type: 'agent_run.completed',
          trigger_config: { state_id: stateId },
          action_type: 'move_to_state',
          action_config: { target_state_id: targetStateId },
        });
        if (error) {
          toast.error(error);
          return;
        }
      }
      onChanged();
    } finally {
      setSaving(null);
    }
  };

  const handleMergeBranchToggle = async (stateId: string, stateName: string, branch: string) => {
    if (!workflowId) return;
    setSaving(stateId);
    const existing = stateRuleMap.get(stateId)?.mergeRule;

    try {
      if (!branch && existing) {
        const { error } = await automationRuleService.remove(workspaceId, existing.id);
        if (error) {
          toast.error(error);
          return;
        }
      } else if (branch && existing) {
        const { error } = await automationRuleService.update(workspaceId, existing.id, { action_config: { target_branch: branch } });
        if (error) {
          toast.error(error);
          return;
        }
      } else if (branch) {
        const { error } = await automationRuleService.create(workspaceId, {
          workspace_id: workspaceId,
          name: `Merge task branch on ${stateName}`,
          workflow_id: workflowId,
          trigger_type: 'task.state_entered',
          trigger_config: { state_id: stateId },
          action_type: 'merge_branch',
          action_config: { target_branch: branch },
        });
        if (error) {
          toast.error(error);
          return;
        }
      }
      onChanged();
    } finally {
      setSaving(null);
    }
  };

  const updateWorkflowState = (stateId: string, updates: Partial<WorkflowState>) => {
    if (!workflow || !onWorkflowUpdate) return;
    onWorkflowUpdate({
      ...workflow,
      states: workflow.states.map((state) => state.id === stateId ? { ...state, ...updates } : state),
    });
  };

  const handleRename = async (state: WorkflowState) => {
    if (!workflowId) return;
    const trimmed = editName.trim();
    if (!trimmed || trimmed === state.name) {
      setEditingId(null);
      return;
    }
    setSaving(state.id);
    const { error } = await pmWorkflowService.updateState(workspaceId, workflowId, state.id, { name: trimmed });
    setSaving(null);
    if (error) {
      toast.error(error);
      return;
    }
    updateWorkflowState(state.id, { name: trimmed });
    setEditingId(null);
  };

  const handleStateTypeChange = async (state: WorkflowState, stateType: StateType) => {
    if (!workflowId || stateType === state.state_type) return;
    setSaving(state.id);
    const { error } = await pmWorkflowService.updateState(workspaceId, workflowId, state.id, { state_type: stateType });
    setSaving(null);
    if (error) {
      toast.error(error);
      return;
    }
    updateWorkflowState(state.id, { state_type: stateType });
  };

  const handleColorChange = async (state: WorkflowState, color: string) => {
    if (!workflowId) return;
    setSaving(state.id);
    const { error } = await pmWorkflowService.updateState(workspaceId, workflowId, state.id, { color });
    setSaving(null);
    if (error) {
      toast.error(error);
      return;
    }
    updateWorkflowState(state.id, { color });
  };

  const openAddState = (afterState?: WorkflowState) => {
    setAddingAfterId(afterState?.id ?? '__end__');
    setNewName('');
    setNewStateType(afterState?.state_type ?? states[states.length - 1]?.state_type ?? 'unstarted');
    setNewColor(PRESET_COLORS[Math.floor(Math.random() * PRESET_COLORS.length)]);
  };

  const handleAddState = async () => {
    if (!workflow || !onWorkflowUpdate || !workflowId) return;
    const trimmed = newName.trim();
    if (!trimmed) return;
    const afterIndex = states.findIndex((state) => state.id === addingAfterId);
    const afterState = afterIndex >= 0 ? states[afterIndex] : states[states.length - 1];
    const position = afterState ? afterState.position + 1 : states.length;

    setSaving('new-state');
    const { data, error } = await pmWorkflowService.createState(workspaceId, workflowId, {
      name: trimmed,
      state_type: newStateType,
      position,
      color: newColor,
    });
    setSaving(null);
    if (error) {
      toast.error(error);
      return;
    }
    if (data && typeof data === 'object' && 'id' in data && 'workflow_id' in data) {
      onWorkflowUpdate({ ...workflow, states: [...workflow.states, data as WorkflowState] });
    }
    setAddingAfterId(null);
    setNewName('');
  };

  const handleDeleteState = async (stateId: string) => {
    if (!workflow || !onWorkflowUpdate || !workflowId) return;
    setSaving(stateId);
    const { error } = await pmWorkflowService.removeState(workspaceId, workflowId, stateId);
    setSaving(null);
    if (error) {
      toast.error(error);
      return;
    }
    onWorkflowUpdate({ ...workflow, states: workflow.states.filter((state) => state.id !== stateId) });
    setDeleteConfirm(null);
  };

  if (states.length === 0) return null;

  if (canManageStates) {
    return (
      <section className="space-y-3">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div className="min-w-0">
            <h3 className="text-sm font-semibold">Workflow</h3>
            <p className="mt-1 text-xs text-muted-foreground">
              Manage states, types, and the automation that runs when tasks enter each state.
            </p>
          </div>
          <div className="rounded-full border border-border bg-muted/30 px-2.5 py-1 text-xs text-muted-foreground">
            {configuredCount}/{states.length} automated
          </div>
        </div>

        <div className="overflow-hidden rounded-lg border border-border bg-card">
          <div className="hidden border-b border-border bg-muted/30 px-3 py-2 text-[11px] font-medium uppercase tracking-wide text-muted-foreground lg:grid lg:grid-cols-[minmax(170px,1fr)_122px_minmax(340px,2fr)_minmax(220px,1.15fr)_64px] lg:gap-3">
            <div>State</div>
            <div>Type</div>
            <div>Automation</div>
            <div>Branch</div>
            <div className="text-right">Actions</div>
          </div>

          <div className="divide-y divide-border">
            {states.map((state, idx) => {
              const entry = stateRuleMap.get(state.id);
              const runRule = entry?.runRule;
              const selectedAgentId = (runRule?.action_config?.agent_id as string) ?? '';
              const selectedAgent = agents.find((agent) => agent.id === selectedAgentId);
              const hasAdvance = !!entry?.advanceRule;
              const selectedTargetStateId = (entry?.advanceRule?.action_config?.target_state_id as string | undefined) ?? '';
              const targetState = states.find((candidate) => candidate.id === selectedTargetStateId) ?? null;
              const mergeBranch = (entry?.mergeRule?.action_config?.target_branch as string) ?? '';
              const isSaving = saving === state.id;
              const nextState = states[idx + 1];
              const defaultTargetState = nextState ?? states.find((candidate) => candidate.id !== state.id) ?? null;
              const destinationOptions = states.filter((candidate) => candidate.id !== state.id);

              return (
                <div key={state.id}>
                  <div
                    className={cn(
                      'grid gap-3 px-3 py-3 transition-colors lg:grid-cols-[minmax(170px,1fr)_122px_minmax(340px,2fr)_minmax(220px,1.15fr)_64px] lg:items-start',
                      isSaving && 'opacity-70',
                    )}
                  >
                    <div className="min-w-0">
                      <div className="mb-1 text-[11px] font-medium uppercase tracking-wide text-muted-foreground lg:hidden">State</div>
                      <div className="flex min-w-0 items-center gap-2">
                        <Popover>
                          <PopoverTrigger asChild>
                            <button
                              type="button"
                              className="h-3.5 w-3.5 shrink-0 rounded-full border border-border/60 transition-transform hover:scale-110 disabled:pointer-events-none"
                              style={{ backgroundColor: state.color ?? '#9ca3af' }}
                              disabled={!editable}
                              aria-label="Change state color"
                            />
                          </PopoverTrigger>
                          {editable && (
                            <PopoverContent className="w-auto p-2" align="start">
                              <ColorPicker value={state.color ?? '#9ca3af'} onChange={(color) => void handleColorChange(state, color)} />
                            </PopoverContent>
                          )}
                        </Popover>
                        {editingId === state.id ? (
                          <Input
                            autoFocus
                            value={editName}
                            onChange={(event) => setEditName(event.target.value)}
                            onBlur={() => void handleRename(state)}
                            onKeyDown={(event) => {
                              if (event.key === 'Enter') void handleRename(state);
                              if (event.key === 'Escape') setEditingId(null);
                            }}
                            className="h-8 min-w-0 text-sm"
                            disabled={isSaving}
                          />
                        ) : (
                          <button
                            type="button"
                            className="min-w-0 truncate text-left text-sm font-medium disabled:pointer-events-none"
                            disabled={!editable}
                            onClick={() => {
                              setEditingId(state.id);
                              setEditName(state.name);
                            }}
                          >
                            {state.name}
                          </button>
                        )}
                      </div>
                    </div>

                    <div>
                      <div className="mb-1 text-[11px] font-medium uppercase tracking-wide text-muted-foreground lg:hidden">Type</div>
                      <Select
                        value={state.state_type}
                        onValueChange={(value) => void handleStateTypeChange(state, value as StateType)}
                        disabled={!editable || isSaving}
                      >
                        <SelectTrigger className="h-8 text-xs">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          {STATE_TYPE_ORDER.map((type) => (
                            <SelectItem key={type} value={type}>
                              <span className="flex items-center gap-2">
                                <StateTypeIcon stateType={type} className="h-3.5 w-3.5" />
                                {STATE_TYPE_LABEL[type]}
                              </span>
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </div>

                    <div className="min-w-0 space-y-2">
                      <div className="mb-1 text-[11px] font-medium uppercase tracking-wide text-muted-foreground lg:hidden">Automation</div>
                      <div className="flex flex-wrap items-center gap-2">
                        <span className="w-28 text-xs font-medium text-muted-foreground">On task entry, start</span>
                        {editable ? (
                          <Select
                            value={selectedAgentId || '__none__'}
                            onValueChange={(v) => void handleAgentChange(state.id, state.name, v === '__none__' ? '' : v)}
                            disabled={isSaving}
                          >
                            <SelectTrigger className="h-8 min-w-44 max-w-full text-xs">
                              <SelectValue placeholder="No agent" />
                            </SelectTrigger>
                            <SelectContent>
                              <SelectItem value="__none__">
                                <span className="text-muted-foreground">No agent</span>
                              </SelectItem>
                              {agents.map((agent) => (
                                <SelectItem key={agent.id} value={agent.id}>
                                  {agent.name}
                                </SelectItem>
                              ))}
                            </SelectContent>
                          </Select>
                        ) : (
                          <span className="truncate text-xs text-muted-foreground">{selectedAgent?.name ?? 'No agent'}</span>
                        )}
                      </div>

                      <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
                        <span className="w-40 font-medium">On run completion, move to</span>
                        {editable ? (
                          <Select
                            value={selectedTargetStateId || '__none__'}
                            onValueChange={(value) => void handleMoveOnCompletionChange(state.id, state.name, value === '__none__' ? null : value)}
                            disabled={!selectedAgentId || isSaving || destinationOptions.length === 0}
                          >
                            <SelectTrigger className="h-8 min-w-36 max-w-full text-xs text-foreground">
                              <SelectValue placeholder="State" />
                            </SelectTrigger>
                            <SelectContent>
                              <SelectItem value="__none__">
                                <span className="text-muted-foreground">State</span>
                              </SelectItem>
                              {destinationOptions.map((candidate) => (
                                <SelectItem key={candidate.id} value={candidate.id}>
                                  {candidate.name}
                                </SelectItem>
                              ))}
                            </SelectContent>
                          </Select>
                        ) : (
                          <span className="font-medium text-foreground">{hasAdvance ? targetState?.name ?? defaultTargetState?.name ?? 'State' : 'State'}</span>
                        )}
                      </div>
                    </div>

                    <div className="min-w-0">
                      <div className="mb-1 text-[11px] font-medium uppercase tracking-wide text-muted-foreground lg:hidden">Branch</div>
                      <div className="space-y-1">
                        <p className="text-xs font-medium text-muted-foreground">On task entry, merge into</p>
                        <MergeBranchInput
                          value={mergeBranch}
                          editable={editable && !isSaving}
                          onChange={(v) => void handleMergeBranchToggle(state.id, state.name, v)}
                        />
                      </div>
                    </div>

                    <div className="flex items-center justify-end gap-1">
                      {isSaving ? <Loading01Icon className="h-3.5 w-3.5 animate-spin text-muted-foreground" /> : null}
                      {editable && (
                        <>
                          <button
                            type="button"
                            className="flex h-7 w-7 items-center justify-center rounded-md text-muted-foreground hover:bg-muted hover:text-foreground"
                            onClick={() => {
                              setEditingId(state.id);
                              setEditName(state.name);
                            }}
                            aria-label="Rename state"
                          >
                            <PencilEdit01Icon className="h-3.5 w-3.5" />
                          </button>
                          {states.length > 1 && (
                            <button
                              type="button"
                              className="flex h-7 w-7 items-center justify-center rounded-md text-muted-foreground hover:bg-muted hover:text-destructive"
                              onClick={() => setDeleteConfirm(state.id)}
                              aria-label="Delete state"
                            >
                              <Delete01Icon className="h-3.5 w-3.5" />
                            </button>
                          )}
                        </>
                      )}
                    </div>
                  </div>

                  {editable && idx < states.length - 1 && addingAfterId !== state.id && (
                    <button
                      type="button"
                      className="group flex h-7 w-full items-center justify-center gap-1 text-xs text-muted-foreground transition-colors hover:bg-muted/50 hover:text-foreground"
                      onClick={() => openAddState(state)}
                    >
                      <PlusSignIcon className="h-3 w-3 opacity-60 group-hover:opacity-100" />
                      Add state here
                    </button>
                  )}

                  {editable && addingAfterId === state.id && (
                    <AddStateRow
                      name={newName}
                      color={newColor}
                      stateType={newStateType}
                      saving={saving === 'new-state'}
                      onNameChange={setNewName}
                      onColorChange={setNewColor}
                      onStateTypeChange={setNewStateType}
                      onCancel={() => setAddingAfterId(null)}
                      onSave={() => void handleAddState()}
                    />
                  )}
                </div>
              );
            })}

            {editable && addingAfterId === '__end__' && (
              <AddStateRow
                name={newName}
                color={newColor}
                stateType={newStateType}
                saving={saving === 'new-state'}
                onNameChange={setNewName}
                onColorChange={setNewColor}
                onStateTypeChange={setNewStateType}
                onCancel={() => setAddingAfterId(null)}
                onSave={() => void handleAddState()}
              />
            )}
          </div>
        </div>

        {editable && addingAfterId === null && (
          <Button type="button" variant="outline" size="sm" className="h-8 gap-1.5 text-xs" onClick={() => openAddState()}>
            <PlusSignIcon className="h-3.5 w-3.5" />
            Add state
          </Button>
        )}

        {!editable && (
          <p className="text-xs text-muted-foreground">You don't have permission to edit this workflow.</p>
        )}

        <ConfirmDialog
          open={deleteConfirm !== null}
          onOpenChange={(open) => {
            if (!open) setDeleteConfirm(null);
          }}
          title="Delete state"
          description="Tasks in this state will need to be moved to another state. This cannot be undone."
          confirmLabel="Delete"
          variant="destructive"
          onConfirm={() => {
            if (deleteConfirm) void handleDeleteState(deleteConfirm);
          }}
        />
      </section>
    );
  }

  return (
    <section className="space-y-3">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="flex items-center gap-2">
            <ZapIcon className="h-4 w-4 text-violet-500" />
            <h3 className="text-sm font-semibold">Pipeline automation</h3>
          </div>
          <p className="mt-1 text-xs text-muted-foreground">
            Configure what happens when tasks enter each workflow state.
          </p>
        </div>
        <div className="rounded-full border border-border bg-muted/30 px-2.5 py-1 text-xs text-muted-foreground">
          {configuredCount}/{states.length} configured
        </div>
      </div>

      <div className="space-y-2">
        {states.map((state, idx) => {
          const entry = stateRuleMap.get(state.id);
          const runRule = entry?.runRule;
          const selectedAgentId = (runRule?.action_config?.agent_id as string) ?? '';
          const selectedAgent = agents.find((agent) => agent.id === selectedAgentId);
          const hasAdvance = !!entry?.advanceRule;
          const selectedTargetStateId = (entry?.advanceRule?.action_config?.target_state_id as string | undefined) ?? '';
          const targetState = states.find((candidate) => candidate.id === selectedTargetStateId) ?? null;
          const mergeBranch = (entry?.mergeRule?.action_config?.target_branch as string) ?? '';
          const isSaving = saving === state.id;
          const hasExecution = !!selectedAgentId;
          const nextState = states[idx + 1];
          const defaultTargetState = nextState ?? states.find((candidate) => candidate.id !== state.id) ?? null;

          return (
            <div
              key={state.id}
              className={cn(
                'rounded-lg border bg-card transition-colors',
                hasExecution || mergeBranch ? 'border-violet-300/70 dark:border-violet-800/70' : 'border-border',
                isSaving && 'opacity-70',
              )}
            >
              <div className="grid gap-3 p-3 lg:grid-cols-[minmax(170px,230px)_1fr]">
                <div className="min-w-0 space-y-2">
                  <div className="flex min-w-0 items-center gap-2">
                    <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-md bg-muted text-[11px] font-medium text-muted-foreground">
                      {idx + 1}
                    </span>
                    {state.color ? (
                      <span className="h-2.5 w-2.5 shrink-0 rounded-full" style={{ backgroundColor: state.color }} />
                    ) : (
                      <StateTypeIcon stateType={state.state_type} className="h-3.5 w-3.5 shrink-0" />
                    )}
                    <span className="truncate text-sm font-medium">{state.name}</span>
                    {isSaving ? <Loading01Icon className="h-3.5 w-3.5 shrink-0 animate-spin text-muted-foreground" /> : null}
                  </div>
                  <div className="flex flex-wrap gap-1.5 pl-8">
                    {hasExecution ? <StatusPill tone="violet">Agent</StatusPill> : null}
                    {hasAdvance ? <StatusPill tone="green">Advances</StatusPill> : null}
                    {mergeBranch ? <StatusPill tone="blue">Merge</StatusPill> : null}
                    {!hasExecution && !hasAdvance && !mergeBranch ? <StatusPill>Manual</StatusPill> : null}
                  </div>
                </div>

                <div className="grid min-w-0 gap-2 md:grid-cols-[minmax(220px,1fr)_minmax(180px,240px)] xl:grid-cols-[minmax(240px,1fr)_220px_minmax(190px,240px)]">
                  <div className="min-w-0 rounded-md border border-border/60 bg-background/60 p-2.5">
                    <div className="mb-2 flex items-center gap-1.5 text-xs font-medium">
                      <BotIcon className="h-3.5 w-3.5 text-violet-500" />
                      Run agent
                    </div>
                    {editable ? (
                      <Select
                        value={selectedAgentId || '__none__'}
                        onValueChange={(v) => void handleAgentChange(state.id, state.name, v === '__none__' ? '' : v)}
                        disabled={isSaving}
                      >
                        <SelectTrigger className="h-8 w-full text-xs">
                          <SelectValue placeholder="No agent" />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="__none__">
                            <span className="text-muted-foreground">No agent</span>
                          </SelectItem>
                          {agents.map((agent) => (
                            <SelectItem key={agent.id} value={agent.id}>
                              {agent.name}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    ) : (
                      <p className="truncate text-xs text-muted-foreground">{selectedAgent?.name ?? 'No agent'}</p>
                    )}
                  </div>

                  <div className="rounded-md border border-border/60 bg-background/60 p-2.5">
                    <div className="mb-2 flex items-center gap-1.5 text-xs font-medium">
                      <CheckmarkCircle02Icon className="h-3.5 w-3.5 text-emerald-500" />
                      Move when done
                    </div>
                    <div className="flex items-center justify-between gap-3">
                      <div className="min-w-0">
                        <p className="truncate text-xs text-muted-foreground">
                          {hasAdvance
                            ? `Move to ${targetState?.name ?? defaultTargetState?.name ?? 'next state'} when agent run completes`
                            : defaultTargetState
                              ? `Can move to ${defaultTargetState.name} when agent run completes`
                              : 'No other state'}
                        </p>
                      </div>
                      <Switch
                        checked={hasAdvance}
                        onCheckedChange={(checked) => {
                          const fallbackTargetId = selectedTargetStateId || defaultTargetState?.id || '';
                          void handleMoveOnCompletionChange(state.id, state.name, checked ? fallbackTargetId : null);
                        }}
                        disabled={!editable || !hasExecution || !defaultTargetState || isSaving}
                      />
                    </div>
                  </div>

                  <div className="rounded-md border border-border/60 bg-background/60 p-2.5 md:col-span-2 xl:col-span-1">
                    <div className="mb-2 flex items-center gap-1.5 text-xs font-medium">
                      <GitBranchIcon className="h-3.5 w-3.5 text-sky-500" />
                      Merge branch
                    </div>
                    <MergeBranchInput
                      value={mergeBranch}
                      editable={editable && !isSaving}
                      onChange={(v) => void handleMergeBranchToggle(state.id, state.name, v)}
                    />
                  </div>
                </div>
              </div>
            </div>
          );
        })}
      </div>
    </section>
  );
}

function AddStateRow({
  name,
  color,
  stateType,
  saving,
  onNameChange,
  onColorChange,
  onStateTypeChange,
  onCancel,
  onSave,
}: {
  name: string;
  color: string;
  stateType: StateType;
  saving: boolean;
  onNameChange: (value: string) => void;
  onColorChange: (value: string) => void;
  onStateTypeChange: (value: StateType) => void;
  onCancel: () => void;
  onSave: () => void;
}) {
  return (
    <div className="grid gap-3 border-t border-border bg-muted/20 px-3 py-3 lg:grid-cols-[minmax(170px,1fr)_122px_minmax(360px,2.4fr)_64px] lg:items-center">
      <div className="flex min-w-0 items-center gap-2">
        <Popover>
          <PopoverTrigger asChild>
            <button
              type="button"
              className="h-3.5 w-3.5 shrink-0 rounded-full border border-border/60 transition-transform hover:scale-110"
              style={{ backgroundColor: color }}
              aria-label="Choose state color"
            />
          </PopoverTrigger>
          <PopoverContent className="w-auto p-2" align="start">
            <ColorPicker value={color} onChange={onColorChange} />
          </PopoverContent>
        </Popover>
        <Input
          autoFocus
          value={name}
          onChange={(event) => onNameChange(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === 'Enter') onSave();
            if (event.key === 'Escape') onCancel();
          }}
          placeholder="State name"
          className="h-8 min-w-0 text-sm"
          disabled={saving}
        />
      </div>

      <Select value={stateType} onValueChange={(value) => onStateTypeChange(value as StateType)} disabled={saving}>
        <SelectTrigger className="h-8 text-xs">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {STATE_TYPE_ORDER.map((type) => (
            <SelectItem key={type} value={type}>
              <span className="flex items-center gap-2">
                <StateTypeIcon stateType={type} className="h-3.5 w-3.5" />
                {STATE_TYPE_LABEL[type]}
              </span>
            </SelectItem>
          ))}
        </SelectContent>
      </Select>

      <div className="hidden text-xs text-muted-foreground lg:block">Configure automation after save</div>
      <div className="flex items-center justify-end gap-1">
        {saving ? <Loading01Icon className="h-3.5 w-3.5 animate-spin text-muted-foreground" /> : null}
        <button
          type="button"
          className="flex h-7 w-7 items-center justify-center rounded-md text-primary hover:bg-muted disabled:pointer-events-none disabled:opacity-50"
          onClick={onSave}
          disabled={saving || !name.trim()}
          aria-label="Save state"
        >
          <Tick01Icon className="h-3.5 w-3.5" />
        </button>
        <button
          type="button"
          className="flex h-7 w-7 items-center justify-center rounded-md text-muted-foreground hover:bg-muted hover:text-foreground"
          onClick={onCancel}
          disabled={saving}
          aria-label="Cancel"
        >
          <Cancel01Icon className="h-3.5 w-3.5" />
        </button>
      </div>
    </div>
  );
}

function StatusPill({ children, tone = 'muted' }: { children: ReactNode; tone?: 'muted' | 'violet' | 'green' | 'blue' }) {
  return (
    <span
      className={cn(
        'inline-flex items-center rounded-full border px-1.5 py-0.5 text-[10px] font-medium',
        tone === 'muted' && 'border-border bg-muted/40 text-muted-foreground',
        tone === 'violet' && 'border-violet-500/20 bg-violet-500/10 text-violet-700 dark:text-violet-300',
        tone === 'green' && 'border-emerald-500/20 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300',
        tone === 'blue' && 'border-sky-500/20 bg-sky-500/10 text-sky-700 dark:text-sky-300',
      )}
    >
      {children}
    </span>
  );
}

function MergeBranchInput({ value, editable, onChange }: { value: string; editable: boolean; onChange: (v: string) => void }) {
  const [editing, setEditing] = useState(false);
  const isBaseBranch = value === BASE_BRANCH_TOKEN;
  const mode = isBaseBranch ? BASE_BRANCH_TOKEN : value ? '__custom__' : '';
  const [customDraft, setCustomDraft] = useState(isBaseBranch ? '' : value);

  if (!editable) {
    return (
      <p className="truncate text-xs text-muted-foreground">
        {value ? describeMergeDestination(value) : 'Branch'}
      </p>
    );
  }

  if (!editing && !value) {
    return (
      <button
        type="button"
        className="inline-flex h-8 w-full items-center justify-start gap-1.5 rounded-md border border-border bg-background px-2 text-xs text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
        onClick={() => { onChange(BASE_BRANCH_TOKEN); }}
      >
        <GitBranchIcon className="h-3.5 w-3.5" />
        Branch
      </button>
    );
  }

  if (!editing) {
    return (
      <div className="flex h-8 min-w-0 items-center gap-1.5 rounded-md border border-border bg-background px-2 text-xs">
        <span className="min-w-0 flex-1 truncate text-foreground">
          {describeMergeDestination(value)}
        </span>
        <button type="button" className="shrink-0 rounded px-1 text-muted-foreground hover:text-foreground" onClick={() => setEditing(true)}>
          Edit
        </button>
        <button type="button" className="shrink-0 text-muted-foreground hover:text-destructive" onClick={() => onChange('')} aria-label="Remove merge action">
          <Cancel01Icon className="h-3.5 w-3.5" />
        </button>
      </div>
    );
  }

  return (
    <div className="space-y-2">
      <Select
        value={mode || BASE_BRANCH_TOKEN}
        onValueChange={(v) => {
          if (v === BASE_BRANCH_TOKEN) {
            onChange(BASE_BRANCH_TOKEN);
            setEditing(false);
          } else {
            setCustomDraft('');
          }
        }}
      >
        <SelectTrigger className="h-8 text-xs">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value={BASE_BRANCH_TOKEN}>Task base branch</SelectItem>
          <SelectItem value="__custom__">Custom branch</SelectItem>
        </SelectContent>
      </Select>
      {mode === '__custom__' && (
        <Input
          className="h-8 text-xs"
          value={customDraft}
          onChange={(e) => setCustomDraft(e.target.value)}
          placeholder="Branch name"
          autoFocus
          onKeyDown={(e) => {
            if (e.key === 'Enter' && customDraft.trim()) { onChange(customDraft.trim()); setEditing(false); }
            if (e.key === 'Escape') setEditing(false);
          }}
          onBlur={() => { if (customDraft.trim()) { onChange(customDraft.trim()); setEditing(false); } }}
        />
      )}
    </div>
  );
}
