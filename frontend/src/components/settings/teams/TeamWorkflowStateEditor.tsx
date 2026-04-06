import { useState } from 'react';
import { Tick01Icon, PencilEdit01Icon, PlusSignIcon, Delete01Icon, Cancel01Icon } from '@/lib/icons';
import { toast } from 'sonner';
import { ConfirmDialog } from '@/components/pm/ConfirmDialog';
import { ColorPicker, PRESET_COLORS } from '@/components/pm/ColorPicker';
import { Input } from '@/components/ui/input';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { pmWorkflowService } from '@/lib/services/pmWorkflowService';
import type { StateType, WorkflowState, WorkflowWithStates } from '@/lib/pmTypes';

const STATE_TYPE_ORDER: StateType[] = ['backlog', 'unstarted', 'started', 'done'];
const STATE_TYPE_LABEL: Record<StateType, string> = {
  backlog: 'Backlog',
  unstarted: 'Not started',
  started: 'Started',
  done: 'Done',
};

export function TeamWorkflowStateEditor({
  workspaceId,
  workflow,
  editable,
  onUpdate,
}: {
  workspaceId: string;
  workflow: WorkflowWithStates;
  editable: boolean;
  onUpdate: (updated: WorkflowWithStates) => void;
}) {
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editName, setEditName] = useState('');
  const [saving, setSaving] = useState(false);
  const [addingType, setAddingType] = useState<StateType | null>(null);
  const [newName, setNewName] = useState('');
  const [newColor, setNewColor] = useState(PRESET_COLORS[0]);
  const [deleteConfirm, setDeleteConfirm] = useState<string | null>(null);

  const sorted = workflow.states.slice().sort((a, b) => a.position - b.position);
  const grouped = STATE_TYPE_ORDER.map((type) => ({
    type,
    label: STATE_TYPE_LABEL[type],
    states: sorted.filter((state) => state.state_type === type),
  })).filter((group) => group.states.length > 0 || addingType === group.type);

  const handleRename = async (state: WorkflowState) => {
    const trimmed = editName.trim();
    if (!trimmed || trimmed === state.name) {
      setEditingId(null);
      return;
    }
    setSaving(true);
    const { data, error } = await pmWorkflowService.updateState(workspaceId, workflow.workflow.id, state.id, { name: trimmed });
    setSaving(false);
    if (error) {
      toast.error(error);
      return;
    }
    if (data) {
      onUpdate({
        ...workflow,
        states: workflow.states.map((candidate) => candidate.id === state.id ? { ...candidate, name: trimmed } : candidate),
      });
    }
    setEditingId(null);
  };

  const handleColorChange = async (state: WorkflowState, color: string) => {
    const { error } = await pmWorkflowService.updateState(workspaceId, workflow.workflow.id, state.id, { color });
    if (error) {
      toast.error(error);
      return;
    }
    onUpdate({
      ...workflow,
      states: workflow.states.map((candidate) => candidate.id === state.id ? { ...candidate, color } : candidate),
    });
  };

  const handleAdd = async () => {
    const trimmed = newName.trim();
    if (!trimmed || !addingType) {
      return;
    }
    setSaving(true);
    const statesOfType = sorted.filter((state) => state.state_type === addingType);
    const position = statesOfType.length > 0 ? statesOfType[statesOfType.length - 1].position + 1 : sorted.length;
    const { data, error } = await pmWorkflowService.createState(workspaceId, workflow.workflow.id, {
      name: trimmed,
      state_type: addingType,
      position,
      color: newColor,
    });
    setSaving(false);
    if (error) {
      toast.error(error);
      return;
    }
    if (data && typeof data === 'object' && 'id' in data && 'workflow_id' in data) {
      onUpdate({ ...workflow, states: [...workflow.states, data as WorkflowState] });
    }
    setNewName('');
    setNewColor(PRESET_COLORS[0]);
    setAddingType(null);
  };

  const handleDelete = async (stateId: string) => {
    setSaving(true);
    const { error } = await pmWorkflowService.removeState(workspaceId, workflow.workflow.id, stateId);
    setSaving(false);
    if (error) {
      toast.error(error);
      return;
    }
    onUpdate({ ...workflow, states: workflow.states.filter((state) => state.id !== stateId) });
    setDeleteConfirm(null);
  };

  return (
    <div className="space-y-4 py-2">
      {grouped.map((group) => (
        <div key={group.type}>
          <p className="mb-1.5 text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
            {group.label}
          </p>
          <div className="space-y-1">
            {group.states.map((state) => (
              <div key={state.id} className="group flex items-center gap-2 rounded-md border border-border bg-background px-2.5 py-1.5">
                <Popover>
                  <PopoverTrigger asChild>
                    <button
                      type="button"
                      className="h-3.5 w-3.5 shrink-0 rounded-full border border-border/60 transition-transform hover:scale-110"
                      style={{ backgroundColor: state.color ?? '#9ca3af' }}
                      disabled={!editable}
                    />
                  </PopoverTrigger>
                  {editable && (
                    <PopoverContent className="w-auto p-2" align="start">
                      <ColorPicker value={state.color ?? '#9ca3af'} onChange={(color) => handleColorChange(state, color)} />
                    </PopoverContent>
                  )}
                </Popover>

                {editingId === state.id ? (
                  <Input
                    autoFocus
                    value={editName}
                    onChange={(event) => setEditName(event.target.value)}
                    onBlur={() => handleRename(state)}
                    onKeyDown={(event) => {
                      if (event.key === 'Enter') handleRename(state);
                      if (event.key === 'Escape') setEditingId(null);
                    }}
                    className="h-6 flex-1 px-1 py-0 text-xs"
                    disabled={saving}
                  />
                ) : (
                  <span className="flex-1 truncate text-sm">{state.name}</span>
                )}

                {editable && editingId !== state.id && (
                  <div className="flex items-center gap-0.5 opacity-0 transition-opacity group-hover:opacity-100">
                    <button
                      type="button"
                      className="flex h-5 w-5 items-center justify-center rounded text-muted-foreground hover:bg-muted hover:text-foreground"
                      onClick={() => {
                        setEditingId(state.id);
                        setEditName(state.name);
                      }}
                    >
                      <PencilEdit01Icon className="h-3 w-3" />
                    </button>
                    {workflow.states.length > 1 && (
                      <button
                        type="button"
                        className="flex h-5 w-5 items-center justify-center rounded text-muted-foreground hover:bg-muted hover:text-destructive"
                        onClick={() => setDeleteConfirm(state.id)}
                      >
                        <Delete01Icon className="h-3 w-3" />
                      </button>
                    )}
                  </div>
                )}
              </div>
            ))}

            {addingType === group.type && (
              <div className="flex items-center gap-2 rounded-md border border-primary/40 bg-background px-2.5 py-1.5">
                <Popover>
                  <PopoverTrigger asChild>
                    <button
                      type="button"
                      className="h-3.5 w-3.5 shrink-0 rounded-full border border-border/60 transition-transform hover:scale-110"
                      style={{ backgroundColor: newColor }}
                    />
                  </PopoverTrigger>
                  <PopoverContent className="w-auto p-2" align="start">
                    <ColorPicker value={newColor} onChange={setNewColor} />
                  </PopoverContent>
                </Popover>
                <Input
                  autoFocus
                  value={newName}
                  onChange={(event) => setNewName(event.target.value)}
                  onKeyDown={(event) => {
                    if (event.key === 'Enter') handleAdd();
                    if (event.key === 'Escape') {
                      setAddingType(null);
                      setNewName('');
                    }
                  }}
                  placeholder="State name..."
                  className="h-6 flex-1 border-0 px-1 py-0 text-xs shadow-none focus-visible:ring-0"
                  disabled={saving}
                />
                <button
                  type="button"
                  className="flex h-5 w-5 items-center justify-center rounded text-primary hover:bg-muted"
                  onClick={handleAdd}
                  disabled={saving || !newName.trim()}
                >
                  <Tick01Icon className="h-3.5 w-3.5" />
                </button>
                <button
                  type="button"
                  className="flex h-5 w-5 items-center justify-center rounded text-muted-foreground hover:bg-muted"
                  onClick={() => {
                    setAddingType(null);
                    setNewName('');
                  }}
                >
                  <Cancel01Icon className="h-3.5 w-3.5" />
                </button>
              </div>
            )}
          </div>

          {editable && addingType !== group.type && (
            <button
              type="button"
              className="mt-1 flex items-center gap-1 text-xs text-muted-foreground transition-colors hover:text-foreground"
              onClick={() => {
                setAddingType(group.type);
                setNewName('');
                setNewColor(PRESET_COLORS[Math.floor(Math.random() * PRESET_COLORS.length)]);
              }}
            >
              <PlusSignIcon className="h-3 w-3" /> Add state
            </button>
          )}
        </div>
      ))}

      {!editable && (
        <p className="text-xs text-muted-foreground">You don't have permission to edit workflow states.</p>
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
          if (deleteConfirm) handleDelete(deleteConfirm);
        }}
      />
    </div>
  );
}
