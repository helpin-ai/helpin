import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { createColumnHelper } from '@tanstack/react-table';
import { Loader2, MoreHorizontal, Pencil, Plus, Search, Tag, Trash2 } from 'lucide-react';
import { ConfirmDialog } from '@/components/pm/ConfirmDialog';
import { useTitle } from '@/hooks/useTitle';
import { Card, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Checkbox } from '@/components/ui/checkbox';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { PMDataTable } from '@/components/pm/PMDataTable';
import { ColorPicker, PRESET_COLORS } from '@/components/pm/ColorPicker';
import { StatCell, emptyForm, type LabelFormState } from '@/components/pm/LabelsSettings';
import { pmLabelService } from '@/lib/services/pmLabelService';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import type { LabelWithStats } from '@/lib/pmTypes';
import { useAccessibleTeams } from '@/hooks/useAccessibleTeams';

// ── Label dialog ────────────────────────────────────────────────────

function LabelDialog({
  open,
  onOpenChange,
  initial,
  teams,
  title,
  onSave,
  saving,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  initial: LabelFormState;
  teams: Array<{ id: string; name: string }>;
  title: string;
  onSave: (form: LabelFormState) => void;
  saving: boolean;
}) {
  const [form, setForm] = useState<LabelFormState>(initial);
  const nameRef = useRef<HTMLInputElement>(null);

  // Only reset form when dialog opens — not on every `initial` reference change
  useEffect(() => {
    if (open) {
      setForm(initial);
      setTimeout(() => nameRef.current?.focus(), 50);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open]);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!form.name.trim()) return;
    onSave({ ...form, name: form.name.trim(), description: form.description.trim() });
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>{title}</DialogTitle>
          </DialogHeader>
          <div className="space-y-4 py-4">
            <div className="space-y-2">
              <label className="text-sm font-medium">Name</label>
              <Input
                ref={nameRef}
                placeholder="Label name"
                value={form.name}
                onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
              />
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">Description</label>
              <Input
                placeholder="Description (optional)"
                value={form.description}
                onChange={(e) => setForm((f) => ({ ...f, description: e.target.value }))}
              />
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">Color</label>
              <ColorPicker value={form.color} onChange={(c) => setForm((f) => ({ ...f, color: c }))} />
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">Scope</label>
              <Select value={form.team_id || '__shared__'} onValueChange={(value) => setForm((f) => ({ ...f, team_id: value === '__shared__' ? '' : value }))}>
                <SelectTrigger className="h-9">
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
            </div>
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)} disabled={saving}>
              Cancel
            </Button>
            <Button type="submit" disabled={saving || !form.name.trim()}>
              {saving && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
              Save
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

// ── Column helper ───────────────────────────────────────────────────

const columnHelper = createColumnHelper<LabelWithStats>();

// ── Main page ───────────────────────────────────────────────────────

export function LabelsPage() {
  useTitle('Labels');
  const workspace = useWorkspaceStore((state) => state.currentWorkspace);
  const { teams } = useAccessibleTeams(workspace?.id ?? '');

  const [labels, setLabels] = useState<LabelWithStats[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Filter state
  const [search, setSearch] = useState('');
  const [onlyArchived, setOnlyArchived] = useState(false);
  const [scopeFilter, setScopeFilter] = useState<string>('__all__');

  // Dialog state
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editingLabel, setEditingLabel] = useState<LabelWithStats | null>(null);
  const [saving, setSaving] = useState(false);
  const [deleteLabelConfirm, setDeleteLabelConfirm] = useState<LabelWithStats | null>(null);

  const workspaceId = workspace?.id;

  const loadData = useCallback(async () => {
    if (!workspaceId) return;
    setLoading(true);
    setError(null);
    const teamId = scopeFilter === '__all__' || scopeFilter === '__shared__' ? undefined : scopeFilter;
    const includeShared = scopeFilter !== '__shared__';
    const res = await pmLabelService.listWithStats(workspaceId, {
      teamId,
      includeShared,
      archived: onlyArchived,
    });
    if (res.error || !res.data) {
      setError(res.error ?? 'Failed to load labels');
      setLoading(false);
      return;
    }
    setLabels(res.data);
    setLoading(false);
  }, [workspaceId, scopeFilter, onlyArchived]);

  useEffect(() => {
    loadData();
  }, [loadData]);

  // Filtered data
  const filteredLabels = useMemo(() => {
    let result = labels;

    if (search.trim()) {
      const q = search.trim().toLowerCase();
      result = result.filter((entry) => entry.label.name.toLowerCase().includes(q));
    }

    return result;
  }, [labels, search, onlyArchived]);

  // Create / edit handlers
  const handleCreate = () => {
    setEditingLabel(null);
    setDialogOpen(true);
  };

  const handleEdit = useCallback((entry: LabelWithStats) => {
    setEditingLabel(entry);
    setDialogOpen(true);
  }, []);

  const handleSave = async (form: LabelFormState) => {
    if (!workspaceId) return;
    setSaving(true);
    if (editingLabel) {
      const { error: err } = await pmLabelService.update(workspaceId, editingLabel.label.id, {
        team_id: form.team_id || undefined,
        name: form.name,
        description: form.description || undefined,
        color: form.color,
      });
      if (err) {
        setError(err);
      }
    } else {
      const { error: err } = await pmLabelService.create({
        workspace_id: workspaceId,
        team_id: form.team_id || undefined,
        name: form.name,
        description: form.description || undefined,
        color: form.color,
      });
      if (err) {
        setError(err);
      }
    }
    setSaving(false);
    setDialogOpen(false);
    loadData();
  };

  const handleDelete = useCallback(async (entry: LabelWithStats) => {
    if (!workspaceId) return;
    setLabels((prev) => prev.filter((e) => e.label.id !== entry.label.id));
    const { error } = await pmLabelService.remove(workspaceId, entry.label.id);
    if (error) loadData();
  }, [workspaceId, loadData]);

  const handleDeleteRef = useRef(handleDelete);
  handleDeleteRef.current = handleDelete;

  // ── Columns — use refs for handlers to avoid stale closures ─────

  const handleEditRef = useRef(handleEdit);
  handleEditRef.current = handleEdit;

  const columns = useMemo(
    () => [
      columnHelper.display({
        id: 'name',
        header: 'Name',
        size: 999,
        cell: (info) => {
          const entry = info.row.original;
          return (
            <div className="flex items-center gap-2.5 min-w-0">
              <span
                className="h-3 w-3 rounded-full shrink-0"
                style={{ backgroundColor: entry.label.color || '#64748b' }}
              />
              <div className="min-w-0">
                <div className="truncate font-medium text-sm">{entry.label.name}</div>
                <div className="text-[10px] uppercase tracking-wide text-muted-foreground">
                  {entry.label.team_id ? 'Team' : 'Shared'}
                </div>
              </div>
            </div>
          );
        },
      }),
      columnHelper.display({
        id: 'tasks',
        header: 'Tasks',
        size: 200,
        cell: (info) => {
          const { stats } = info.row.original;
          return <StatCell done={stats.done_task_count} total={stats.task_count} entity="Tasks" />;
        },
      }),
      columnHelper.display({
        id: 'epics',
        header: 'Epics',
        size: 200,
        cell: (info) => {
          const { stats } = info.row.original;
          return <StatCell done={stats.done_epic_count} total={stats.epic_count} entity="Epics" />;
        },
      }),
      columnHelper.display({
        id: 'actions',
        header: '',
        size: 50,
        cell: (info) => {
          const entry = info.row.original;
          return (
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button
                  variant="ghost"
                  size="icon-xs"
                  className="text-muted-foreground hover:text-foreground"
                  onClick={(e) => e.stopPropagation()}
                >
                  <MoreHorizontal className="h-4 w-4" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" onClick={(e) => e.stopPropagation()}>
                <DropdownMenuItem onClick={() => handleEditRef.current(entry)}>
                  <Pencil className="h-4 w-4" />
                  Edit
                </DropdownMenuItem>
                <DropdownMenuItem
                  className="text-destructive focus:text-destructive"
                  onClick={() => setDeleteLabelConfirm(entry)}
                >
                  <Trash2 className="h-4 w-4" />
                  Delete
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          );
        },
      }),
    ],
    [],
  );

  if (!workspace) {
    return <p className="text-sm text-muted-foreground">Workspace not found.</p>;
  }

  return (
    <div className="space-y-4">
      <header className="flex items-center justify-between">
        <div>
          <h2 className="text-xl font-semibold">Labels</h2>
          <p className="text-sm text-muted-foreground">
            Organize and track work across tasks and epics with labels.
          </p>
        </div>
        <Button size="sm" onClick={handleCreate}>
          <Plus className="mr-1.5 h-4 w-4" />
          Create Label
        </Button>
      </header>

      {/* Filter bar */}
      <div className="flex items-center gap-3">
        <div className="relative max-w-xs flex-1">
          <Search className="absolute left-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            placeholder="Filter Labels by name"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="h-8 pl-8 text-sm"
          />
        </div>
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
          <Checkbox
            id="only-archived"
            checked={onlyArchived}
            onCheckedChange={(checked) => setOnlyArchived(checked === true)}
          />
          <label htmlFor="only-archived" className="text-sm text-muted-foreground cursor-pointer select-none">
            Only Archived
          </label>
        </div>
      </div>

      {error ? (
        <div className="rounded-md border border-destructive/30 bg-destructive/5 px-3 py-2 text-sm text-destructive">
          {error}
        </div>
      ) : null}

      {loading ? (
        <p className="text-sm text-muted-foreground">Loading labels...</p>
      ) : filteredLabels.length === 0 ? (
        <Card>
          <CardContent className="flex flex-col items-center gap-3 py-12 text-center">
            <Tag className="h-10 w-10 text-muted-foreground/40" />
            <div>
              <p className="text-sm font-medium text-muted-foreground">No labels found</p>
              <p className="text-xs text-muted-foreground/70">
                {search || onlyArchived
                  ? 'Try adjusting your filters.'
                  : 'Create your first label to get started.'}
              </p>
            </div>
            {!search && !onlyArchived && (
              <Button size="sm" variant="outline" onClick={handleCreate}>
                <Plus className="mr-1.5 h-4 w-4" />
                Create Label
              </Button>
            )}
          </CardContent>
        </Card>
      ) : (
        <PMDataTable
          data={filteredLabels}
          columns={columns}
        />
      )}

      {/* Create / Edit dialog */}
      <LabelDialog
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        initial={
          editingLabel
            ? {
                name: editingLabel.label.name,
                description: editingLabel.label.description || '',
                color: editingLabel.label.color || PRESET_COLORS[0],
                team_id: editingLabel.label.team_id || '',
              }
            : emptyForm
        }
        teams={teams}
        title={editingLabel ? 'Edit Label' : 'Create Label'}
        onSave={handleSave}
        saving={saving}
      />

      <ConfirmDialog
        open={deleteLabelConfirm !== null}
        onOpenChange={(open) => { if (!open) setDeleteLabelConfirm(null); }}
        title="Delete label"
        description={`This will permanently delete "${deleteLabelConfirm?.label.name ?? ''}". It will be removed from all tasks. This action cannot be undone.`}
        confirmLabel="Delete"
        variant="destructive"
        onConfirm={() => { if (deleteLabelConfirm) handleDeleteRef.current(deleteLabelConfirm); setDeleteLabelConfirm(null); }}
      />
    </div>
  );
}
