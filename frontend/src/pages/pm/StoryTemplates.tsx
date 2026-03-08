import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { createColumnHelper } from '@tanstack/react-table';
import { FileText, MoreHorizontal, Pencil, Plus, Search, Trash2 } from 'lucide-react';
import { ConfirmDialog } from '@/components/pm/ConfirmDialog';
import { CreateStoryModal } from '@/components/pm/CreateStoryModal';
import { useTitle } from '@/hooks/useTitle';
import { Card, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Checkbox } from '@/components/ui/checkbox';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { PMDataTable } from '@/components/pm/PMDataTable';
import { pmStoryTemplateService } from '@/lib/services/pmStoryTemplateService';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import type { StoryTemplate } from '@/lib/pmTypes';
import { useWorkspaceTeams } from '@/hooks/useWorkspaceTeams';

const columnHelper = createColumnHelper<StoryTemplate>();

export function StoryTemplatesPage() {
  useTitle('Story Templates');
  const workspace = useWorkspaceStore((state) => state.currentWorkspace);
  const { teams } = useWorkspaceTeams(workspace?.id);

  const [templates, setTemplates] = useState<StoryTemplate[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [search, setSearch] = useState('');
  const [onlyArchived, setOnlyArchived] = useState(false);
  const [scopeFilter, setScopeFilter] = useState<string>('__all__');

  const [dialogOpen, setDialogOpen] = useState(false);
  const [editingTemplate, setEditingTemplate] = useState<StoryTemplate | null>(null);
  const [deleteConfirm, setDeleteConfirm] = useState<StoryTemplate | null>(null);

  const workspaceId = workspace?.id;

  const loadData = useCallback(async () => {
    if (!workspaceId) return;
    setLoading(true);
    setError(null);
    const teamId = scopeFilter === '__all__' || scopeFilter === '__shared__' ? undefined : scopeFilter;
    const includeShared = scopeFilter !== '__shared__';
    const res = await pmStoryTemplateService.list(workspaceId, {
      teamId,
      includeShared,
      archived: onlyArchived,
    });
    if (res.error || !res.data) {
      setError(res.error ?? 'Failed to load templates');
      setLoading(false);
      return;
    }
    setTemplates(res.data);
    setLoading(false);
  }, [workspaceId, scopeFilter, onlyArchived]);

  useEffect(() => {
    loadData();
  }, [loadData]);

  const filteredTemplates = useMemo(() => {
    let result = templates;
    if (search.trim()) {
      const q = search.trim().toLowerCase();
      result = result.filter((t) => t.name.toLowerCase().includes(q));
    }
    return result;
  }, [templates, search]);

  const handleCreate = () => {
    setEditingTemplate(null);
    setDialogOpen(true);
  };

  const handleEdit = useCallback((tmpl: StoryTemplate) => {
    setEditingTemplate(tmpl);
    setDialogOpen(true);
  }, []);

  const handleDelete = useCallback(async (tmpl: StoryTemplate) => {
    if (!workspaceId) return;
    setTemplates((prev) => prev.filter((t) => t.id !== tmpl.id));
    const { error } = await pmStoryTemplateService.remove(workspaceId, tmpl.id);
    if (error) loadData();
  }, [workspaceId, loadData]);

  const handleDeleteRef = useRef(handleDelete);
  handleDeleteRef.current = handleDelete;

  const handleEditRef = useRef(handleEdit);
  handleEditRef.current = handleEdit;

  const columns = useMemo(
    () => [
      columnHelper.display({
        id: 'name',
        header: 'Name',
        size: 999,
        cell: (info) => {
          const tmpl = info.row.original;
          return (
            <div className="min-w-0">
              <div className="flex items-center gap-2">
                <span className="truncate font-medium text-sm">{tmpl.name}</span>
                <span className="text-[10px] uppercase tracking-wide text-muted-foreground">
                  {tmpl.team_id ? 'Team' : 'Shared'}
                </span>
              </div>
            </div>
          );
        },
      }),
      columnHelper.display({
        id: 'description',
        header: 'Description',
        size: 300,
        cell: (info) => {
          const desc = info.row.original.description;
          if (!desc) return <span className="text-muted-foreground text-xs">-</span>;
          return <span className="text-xs text-muted-foreground truncate block max-w-[280px]">{desc.replace(/<[^>]*>/g, '')}</span>;
        },
      }),
      columnHelper.display({
        id: 'actions',
        header: '',
        size: 50,
        cell: (info) => {
          const tmpl = info.row.original;
          return (
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button variant="ghost" size="icon-xs" className="text-muted-foreground hover:text-foreground" onClick={(e) => e.stopPropagation()}>
                  <MoreHorizontal className="h-4 w-4" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" onClick={(e) => e.stopPropagation()}>
                <DropdownMenuItem onClick={() => handleEditRef.current(tmpl)}>
                  <Pencil className="h-4 w-4" />
                  Edit
                </DropdownMenuItem>
                <DropdownMenuItem className="text-destructive focus:text-destructive" onClick={() => setDeleteConfirm(tmpl)}>
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
          <h2 className="text-xl font-semibold">Story Templates</h2>
          <p className="text-sm text-muted-foreground">
            Define reusable templates to quickly create stories with pre-filled fields.
          </p>
        </div>
        <Button size="sm" onClick={handleCreate}>
          <Plus className="mr-1.5 h-4 w-4" />
          Create Template
        </Button>
      </header>

      <div className="flex items-center gap-3">
        <div className="relative max-w-xs flex-1">
          <Search className="absolute left-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            placeholder="Filter templates by name"
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
              <SelectItem value="__all__">All templates</SelectItem>
              <SelectItem value="__shared__">For everyone</SelectItem>
              {teams.map((team) => (
                <SelectItem key={team.id} value={team.id}>
                  {team.name} templates
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Checkbox
            id="only-archived-templates"
            checked={onlyArchived}
            onCheckedChange={(checked) => setOnlyArchived(checked === true)}
          />
          <label htmlFor="only-archived-templates" className="text-sm text-muted-foreground cursor-pointer select-none">
            Only Archived
          </label>
        </div>
      </div>

      {error && (
        <div className="rounded-md border border-destructive/30 bg-destructive/5 px-3 py-2 text-sm text-destructive">
          {error}
        </div>
      )}

      {loading ? (
        <p className="text-sm text-muted-foreground">Loading templates...</p>
      ) : filteredTemplates.length === 0 ? (
        <Card>
          <CardContent className="flex flex-col items-center gap-3 py-12 text-center">
            <FileText className="h-10 w-10 text-muted-foreground/40" />
            <div>
              <p className="text-sm font-medium text-muted-foreground">No templates found</p>
              <p className="text-xs text-muted-foreground/70">
                {search || onlyArchived
                  ? 'Try adjusting your filters.'
                  : 'Create your first template to get started.'}
              </p>
            </div>
            {!search && !onlyArchived && (
              <Button size="sm" variant="outline" onClick={handleCreate}>
                <Plus className="mr-1.5 h-4 w-4" />
                Create Template
              </Button>
            )}
          </CardContent>
        </Card>
      ) : (
        <PMDataTable data={filteredTemplates} columns={columns} />
      )}

      {/* Reuse CreateStoryModal in template mode */}
      <CreateStoryModal
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        workspaceId={workspaceId!}
        mode="template"
        editingTemplate={editingTemplate}
        onSaveTemplate={() => loadData()}
      />

      <ConfirmDialog
        open={deleteConfirm !== null}
        onOpenChange={(open) => { if (!open) setDeleteConfirm(null); }}
        title="Delete template"
        description={`This will permanently delete "${deleteConfirm?.name ?? ''}". This action cannot be undone.`}
        confirmLabel="Delete"
        variant="destructive"
        onConfirm={() => { if (deleteConfirm) handleDeleteRef.current(deleteConfirm); setDeleteConfirm(null); }}
      />
    </div>
  );
}
