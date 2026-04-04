import { useEffect, useState, type FormEvent } from 'react';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { pmWorkflowService } from '@/lib/services/pmWorkflowService';
import type { WorkspaceTeam } from '@/lib/types';
import type { WorkflowWithStates } from '@/lib/pmTypes';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { ConfirmDialog } from '@/components/pm/ConfirmDialog';
import { HierarchyIcon, PencilEdit01Icon, PlusSignIcon, Delete01Icon } from '@/lib/icons';
import { useNavigate } from '@tanstack/react-router';
import { toast } from 'sonner';
import { LINEAR_CARD_CLASS } from './settingsConstants';

export function WorkflowsTab({ workspaceId, teams, editable, initialTeamId }: {
  workspaceId: string;
  teams: WorkspaceTeam[];
  editable: boolean;
  initialTeamId?: string;
}) {
  const navigate = useNavigate();
  const { currentWorkspace } = useWorkspaceStore();
  const [workflows, setWorkflows] = useState<WorkflowWithStates[]>([]);
  const [loading, setLoading] = useState(true);
  const [teamFilter, setTeamFilter] = useState<string>(initialTeamId || '__all__');
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editWorkflow, setEditWorkflow] = useState<WorkflowWithStates | null>(null);
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [teamID, setTeamID] = useState<string>('none');
  const [autoAssignOwner, setAutoAssignOwner] = useState(false);
  const [saving, setSaving] = useState(false);
  const [deleteWorkflowConfirm, setDeleteWorkflowConfirm] = useState<string | null>(null);

  const loadWorkflows = async () => {
    setLoading(true);
    const { data, error } = await pmWorkflowService.list(workspaceId);
    if (error) {
      toast.error(error);
      setWorkflows([]);
    } else {
      setWorkflows(data ?? []);
    }
    setLoading(false);
  };

  useEffect(() => { loadWorkflows(); }, [workspaceId]);

  const openCreate = () => {
    setEditWorkflow(null);
    setName('');
    setDescription('');
    setTeamID('none');
    setAutoAssignOwner(false);
    setDialogOpen(true);
  };

  const openEdit = (workflow: WorkflowWithStates) => {
    setEditWorkflow(workflow);
    setName(workflow.workflow.name);
    setDescription(workflow.workflow.description ?? '');
    setTeamID(workflow.workflow.team_id ?? 'none');
    setAutoAssignOwner(workflow.workflow.auto_assign_owner);
    setDialogOpen(true);
  };

  const handleSave = async (e: FormEvent) => {
    e.preventDefault();
    if (!name.trim()) return;
    setSaving(true);

    const normalizedTeamID = teamID === 'none' ? undefined : teamID;
    if (editWorkflow) {
      const { error } = await pmWorkflowService.update(workspaceId, editWorkflow.workflow.id, {
        name: name.trim(),
        description: description.trim() || undefined,
        team_id: normalizedTeamID,
        auto_assign_owner: autoAssignOwner,
      });
      if (error) toast.error(error);
      else {
        toast.success('Workflow updated');
        setDialogOpen(false);
        await loadWorkflows();
      }
    } else {
      const { error } = await pmWorkflowService.create({
        workspace_id: workspaceId,
        name: name.trim(),
        description: description.trim() || undefined,
        team_id: normalizedTeamID,
        auto_assign_owner: autoAssignOwner,
      });
      if (error) toast.error(error);
      else {
        toast.success('Workflow created');
        setDialogOpen(false);
        await loadWorkflows();
      }
    }
    setSaving(false);
  };

  const handleDelete = async (workflowID: string) => {
    const { error } = await pmWorkflowService.remove(workspaceId, workflowID);
    if (error) toast.error(error);
    else {
      toast.success('Workflow deleted');
      await loadWorkflows();
    }
  };

  const findTeamName = (id?: string) => {
    if (!id) return 'Workspace Default';
    return teams.find((team) => team.id === id)?.name ?? 'Unknown Team';
  };

  const filteredWorkflows = teamFilter === '__all__'
    ? workflows
    : teamFilter === '__default__'
      ? workflows.filter((w) => !w.workflow.team_id)
      : workflows.filter((w) => w.workflow.team_id === teamFilter);

  return (
    <Card className={LINEAR_CARD_CLASS}>
      <CardHeader>
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <CardTitle className="text-base">Workflows</CardTitle>
            <Badge variant="outline" className="text-xs font-normal">{filteredWorkflows.length}</Badge>
          </div>
          <div className="flex items-center gap-2">
            <Select value={teamFilter} onValueChange={setTeamFilter}>
              <SelectTrigger className="h-8 w-[220px] text-sm">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="__all__">All workflows</SelectItem>
                <SelectItem value="__default__">Workspace default</SelectItem>
                {teams.map((team) => (
                  <SelectItem key={team.id} value={team.id}>
                    {team.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            {editable && (
              <Button size="sm" onClick={openCreate}>
                <PlusSignIcon className="h-4 w-4 mr-1" /> Create Workflow
              </Button>
            )}
          </div>
        </div>
      </CardHeader>
      <CardContent>
        {loading ? (
          <p className="text-sm text-muted-foreground text-center py-6">Loading workflows...</p>
        ) : filteredWorkflows.length === 0 ? (
          <p className="text-sm text-muted-foreground text-center py-6">No workflows found.</p>
        ) : (
          <div className="space-y-2">
            {filteredWorkflows.map((workflow) => (
              <div key={workflow.workflow.id} className="rounded-xl border border-border px-4 py-3">
                <div className="flex items-center justify-between gap-3">
                  <div>
                    <div className="flex items-center gap-2">
                      <p className="font-medium">{workflow.workflow.name}</p>
                      {!workflow.workflow.team_id && (
                        <Badge variant="secondary" className="text-xs">Workspace Default</Badge>
                      )}
                    </div>
                    <p className="text-sm text-muted-foreground">
                      {workflow.workflow.description || 'No description'}
                    </p>
                  </div>
                  <div className="flex items-center gap-2 text-sm text-muted-foreground">
                    <span>{workflow.states.length} states</span>
                    <span>•</span>
                    <span>{findTeamName(workflow.workflow.team_id)}</span>
                    <span>•</span>
                    <span>{workflow.workflow.auto_assign_owner ? 'Auto-assign owner' : 'Manual owner'}</span>
                  </div>
                </div>
                {editable && (
                  <div className="mt-2 flex justify-end gap-1">
                    <Button
                      size="sm"
                      variant="ghost"
                      className="h-7 gap-1 text-xs"
                      onClick={() => {
                        const slug = currentWorkspace?.slug;
                        if (slug) {
                          navigate({
                            to: '/w/$slug/settings/$section',
                            params: { slug, section: 'workflowstates' },
                            search: { workflow: workflow.workflow.id },
                          });
                        }
                      }}
                    >
                      <HierarchyIcon className="h-3.5 w-3.5" />
                      Modify States
                    </Button>
                    <Button size="icon" variant="ghost" onClick={() => openEdit(workflow)}>
                      <PencilEdit01Icon className="h-3.5 w-3.5" />
                    </Button>
                    <Button size="icon" variant="ghost" className="text-destructive hover:text-destructive" onClick={() => setDeleteWorkflowConfirm(workflow.workflow.id)}>
                      <Delete01Icon className="h-3.5 w-3.5" />
                    </Button>
                  </div>
                )}
              </div>
            ))}
          </div>
        )}
      </CardContent>

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent>
          <form onSubmit={handleSave}>
            <DialogHeader>
              <DialogTitle>{editWorkflow ? 'Edit Workflow' : 'Create Workflow'}</DialogTitle>
            </DialogHeader>
            <div className="space-y-4 py-4">
              <div className="space-y-2">
                <Label>Name</Label>
                <Input value={name} onChange={(e) => setName(e.target.value)} required />
              </div>
              <div className="space-y-2">
                <Label>Description</Label>
                <Input value={description} onChange={(e) => setDescription(e.target.value)} />
              </div>
              <div className="space-y-2">
                <Label>Team</Label>
                <Select value={teamID} onValueChange={setTeamID}>
                  <SelectTrigger><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="none">Workspace Default</SelectItem>
                    {teams.map((team) => (
                      <SelectItem key={team.id} value={team.id}>{team.name}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="flex items-center justify-between rounded-xl border border-border px-3 py-2">
                <div>
                  <Label>Auto assign owner when moved to started state</Label>
                  <p className="text-xs text-muted-foreground">Assign current user when a task enters a started state and has no owner.</p>
                </div>
                <Switch checked={autoAssignOwner} onCheckedChange={setAutoAssignOwner} />
              </div>
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setDialogOpen(false)}>Cancel</Button>
              <Button type="submit" disabled={saving}>{saving ? 'Saving...' : 'Save'}</Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      <ConfirmDialog
        open={deleteWorkflowConfirm !== null}
        onOpenChange={(open) => { if (!open) setDeleteWorkflowConfirm(null); }}
        title="Delete workflow"
        description="This will permanently delete the workflow and all its states. Tasks using this workflow will need to be reassigned. This action cannot be undone."
        confirmLabel="Delete"
        variant="destructive"
        onConfirm={() => { if (deleteWorkflowConfirm) handleDelete(deleteWorkflowConfirm); setDeleteWorkflowConfirm(null); }}
      />
    </Card>
  );
}
