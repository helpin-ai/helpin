import { useCallback, useEffect, useRef, useState } from 'react';
import { Loader2, Pencil, Plus, Tag, Trash2 } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Progress } from '@/components/ui/progress';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { ColorPicker, PRESET_COLORS } from '@/components/pm/ColorPicker';
import { pmLabelService } from '@/lib/services/pmLabelService';
import type { LabelWithStats } from '@/lib/pmTypes';
import { useWorkspaceTeams } from '@/hooks/useWorkspaceTeams';

interface LabelsSettingsProps {
  workspaceId: string;
  initialTeamId?: string;
  editable?: boolean;
}

export interface LabelFormState {
  name: string;
  description: string;
  color: string;
  team_id: string;
}

export const emptyForm: LabelFormState = { name: '', description: '', color: PRESET_COLORS[0], team_id: '' };

export function pct(done: number, total: number) {
  if (total === 0) return 0;
  return Math.round((done / total) * 100);
}

export function StatCell({ done, total, entity }: { done: number; total: number; entity: string }) {
  const p = pct(done, total);
  return (
    <div className="space-y-1 min-w-[140px]">
      <span className="text-xs font-medium text-foreground">{p}% Completed</span>
      <Progress value={p} className="h-1.5 bg-emerald-500/15 [&>[data-slot=progress-indicator]]:bg-emerald-500" />
      <span className="text-[11px] text-muted-foreground">
        {done} of {total} {entity} Completed
      </span>
    </div>
  );
}

function LabelRow({
  entry,
  onEdit,
  onDelete,
  editable = true,
}: {
  entry: LabelWithStats;
  onEdit: () => void;
  onDelete: () => void;
  editable?: boolean;
}) {
  const { label, stats } = entry;
  return (
    <div className="flex items-center gap-4 rounded-lg border border-border/60 bg-muted/30 px-3.5 py-3 mb-2 last:mb-0 hover:bg-muted/50 transition-colors">
      <div className="flex items-center gap-2.5 min-w-0 flex-1">
        <span
          className="h-3 w-3 rounded-full shrink-0"
          style={{ backgroundColor: label.color || '#64748b' }}
        />
        <div className="min-w-0">
          <div className="flex items-center gap-2">
            <span className="text-sm font-medium text-foreground">{label.name}</span>
            <span className="text-[10px] uppercase tracking-wide text-muted-foreground">
              {label.team_id ? 'Team' : 'Shared'}
            </span>
          </div>
          {label.description && (
            <p className="text-xs text-muted-foreground truncate">{label.description}</p>
          )}
        </div>
      </div>
      <StatCell done={stats.done_story_count} total={stats.story_count} entity="Stories" />
      <StatCell done={stats.done_epic_count} total={stats.epic_count} entity="Epics" />
      {editable && (
        <div className="flex items-center gap-1">
          <Button
            variant="ghost"
            size="icon-xs"
            className="text-muted-foreground hover:text-foreground"
            onClick={onEdit}
          >
            <Pencil className="h-3 w-3" />
          </Button>
          <Button
            variant="ghost"
            size="icon-xs"
            className="text-muted-foreground hover:text-destructive"
            onClick={onDelete}
          >
            <Trash2 className="h-3 w-3" />
          </Button>
        </div>
      )}
    </div>
  );
}

