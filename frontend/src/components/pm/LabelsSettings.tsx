import { useCallback, useEffect, useRef, useState } from 'react';
import { Check, Loader2, Pencil, Plus, Tag, Trash2, X } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Progress } from '@/components/ui/progress';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { ColorPicker, PRESET_COLORS } from '@/components/pm/ColorPicker';
import { pmLabelService } from '@/lib/services/pmLabelService';
import type { LabelWithStats } from '@/lib/pmTypes';
import { useWorkspaceTeams } from '@/hooks/useWorkspaceTeams';

interface LabelsSettingsProps {
  workspaceId: string;
  initialTeamId?: string;
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
      <Progress value={p} className="h-1.5" />
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
}: {
  entry: LabelWithStats;
  onEdit: () => void;
  onDelete: () => void;
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
    </div>
  );
}

function LabelForm({
  initial,
  teams,
  onSave,
  onCancel,
  saving,
}: {
  initial: LabelFormState;
  teams: Array<{ id: string; name: string }>;
  onSave: (form: LabelFormState) => void;
  onCancel: () => void;
  saving: boolean;
}) {
  const [form, setForm] = useState<LabelFormState>(initial);
  const nameRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    nameRef.current?.focus();
  }, []);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!form.name.trim()) return;
    onSave({ ...form, name: form.name.trim(), description: form.description.trim() });
  };

  return (
    <form onSubmit={handleSubmit} className="rounded-lg border border-border/60 bg-muted/30 p-3.5 space-y-3">
      <div className="flex items-center gap-2.5">
        <span
          className="h-3.5 w-3.5 rounded-full shrink-0 ring-2 ring-background"
          style={{ backgroundColor: form.color || '#64748b' }}
        />
        <Input
          ref={nameRef}
          placeholder="Label name"
          value={form.name}
          onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
          className="h-8 text-sm"
        />
      </div>
      <Input
        placeholder="Description (optional)"
        value={form.description}
        onChange={(e) => setForm((f) => ({ ...f, description: e.target.value }))}
        className="h-8 text-sm"
      />
      <div className="flex items-center gap-2">
        <Select value={form.team_id || '__shared__'} onValueChange={(value) => setForm((f) => ({ ...f, team_id: value === '__shared__' ? '' : value }))}>
          <SelectTrigger className="h-8 w-[180px] text-sm">
            <SelectValue placeholder="For everyone" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="__shared__">For everyone</SelectItem>
            {teams.map((team) => (
              <SelectItem key={team.id} value={team.id}>
                {team.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <ColorPicker value={form.color} onChange={(c) => setForm((f) => ({ ...f, color: c }))} />
      </div>
      <div className="flex items-center gap-2 pt-1">
        <Button type="submit" size="sm" disabled={saving || !form.name.trim()}>
          {saving ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Check className="h-3.5 w-3.5" />}
          Save
        </Button>
        <Button type="button" variant="ghost" size="sm" onClick={onCancel} disabled={saving}>
          <X className="h-3.5 w-3.5" />
          Cancel
        </Button>
      </div>
    </form>
  );
}

export function LabelsSettings({ workspaceId, initialTeamId }: LabelsSettingsProps) {
  const { teams } = useWorkspaceTeams(workspaceId);
  const [labels, setLabels] = useState<LabelWithStats[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [showCreate, setShowCreate] = useState(false);
  const [deleteConfirmId, setDeleteConfirmId] = useState<string | null>(null);
  const [scopeFilter, setScopeFilter] = useState<string>(initialTeamId || '__all__');

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
      setShowCreate(false);
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
            <SelectItem value="__shared__">For everyone</SelectItem>
            {teams.map((team) => (
              <SelectItem key={team.id} value={team.id}>
                {team.name} labels
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        {!showCreate && (
          <Button
            variant="outline"
            size="sm"
            className="ml-auto"
            onClick={() => {
              setShowCreate(true);
              setEditingId(null);
            }}
          >
            <Plus className="h-3.5 w-3.5" />
            Add label
          </Button>
        )}
      </div>

      {showCreate && (
        <LabelForm
          initial={emptyForm}
          teams={teams}
          onSave={handleCreate}
          onCancel={() => setShowCreate(false)}
          saving={saving}
        />
      )}

      <div>
        {labels.length === 0 && !showCreate && (
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
            <Button
              variant="outline"
              size="sm"
              onClick={() => {
                setShowCreate(true);
                setEditingId(null);
              }}
            >
              <Plus className="h-3.5 w-3.5" />
              Create your first label
            </Button>
          </div>
        )}
        {labels.map((entry) =>
          editingId === entry.label.id ? (
            <LabelForm
              key={entry.label.id}
              initial={{
                name: entry.label.name,
                description: entry.label.description || '',
                color: entry.label.color || PRESET_COLORS[0],
                team_id: entry.label.team_id || '',
              }}
              teams={teams}
              onSave={(form) => handleUpdate(entry.label.id, form)}
              onCancel={() => setEditingId(null)}
              saving={saving}
            />
          ) : deleteConfirmId === entry.label.id ? (
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
              onEdit={() => {
                setEditingId(entry.label.id);
                setShowCreate(false);
              }}
              onDelete={() => setDeleteConfirmId(entry.label.id)}
            />
          ),
        )}
      </div>
    </div>
  );
}
