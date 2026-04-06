import { useEffect, useState, type FormEvent } from 'react';
import { pmWorkflowService } from '@/lib/services/pmWorkflowService';
import { StateTypeIcon } from '@/lib/pmConstants';
import type { StateType, WorkflowState, WorkflowWithStates } from '@/lib/pmTypes';
import { Card, CardContent } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { ConfirmDialog } from '@/components/pm/ConfirmDialog';
import { ArrowDown02Icon, ArrowUp02Icon, PencilEdit01Icon, PlusSignIcon, Delete01Icon } from '@/lib/icons';
import { toast } from 'sonner';
import { LINEAR_CARD_CLASS } from './settingsConstants';

const STATE_TYPE_ORDER: StateType[] = ['backlog', 'unstarted', 'started', 'done'];
const STATE_TYPE_LABEL: Record<StateType, string> = {
  backlog: 'Backlog',
  unstarted: 'Unstarted',
  started: 'Started',
  done: 'Done',
};

export function WorkflowStatesTab({ workspaceId, editable, initialWorkflowId }: {
  workspaceId: string;
  editable: boolean;
  initialWorkflowId?: string;
}) {
  const [workflows, setWorkflows] = useState<WorkflowWithStates[]>([]);
  const [selectedWorkflowID, setSelectedWorkflowID] = useState<string>('');
  const [loading, setLoading] = useState(true);

  const [dialogOpen, setDialogOpen] = useState(false);
  const [editState, setEditState] = useState<WorkflowState | null>(null);
  const [newStateType, setNewStateType] = useState<StateType>('unstarted');
  const [stateName, setStateName] = useState('');
  const [stateDescription, setStateDescription] = useState('');
  const [stateColor, setStateColor] = useState('');
  const [stateWIP, setStateWIP] = useState('');
  const [stateDefault, setStateDefault] = useState(false);
  const [deleteStateConfirm, setDeleteStateConfirm] = useState(false);
  const [saving, setSaving] = useState(false);

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
    setSelectedWorkflowID((prev) => {
      // Prefer URL param, then keep existing selection, then fall back to first
      if (initialWorkflowId && next.some((workflow) => workflow.workflow.id === initialWorkflowId)) return initialWorkflowId;
      if (prev && next.some((workflow) => workflow.workflow.id === prev)) return prev;
      return next[0]?.workflow.id ?? '';
    });
    setLoading(false);
  };

  useEffect(() => { loadWorkflows(); }, [workspaceId]);

  const selectedWorkflow = workflows.find((workflow) => workflow.workflow.id === selectedWorkflowID) ?? null;
  const sortedStates = selectedWorkflow
    ? [...selectedWorkflow.states].sort((a, b) => a.position - b.position)
    : [];

  const statesByType: Record<StateType, WorkflowState[]> = {
    backlog: sortedStates.filter((state) => state.state_type === 'backlog'),
    unstarted: sortedStates.filter((state) => state.state_type === 'unstarted'),
    started: sortedStates.filter((state) => state.state_type === 'started'),
    done: sortedStates.filter((state) => state.state_type === 'done'),
  };

  const orderedStateIDs = (workflow: WorkflowWithStates) =>
    STATE_TYPE_ORDER.flatMap((type) =>
      [...workflow.states]
        .filter((state) => state.state_type === type)
        .sort((a, b) => a.position - b.position)
        .map((state) => state.id)
    );

  const normalizeOrdering = async (workflow: WorkflowWithStates) => {
    const targetIDs = orderedStateIDs(workflow);
    const currentIDs = [...workflow.states].sort((a, b) => a.position - b.position).map((state) => state.id);
    if (targetIDs.length === currentIDs.length && targetIDs.every((id, idx) => currentIDs[idx] === id)) return;
    await pmWorkflowService.reorderStates(workspaceId, workflow.workflow.id, targetIDs);
  };

  const openCreateForType = (type: StateType) => {
    setEditState(null);
    setNewStateType(type);
    setStateName('');
    setStateDescription('');
    setStateColor('');
    setStateWIP('');
    setStateDefault(false);
    setDialogOpen(true);
  };

  const openEdit = (state: WorkflowState) => {
    setEditState(state);
    setNewStateType(state.state_type);
    setStateName(state.name);
    setStateDescription(state.description ?? '');
    setStateColor(state.color ?? '');
    setStateWIP(state.wip_limit ? String(state.wip_limit) : '');
    setStateDefault(state.is_default);
    setDialogOpen(true);
  };

  const handleSaveState = async (e: FormEvent) => {
    e.preventDefault();
    if (!selectedWorkflow || !stateName.trim()) return;
    setSaving(true);
    const payload = {
      name: stateName.trim(),
      state_type: newStateType,
      description: stateDescription.trim() || undefined,
      color: stateColor.trim() || undefined,
      wip_limit: stateWIP.trim() ? Number(stateWIP) : undefined,
      is_default: stateDefault,
    };

    if (editState) {
      const { error } = await pmWorkflowService.updateState(workspaceId, selectedWorkflow.workflow.id, editState.id, payload);
      if (error) toast.error(error);
      else toast.success('State updated');
    } else {
      const { error } = await pmWorkflowService.createState(workspaceId, selectedWorkflow.workflow.id, payload);
      if (error) toast.error(error);
      else {
        toast.success('State created');
        const refreshed = await pmWorkflowService.get(workspaceId, selectedWorkflow.workflow.id);
        if (refreshed.data) {
          await normalizeOrdering(refreshed.data);
        }
      }
    }
    await loadWorkflows();
    setDialogOpen(false);
    setSaving(false);
  };

  const handleDeleteState = async () => {
    if (!selectedWorkflow || !editState) return;
    const { error } = await pmWorkflowService.removeState(workspaceId, selectedWorkflow.workflow.id, editState.id);
    if (error) toast.error(error);
    else {
      toast.success('State deleted');
      setDialogOpen(false);
      await loadWorkflows();
    }
  };

  const handleMoveWithinType = async (type: StateType, stateID: string, direction: 'up' | 'down') => {
    if (!selectedWorkflow) return;
    const typed = [...statesByType[type]];
    const idx = typed.findIndex((state) => state.id === stateID);
    if (idx < 0) return;
    const swapIdx = direction === 'up' ? idx - 1 : idx + 1;
    if (swapIdx < 0 || swapIdx >= typed.length) return;
    const copy = [...typed];
    const [current] = copy.splice(idx, 1);
    copy.splice(swapIdx, 0, current);

    const idsByType: Record<StateType, string[]> = {
      backlog: statesByType.backlog.map((state) => state.id),
      unstarted: statesByType.unstarted.map((state) => state.id),
      started: statesByType.started.map((state) => state.id),
      done: statesByType.done.map((state) => state.id),
    };
    idsByType[type] = copy.map((state) => state.id);
    const nextIDs = STATE_TYPE_ORDER.flatMap((stateType) => idsByType[stateType]);

    const { error } = await pmWorkflowService.reorderStates(workspaceId, selectedWorkflow.workflow.id, nextIDs);
    if (error) toast.error(error);
    else await loadWorkflows();
  };

  const toggleAutoAssignOwner = async (checked: boolean) => {
    if (!selectedWorkflow) return;
    const { error } = await pmWorkflowService.update(workspaceId, selectedWorkflow.workflow.id, {
      auto_assign_owner: checked,
    });
    if (error) toast.error(error);
    else await loadWorkflows();
  };

  return (
    <Card className={LINEAR_CARD_CLASS}>
      <CardContent className="pt-6 space-y-5">
        {loading ? (
          <p className="text-sm text-muted-foreground">Loading workflows...</p>
        ) : workflows.length === 0 ? (
          <p className="text-sm text-muted-foreground">No workflows found. Create one in the Workflows tab first.</p>
        ) : (
          <>
            <div className="grid gap-3 md:grid-cols-2">
              <div className="space-y-2">
                <Label>Workflow</Label>
                <Select value={selectedWorkflowID} onValueChange={setSelectedWorkflowID}>
                  <SelectTrigger><SelectValue /></SelectTrigger>
                  <SelectContent>
                    {workflows.map((workflow) => (
                      <SelectItem key={workflow.workflow.id} value={workflow.workflow.id}>
                        {workflow.workflow.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              {selectedWorkflow && (
                <div className="flex items-end">
                  <div className="flex w-full items-center justify-between rounded-xl border border-border px-3 py-2.5">
                    <div>
                      <Label>Auto assign owner</Label>
                      <p className="text-xs text-muted-foreground">Assign current user when tasks move into a started state without an owner.</p>
                    </div>
                    <Switch
                      checked={selectedWorkflow.workflow.auto_assign_owner}
                      disabled={!editable}
                      onCheckedChange={toggleAutoAssignOwner}
                    />
                  </div>
                </div>
              )}
            </div>

            {selectedWorkflow && (
              <div className="space-y-5">
                {STATE_TYPE_ORDER.map((type) => (
                  <section key={type} className="space-y-2">
                    <div className="flex items-center justify-between">
                      <h4 className="flex items-center gap-1.5 text-sm font-medium">
                        <StateTypeIcon stateType={type} className="h-4 w-4" />
                        {STATE_TYPE_LABEL[type]}
                      </h4>
                      {editable && (
                        <Button variant="ghost" size="sm" onClick={() => openCreateForType(type)}>
                          <PlusSignIcon className="h-3.5 w-3.5 mr-1" /> Add
                        </Button>
                      )}
                    </div>
                    {statesByType[type].length === 0 ? (
                      <div className="rounded-lg border border-dashed border-border px-3 py-3 text-sm text-muted-foreground">
                        No states in this group.
                      </div>
                    ) : (
                      <div className="space-y-2">
                        {statesByType[type].map((state, idx) => (
                          <div key={state.id} className="rounded-xl border border-border px-3 py-2.5">
                            <div className="flex items-start justify-between gap-3">
                              <div>
                                <div className="flex items-center gap-2">
                                  <StateTypeIcon stateType={state.state_type} className="h-4 w-4" />
                                  <p className="font-medium">{state.name}</p>
                                  {state.is_default && (
                                    <Badge variant="secondary" className="text-xs">Default</Badge>
                                  )}
                                  {state.wip_limit ? (
                                    <Badge variant="outline" className="text-xs">WIP {state.wip_limit}</Badge>
                                  ) : null}
                                </div>
                                <p className="text-sm text-muted-foreground">{state.description || 'No description'}</p>
                              </div>
                              {editable && (
                                <div className="flex items-center gap-1">
                                  <Button
                                    size="icon"
                                    variant="ghost"
                                    disabled={idx === 0}
                                    onClick={() => handleMoveWithinType(type, state.id, 'up')}
                                  >
                                    <ArrowUp02Icon className="h-3.5 w-3.5" />
                                  </Button>
                                  <Button
                                    size="icon"
                                    variant="ghost"
                                    disabled={idx === statesByType[type].length - 1}
                                    onClick={() => handleMoveWithinType(type, state.id, 'down')}
                                  >
                                    <ArrowDown02Icon className="h-3.5 w-3.5" />
                                  </Button>
                                  <Button size="icon" variant="ghost" onClick={() => openEdit(state)}>
                                    <PencilEdit01Icon className="h-3.5 w-3.5" />
                                  </Button>
                                </div>
                              )}
                            </div>
                          </div>
                        ))}
                      </div>
                    )}
                  </section>
                ))}
              </div>
            )}
          </>
        )}
      </CardContent>

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent>
          <form onSubmit={handleSaveState}>
            <DialogHeader>
              <DialogTitle>{editState ? 'Edit Workflow State' : 'Add Workflow State'}</DialogTitle>
            </DialogHeader>
            <div className="space-y-4 py-4">
              <div className="space-y-2">
                <Label>State Type</Label>
                <Input value={STATE_TYPE_LABEL[newStateType]} disabled />
              </div>
              <div className="space-y-2">
                <Label>State Name</Label>
                <Input value={stateName} onChange={(e) => setStateName(e.target.value)} required />
              </div>
              <div className="space-y-2">
                <Label>Description</Label>
                <Input value={stateDescription} onChange={(e) => setStateDescription(e.target.value)} />
              </div>
              <div className="grid gap-3 sm:grid-cols-2">
                <div className="space-y-2">
                  <Label>Color</Label>
                  <Input value={stateColor} onChange={(e) => setStateColor(e.target.value)} placeholder="#3b82f6" />
                </div>
                <div className="space-y-2">
                  <Label>WIP Limit</Label>
                  <Input type="number" min={0} value={stateWIP} onChange={(e) => setStateWIP(e.target.value)} />
                </div>
              </div>
              <div className="flex items-center justify-between rounded-xl border border-border px-3 py-2">
                <div>
                  <Label>Default state</Label>
                  <p className="text-xs text-muted-foreground">Tasks are created in this state by default.</p>
                </div>
                <Switch checked={stateDefault} onCheckedChange={setStateDefault} />
              </div>
            </div>
            <DialogFooter className="justify-between">
              <div>
                {editState && editable && (
                  <Button type="button" variant="ghost" className="text-destructive" onClick={() => setDeleteStateConfirm(true)}>
                    <Delete01Icon className="h-3.5 w-3.5 mr-1" /> Delete State
                  </Button>
                )}
              </div>
              <div className="flex gap-2">
                <Button type="button" variant="outline" onClick={() => setDialogOpen(false)}>Cancel</Button>
                <Button type="submit" disabled={saving || !editable}>{saving ? 'Saving...' : 'Save Changes'}</Button>
              </div>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      <ConfirmDialog
        open={deleteStateConfirm}
        onOpenChange={setDeleteStateConfirm}
        title="Delete workflow state"
        description="This will permanently delete this state. Tasks in this state will need to be moved to another state. This action cannot be undone."
        confirmLabel="Delete"
        variant="destructive"
        onConfirm={() => { handleDeleteState(); setDeleteStateConfirm(false); }}
      />
    </Card>
  );
}