function LabelForm({
  initial,
  teams,
  onSave,
  saving,
  className,
}: {
  initial: LabelFormState;
  teams: Array<{ id: string; name: string }>;
  onSave: (form: LabelFormState) => void;
  saving: boolean;
  className?: string;
}) {
  const [form, setForm] = useState<LabelFormState>(initial);
  const nameRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    setForm(initial);
  }, [initial]);

  useEffect(() => {
    nameRef.current?.focus();
  }, [initial]);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!form.name.trim()) return;
    onSave({ ...form, name: form.name.trim(), description: form.description.trim() });
  };

  return (
    <form onSubmit={handleSubmit} className={className ?? 'rounded-lg border border-border/60 bg-muted/30 p-3.5 space-y-4'}>
      <div className="space-y-2">
        <Label>Name</Label>
        <Input
          ref={nameRef}
          placeholder="Label name"
          value={form.name}
          onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
          className="h-8 text-sm"
        />
      </div>
      <div className="space-y-2">
        <Label>Description</Label>
        <Input
          placeholder="Description (optional)"
          value={form.description}
          onChange={(e) => setForm((f) => ({ ...f, description: e.target.value }))}
          className="h-8 text-sm"
        />
      </div>
      <div className="space-y-2">
        <Label>Scope</Label>
        <Select value={form.team_id || '__shared__'} onValueChange={(value) => setForm((f) => ({ ...f, team_id: value === '__shared__' ? '' : value }))}>
          <SelectTrigger className="h-8 w-full text-sm">
            <SelectValue placeholder="All teams" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="__shared__">All teams</SelectItem>
            {teams.map((team) => (
              <SelectItem key={team.id} value={team.id}>
                {team.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
      <div className="space-y-2">
        <Label>Color</Label>
        <ColorPicker value={form.color} onChange={(c) => setForm((f) => ({ ...f, color: c }))} />
      </div>
      <DialogFooter className="pt-2">
        <Button type="submit" disabled={saving || !form.name.trim()}>
          {saving ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : null}
          {saving ? 'Saving...' : 'Save'}
        </Button>
      </DialogFooter>
    </form>
  );
}

export function LabelsSettings({ workspaceId, initialTeamId, editable = true }: LabelsSettingsProps) {
  const { teams } = useWorkspaceTeams(workspaceId);
  const [labels, setLabels] = useState<LabelWithStats[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [labelDialogOpen, setLabelDialogOpen] = useState(false);
  const [deleteConfirmId, setDeleteConfirmId] = useState<string | null>(null);
  const [scopeFilter, setScopeFilter] = useState<string>(initialTeamId || '__all__');

  const getCreateInitialForm = (): LabelFormState => ({
    ...emptyForm,
    team_id: scopeFilter !== '__all__' && scopeFilter !== '__shared__' ? scopeFilter : '',
  });

  const editingEntry = editingId ? labels.find((entry) => entry.label.id === editingId) ?? null : null;
  const dialogInitialForm: LabelFormState = editingEntry
    ? {
        name: editingEntry.label.name,
        description: editingEntry.label.description || '',
        color: editingEntry.label.color || PRESET_COLORS[0],
        team_id: editingEntry.label.team_id || '',
      }
    : getCreateInitialForm();

  const reload = useCallback(async () => {
    const teamId = scopeFilter === '__all__' || scopeFilter === '__shared__' ? undefined : scopeFilter;
    const includeShared = scopeFilter !== '__shared__';
    const { data } = await pmLabelService.listWithStats(workspaceId, { teamId, includeShared, archived: false });
    if (data) setLabels(data.filter((e) => !e.label.archived));
    setLoading(false);
  }, [workspaceId, scopeFilter]);

  useEffect(() => {
    reload();
  }, [reload]);

  const handleCreate = async (form: LabelFormState) => {
    setSaving(true);
    const { error } = await pmLabelService.create({
      workspace_id: workspaceId,
      team_id: form.team_id || undefined,
      name: form.name,
      description: form.description || undefined,
      color: form.color,
    });
    setSaving(false);
    if (!error) {
      setLabelDialogOpen(false);
      reload();
    }
  };

  const handleUpdate = async (id: string, form: LabelFormState) => {
    setSaving(true);
    const { error } = await pmLabelService.update(workspaceId, id, {
      team_id: form.team_id || undefined,
      name: form.name,
      description: form.description || undefined,
      color: form.color,
    });
    setSaving(false);
    if (!error) {
      setLabelDialogOpen(false);
      setEditingId(null);
      reload();
    }
  };

  const handleDelete = async (id: string) => {
    setDeleteConfirmId(null);
    setLabels((prev) => prev.filter((e) => e.label.id !== id));
    const { error } = await pmLabelService.remove(workspaceId, id);
    if (error) reload();
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center py-12">
        <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-2">
        <Select value={scopeFilter} onValueChange={setScopeFilter}>
          <SelectTrigger className="h-8 w-[220px] text-sm">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
              <SelectItem value="__all__">All labels</SelectItem>
            <SelectItem value="__shared__">All teams</SelectItem>
            {teams.map((team) => (
              <SelectItem key={team.id} value={team.id}>
                {team.name} labels
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        {editable && (
          <Button
            variant="outline"
            size="sm"
            className="ml-auto"
            onClick={() => {
              setLabelDialogOpen(true);
              setEditingId(null);
            }}
          >
            <Plus className="h-3.5 w-3.5" />
            Add label
          </Button>
        )}
      </div>

      <Dialog
        open={labelDialogOpen}
        onOpenChange={(open) => {
          setLabelDialogOpen(open);
          if (!open) setEditingId(null);
        }}
      >
        <DialogContent className="sm:max-w-[480px]">
          <DialogHeader>
            <DialogTitle>{editingEntry ? 'Edit label' : 'Create label'}</DialogTitle>
          </DialogHeader>
          <LabelForm
            initial={dialogInitialForm}
            teams={teams}
            onSave={(form) => {
              if (editingEntry) {
                return handleUpdate(editingEntry.label.id, form);
              }
              return handleCreate(form);
            }}
            saving={saving}
            className="space-y-3"
          />
        </DialogContent>
      </Dialog>

      <div>
        {labels.length === 0 && !labelDialogOpen && (
          <div className="flex flex-col items-center gap-3 py-12 text-center">
            <div className="flex h-12 w-12 items-center justify-center rounded-full bg-muted">
              <Tag className="h-6 w-6 text-muted-foreground/60" />
            </div>
            <div>
              <p className="text-sm font-medium text-foreground">No labels yet</p>
              <p className="mt-1 text-xs text-muted-foreground">
                Labels help you categorize and filter stories across your workspace.
              </p>
            </div>
            {editable && (
              <Button
                variant="outline"
                size="sm"
                onClick={() => {
                  setLabelDialogOpen(true);
                  setEditingId(null);
                }}
              >
                <Plus className="h-3.5 w-3.5" />
                Create your first label
              </Button>
            )}
          </div>
        )}
        {labels.map((entry) =>
          deleteConfirmId === entry.label.id ? (
            <div
              key={entry.label.id}
              className="flex items-center gap-3 rounded-md border border-destructive/30 bg-destructive/5 px-3 py-2.5"
            >
              <span
                className="h-3 w-3 rounded-full shrink-0"
                style={{ backgroundColor: entry.label.color || '#64748b' }}
              />
              <span className="flex-1 text-sm text-foreground">
                Delete <span className="font-medium">{entry.label.name}</span>?
              </span>
              <div className="flex items-center gap-1">
                <Button
                  variant="destructive"
                  size="xs"
                  onClick={() => handleDelete(entry.label.id)}
                >
                  Delete
                </Button>
                <Button
                  variant="ghost"
                  size="xs"
                  onClick={() => setDeleteConfirmId(null)}
                >
                  Cancel
                </Button>
              </div>
            </div>
          ) : (
            <LabelRow
              key={entry.label.id}
              entry={entry}
              editable={editable}
              onEdit={() => {
                setEditingId(entry.label.id);
                setLabelDialogOpen(true);
              }}
              onDelete={() => setDeleteConfirmId(entry.label.id)}
            />
          ),
        )}
      </div>
    </div>
  );
}
