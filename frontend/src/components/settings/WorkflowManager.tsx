import { useEffect, useState, type FormEvent } from 'react';
import { pmWorkflowService } from '@/lib/services/pmWorkflowService';
import { StateTypeIcon } from '@/lib/pmConstants';
import type { WorkspaceTeam } from '@/lib/types';
import type { StateType, WorkflowState, WorkflowWithStates } from '@/lib/pmTypes';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { ConfirmDialog } from '@/components/pm/ConfirmDialog';
import { ArrowDown, ArrowUp, Check, GitBranch, Pencil, Plus, Trash2 } from 'lucide-react';
import { toast } from 'sonner';
import { ColorPicker } from '@/components/pm/ColorPicker';
import { QuickTooltip } from '@/components/ui/quick-tooltip';
import { cn } from '@/lib/utils';

const STATE_TYPE_ORDER: StateType[] = ['backlog', 'unstarted', 'started', 'done'];
const STATE_TYPE_LABEL: Record<StateType, string> = {
  backlog: 'Backlog',
  unstarted: 'Unstarted',
  started: 'Started',
  done: 'Done',
};

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
      if (prev && next.some((w) => w.workflow.id === prev)) return prev;
      return next[0]?.workflow.id ?? '';
    });
    setLoading(false);
  };

  useEffect(() => { loadWorkflows(); }, [workspaceId]);

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
    setWfTeamID('none');
    setWfAutoAssign(false);
    setWorkflowDialogOpen(true);
  };

  const openEditWorkflow = () => {
    if (!selected) return;
    setEditWorkflow(selected);
    setWfName(selected.workflow.name);
    setWfDescription(selected.workflow.description ?? '');
    setWfTeamID(selected.workflow.team_id ?? 'none');
    setWfAutoAssign(selected.workflow.auto_assign_owner);
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
              <Plus className="h-3.5 w-3.5" /> New
            </Button>
          )}
        </div>

        <div className="space-y-1">
          {workflows.length === 0 ? (
            <p className="text-sm text-muted-foreground py-4 text-center">No workflows found.</p>
          ) : (
            workflows.map((entry) => (
              <button
                key={entry.workflow.id}
                onClick={() => setSelectedId(entry.workflow.id)}
                className={cn(
                  'w-full rounded-md border px-3 py-2.5 text-left transition-colors',
                  entry.workflow.id === selectedId
                    ? 'border-primary/30 bg-primary/5'
                    : 'border-transparent hover:bg-muted/50'
                )}
              >
                <div className="flex items-center gap-2">
                  <GitBranch className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                  <span className="text-sm font-medium truncate">{entry.workflow.name}</span>
                </div>
                <div className="mt-0.5 ml-5.5 flex items-center gap-1.5 text-xs text-muted-foreground">
                  <span>{entry.states.length} states</span>
                  <span>&middot;</span>
                  <span className="truncate">{findTeamName(entry.workflow.team_id)}</span>
                </div>
              </button>
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
                    <Plus className="h-4 w-4 mr-1" /> Create your first workflow
                  </Button>
                )}
              </div>
            ) : (
              <p>Select a workflow from the list.</p>
            )}
          </div>
        ) : (
          <div className="space-y-6">
            {/* Workflow header */}
            <div className="flex items-start justify-between gap-4">
              <div className="min-w-0">
                <h3 className="text-lg font-semibold">{selected.workflow.name}</h3>
                {selected.workflow.description && (
                  <p className="text-sm text-muted-foreground mt-0.5">{selected.workflow.description}</p>
                )}
              </div>
              {editable && (
                <div className="flex items-center gap-1 shrink-0">
                  <Button variant="ghost" size="sm" className="h-7 gap-1 text-xs" onClick={openEditWorkflow}>
                    <Pencil className="h-3.5 w-3.5" /> Edit
                  </Button>
                  <Button
                    variant="ghost"
                    size="sm"
                    className="h-7 text-xs text-destructive hover:text-destructive"
                    onClick={() => setDeleteWorkflowConfirm(selected.workflow.id)}
                  >
                    <Trash2 className="h-3.5 w-3.5" />
                  </Button>
                </div>
              )}
            </div>

            {/* States */}
            <div className="space-y-5">
              <h4 className="text-sm font-medium">States</h4>
              {STATE_TYPE_ORDER.map((type) => (
                <section key={type} className="space-y-2">
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-1.5 text-sm font-medium">
                      <StateTypeIcon stateType={type} className="h-4 w-4" />
                      {STATE_TYPE_LABEL[type]}
                      <span className="text-xs font-normal text-muted-foreground">({statesByType[type].length})</span>
                    </div>
                    {editable && (
                      <Button variant="ghost" size="sm" className="h-7 gap-1 text-xs" onClick={() => openCreateState(type)}>
                        <Plus className="h-3.5 w-3.5" /> Add
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
                          </div>
                          <div className="flex items-center gap-0.5 shrink-0">
                            {state.is_default ? (
                              <QuickTooltip label="New stories are created in this state">
                                <Badge variant="secondary" className="text-xs gap-1 shrink-0 cursor-default">
                                  <Check className="h-3 w-3" /> Default
                                </Badge>
                              </QuickTooltip>
                            ) : editable ? (
                              <Button
                                variant="ghost"
                                size="sm"
                                className="h-6 text-xs text-muted-foreground opacity-0 group-hover:opacity-100 transition-opacity"
                                onClick={() => handleSetDefault(state.id)}
                              >
                                Set as default
                              </Button>
                            ) : null}
                            {editable && (
                              <>
                                <Button
                                  size="icon"
                                  variant="ghost"
                                  className="h-6 w-6"
                                  disabled={idx === 0}
                                  onClick={() => handleMoveWithinType(type, state.id, 'up')}
                                >
                                  <ArrowUp className="h-3 w-3" />
                                </Button>
                                <Button
                                  size="icon"
                                  variant="ghost"
                                  className="h-6 w-6"
                                  disabled={idx === statesByType[type].length - 1}
                                  onClick={() => handleMoveWithinType(type, state.id, 'down')}
                                >
                                  <ArrowDown className="h-3 w-3" />
                                </Button>
                                <Button size="icon" variant="ghost" className="h-6 w-6" onClick={() => openEditState(state)}>
                                  <Pencil className="h-3 w-3" />
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
                  <p className="text-xs text-muted-foreground">Assign current user when story enters a started state and has no owner.</p>
                </div>
                <Switch checked={wfAutoAssign} onCheckedChange={setWfAutoAssign} />
              </div>
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setWorkflowDialogOpen(false)}>Cancel</Button>
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
                <ColorPicker value={stateColor || '#3b82f6'} onChange={setStateColor} />
              </div>
            </div>
            <DialogFooter className="justify-between">
              <div>
                {editState && editable && (
                  <Button type="button" variant="ghost" className="text-destructive" onClick={() => setDeleteStateConfirm(true)}>
                    <Trash2 className="h-3.5 w-3.5 mr-1" /> Delete State
                  </Button>
                )}
              </div>
              <div className="flex gap-2">
                <Button type="button" variant="outline" onClick={() => setStateDialogOpen(false)}>Cancel</Button>
                <Button type="submit" disabled={stateSaving || !editable}>{stateSaving ? 'Saving...' : 'Save'}</Button>
              </div>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      {/* Delete workflow confirm */}
      <ConfirmDialog
        open={deleteWorkflowConfirm !== null}
        onOpenChange={(open) => { if (!open) setDeleteWorkflowConfirm(null); }}
        title="Delete workflow"
        description="This will permanently delete the workflow and all its states. Stories using this workflow will need to be reassigned. This action cannot be undone."
        confirmLabel="Delete"
        variant="destructive"
        onConfirm={() => { if (deleteWorkflowConfirm) handleDeleteWorkflow(deleteWorkflowConfirm); setDeleteWorkflowConfirm(null); }}
      />

      {/* Delete state confirm */}
      <ConfirmDialog
        open={deleteStateConfirm}
        onOpenChange={setDeleteStateConfirm}
        title="Delete workflow state"
        description="This will permanently delete this state. Stories in this state will need to be moved to another state. This action cannot be undone."
        confirmLabel="Delete"
        variant="destructive"
        onConfirm={() => { handleDeleteState(); setDeleteStateConfirm(false); }}
      />
    </div>
  );
}
